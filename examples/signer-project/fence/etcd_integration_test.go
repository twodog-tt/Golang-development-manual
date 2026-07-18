//go:build etcd_integration

package fence_test

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/twodog-tt/Golang-development-manual/examples/signer-project/fence"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const etcdIntegrationKeyID = "integration-custody-key"

type etcdTestBackend struct {
	private            ed25519.PrivateKey
	public             ed25519.PublicKey
	calls              atomic.Uint64
	entered            chan struct{}
	release            chan struct{}
	ignoreCancellation bool
	failIfCalled       bool
	enterOnce          sync.Once
}

func newEtcdTestBackend(material string) *etcdTestBackend {
	seed := sha256.Sum256([]byte(material))
	private := ed25519.NewKeyFromSeed(seed[:])
	public := private.Public().(ed25519.PublicKey)
	return &etcdTestBackend{
		private: private,
		public:  append(ed25519.PublicKey(nil), public...),
	}
}

func (b *etcdTestBackend) Sign(
	ctx context.Context,
	keyID string,
	digest fence.Digest,
) (fence.BackendResult, error) {
	b.calls.Add(1)
	if b.failIfCalled {
		return fence.BackendResult{}, errors.New("backend must not be called")
	}
	if keyID != etcdIntegrationKeyID {
		return fence.BackendResult{}, fmt.Errorf("unexpected key ID %q", keyID)
	}
	if b.entered != nil {
		b.enterOnce.Do(func() { close(b.entered) })
	}
	if b.release != nil {
		if b.ignoreCancellation {
			<-b.release
		} else {
			select {
			case <-b.release:
			case <-ctx.Done():
				return fence.BackendResult{}, ctx.Err()
			}
		}
	}
	return fence.BackendResult{
		Algorithm: "Ed25519-test-only",
		PublicKey: append([]byte(nil), b.public...),
		Signature: ed25519.Sign(b.private, digest[:]),
	}, nil
}

func TestEtcdTwoReplicasConcurrentSameRequest(t *testing.T) {
	endpoints := etcdIntegrationEndpoints(t)
	prefix := etcdIntegrationPrefix(t)
	backend := newEtcdTestBackend("same-request")
	replicaA, clientA := newEtcdReplica(t, endpoints, prefix, backend)
	replicaB, clientB := newEtcdReplica(t, endpoints, prefix, backend)
	defer closeEtcdReplica(replicaA, clientA)
	defer closeEtcdReplica(replicaB, clientB)
	cleanupEtcdPrefix(t, endpoints, prefix)

	request := etcdIntegrationRequest(
		"same-request",
		"control-plane-owner",
		1,
		"withdrawal-1",
	)
	start := make(chan struct{})
	type result struct {
		receipt fence.Receipt
		err     error
	}
	results := make(chan result, 2)
	for _, signer := range []*fence.EtcdSigner{replicaA, replicaB} {
		signer := signer
		go func() {
			<-start
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			receipt, err := signer.Sign(ctx, request)
			results <- result{receipt: receipt, err: err}
		}()
	}
	close(start)

	first := <-results
	second := <-results
	if first.err != nil || second.err != nil {
		t.Fatalf("replica results: first=%v second=%v", first.err, second.err)
	}
	if !reflect.DeepEqual(first.receipt, second.receipt) {
		t.Fatal("replicas did not return the same durable receipt")
	}
	if calls := backend.calls.Load(); calls != 1 {
		t.Fatalf("backend calls=%d, want 1 while ownership remained uninterrupted", calls)
	}
}

func TestEtcdSingleSessionSerializesSameKey(t *testing.T) {
	endpoints := etcdIntegrationEndpoints(t)
	prefix := etcdIntegrationPrefix(t)
	backend := newEtcdTestBackend("single-session")
	replica, client := newEtcdReplica(t, endpoints, prefix, backend)
	defer closeEtcdReplica(replica, client)
	cleanupEtcdPrefix(t, endpoints, prefix)

	request := etcdIntegrationRequest(
		"single-session-request",
		"control-plane-owner",
		2,
		"same-session-payload",
	)
	const workers = 16
	start := make(chan struct{})
	type result struct {
		receipt fence.Receipt
		err     error
	}
	results := make(chan result, workers)
	for range workers {
		go func() {
			<-start
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			receipt, err := replica.Sign(ctx, request)
			results <- result{receipt: receipt, err: err}
		}()
	}
	close(start)

	var first *fence.Receipt
	for range workers {
		result := <-results
		if result.err != nil {
			t.Fatalf("same Session Sign: %v", result.err)
		}
		if first == nil {
			receipt := result.receipt
			first = &receipt
			continue
		}
		if !reflect.DeepEqual(*first, result.receipt) {
			t.Fatal("same Session retries did not return one durable receipt")
		}
	}
	if calls := backend.calls.Load(); calls != 1 {
		t.Fatalf(
			"backend calls=%d, per-key local lock did not prevent Session re-entry",
			calls,
		)
	}
}

func TestEtcdSameEpochOwnerConflictAcrossReplicas(t *testing.T) {
	endpoints := etcdIntegrationEndpoints(t)
	prefix := etcdIntegrationPrefix(t)
	backend := newEtcdTestBackend("owner-conflict")
	replicaA, clientA := newEtcdReplica(t, endpoints, prefix, backend)
	replicaB, clientB := newEtcdReplica(t, endpoints, prefix, backend)
	defer closeEtcdReplica(replicaA, clientA)
	defer closeEtcdReplica(replicaB, clientB)
	cleanupEtcdPrefix(t, endpoints, prefix)

	requests := []fence.Request{
		etcdIntegrationRequest("owner-a-request", "owner-a", 10, "payload-a"),
		etcdIntegrationRequest("owner-b-request", "owner-b", 10, "payload-b"),
	}
	signers := []*fence.EtcdSigner{replicaA, replicaB}
	start := make(chan struct{})
	results := make(chan error, 2)
	for index := range signers {
		index := index
		go func() {
			<-start
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			_, err := signers[index].Sign(ctx, requests[index])
			results <- err
		}()
	}
	close(start)

	var successes, conflicts int
	for range 2 {
		switch err := <-results; {
		case err == nil:
			successes++
		case errors.Is(err, fence.ErrOwnerConflict):
			conflicts++
		default:
			t.Fatalf("unexpected result: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d, want 1/1", successes, conflicts)
	}
	if calls := backend.calls.Load(); calls != 1 {
		t.Fatalf("backend calls=%d, conflict path invoked backend", calls)
	}
}

func TestEtcdHigherEpochFencesOldOwner(t *testing.T) {
	endpoints := etcdIntegrationEndpoints(t)
	prefix := etcdIntegrationPrefix(t)
	backend := newEtcdTestBackend("higher-epoch")
	replicaA, clientA := newEtcdReplica(t, endpoints, prefix, backend)
	replicaB, clientB := newEtcdReplica(t, endpoints, prefix, backend)
	defer closeEtcdReplica(replicaA, clientA)
	defer closeEtcdReplica(replicaB, clientB)
	cleanupEtcdPrefix(t, endpoints, prefix)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	first := etcdIntegrationRequest("epoch-20", "owner-a", 20, "payload-a")
	if _, err := replicaA.Sign(ctx, first); err != nil {
		t.Fatal(err)
	}
	higher := etcdIntegrationRequest("epoch-21", "owner-b", 21, "payload-b")
	if _, err := replicaB.Sign(ctx, higher); err != nil {
		t.Fatal(err)
	}
	stale := etcdIntegrationRequest("stale-20", "owner-a", 20, "stale")
	if _, err := replicaA.Sign(ctx, stale); !errors.Is(err, fence.ErrStaleEpoch) {
		t.Fatalf("stale Sign error=%v, want ErrStaleEpoch", err)
	}
	conflict := etcdIntegrationRequest("conflict-21", "owner-a", 21, "split-brain")
	if _, err := replicaA.Sign(ctx, conflict); !errors.Is(err, fence.ErrOwnerConflict) {
		t.Fatalf("same epoch Sign error=%v, want ErrOwnerConflict", err)
	}
	state, found, err := replicaB.State(ctx, etcdIntegrationKeyID)
	if err != nil {
		t.Fatal(err)
	}
	if !found || state.Owner != "owner-b" || state.Epoch != 21 {
		t.Fatalf("state=%+v found=%v, want owner-b epoch 21", state, found)
	}
}

func TestEtcdLeaseLossCannotCommitReceipt(t *testing.T) {
	endpoints := etcdIntegrationEndpoints(t)
	prefix := etcdIntegrationPrefix(t)
	backend := newEtcdTestBackend("lease-loss")
	backend.entered = make(chan struct{})
	backend.release = make(chan struct{})
	backend.ignoreCancellation = true
	replicaA, clientA := newEtcdReplica(t, endpoints, prefix, backend)
	defer closeEtcdReplica(replicaA, clientA)
	cleanupEtcdPrefix(t, endpoints, prefix)

	request := etcdIntegrationRequest(
		"lease-loss-request",
		"owner-a",
		30,
		"backend-in-flight",
	)
	type result struct {
		receipt fence.Receipt
		err     error
	}
	resultChannel := make(chan result, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		receipt, err := replicaA.Sign(ctx, request)
		resultChannel <- result{receipt: receipt, err: err}
	}()

	select {
	case <-backend.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("backend was not reached")
	}
	revokeContext, revokeCancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	if _, err := clientA.Revoke(revokeContext, replicaA.LeaseID()); err != nil {
		revokeCancel()
		t.Fatalf("revoke signer lease: %v", err)
	}
	revokeCancel()
	close(backend.release)

	outcome := <-resultChannel
	if !errors.Is(outcome.err, fence.ErrEtcdOwnershipLost) {
		t.Fatalf("Sign error=%v, want ErrEtcdOwnershipLost", outcome.err)
	}
	if len(outcome.receipt.Signature) != 0 {
		t.Fatal("signature escaped after lease ownership was lost")
	}

	recoveryBackend := newEtcdTestBackend("lease-loss")
	replicaB, clientB := newEtcdReplica(t, endpoints, prefix, recoveryBackend)
	defer closeEtcdReplica(replicaB, clientB)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	record, found, err := replicaB.LookupRequest(
		ctx,
		request.KeyID,
		request.RequestID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !found || record.Status != fence.StatusPending || record.Receipt != nil {
		t.Fatalf("record=%+v found=%v, want PENDING without receipt", record, found)
	}
	recovered, err := replicaB.Sign(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(recovered.Signature) == 0 || recoveryBackend.calls.Load() != 1 {
		t.Fatal("new owner did not recover the PENDING request")
	}
}

func TestEtcdMutexOwnershipLossCannotCommitReceipt(t *testing.T) {
	endpoints := etcdIntegrationEndpoints(t)
	prefix := etcdIntegrationPrefix(t)
	backend := newEtcdTestBackend("mutex-ownership-loss")
	backend.entered = make(chan struct{})
	backend.release = make(chan struct{})
	backend.ignoreCancellation = true
	replica, client := newEtcdReplica(t, endpoints, prefix, backend)
	defer closeEtcdReplica(replica, client)
	cleanupEtcdPrefix(t, endpoints, prefix)

	request := etcdIntegrationRequest(
		"mutex-loss-request",
		"owner-a",
		35,
		"backend-in-flight",
	)
	type signResult struct {
		receipt fence.Receipt
		err     error
	}
	resultChannel := make(chan signResult, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		receipt, err := replica.Sign(ctx, request)
		resultChannel <- signResult{receipt: receipt, err: err}
	}()

	select {
	case <-backend.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("backend was not reached")
	}
	deleteContext, deleteCancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	deleteResponse, err := client.Delete(
		deleteContext,
		prefix+"/locks/",
		clientv3.WithPrefix(),
	)
	deleteCancel()
	if err != nil {
		t.Fatalf("delete mutex owner key: %v", err)
	}
	if deleteResponse.Deleted == 0 {
		t.Fatal("no mutex owner key was deleted")
	}
	close(backend.release)

	outcome := <-resultChannel
	if !errors.Is(outcome.err, fence.ErrEtcdOwnershipLost) {
		t.Fatalf("Sign error=%v, want ErrEtcdOwnershipLost", outcome.err)
	}
	if len(outcome.receipt.Signature) != 0 {
		t.Fatal("signature escaped after Mutex ownership was lost")
	}

	lookupContext, lookupCancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer lookupCancel()
	record, found, err := replica.LookupRequest(
		lookupContext,
		request.KeyID,
		request.RequestID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !found || record.Status != fence.StatusPending || record.Receipt != nil {
		t.Fatalf("record=%+v found=%v, want PENDING without receipt", record, found)
	}
}

func TestEtcdCompletedReceiptIsIdempotentAcrossReplicas(t *testing.T) {
	endpoints := etcdIntegrationEndpoints(t)
	prefix := etcdIntegrationPrefix(t)
	backendA := newEtcdTestBackend("receipt-recovery")
	replicaA, clientA := newEtcdReplica(t, endpoints, prefix, backendA)
	cleanupEtcdPrefix(t, endpoints, prefix)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	request := etcdIntegrationRequest(
		"completed-retry",
		"owner-a",
		40,
		"idempotent-payload",
	)
	first, err := replicaA.Sign(ctx, request)
	cancel()
	if err != nil {
		closeEtcdReplica(replicaA, clientA)
		t.Fatal(err)
	}
	closeEtcdReplica(replicaA, clientA)

	backendB := newEtcdTestBackend("receipt-recovery")
	backendB.failIfCalled = true
	replicaB, clientB := newEtcdReplica(t, endpoints, prefix, backendB)
	defer closeEtcdReplica(replicaB, clientB)
	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	second, err := replicaB.Sign(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("retry did not return the persisted receipt")
	}
	if calls := backendB.calls.Load(); calls != 0 {
		t.Fatalf("recovery backend calls=%d, want 0", calls)
	}
}

func etcdIntegrationEndpoints(t *testing.T) []string {
	t.Helper()
	raw := os.Getenv("ETCD_ENDPOINTS")
	if raw == "" {
		t.Skip("set ETCD_ENDPOINTS to run the opt-in etcd integration suite")
	}
	var endpoints []string
	for _, endpoint := range strings.Split(raw, ",") {
		if endpoint = strings.TrimSpace(endpoint); endpoint != "" {
			endpoints = append(endpoints, endpoint)
		}
	}
	if len(endpoints) == 0 {
		t.Fatal("ETCD_ENDPOINTS did not contain a usable endpoint")
	}
	return endpoints
}

func etcdIntegrationPrefix(t *testing.T) string {
	t.Helper()
	name := strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	return fmt.Sprintf(
		"/signer-project/integration/%d/%s",
		time.Now().UnixNano(),
		name,
	)
}

func newEtcdReplica(
	t *testing.T,
	endpoints []string,
	prefix string,
	backend fence.Backend,
) (*fence.EtcdSigner, *clientv3.Client) {
	t.Helper()
	client, err := clientv3.New(clientv3.Config{
		Endpoints:            append([]string(nil), endpoints...),
		DialTimeout:          5 * time.Second,
		DialKeepAliveTime:    2 * time.Second,
		DialKeepAliveTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("ETCD_REQUIRE_THREE_MEMBERS") == "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		members, memberErr := client.MemberList(ctx)
		cancel()
		if memberErr != nil {
			_ = client.Close()
			t.Fatalf("list etcd members: %v", memberErr)
		}
		if len(members.Members) < 3 {
			_ = client.Close()
			t.Fatalf("etcd members=%d, require at least 3", len(members.Members))
		}
	}
	signer, err := fence.NewEtcdSigner(client, backend, fence.EtcdSignerConfig{
		Prefix:     prefix,
		SessionTTL: 5,
	})
	if err != nil {
		_ = client.Close()
		t.Fatal(err)
	}
	return signer, client
}

func cleanupEtcdPrefix(t *testing.T, endpoints []string, prefix string) {
	t.Helper()
	t.Cleanup(func() {
		client, err := clientv3.New(clientv3.Config{
			Endpoints:   append([]string(nil), endpoints...),
			DialTimeout: 5 * time.Second,
		})
		if err != nil {
			t.Logf("create cleanup etcd client: %v", err)
			return
		}
		defer func() { _ = client.Close() }()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err = client.Delete(ctx, prefix, clientv3.WithPrefix()); err != nil {
			t.Logf("cleanup etcd prefix: %v", err)
		}
	})
}

func closeEtcdReplica(signer *fence.EtcdSigner, client *clientv3.Client) {
	_ = signer.Close()
	_ = client.Close()
}

func etcdIntegrationRequest(
	requestID, owner string,
	epoch uint64,
	payload string,
) fence.Request {
	return fence.Request{
		KeyID:     etcdIntegrationKeyID,
		Owner:     owner,
		Epoch:     epoch,
		RequestID: requestID,
		Payload:   []byte(payload),
	}
}
