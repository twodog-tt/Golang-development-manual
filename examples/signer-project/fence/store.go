package fence

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	rootBucket     = []byte("signer-fence-v1")
	keysBucket     = []byte("keys")
	stateKey       = []byte("state")
	requestsBucket = []byte("requests")
)

type persistedState struct {
	Version          uint16 `json:"version"`
	Owner            string `json:"owner"`
	Epoch            uint64 `json:"epoch"`
	BackendAlgorithm string `json:"backend_algorithm,omitempty"`
	BackendPublicKey []byte `json:"backend_public_key,omitempty"`
}

type persistedRequest struct {
	Version       uint16        `json:"version"`
	Status        RequestStatus `json:"status"`
	Owner         string        `json:"owner"`
	Epoch         uint64        `json:"epoch"`
	PayloadDigest Digest        `json:"payload_digest"`
	Receipt       *Receipt      `json:"receipt,omitempty"`
}

type prepareResult struct {
	digest    Digest
	completed bool
	receipt   Receipt
}

func openBolt(path string) (*bolt.DB, error) {
	db, err := bolt.Open(path, 0o600, &bolt.Options{
		Timeout: time.Second,
		NoSync:  false,
	})
	if err != nil {
		return nil, fmt.Errorf("fence: open bbolt: %w", err)
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		root, err := tx.CreateBucketIfNotExists(rootBucket)
		if err != nil {
			return err
		}
		_, err = root.CreateBucketIfNotExists(keysBucket)
		return err
	}); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("fence: initialize bbolt: %w", err)
	}
	return db, nil
}

func (s *Signer) prepare(request Request) (prepareResult, error) {
	result := prepareResult{digest: DigestPayload(request.Payload)}
	var logicalErr error

	err := s.db.Update(func(tx *bolt.Tx) error {
		keys, err := writableKeysBucket(tx)
		if err != nil {
			return err
		}
		key, err := keys.CreateBucketIfNotExists([]byte(request.KeyID))
		if err != nil {
			return err
		}
		requests, err := key.CreateBucketIfNotExists(requestsBucket)
		if err != nil {
			return err
		}

		state, exists, err := readState(key)
		if err != nil {
			logicalErr = err
			return nil
		}
		switch {
		case exists && request.Epoch < state.Epoch:
			logicalErr = fmt.Errorf("%w: got=%d highest=%d", ErrStaleEpoch, request.Epoch, state.Epoch)
			return nil
		case exists && request.Epoch == state.Epoch && request.Owner != state.Owner:
			logicalErr = fmt.Errorf("%w: epoch=%d current=%q requested=%q", ErrOwnerConflict, request.Epoch, state.Owner, request.Owner)
			return nil
		case !exists:
			state = persistedState{Version: storeVersion, Owner: request.Owner, Epoch: request.Epoch}
			if err := putJSON(key, stateKey, state); err != nil {
				return err
			}
		case request.Epoch > state.Epoch:
			state.Owner = request.Owner
			state.Epoch = request.Epoch
			if err := putJSON(key, stateKey, state); err != nil {
				return err
			}
		}

		raw := requests.Get([]byte(request.RequestID))
		if raw != nil {
			record, err := decodeRequestRecord(raw, request.KeyID, request.RequestID)
			if err != nil {
				// The higher epoch, if any, has already been written. Commit that
				// fence before surfacing a record-level corruption error.
				logicalErr = err
				return nil
			}
			if record.Owner != request.Owner || record.Epoch != request.Epoch || record.PayloadDigest != result.digest {
				logicalErr = ErrRequestConflict
				return nil
			}
			if record.Status == StatusCompleted {
				if err := validateRecordBackendIdentity(record, state); err != nil {
					logicalErr = err
					return nil
				}
				result.completed = true
				result.receipt = cloneReceipt(*record.Receipt)
			}
			return nil
		}

		record := persistedRequest{
			Version:       storeVersion,
			Status:        StatusPending,
			Owner:         request.Owner,
			Epoch:         request.Epoch,
			PayloadDigest: result.digest,
		}
		return putJSON(requests, []byte(request.RequestID), record)
	})
	if err != nil {
		return prepareResult{}, fmt.Errorf("fence: commit reservation: %w", err)
	}
	if logicalErr != nil {
		return prepareResult{}, logicalErr
	}
	return result, nil
}

func (s *Signer) complete(request Request, digest Digest, backend BackendResult) (Receipt, error) {
	receipt := Receipt{
		Version:       storeVersion,
		KeyID:         request.KeyID,
		Owner:         request.Owner,
		Epoch:         request.Epoch,
		RequestID:     request.RequestID,
		PayloadDigest: digest,
		Algorithm:     backend.Algorithm,
		PublicKey:     append([]byte(nil), backend.PublicKey...),
		Signature:     append([]byte(nil), backend.Signature...),
	}

	err := s.db.Update(func(tx *bolt.Tx) error {
		key, err := readKeyBucket(tx, request.KeyID)
		if err != nil {
			return err
		}
		state, exists, err := readState(key)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("%w: missing fence state", ErrCorruptStore)
		}
		if request.Epoch < state.Epoch {
			return fmt.Errorf("%w: got=%d highest=%d", ErrStaleEpoch, request.Epoch, state.Epoch)
		}
		if request.Epoch != state.Epoch || request.Owner != state.Owner {
			return fmt.Errorf("%w: completion authority changed", ErrOwnerConflict)
		}
		requests := key.Bucket(requestsBucket)
		if requests == nil {
			return fmt.Errorf("%w: missing request bucket", ErrCorruptStore)
		}
		raw := requests.Get([]byte(request.RequestID))
		if raw == nil {
			return fmt.Errorf("%w: missing request reservation", ErrCorruptStore)
		}
		record, err := decodeRequestRecord(raw, request.KeyID, request.RequestID)
		if err != nil {
			return err
		}
		if record.Owner != request.Owner || record.Epoch != request.Epoch || record.PayloadDigest != digest {
			return ErrRequestConflict
		}
		if record.Status == StatusCompleted {
			if err := validateRecordBackendIdentity(record, state); err != nil {
				return err
			}
			receipt = cloneReceipt(*record.Receipt)
			return nil
		}

		switch {
		case state.BackendAlgorithm == "" && len(state.BackendPublicKey) == 0:
			state.BackendAlgorithm = backend.Algorithm
			state.BackendPublicKey = append([]byte(nil), backend.PublicKey...)
			if err := putJSON(key, stateKey, state); err != nil {
				return err
			}
		case state.BackendAlgorithm != backend.Algorithm || !bytes.Equal(state.BackendPublicKey, backend.PublicKey):
			return fmt.Errorf(
				"%w: key=%q persisted_algorithm=%q returned_algorithm=%q",
				ErrBackendIdentity,
				request.KeyID,
				state.BackendAlgorithm,
				backend.Algorithm,
			)
		}

		record.Status = StatusCompleted
		record.Receipt = &receipt
		return putJSON(requests, []byte(request.RequestID), record)
	})
	if err != nil {
		return Receipt{}, fmt.Errorf("fence: commit receipt: %w", err)
	}
	return cloneReceipt(receipt), nil
}

func writableKeysBucket(tx *bolt.Tx) (*bolt.Bucket, error) {
	root := tx.Bucket(rootBucket)
	if root == nil {
		return nil, fmt.Errorf("%w: missing root bucket", ErrCorruptStore)
	}
	keys := root.Bucket(keysBucket)
	if keys == nil {
		return nil, fmt.Errorf("%w: missing keys bucket", ErrCorruptStore)
	}
	return keys, nil
}

func readKeyBucket(tx *bolt.Tx, keyID string) (*bolt.Bucket, error) {
	keys, err := writableKeysBucket(tx)
	if err != nil {
		return nil, err
	}
	key := keys.Bucket([]byte(keyID))
	if key == nil {
		return nil, fmt.Errorf("%w: missing key bucket", ErrCorruptStore)
	}
	return key, nil
}

func readState(key *bolt.Bucket) (persistedState, bool, error) {
	raw := key.Get(stateKey)
	if raw == nil {
		return persistedState{}, false, nil
	}
	var state persistedState
	if err := json.Unmarshal(raw, &state); err != nil {
		return persistedState{}, false, fmt.Errorf("%w: decode state: %v", ErrCorruptStore, err)
	}
	if state.Version != storeVersion || state.Owner == "" || state.Epoch == 0 ||
		(state.BackendAlgorithm == "") != (len(state.BackendPublicKey) == 0) {
		return persistedState{}, false, fmt.Errorf("%w: invalid state", ErrCorruptStore)
	}
	return state, true, nil
}

func decodeRequestRecord(raw []byte, expectedKeyID, expectedRequestID string) (persistedRequest, error) {
	var record persistedRequest
	if err := json.Unmarshal(raw, &record); err != nil {
		return persistedRequest{}, fmt.Errorf("%w: decode request: %v", ErrCorruptStore, err)
	}
	if record.Version != storeVersion || record.Owner == "" || record.Epoch == 0 {
		return persistedRequest{}, fmt.Errorf("%w: invalid request binding", ErrCorruptStore)
	}
	switch record.Status {
	case StatusPending:
		if record.Receipt != nil {
			return persistedRequest{}, fmt.Errorf("%w: pending request has receipt", ErrCorruptStore)
		}
	case StatusCompleted:
		if record.Receipt == nil {
			return persistedRequest{}, fmt.Errorf("%w: completed request has no receipt", ErrCorruptStore)
		}
		if err := validateStoredReceipt(*record.Receipt, record, expectedKeyID, expectedRequestID); err != nil {
			return persistedRequest{}, err
		}
	default:
		return persistedRequest{}, fmt.Errorf("%w: unknown request status %q", ErrCorruptStore, record.Status)
	}
	return record, nil
}

func validateStoredReceipt(receipt Receipt, record persistedRequest, expectedKeyID, expectedRequestID string) error {
	if receipt.Version != storeVersion || receipt.KeyID != expectedKeyID || receipt.RequestID != expectedRequestID ||
		receipt.Owner != record.Owner || receipt.Epoch != record.Epoch ||
		receipt.PayloadDigest != record.PayloadDigest {
		return fmt.Errorf("%w: receipt does not match request binding", ErrCorruptStore)
	}
	if err := validateBackendResult(BackendResult{
		Algorithm: receipt.Algorithm,
		PublicKey: receipt.PublicKey,
		Signature: receipt.Signature,
	}); err != nil {
		return fmt.Errorf("%w: invalid persisted receipt: %v", ErrCorruptStore, err)
	}
	return nil
}

func validateRecordBackendIdentity(record persistedRequest, state persistedState) error {
	if record.Status != StatusCompleted || record.Receipt == nil {
		return nil
	}
	if state.BackendAlgorithm == "" || len(state.BackendPublicKey) == 0 {
		return fmt.Errorf("%w: completed receipt has no bound backend identity", ErrCorruptStore)
	}
	if record.Receipt.Algorithm != state.BackendAlgorithm ||
		!bytes.Equal(record.Receipt.PublicKey, state.BackendPublicKey) {
		return fmt.Errorf("%w: receipt backend identity does not match key state", ErrCorruptStore)
	}
	return nil
}

func putJSON(bucket *bolt.Bucket, key []byte, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return bucket.Put(key, raw)
}

func isBoltClosed(err error) bool {
	return errors.Is(err, bolt.ErrDatabaseNotOpen)
}
