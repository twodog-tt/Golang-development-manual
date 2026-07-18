package signerfencing

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSignerEnforcesFencingAndIdempotency(t *testing.T) {
	controlPublic, controlPrivate, signerPublic, signer := testSigner(t)
	_ = controlPublic
	now := time.Unix(2_000_000_000, 0)
	policy := sha256.Sum256([]byte("withdrawal-policy-v7"))
	intent := sha256.Sum256([]byte("chain=1,to=alice,asset=USDC,amount=10"))
	request := Request{ID: "withdrawal-42", IntentDigest: intent, PolicyDigest: policy}
	grant := mustGrant(t, controlPrivate, "worker-a", 1, request, now)

	first, err := signer.Sign(now, "worker-a", grant, request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := signer.Sign(now, "worker-a", grant, request)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Signature, second.Signature) || !VerifyReceipt(signerPublic, first) {
		t.Fatal("idempotent retry must return the original verifiable receipt")
	}

	request.IntentDigest = sha256.Sum256([]byte("tampered amount"))
	conflictingGrant := mustGrant(t, controlPrivate, "worker-a", 1, request, now)
	if _, err = signer.Sign(now, "worker-a", conflictingGrant, request); !errors.Is(err, ErrRequestConflict) {
		t.Fatalf("request ID reuse: got %v, want ErrRequestConflict", err)
	}
}

func TestOldOwnerIsFencedAtSignerBoundary(t *testing.T) {
	_, controlPrivate, _, signer := testSigner(t)
	now := time.Unix(2_000_000_000, 0)
	policy := sha256.Sum256([]byte("policy"))

	oldRequest := testRequest("stale-request", policy)
	newRequest := testRequest("new-request", policy)
	oldGrant := mustGrant(t, controlPrivate, "old-leader", 7, oldRequest, now)
	newGrant := mustGrant(t, controlPrivate, "new-leader", 8, newRequest, now)
	if _, err := signer.Sign(now, "new-leader", newGrant, newRequest); err != nil {
		t.Fatal(err)
	}
	if _, err := signer.Sign(now, "old-leader", oldGrant, oldRequest); !errors.Is(err, ErrStaleEpoch) {
		t.Fatalf("stale owner: got %v, want ErrStaleEpoch", err)
	}

	conflictRequest := testRequest("conflict-request", policy)
	conflicting := mustGrant(t, controlPrivate, "split-brain", 8, conflictRequest, now)
	if _, err := signer.Sign(now, "split-brain", conflicting, conflictRequest); !errors.Is(err, ErrOwnerConflict) {
		t.Fatalf("same epoch owner conflict: got %v, want ErrOwnerConflict", err)
	}
}

func TestSignerRejectsGrantAndPolicyFailures(t *testing.T) {
	_, controlPrivate, _, signer := testSigner(t)
	now := time.Unix(2_000_000_000, 0)
	policy := sha256.Sum256([]byte("policy"))
	request := testRequest("request", policy)
	grant := mustGrant(t, controlPrivate, "worker", 1, request, now)

	tampered := grant
	tampered.Owner = "attacker"
	if _, err := signer.Sign(now, "worker", tampered, request); !errors.Is(err, ErrUnauthorizedGrant) {
		t.Fatalf("tampered grant: got %v, want ErrUnauthorizedGrant", err)
	}

	otherPolicy := sha256.Sum256([]byte("other-policy"))
	wrongPolicy := request
	wrongPolicy.PolicyDigest = otherPolicy
	if _, err := signer.Sign(now, "worker", grant, wrongPolicy); !errors.Is(err, ErrPolicyMismatch) {
		t.Fatalf("policy mismatch: got %v, want ErrPolicyMismatch", err)
	}
	if _, err := signer.Sign(time.Unix(grant.ExpiresAt, 0), "worker", grant, request); !errors.Is(err, ErrGrantExpired) {
		t.Fatalf("expired grant: got %v, want ErrGrantExpired", err)
	}
	if _, err := signer.Sign(time.Unix(grant.NotBefore-1, 0), "worker", grant, request); !errors.Is(err, ErrGrantNotYetValid) {
		t.Fatalf("early grant: got %v, want ErrGrantNotYetValid", err)
	}
	if _, err := signer.Sign(now, "other-worker", grant, request); !errors.Is(err, ErrCallerMismatch) {
		t.Fatalf("caller mismatch: got %v, want ErrCallerMismatch", err)
	}
	tamperedIntent := request
	tamperedIntent.IntentDigest = sha256.Sum256([]byte("different-intent"))
	if _, err := signer.Sign(now, "worker", grant, tamperedIntent); !errors.Is(err, ErrIntentMismatch) {
		t.Fatalf("intent mismatch: got %v, want ErrIntentMismatch", err)
	}
}

func TestConcurrentConflictingRequestHasOneWinner(t *testing.T) {
	_, controlPrivate, _, signer := testSigner(t)
	now := time.Unix(2_000_000_000, 0)
	policy := sha256.Sum256([]byte("policy"))
	requests := []Request{
		{ID: "same-id", IntentDigest: sha256.Sum256([]byte("intent-a")), PolicyDigest: policy},
		{ID: "same-id", IntentDigest: sha256.Sum256([]byte("intent-b")), PolicyDigest: policy},
	}
	var success, conflict atomic.Int32
	var wg sync.WaitGroup
	for _, request := range requests {
		request := request
		grant := mustGrant(t, controlPrivate, "worker", 1, request, now)
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := signer.Sign(now, "worker", grant, request)
			switch {
			case err == nil:
				success.Add(1)
			case errors.Is(err, ErrRequestConflict):
				conflict.Add(1)
			default:
				t.Errorf("Sign: %v", err)
			}
		}()
	}
	wg.Wait()
	if success.Load() != 1 || conflict.Load() != 1 {
		t.Fatalf("success=%d conflict=%d, want 1/1", success.Load(), conflict.Load())
	}
}

func testSigner(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey, ed25519.PublicKey, *Signer) {
	t.Helper()
	controlPublic, controlPrivate, err := ed25519.GenerateKey(bytes.NewReader(bytes.Repeat([]byte{0x11}, 64)))
	if err != nil {
		t.Fatal(err)
	}
	signerPublic, signerPrivate, err := ed25519.GenerateKey(bytes.NewReader(bytes.Repeat([]byte{0x22}, 64)))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := New(controlPublic, signerPrivate)
	if err != nil {
		t.Fatal(err)
	}
	return controlPublic, controlPrivate, signerPublic, signer
}

func mustGrant(
	t *testing.T,
	controlPrivate ed25519.PrivateKey,
	owner string,
	epoch uint64,
	request Request,
	now time.Time,
) Grant {
	t.Helper()
	grant, err := IssueGrant(controlPrivate, Grant{
		KeyID:        "custody-key-1",
		Owner:        owner,
		Epoch:        epoch,
		NotBefore:    now.Add(-time.Minute).Unix(),
		ExpiresAt:    now.Add(time.Minute).Unix(),
		RequestID:    request.ID,
		IntentDigest: request.IntentDigest,
		PolicyDigest: request.PolicyDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	return grant
}

func testRequest(id string, policy [32]byte) Request {
	return Request{
		ID:           id,
		IntentDigest: sha256.Sum256([]byte("intent:" + id)),
		PolicyDigest: policy,
	}
}
