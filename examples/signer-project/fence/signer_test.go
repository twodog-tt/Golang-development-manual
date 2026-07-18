package fence_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/twodog-tt/Golang-development-manual/examples/signer-project/backend/software"
	"github.com/twodog-tt/Golang-development-manual/examples/signer-project/fence"
	bolt "go.etcd.io/bbolt"
)

const testKeyID = "custody-key-1"

type backendFunc func(context.Context, string, fence.Digest) (fence.BackendResult, error)

type storedRequestForTest struct {
	Version       uint16              `json:"version"`
	Status        fence.RequestStatus `json:"status"`
	Owner         string              `json:"owner"`
	Epoch         uint64              `json:"epoch"`
	PayloadDigest fence.Digest        `json:"payload_digest"`
	Receipt       *fence.Receipt      `json:"receipt,omitempty"`
}

func (f backendFunc) Sign(ctx context.Context, keyID string, digest fence.Digest) (fence.BackendResult, error) {
	return f(ctx, keyID, digest)
}

func TestConcurrentSameRequestSignsOnceAndPersistsReceipt(t *testing.T) {
	backend := newSoftwareBackend(t)
	signer := openSigner(t, backend)
	defer closeSigner(t, signer)

	request := testRequest("same-request", "worker-a", 1, "pay alice 10")
	const workers = 32
	start := make(chan struct{})
	receipts := make(chan fence.Receipt, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			receipt, err := signer.Sign(context.Background(), request)
			if err != nil {
				errs <- err
				return
			}
			receipts <- receipt
		}()
	}
	close(start)
	wg.Wait()
	close(receipts)
	close(errs)
	for err := range errs {
		t.Fatalf("Sign: %v", err)
	}

	var first *fence.Receipt
	count := 0
	for receipt := range receipts {
		count++
		if !software.Verify(receipt) {
			t.Fatal("receipt signature did not verify")
		}
		if first == nil {
			copy := receipt
			first = &copy
			continue
		}
		if !reflect.DeepEqual(*first, receipt) {
			t.Fatal("concurrent retry did not return the persisted receipt")
		}
	}
	if count != workers {
		t.Fatalf("got %d receipts, want %d", count, workers)
	}
	if calls := backend.Calls(); calls != 1 {
		t.Fatalf("backend calls=%d, want 1 during an uninterrupted process", calls)
	}
	record, found, err := signer.LookupRequest(request.KeyID, request.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if !found || record.Status != fence.StatusCompleted || record.Receipt == nil {
		t.Fatalf("record=%+v found=%v, want COMPLETED receipt", record, found)
	}
}

func TestRestartPreservesFenceOwnerAndReceipt(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fence.db")
	backend1 := newSoftwareBackend(t)
	signer1, err := fence.Open(dbPath, backend1)
	if err != nil {
		t.Fatal(err)
	}
	request := testRequest("restart-request", "worker-a", 7, "withdrawal 42")
	first, err := signer1.Sign(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	closeSigner(t, signer1)

	backend2 := newSoftwareBackend(t)
	signer2, err := fence.Open(dbPath, backend2)
	if err != nil {
		t.Fatal(err)
	}
	defer closeSigner(t, signer2)
	second, err := signer2.Sign(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("restart retry did not return the original persisted receipt")
	}
	if calls := backend2.Calls(); calls != 0 {
		t.Fatalf("backend calls after completed retry=%d, want 0", calls)
	}

	stale := testRequest("stale", "worker-a", 6, "old operation")
	if _, err := signer2.Sign(context.Background(), stale); !errors.Is(err, fence.ErrStaleEpoch) {
		t.Fatalf("stale epoch: got %v, want ErrStaleEpoch", err)
	}
	conflict := testRequest("owner-conflict", "worker-b", 7, "split brain")
	if _, err := signer2.Sign(context.Background(), conflict); !errors.Is(err, fence.ErrOwnerConflict) {
		t.Fatalf("same epoch owner: got %v, want ErrOwnerConflict", err)
	}
}

func TestBackendFailureLeavesPendingAndPermanentlyFencesOldEpoch(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fence.db")
	softwareBackend := newSoftwareBackend(t)
	backendFailure := errors.New("backend unavailable")
	var failedCalls atomic.Uint64
	failing := backendFunc(func(ctx context.Context, keyID string, digest fence.Digest) (fence.BackendResult, error) {
		failedCalls.Add(1)
		return fence.BackendResult{}, backendFailure
	})
	signer1, err := fence.Open(dbPath, failing)
	if err != nil {
		t.Fatal(err)
	}
	request := testRequest("pending-request", "new-owner", 9, "critical payload")
	receipt, err := signer1.Sign(context.Background(), request)
	if !errors.Is(err, backendFailure) {
		t.Fatalf("backend error: got %v, want injected failure", err)
	}
	if len(receipt.Signature) != 0 {
		t.Fatal("backend failure released a signature")
	}
	state, found, err := signer1.State(request.KeyID)
	if err != nil {
		t.Fatal(err)
	}
	if !found || state.Epoch != request.Epoch || state.Owner != request.Owner {
		t.Fatalf("state=%+v found=%v, want durable epoch 9", state, found)
	}
	record, found, err := signer1.LookupRequest(request.KeyID, request.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if !found || record.Status != fence.StatusPending || record.Receipt != nil {
		t.Fatalf("record=%+v found=%v, want PENDING", record, found)
	}
	closeSigner(t, signer1)

	signer2, err := fence.Open(dbPath, softwareBackend)
	if err != nil {
		t.Fatal(err)
	}
	defer closeSigner(t, signer2)
	old := testRequest("old-owner-request", "old-owner", 8, "must stay fenced")
	if _, err := signer2.Sign(context.Background(), old); !errors.Is(err, fence.ErrStaleEpoch) {
		t.Fatalf("old epoch after restart: got %v, want ErrStaleEpoch", err)
	}
	changed := request
	changed.Payload = []byte("different payload")
	if _, err := signer2.Sign(context.Background(), changed); !errors.Is(err, fence.ErrRequestConflict) {
		t.Fatalf("pending content conflict: got %v, want ErrRequestConflict", err)
	}
	completed, err := signer2.Sign(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !software.Verify(completed) {
		t.Fatal("recovered pending request produced an invalid receipt")
	}
	if failedCalls.Load() != 1 || softwareBackend.Calls() != 1 {
		t.Fatalf("failure calls=%d recovery calls=%d, want 1/1", failedCalls.Load(), softwareBackend.Calls())
	}
}

func TestCompletionFailureDoesNotReleaseSignatureAndKeepsReservation(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fence.db")
	softwareBackend := newSoftwareBackend(t)
	var signer1 *fence.Signer
	closingBackend := backendFunc(func(ctx context.Context, keyID string, digest fence.Digest) (fence.BackendResult, error) {
		result, err := softwareBackend.Sign(ctx, keyID, digest)
		if err != nil {
			return fence.BackendResult{}, err
		}
		if err := signer1.Close(); err != nil {
			return fence.BackendResult{}, fmt.Errorf("close before completion: %w", err)
		}
		return result, nil
	})
	var err error
	signer1, err = fence.Open(dbPath, closingBackend)
	if err != nil {
		t.Fatal(err)
	}
	request := testRequest("crash-window", "new-owner", 12, "backend succeeded before process loss")
	receipt, err := signer1.Sign(context.Background(), request)
	if err == nil || !fence.IsDatabaseClosed(err) {
		t.Fatalf("completion failure: got %v, want closed database error", err)
	}
	if len(receipt.Signature) != 0 {
		t.Fatal("signature escaped when COMPLETED could not commit")
	}

	recoveryBackend := newSoftwareBackend(t)
	signer2, err := fence.Open(dbPath, recoveryBackend)
	if err != nil {
		t.Fatal(err)
	}
	defer closeSigner(t, signer2)
	state, found, err := signer2.State(request.KeyID)
	if err != nil {
		t.Fatal(err)
	}
	if !found || state.Epoch != 12 || state.Owner != "new-owner" {
		t.Fatalf("state=%+v found=%v, reservation commit was lost", state, found)
	}
	record, found, err := signer2.LookupRequest(request.KeyID, request.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if !found || record.Status != fence.StatusPending {
		t.Fatalf("record=%+v found=%v, want PENDING after completion failure", record, found)
	}
	old := testRequest("old-after-crash", "old-owner", 11, "must remain fenced")
	if _, err := signer2.Sign(context.Background(), old); !errors.Is(err, fence.ErrStaleEpoch) {
		t.Fatalf("old epoch after completion failure: got %v, want ErrStaleEpoch", err)
	}
	completed, err := signer2.Sign(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !software.Verify(completed) || recoveryBackend.Calls() != 1 {
		t.Fatal("pending request did not recover through a fresh backend call")
	}
}

func TestConcurrentSameEpochOwnersHaveOneWinner(t *testing.T) {
	backend := newSoftwareBackend(t)
	signer := openSigner(t, backend)
	defer closeSigner(t, signer)

	requests := []fence.Request{
		testRequest("owner-a-request", "owner-a", 20, "payload-a"),
		testRequest("owner-b-request", "owner-b", 20, "payload-b"),
	}
	start := make(chan struct{})
	errs := make(chan error, len(requests))
	var wg sync.WaitGroup
	for _, request := range requests {
		request := request
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := signer.Sign(context.Background(), request)
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	var successes, conflicts int
	for err := range errs {
		switch {
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
	if backend.Calls() != 1 {
		t.Fatalf("backend calls=%d, want 1", backend.Calls())
	}
}

func TestHigherEpochCommitsEvenWhenRequestIDConflicts(t *testing.T) {
	backend := newSoftwareBackend(t)
	signer := openSigner(t, backend)
	defer closeSigner(t, signer)

	first := testRequest("reused-id", "owner-a", 30, "original payload")
	if _, err := signer.Sign(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	higherConflict := testRequest("reused-id", "owner-b", 31, "conflicting payload")
	if _, err := signer.Sign(context.Background(), higherConflict); !errors.Is(err, fence.ErrRequestConflict) {
		t.Fatalf("higher request conflict: got %v, want ErrRequestConflict", err)
	}
	state, found, err := signer.State(testKeyID)
	if err != nil {
		t.Fatal(err)
	}
	if !found || state.Epoch != 31 || state.Owner != "owner-b" {
		t.Fatalf("state=%+v found=%v, higher fence rolled back with logical conflict", state, found)
	}
	old := testRequest("new-id-at-old-epoch", "owner-a", 30, "old owner retry")
	if _, err := signer.Sign(context.Background(), old); !errors.Is(err, fence.ErrStaleEpoch) {
		t.Fatalf("old owner after higher conflict: got %v, want ErrStaleEpoch", err)
	}
	if backend.Calls() != 1 {
		t.Fatalf("backend calls=%d, conflict paths must not invoke it", backend.Calls())
	}
}

func TestCompletedRequestRejectsChangedContent(t *testing.T) {
	backend := newSoftwareBackend(t)
	signer := openSigner(t, backend)
	defer closeSigner(t, signer)

	request := testRequest("completed-conflict", "owner", 1, "payload-a")
	if _, err := signer.Sign(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	request.Payload = []byte("payload-b")
	if _, err := signer.Sign(context.Background(), request); !errors.Is(err, fence.ErrRequestConflict) {
		t.Fatalf("completed content conflict: got %v, want ErrRequestConflict", err)
	}
	if backend.Calls() != 1 {
		t.Fatalf("backend calls=%d, want 1", backend.Calls())
	}
}

func TestRestartRejectsChangedBackendKeyIdentity(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fence.db")
	backend1 := softwareBackendForMaterial(t, "first token key")
	signer1, err := fence.Open(dbPath, backend1)
	if err != nil {
		t.Fatal(err)
	}
	firstRequest := testRequest("first-receipt", "owner", 1, "payload-a")
	firstReceipt, err := signer1.Sign(context.Background(), firstRequest)
	if err != nil {
		t.Fatal(err)
	}
	closeSigner(t, signer1)

	backend2 := softwareBackendForMaterial(t, "replacement token key")
	signer2, err := fence.Open(dbPath, backend2)
	if err != nil {
		t.Fatal(err)
	}
	defer closeSigner(t, signer2)
	replacementRequest := testRequest("new-request", "owner", 1, "payload-b")
	receipt, err := signer2.Sign(context.Background(), replacementRequest)
	if !errors.Is(err, fence.ErrBackendIdentity) {
		t.Fatalf("replacement key: got %v, want ErrBackendIdentity", err)
	}
	if len(receipt.Signature) != 0 {
		t.Fatal("replacement key signature escaped the identity check")
	}
	record, found, err := signer2.LookupRequest(testKeyID, firstRequest.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if !found || record.Status != fence.StatusCompleted || record.Receipt == nil ||
		!reflect.DeepEqual(firstReceipt, *record.Receipt) {
		t.Fatal("historical receipt was not readable after backend identity mismatch")
	}
	pending, found, err := signer2.LookupRequest(testKeyID, replacementRequest.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if !found || pending.Status != fence.StatusPending {
		t.Fatalf("replacement request=%+v found=%v, want PENDING", pending, found)
	}
}

func TestCorruptReceiptContextAndBackendIdentityAreRejected(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*fence.Receipt)
	}{
		{
			name: "cross-key receipt",
			mutate: func(receipt *fence.Receipt) {
				receipt.KeyID = "different-key"
			},
		},
		{
			name: "cross-request receipt",
			mutate: func(receipt *fence.Receipt) {
				receipt.RequestID = "different-request"
			},
		},
		{
			name: "backend public key drift",
			mutate: func(receipt *fence.Receipt) {
				receipt.PublicKey[0] ^= 0xff
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "fence.db")
			backend1 := newSoftwareBackend(t)
			signer1, err := fence.Open(dbPath, backend1)
			if err != nil {
				t.Fatal(err)
			}
			request := testRequest("corruption-target", "owner", 1, "payload")
			if _, err := signer1.Sign(context.Background(), request); err != nil {
				t.Fatal(err)
			}
			closeSigner(t, signer1)

			tamperStoredReceipt(t, dbPath, request.KeyID, request.RequestID, test.mutate)

			backend2 := newSoftwareBackend(t)
			signer2, err := fence.Open(dbPath, backend2)
			if err != nil {
				t.Fatal(err)
			}
			defer closeSigner(t, signer2)
			if _, err := signer2.Sign(context.Background(), request); !errors.Is(err, fence.ErrCorruptStore) {
				t.Fatalf("Sign with corrupt receipt: got %v, want ErrCorruptStore", err)
			}
			if backend2.Calls() != 0 {
				t.Fatalf("backend calls=%d, corrupt COMPLETED receipt must fail before signing", backend2.Calls())
			}
			if _, _, err := signer2.LookupRequest(request.KeyID, request.RequestID); !errors.Is(err, fence.ErrCorruptStore) {
				t.Fatalf("LookupRequest with corrupt receipt: got %v, want ErrCorruptStore", err)
			}
		})
	}
}

func newSoftwareBackend(t *testing.T) *software.Backend {
	t.Helper()
	return softwareBackendForMaterial(t, "signer-project fence tests")
}

func softwareBackendForMaterial(t *testing.T, material string) *software.Backend {
	t.Helper()
	seedHash := sha256.Sum256([]byte(material))
	var seed [ed25519.SeedSize]byte
	copy(seed[:], seedHash[:])
	backend, err := software.NewFromSeed(testKeyID, seed)
	if err != nil {
		t.Fatal(err)
	}
	return backend
}

func openSigner(t *testing.T, backend fence.Backend) *fence.Signer {
	t.Helper()
	signer, err := fence.Open(filepath.Join(t.TempDir(), "fence.db"), backend)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func closeSigner(t *testing.T, signer *fence.Signer) {
	t.Helper()
	if err := signer.Close(); err != nil {
		t.Fatal(err)
	}
}

func tamperStoredReceipt(
	t *testing.T,
	dbPath, keyID, requestID string,
	mutate func(*fence.Receipt),
) {
	t.Helper()
	db, err := bolt.Open(dbPath, 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	if err := db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte("signer-fence-v1"))
		if root == nil {
			return errors.New("missing root bucket")
		}
		keys := root.Bucket([]byte("keys"))
		if keys == nil {
			return errors.New("missing keys bucket")
		}
		key := keys.Bucket([]byte(keyID))
		if key == nil {
			return errors.New("missing key bucket")
		}
		requests := key.Bucket([]byte("requests"))
		if requests == nil {
			return errors.New("missing requests bucket")
		}
		raw := requests.Get([]byte(requestID))
		if raw == nil {
			return errors.New("missing request record")
		}
		var record storedRequestForTest
		if err := json.Unmarshal(raw, &record); err != nil {
			return err
		}
		if record.Receipt == nil {
			return errors.New("missing receipt")
		}
		mutate(record.Receipt)
		updated, err := json.Marshal(record)
		if err != nil {
			return err
		}
		return requests.Put([]byte(requestID), updated)
	}); err != nil {
		t.Fatal(err)
	}
}

func testRequest(id, owner string, epoch uint64, payload string) fence.Request {
	return fence.Request{
		KeyID:     testKeyID,
		Owner:     owner,
		Epoch:     epoch,
		RequestID: id,
		Payload:   bytes.Clone([]byte(payload)),
	}
}
