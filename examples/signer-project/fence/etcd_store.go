package fence

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	mvccpb "go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

type etcdPersistedState struct {
	Version          uint16 `json:"version"`
	KeyID            string `json:"key_id"`
	Owner            string `json:"owner"`
	Epoch            uint64 `json:"epoch"`
	BackendAlgorithm string `json:"backend_algorithm,omitempty"`
	BackendPublicKey []byte `json:"backend_public_key,omitempty"`
}

type etcdPersistedRequest struct {
	Version       uint16        `json:"version"`
	KeyID         string        `json:"key_id"`
	RequestID     string        `json:"request_id"`
	Status        RequestStatus `json:"status"`
	Owner         string        `json:"owner"`
	Epoch         uint64        `json:"epoch"`
	PayloadDigest Digest        `json:"payload_digest"`
	Receipt       *Receipt      `json:"receipt,omitempty"`
}

type etcdKVSnapshot struct {
	raw         []byte
	modRevision int64
	exists      bool
}

type etcdOwnerToken struct {
	key            string
	createRevision int64
	lease          clientv3.LeaseID
}

type etcdPrepared struct {
	digest          Digest
	completed       bool
	receipt         Receipt
	stateKey        string
	requestKey      string
	state           etcdPersistedState
	record          etcdPersistedRequest
	stateSnapshot   etcdKVSnapshot
	requestSnapshot etcdKVSnapshot
	owner           etcdOwnerToken
}

func (s *EtcdSigner) prepareEtcd(
	ctx context.Context,
	mutex *concurrency.Mutex,
	request Request,
) (etcdPrepared, error) {
	prepared := etcdPrepared{
		digest:     DigestPayload(request.Payload),
		stateKey:   etcdStateKey(s.prefix, request.KeyID),
		requestKey: etcdRequestKey(s.prefix, request.KeyID, request.RequestID),
	}

	owner, stateSnapshot, requestSnapshot, err := s.readEtcdForOwner(
		ctx,
		mutex,
		prepared.stateKey,
		prepared.requestKey,
	)
	if err != nil {
		return etcdPrepared{}, err
	}
	prepared.owner = owner
	prepared.stateSnapshot = stateSnapshot
	prepared.requestSnapshot = requestSnapshot

	var state etcdPersistedState
	stateChanged := false
	if stateSnapshot.exists {
		state, err = decodeEtcdState(stateSnapshot.raw, request.KeyID)
		if err != nil {
			return etcdPrepared{}, err
		}
	} else {
		if requestSnapshot.exists {
			return etcdPrepared{}, fmt.Errorf(
				"%w: etcd request exists without fence state",
				ErrCorruptStore,
			)
		}
		state = etcdPersistedState{
			Version: storeVersion,
			KeyID:   request.KeyID,
			Owner:   request.Owner,
			Epoch:   request.Epoch,
		}
		stateChanged = true
	}
	persistedHighestEpoch := state.Epoch

	switch {
	case request.Epoch < state.Epoch:
		return etcdPrepared{}, fmt.Errorf(
			"%w: got=%d highest=%d",
			ErrStaleEpoch,
			request.Epoch,
			state.Epoch,
		)
	case request.Epoch == state.Epoch && request.Owner != state.Owner:
		return etcdPrepared{}, fmt.Errorf(
			"%w: epoch=%d current=%q requested=%q",
			ErrOwnerConflict,
			request.Epoch,
			state.Owner,
			request.Owner,
		)
	case request.Epoch > state.Epoch:
		state.Owner = request.Owner
		state.Epoch = request.Epoch
		stateChanged = true
	}
	prepared.state = state

	if requestSnapshot.exists {
		record, decodeErr := decodeEtcdRequest(
			requestSnapshot.raw,
			request.KeyID,
			request.RequestID,
		)
		if decodeErr != nil {
			if stateChanged {
				if err := s.commitEtcdReservation(
					ctx,
					mutex,
					&prepared,
					state,
					nil,
				); err != nil {
					return etcdPrepared{}, err
				}
			}
			return etcdPrepared{}, decodeErr
		}
		if record.Epoch > persistedHighestEpoch {
			return etcdPrepared{}, fmt.Errorf(
				"%w: request epoch exceeds highest fence",
				ErrCorruptStore,
			)
		}
		if record.Owner != request.Owner ||
			record.Epoch != request.Epoch ||
			record.PayloadDigest != prepared.digest {
			if stateChanged {
				if err := s.commitEtcdReservation(
					ctx,
					mutex,
					&prepared,
					state,
					nil,
				); err != nil {
					return etcdPrepared{}, err
				}
			}
			return etcdPrepared{}, ErrRequestConflict
		}
		if record.Status == StatusCompleted {
			if err := validateEtcdRecordBackendIdentity(record, state); err != nil {
				return etcdPrepared{}, err
			}
			prepared.completed = true
			prepared.receipt = cloneReceipt(*record.Receipt)
			return prepared, nil
		}
		prepared.record = record
		if stateChanged {
			if err := s.commitEtcdReservation(
				ctx,
				mutex,
				&prepared,
				state,
				nil,
			); err != nil {
				return etcdPrepared{}, err
			}
		}
		return prepared, nil
	}

	record := etcdPersistedRequest{
		Version:       storeVersion,
		KeyID:         request.KeyID,
		RequestID:     request.RequestID,
		Status:        StatusPending,
		Owner:         request.Owner,
		Epoch:         request.Epoch,
		PayloadDigest: prepared.digest,
	}
	if err := s.commitEtcdReservation(
		ctx,
		mutex,
		&prepared,
		state,
		&record,
	); err != nil {
		return etcdPrepared{}, err
	}
	prepared.record = record
	return prepared, nil
}

func (s *EtcdSigner) commitEtcdReservation(
	ctx context.Context,
	mutex *concurrency.Mutex,
	prepared *etcdPrepared,
	state etcdPersistedState,
	record *etcdPersistedRequest,
) error {
	stateRaw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("fence: encode etcd state: %w", err)
	}

	comparisons := s.etcdMutationComparisons(mutex, *prepared)
	operations := []clientv3.Op{
		clientv3.OpPut(prepared.stateKey, string(stateRaw)),
	}
	var requestRaw []byte
	if record != nil {
		requestRaw, err = json.Marshal(record)
		if err != nil {
			return fmt.Errorf("fence: encode etcd request: %w", err)
		}
		operations = append(
			operations,
			clientv3.OpPut(prepared.requestKey, string(requestRaw)),
		)
	}

	response, err := s.client.Txn(ctx).
		If(comparisons...).
		Then(operations...).
		Else(
			clientv3.OpGet(prepared.owner.key),
			clientv3.OpGet(prepared.stateKey),
			clientv3.OpGet(prepared.requestKey),
		).
		Commit()
	if err != nil {
		return s.operationError(ctx, "commit etcd reservation", err)
	}
	if !response.Succeeded {
		return classifyEtcdCompareFailure(response, prepared.owner)
	}
	prepared.state = state
	prepared.stateSnapshot = etcdKVSnapshot{
		raw:         append([]byte(nil), stateRaw...),
		modRevision: response.Header.Revision,
		exists:      true,
	}
	if record != nil {
		prepared.record = *record
		prepared.requestSnapshot = etcdKVSnapshot{
			raw:         append([]byte(nil), requestRaw...),
			modRevision: response.Header.Revision,
			exists:      true,
		}
	}
	return nil
}

func (s *EtcdSigner) completeEtcd(
	ctx context.Context,
	mutex *concurrency.Mutex,
	request Request,
	prepared etcdPrepared,
	backend BackendResult,
) (Receipt, error) {
	receipt := Receipt{
		Version:       storeVersion,
		KeyID:         request.KeyID,
		Owner:         request.Owner,
		Epoch:         request.Epoch,
		RequestID:     request.RequestID,
		PayloadDigest: prepared.digest,
		Algorithm:     backend.Algorithm,
		PublicKey:     append([]byte(nil), backend.PublicKey...),
		Signature:     append([]byte(nil), backend.Signature...),
	}

	state := prepared.state
	switch {
	case state.BackendAlgorithm == "" && len(state.BackendPublicKey) == 0:
		state.BackendAlgorithm = backend.Algorithm
		state.BackendPublicKey = append([]byte(nil), backend.PublicKey...)
	case state.BackendAlgorithm != backend.Algorithm ||
		!bytes.Equal(state.BackendPublicKey, backend.PublicKey):
		return Receipt{}, fmt.Errorf(
			"%w: key=%q persisted_algorithm=%q returned_algorithm=%q",
			ErrBackendIdentity,
			request.KeyID,
			state.BackendAlgorithm,
			backend.Algorithm,
		)
	}

	record := prepared.record
	if record.Status != StatusPending ||
		record.Owner != request.Owner ||
		record.Epoch != request.Epoch ||
		record.PayloadDigest != prepared.digest {
		return Receipt{}, fmt.Errorf(
			"%w: invalid prepared request binding",
			ErrCorruptStore,
		)
	}
	record.Status = StatusCompleted
	record.Receipt = &receipt

	stateRaw, err := json.Marshal(state)
	if err != nil {
		return Receipt{}, fmt.Errorf("fence: encode etcd state: %w", err)
	}
	requestRaw, err := json.Marshal(record)
	if err != nil {
		return Receipt{}, fmt.Errorf("fence: encode etcd request: %w", err)
	}

	// Re-read ownership in the transaction through Mutex.IsOwner rather than
	// trusting that the lease survived the potentially slow backend call.
	comparisons := []clientv3.Cmp{
		mutex.IsOwner(),
		clientv3.Compare(
			clientv3.LeaseValue(prepared.owner.key),
			"=",
			prepared.owner.lease,
		),
	}
	comparisons = append(
		comparisons,
		etcdSnapshotComparisons(prepared.stateKey, prepared.stateSnapshot)...,
	)
	comparisons = append(
		comparisons,
		etcdSnapshotComparisons(prepared.requestKey, prepared.requestSnapshot)...,
	)

	response, err := s.client.Txn(ctx).
		If(comparisons...).
		Then(
			clientv3.OpPut(prepared.stateKey, string(stateRaw)),
			clientv3.OpPut(prepared.requestKey, string(requestRaw)),
		).
		Else(
			clientv3.OpGet(prepared.owner.key),
			clientv3.OpGet(prepared.stateKey),
			clientv3.OpGet(prepared.requestKey),
		).
		Commit()
	if err != nil {
		// A timeout/disconnect can make the outcome indeterminate. Never release
		// the local signature here; a retry must linearly read COMPLETED first.
		return Receipt{}, s.operationError(ctx, "commit etcd receipt", err)
	}
	if !response.Succeeded {
		return Receipt{}, classifyEtcdCompareFailure(response, prepared.owner)
	}
	return cloneReceipt(receipt), nil
}

func (s *EtcdSigner) readEtcdForOwner(
	ctx context.Context,
	mutex *concurrency.Mutex,
	stateKey, requestKey string,
) (
	etcdOwnerToken,
	etcdKVSnapshot,
	etcdKVSnapshot,
	error,
) {
	// A Txn is linearizable by default. The owner comparison and all three
	// reads share one Raft-applied transaction revision.
	response, err := s.client.Txn(ctx).
		If(
			mutex.IsOwner(),
			clientv3.Compare(
				clientv3.LeaseValue(mutex.Key()),
				"=",
				s.session.Lease(),
			),
		).
		Then(
			clientv3.OpGet(mutex.Key()),
			clientv3.OpGet(stateKey),
			clientv3.OpGet(requestKey),
		).
		Else(clientv3.OpGet(mutex.Key())).
		Commit()
	if err != nil {
		return etcdOwnerToken{}, etcdKVSnapshot{}, etcdKVSnapshot{},
			s.operationError(ctx, "linearizable etcd owner read", err)
	}
	if !response.Succeeded {
		return etcdOwnerToken{}, etcdKVSnapshot{}, etcdKVSnapshot{},
			ErrEtcdOwnershipLost
	}
	if len(response.Responses) != 3 {
		return etcdOwnerToken{}, etcdKVSnapshot{}, etcdKVSnapshot{},
			fmt.Errorf("%w: invalid etcd owner transaction response", ErrCorruptStore)
	}

	lockRange := response.Responses[0].GetResponseRange()
	if lockRange == nil || len(lockRange.Kvs) != 1 {
		return etcdOwnerToken{}, etcdKVSnapshot{}, etcdKVSnapshot{},
			ErrEtcdOwnershipLost
	}
	lock := lockRange.Kvs[0]
	if string(lock.Key) != mutex.Key() ||
		clientv3.LeaseID(lock.Lease) != s.session.Lease() {
		return etcdOwnerToken{}, etcdKVSnapshot{}, etcdKVSnapshot{},
			ErrEtcdOwnershipLost
	}

	stateSnapshot, err := etcdSnapshotFromRange(
		response.Responses[1].GetResponseRange().Kvs,
	)
	if err != nil {
		return etcdOwnerToken{}, etcdKVSnapshot{}, etcdKVSnapshot{}, err
	}
	requestSnapshot, err := etcdSnapshotFromRange(
		response.Responses[2].GetResponseRange().Kvs,
	)
	if err != nil {
		return etcdOwnerToken{}, etcdKVSnapshot{}, etcdKVSnapshot{}, err
	}
	return etcdOwnerToken{
		key:            mutex.Key(),
		createRevision: lock.CreateRevision,
		lease:          s.session.Lease(),
	}, stateSnapshot, requestSnapshot, nil
}

func (s *EtcdSigner) etcdMutationComparisons(
	mutex *concurrency.Mutex,
	prepared etcdPrepared,
) []clientv3.Cmp {
	comparisons := []clientv3.Cmp{
		mutex.IsOwner(),
		clientv3.Compare(
			clientv3.LeaseValue(prepared.owner.key),
			"=",
			prepared.owner.lease,
		),
	}
	comparisons = append(
		comparisons,
		etcdSnapshotComparisons(prepared.stateKey, prepared.stateSnapshot)...,
	)
	comparisons = append(
		comparisons,
		etcdSnapshotComparisons(prepared.requestKey, prepared.requestSnapshot)...,
	)
	return comparisons
}

func etcdSnapshotComparisons(
	key string,
	snapshot etcdKVSnapshot,
) []clientv3.Cmp {
	if !snapshot.exists {
		return []clientv3.Cmp{
			clientv3.Compare(clientv3.Version(key), "=", 0),
		}
	}
	return []clientv3.Cmp{
		clientv3.Compare(
			clientv3.ModRevision(key),
			"=",
			snapshot.modRevision,
		),
		clientv3.Compare(clientv3.Value(key), "=", string(snapshot.raw)),
	}
}

func classifyEtcdCompareFailure(
	response *clientv3.TxnResponse,
	owner etcdOwnerToken,
) error {
	if response == nil || len(response.Responses) < 1 {
		return ErrEtcdConcurrentMutation
	}
	lockRange := response.Responses[0].GetResponseRange()
	if lockRange == nil || len(lockRange.Kvs) != 1 {
		return ErrEtcdOwnershipLost
	}
	lock := lockRange.Kvs[0]
	if string(lock.Key) != owner.key ||
		lock.CreateRevision != owner.createRevision ||
		clientv3.LeaseID(lock.Lease) != owner.lease {
		return ErrEtcdOwnershipLost
	}
	return ErrEtcdConcurrentMutation
}

func etcdSnapshotFromRange(kvs []*mvccpb.KeyValue) (etcdKVSnapshot, error) {
	switch len(kvs) {
	case 0:
		return etcdKVSnapshot{}, nil
	case 1:
		return etcdKVSnapshot{
			raw:         append([]byte(nil), kvs[0].Value...),
			modRevision: kvs[0].ModRevision,
			exists:      true,
		}, nil
	default:
		return etcdKVSnapshot{}, fmt.Errorf(
			"%w: exact etcd key returned multiple values",
			ErrCorruptStore,
		)
	}
}

func decodeEtcdState(raw []byte, expectedKeyID string) (etcdPersistedState, error) {
	var state etcdPersistedState
	if err := json.Unmarshal(raw, &state); err != nil {
		return etcdPersistedState{}, fmt.Errorf(
			"%w: decode etcd state: %v",
			ErrCorruptStore,
			err,
		)
	}
	if state.Version != storeVersion ||
		state.KeyID != expectedKeyID ||
		state.Owner == "" ||
		state.Epoch == 0 ||
		(state.BackendAlgorithm == "") != (len(state.BackendPublicKey) == 0) {
		return etcdPersistedState{}, fmt.Errorf(
			"%w: invalid etcd state",
			ErrCorruptStore,
		)
	}
	if state.BackendAlgorithm != "" {
		if len(state.BackendAlgorithm) > 128 ||
			len(state.BackendPublicKey) > 64*1024 {
			return etcdPersistedState{}, fmt.Errorf(
				"%w: invalid etcd backend identity",
				ErrCorruptStore,
			)
		}
	}
	return state, nil
}

func decodeEtcdRequest(
	raw []byte,
	expectedKeyID, expectedRequestID string,
) (etcdPersistedRequest, error) {
	var record etcdPersistedRequest
	if err := json.Unmarshal(raw, &record); err != nil {
		return etcdPersistedRequest{}, fmt.Errorf(
			"%w: decode etcd request: %v",
			ErrCorruptStore,
			err,
		)
	}
	if record.Version != storeVersion ||
		record.KeyID != expectedKeyID ||
		record.RequestID != expectedRequestID ||
		record.Owner == "" ||
		record.Epoch == 0 {
		return etcdPersistedRequest{}, fmt.Errorf(
			"%w: invalid etcd request binding",
			ErrCorruptStore,
		)
	}
	switch record.Status {
	case StatusPending:
		if record.Receipt != nil {
			return etcdPersistedRequest{}, fmt.Errorf(
				"%w: pending etcd request has receipt",
				ErrCorruptStore,
			)
		}
	case StatusCompleted:
		if record.Receipt == nil {
			return etcdPersistedRequest{}, fmt.Errorf(
				"%w: completed etcd request has no receipt",
				ErrCorruptStore,
			)
		}
		if err := validateEtcdStoredReceipt(
			*record.Receipt,
			record,
			expectedKeyID,
			expectedRequestID,
		); err != nil {
			return etcdPersistedRequest{}, err
		}
	default:
		return etcdPersistedRequest{}, fmt.Errorf(
			"%w: unknown etcd request status %q",
			ErrCorruptStore,
			record.Status,
		)
	}
	return record, nil
}

func validateEtcdStoredReceipt(
	receipt Receipt,
	record etcdPersistedRequest,
	expectedKeyID, expectedRequestID string,
) error {
	if receipt.Version != storeVersion ||
		receipt.KeyID != expectedKeyID ||
		receipt.RequestID != expectedRequestID ||
		receipt.Owner != record.Owner ||
		receipt.Epoch != record.Epoch ||
		receipt.PayloadDigest != record.PayloadDigest {
		return fmt.Errorf(
			"%w: etcd receipt does not match request binding",
			ErrCorruptStore,
		)
	}
	if err := validateBackendResult(BackendResult{
		Algorithm: receipt.Algorithm,
		PublicKey: receipt.PublicKey,
		Signature: receipt.Signature,
	}); err != nil {
		return fmt.Errorf(
			"%w: invalid persisted etcd receipt: %v",
			ErrCorruptStore,
			err,
		)
	}
	return nil
}

func validateEtcdRecordBackendIdentity(
	record etcdPersistedRequest,
	state etcdPersistedState,
) error {
	if record.Status != StatusCompleted || record.Receipt == nil {
		return nil
	}
	if state.BackendAlgorithm == "" || len(state.BackendPublicKey) == 0 {
		return fmt.Errorf(
			"%w: completed etcd receipt has no backend identity",
			ErrCorruptStore,
		)
	}
	if record.Receipt.Algorithm != state.BackendAlgorithm ||
		!bytes.Equal(record.Receipt.PublicKey, state.BackendPublicKey) {
		return fmt.Errorf(
			"%w: etcd receipt backend identity does not match key state",
			ErrCorruptStore,
		)
	}
	return nil
}

func etcdLockPrefix(prefix, keyID string) string {
	return prefix + "/locks/" + etcdEncodeComponent(keyID)
}

func etcdStateKey(prefix, keyID string) string {
	return prefix + "/keys/" + etcdEncodeComponent(keyID) + "/state"
}

func etcdRequestKey(prefix, keyID, requestID string) string {
	return prefix + "/keys/" + etcdEncodeComponent(keyID) +
		"/requests/" + etcdEncodeComponent(requestID)
}

func etcdEncodeComponent(value string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}
