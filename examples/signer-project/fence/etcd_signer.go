package fence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

const (
	DefaultEtcdFencePrefix = "/signer-project/fence/v1"
	DefaultEtcdSessionTTL  = 15
)

var (
	// ErrEtcdOwnershipLost means the replica's session lease or per-key Mutex
	// ownership is no longer valid. The guarded etcd transaction does not write
	// a PENDING reservation or COMPLETED receipt when this error is returned.
	ErrEtcdOwnershipLost = errors.New("fence: etcd lease or mutex ownership lost")

	// ErrEtcdConcurrentMutation means data changed outside the expected
	// Mutex-guarded compare-and-swap path. Treat it as an integrity/operations
	// incident rather than retrying the backend blindly.
	ErrEtcdConcurrentMutation = errors.New("fence: etcd state changed while mutex ownership was retained")

	ErrEtcdClosed = errors.New("fence: etcd signer is closed")
)

// EtcdSignerConfig configures one signer replica. Prefix must be dedicated to
// one logical signer deployment and protected with etcd RBAC in production.
type EtcdSignerConfig struct {
	Prefix     string
	SessionTTL int
}

// EtcdSigner coordinates replicas with an etcd Session and per-key Mutex.
//
// EtcdSigner owns its Session but not Client. Callers must Close the signer
// before closing the supplied client. A single Session is intentionally shared
// across keys, while a process-local keyed lock prevents concurrent goroutines
// in one replica from treating the same Session as re-entrant Mutex ownership.
type EtcdSigner struct {
	client  *clientv3.Client
	session *concurrency.Session
	backend Backend
	prefix  string
	ttl     int
	locks   *keyedLocks

	leaseContext context.Context
	leaseCancel  context.CancelCauseFunc
	closed       atomic.Bool
	closeOnce    sync.Once
	closeErr     error
}

// NewEtcdSigner starts a lease-backed concurrency Session. The etcd client is
// caller-owned. The constructor performs network I/O while granting the lease.
func NewEtcdSigner(
	client *clientv3.Client,
	backend Backend,
	config EtcdSignerConfig,
) (*EtcdSigner, error) {
	if client == nil {
		return nil, errors.New("fence: etcd client is required")
	}
	if backend == nil {
		return nil, errors.New("fence: backend is required")
	}

	prefix, err := normalizeEtcdPrefix(config.Prefix)
	if err != nil {
		return nil, err
	}
	ttl := config.SessionTTL
	if ttl == 0 {
		ttl = DefaultEtcdSessionTTL
	}
	if ttl < 1 {
		return nil, fmt.Errorf("%w: etcd session TTL must be positive", ErrInvalidRequest)
	}

	session, err := concurrency.NewSession(client, concurrency.WithTTL(ttl))
	if err != nil {
		return nil, fmt.Errorf("fence: create etcd session: %w", err)
	}

	leaseContext, leaseCancel := context.WithCancelCause(context.Background())
	signer := &EtcdSigner{
		client:       client,
		session:      session,
		backend:      backend,
		prefix:       prefix,
		ttl:          ttl,
		locks:        newKeyedLocks(),
		leaseContext: leaseContext,
		leaseCancel:  leaseCancel,
	}
	go func() {
		select {
		case <-session.Done():
			leaseCancel(ErrEtcdOwnershipLost)
		case <-leaseContext.Done():
		}
	}()
	return signer, nil
}

// Close revokes the replica Session lease and releases its Mutex keys. It does
// not close the caller-owned etcd client.
func (s *EtcdSigner) Close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		s.closed.Store(true)
		s.leaseCancel(ErrEtcdClosed)
		s.closeErr = s.session.Close()
	})
	return s.closeErr
}

// LeaseID exposes the current Session lease for monitoring and fault-injection.
// Production callers must not transfer or reuse this lease in another replica.
func (s *EtcdSigner) LeaseID() clientv3.LeaseID {
	if s == nil || s.session == nil {
		return clientv3.NoLease
	}
	return s.session.Lease()
}

// Sign commits a PENDING reservation before invoking Backend and commits a
// COMPLETED receipt before returning it. Both writes are etcd transactions
// guarded by Mutex.IsOwner, the Session lease ID, and exact state/request CAS.
//
// This prevents a replica that lost ownership from committing a receipt. It
// does not make Backend.Sign exactly-once: a crash after Backend.Sign succeeds
// but before COMPLETED is acknowledged leaves a recoverable PENDING request,
// and a later owner may invoke the backend again.
func (s *EtcdSigner) Sign(ctx context.Context, request Request) (Receipt, error) {
	if ctx == nil {
		return Receipt{}, fmt.Errorf("%w: context is nil", ErrInvalidRequest)
	}
	if err := validateRequest(request); err != nil {
		return Receipt{}, err
	}
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	if s == nil || s.client == nil || s.session == nil {
		return Receipt{}, ErrEtcdClosed
	}
	if s.closed.Load() {
		return Receipt{}, ErrEtcdClosed
	}

	operationContext, stop := s.withLeaseContext(ctx)
	defer stop()
	if err := context.Cause(operationContext); err != nil {
		return Receipt{}, err
	}

	unlockLocal := s.locks.lock(request.KeyID)
	defer unlockLocal()

	mutex := concurrency.NewMutex(s.session, etcdLockPrefix(s.prefix, request.KeyID))
	if err := mutex.Lock(operationContext); err != nil {
		return Receipt{}, s.operationError(operationContext, "acquire etcd mutex", err)
	}
	defer s.unlockMutex(mutex)

	prepared, err := s.prepareEtcd(operationContext, mutex, request)
	if err != nil {
		return Receipt{}, err
	}
	if prepared.completed {
		return cloneReceipt(prepared.receipt), nil
	}
	if err := context.Cause(operationContext); err != nil {
		return Receipt{}, err
	}

	result, err := s.backend.Sign(operationContext, request.KeyID, prepared.digest)
	if err != nil {
		if operationErr := context.Cause(operationContext); operationErr != nil {
			return Receipt{}, operationErr
		}
		return Receipt{}, fmt.Errorf("fence: backend sign: %w", err)
	}
	if err := context.Cause(operationContext); err != nil {
		return Receipt{}, err
	}
	if err := validateBackendResult(result); err != nil {
		return Receipt{}, fmt.Errorf("%w: %v", ErrBackendResult, err)
	}

	receipt, err := s.completeEtcd(operationContext, mutex, request, prepared, result)
	if err != nil {
		return Receipt{}, err
	}
	return receipt, nil
}

// State performs an etcd linearizable Get. No WithSerializable option is used.
func (s *EtcdSigner) State(
	ctx context.Context,
	keyID string,
) (FenceState, bool, error) {
	if ctx == nil {
		return FenceState{}, false, fmt.Errorf("%w: context is nil", ErrInvalidRequest)
	}
	if err := validateName("key ID", keyID, 512); err != nil {
		return FenceState{}, false, err
	}
	if s == nil || s.client == nil || s.closed.Load() {
		return FenceState{}, false, ErrEtcdClosed
	}

	key := etcdStateKey(s.prefix, keyID)
	response, err := s.client.Get(ctx, key)
	if err != nil {
		return FenceState{}, false, fmt.Errorf("fence: linearizable etcd state read: %w", err)
	}
	snapshot, err := etcdSnapshotFromRange(response.Kvs)
	if err != nil {
		return FenceState{}, false, err
	}
	if !snapshot.exists {
		return FenceState{}, false, nil
	}
	state, err := decodeEtcdState(snapshot.raw, keyID)
	if err != nil {
		return FenceState{}, false, err
	}
	return FenceState{KeyID: keyID, Owner: state.Owner, Epoch: state.Epoch}, true, nil
}

// LookupRequest reads state and request in one linearizable read-only
// transaction and validates the persisted backend identity before returning.
func (s *EtcdSigner) LookupRequest(
	ctx context.Context,
	keyID, requestID string,
) (RequestRecord, bool, error) {
	if ctx == nil {
		return RequestRecord{}, false, fmt.Errorf("%w: context is nil", ErrInvalidRequest)
	}
	if err := validateName("key ID", keyID, 512); err != nil {
		return RequestRecord{}, false, err
	}
	if err := validateName("request ID", requestID, 512); err != nil {
		return RequestRecord{}, false, err
	}
	if s == nil || s.client == nil || s.closed.Load() {
		return RequestRecord{}, false, ErrEtcdClosed
	}

	stateKey := etcdStateKey(s.prefix, keyID)
	requestKey := etcdRequestKey(s.prefix, keyID, requestID)
	response, err := s.client.Txn(ctx).
		Then(clientv3.OpGet(stateKey), clientv3.OpGet(requestKey)).
		Commit()
	if err != nil {
		return RequestRecord{}, false, fmt.Errorf(
			"fence: linearizable etcd request read: %w",
			err,
		)
	}
	if len(response.Responses) != 2 {
		return RequestRecord{}, false, fmt.Errorf(
			"%w: invalid etcd read transaction response",
			ErrCorruptStore,
		)
	}

	stateSnapshot, err := etcdSnapshotFromRange(
		response.Responses[0].GetResponseRange().Kvs,
	)
	if err != nil {
		return RequestRecord{}, false, err
	}
	requestSnapshot, err := etcdSnapshotFromRange(
		response.Responses[1].GetResponseRange().Kvs,
	)
	if err != nil {
		return RequestRecord{}, false, err
	}
	if !requestSnapshot.exists {
		return RequestRecord{}, false, nil
	}
	if !stateSnapshot.exists {
		return RequestRecord{}, false, fmt.Errorf(
			"%w: etcd request exists without fence state",
			ErrCorruptStore,
		)
	}

	state, err := decodeEtcdState(stateSnapshot.raw, keyID)
	if err != nil {
		return RequestRecord{}, false, err
	}
	record, err := decodeEtcdRequest(requestSnapshot.raw, keyID, requestID)
	if err != nil {
		return RequestRecord{}, false, err
	}
	if record.Epoch > state.Epoch {
		return RequestRecord{}, false, fmt.Errorf(
			"%w: request epoch exceeds highest fence",
			ErrCorruptStore,
		)
	}
	if err := validateEtcdRecordBackendIdentity(record, state); err != nil {
		return RequestRecord{}, false, err
	}

	result := RequestRecord{
		Status:        record.Status,
		Owner:         record.Owner,
		Epoch:         record.Epoch,
		PayloadDigest: record.PayloadDigest,
		Receipt:       record.Receipt,
	}
	return cloneRequestRecord(result), true, nil
}

func normalizeEtcdPrefix(prefix string) (string, error) {
	if prefix == "" {
		return DefaultEtcdFencePrefix, nil
	}
	if !utf8.ValidString(prefix) {
		return "", fmt.Errorf("%w: etcd prefix is not valid UTF-8", ErrInvalidRequest)
	}
	if len(prefix) > 1024 {
		return "", fmt.Errorf("%w: etcd prefix exceeds 1024 bytes", ErrInvalidRequest)
	}
	if !strings.HasPrefix(prefix, "/") {
		return "", fmt.Errorf("%w: etcd prefix must start with /", ErrInvalidRequest)
	}
	prefix = strings.TrimRight(prefix, "/")
	if prefix == "" {
		return "", fmt.Errorf("%w: etcd prefix cannot be root", ErrInvalidRequest)
	}
	return prefix, nil
}

func (s *EtcdSigner) withLeaseContext(parent context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancelCause(parent)
	if cause := context.Cause(s.leaseContext); cause != nil {
		cancel(cause)
		return ctx, func() { cancel(context.Canceled) }
	}
	stopLeasePropagation := context.AfterFunc(s.leaseContext, func() {
		cause := context.Cause(s.leaseContext)
		if cause == nil {
			cause = ErrEtcdOwnershipLost
		}
		cancel(cause)
	})
	return ctx, func() {
		stopLeasePropagation()
		cancel(context.Canceled)
	}
}

func (s *EtcdSigner) operationError(ctx context.Context, operation string, err error) error {
	if cause := context.Cause(ctx); cause != nil {
		return cause
	}
	return fmt.Errorf("fence: %s: %w", operation, err)
}

func (s *EtcdSigner) unlockMutex(mutex *concurrency.Mutex) {
	timeout := 2 * time.Second
	if ttlTimeout := time.Duration(s.ttl) * time.Second; ttlTimeout < timeout {
		timeout = ttlTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_ = mutex.Unlock(ctx)
}
