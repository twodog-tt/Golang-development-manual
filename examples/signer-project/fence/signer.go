package fence

import (
	"context"
	"errors"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

// Signer coordinates the durable fence store and a cryptographic Backend.
// bbolt's exclusive file lock and these process-local per-key locks intentionally
// implement a single-active-instance model, not a replicated HA store.
type Signer struct {
	db      *bolt.DB
	backend Backend
	locks   *keyedLocks
}

// Open opens or creates a durable signer database. NoSync is never enabled:
// successful bbolt commits are the release boundary for reservations/receipts.
func Open(path string, backend Backend) (*Signer, error) {
	if backend == nil {
		return nil, errors.New("fence: backend is required")
	}
	db, err := openBolt(path)
	if err != nil {
		return nil, err
	}
	return &Signer{db: db, backend: backend, locks: newKeyedLocks()}, nil
}

// Close closes the bbolt database.
func (s *Signer) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Sign executes four ordered stages while holding a process-local lock for the
// key: commit fence+PENDING, invoke the backend, commit receipt+COMPLETED, then
// release the receipt. A backend or completion failure leaves the first commit
// intact so a higher epoch cannot roll back with the cryptographic operation.
func (s *Signer) Sign(ctx context.Context, request Request) (Receipt, error) {
	if ctx == nil {
		return Receipt{}, fmt.Errorf("%w: context is nil", ErrInvalidRequest)
	}
	if err := validateRequest(request); err != nil {
		return Receipt{}, err
	}
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}

	unlock := s.locks.lock(request.KeyID)
	defer unlock()

	prepared, err := s.prepare(request)
	if err != nil {
		return Receipt{}, err
	}
	if prepared.completed {
		return cloneReceipt(prepared.receipt), nil
	}
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}

	result, err := s.backend.Sign(ctx, request.KeyID, prepared.digest)
	if err != nil {
		return Receipt{}, fmt.Errorf("fence: backend sign: %w", err)
	}
	if err := validateBackendResult(result); err != nil {
		return Receipt{}, fmt.Errorf("%w: %v", ErrBackendResult, err)
	}

	receipt, err := s.complete(request, prepared.digest, result)
	if err != nil {
		return Receipt{}, err
	}
	return receipt, nil
}

// State returns the highest durable epoch and owner for keyID.
func (s *Signer) State(keyID string) (FenceState, bool, error) {
	if err := validateName("key ID", keyID, 512); err != nil {
		return FenceState{}, false, err
	}
	var result FenceState
	var found bool
	err := s.db.View(func(tx *bolt.Tx) error {
		keys, err := writableKeysBucket(tx)
		if err != nil {
			return err
		}
		key := keys.Bucket([]byte(keyID))
		if key == nil {
			return nil
		}
		state, exists, err := readState(key)
		if err != nil || !exists {
			return err
		}
		result = FenceState{KeyID: keyID, Owner: state.Owner, Epoch: state.Epoch}
		found = true
		return nil
	})
	if err != nil {
		return FenceState{}, false, fmt.Errorf("fence: read state: %w", err)
	}
	return result, found, nil
}

// LookupRequest returns a copy of the durable request record.
func (s *Signer) LookupRequest(keyID, requestID string) (RequestRecord, bool, error) {
	if err := validateName("key ID", keyID, 512); err != nil {
		return RequestRecord{}, false, err
	}
	if err := validateName("request ID", requestID, 512); err != nil {
		return RequestRecord{}, false, err
	}
	var result RequestRecord
	var found bool
	err := s.db.View(func(tx *bolt.Tx) error {
		keys, err := writableKeysBucket(tx)
		if err != nil {
			return err
		}
		key := keys.Bucket([]byte(keyID))
		if key == nil {
			return nil
		}
		state, exists, err := readState(key)
		if err != nil {
			return err
		}
		requests := key.Bucket(requestsBucket)
		if requests == nil {
			return nil
		}
		raw := requests.Get([]byte(requestID))
		if raw == nil {
			return nil
		}
		if !exists {
			return fmt.Errorf("%w: request exists without fence state", ErrCorruptStore)
		}
		record, err := decodeRequestRecord(raw, keyID, requestID)
		if err != nil {
			return err
		}
		if err := validateRecordBackendIdentity(record, state); err != nil {
			return err
		}
		result = RequestRecord{
			Status:        record.Status,
			Owner:         record.Owner,
			Epoch:         record.Epoch,
			PayloadDigest: record.PayloadDigest,
			Receipt:       record.Receipt,
		}
		found = true
		return nil
	})
	if err != nil {
		return RequestRecord{}, false, fmt.Errorf("fence: read request: %w", err)
	}
	return cloneRequestRecord(result), found, nil
}

// IsDatabaseClosed reports whether an error chain contains bbolt's closed-DB
// sentinel. It is useful for failure-injection tests and operational logging.
func IsDatabaseClosed(err error) bool {
	return isBoltClosed(err)
}
