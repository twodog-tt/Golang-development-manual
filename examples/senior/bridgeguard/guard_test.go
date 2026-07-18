package bridgeguard

import (
	"context"
	"crypto/sha256"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

type fakeVerifier struct {
	err                    error
	authenticatedMessageID string
	calls                  atomic.Int32
}

func (v *fakeVerifier) Verify(_ context.Context, envelope Envelope, _ []byte) (VerifiedSource, error) {
	v.calls.Add(1)
	if v.err != nil {
		return VerifiedSource{}, v.err
	}
	messageID := v.authenticatedMessageID
	if messageID == "" {
		messageID = MessageID(envelope)
	}
	return VerifiedSource{
		CanonicalEventID:       envelope.SourceEventID,
		AuthenticatedMessageID: messageID,
	}, nil
}

func TestReserveBindsRoutePayloadProofAndReplay(t *testing.T) {
	verifier := &fakeVerifier{}
	guard := newTestGuard(t, verifier)
	payload := []byte(`{"recipient":"alice","amount":"40"}`)
	envelope := testEnvelope(payload, 40, "event-1", "nonce-1")

	reservation, err := guard.Reserve(context.Background(), envelope, payload, []byte("proof"))
	if err != nil {
		t.Fatal(err)
	}
	if reservation.MessageID != MessageID(envelope) || guard.PendingAmount() != 40 {
		t.Fatalf("reservation=%+v pending=%d", reservation, guard.PendingAmount())
	}
	if _, err = guard.Reserve(context.Background(), envelope, payload, []byte("proof")); !errors.Is(err, ErrReplay) {
		t.Fatalf("duplicate reserve: got %v, want ErrReplay", err)
	}
	if err = guard.Complete(reservation.MessageID); err != nil {
		t.Fatal(err)
	}
	if guard.PendingAmount() != 0 {
		t.Fatalf("pending = %d, want 0", guard.PendingAmount())
	}
	if err = guard.Complete(reservation.MessageID); err != nil {
		t.Fatalf("complete must be idempotent: %v", err)
	}
}

func TestRejectsWrongRoutePayloadProofAndLimits(t *testing.T) {
	verifier := &fakeVerifier{}
	guard := newTestGuard(t, verifier)
	payload := []byte("payload")

	wrongRoute := testEnvelope(payload, 1, "event-route", "nonce-route")
	wrongRoute.DestinationDomain = 99
	if _, err := guard.Reserve(context.Background(), wrongRoute, payload, nil); !errors.Is(err, ErrRouteMismatch) {
		t.Fatalf("wrong route: %v", err)
	}

	wrongPayload := testEnvelope(payload, 1, "event-payload", "nonce-payload")
	if _, err := guard.Reserve(context.Background(), wrongPayload, []byte("tampered"), nil); !errors.Is(err, ErrPayloadMismatch) {
		t.Fatalf("wrong payload: %v", err)
	}

	verifier.err = errors.New("invalid quorum signature")
	if _, err := guard.Reserve(context.Background(), testEnvelope(payload, 1, "event-proof", "nonce-proof"), payload, nil); err == nil {
		t.Fatal("expected proof error")
	}
	verifier.err = nil

	verifier.authenticatedMessageID = "wrong-message"
	if _, err := guard.Reserve(context.Background(), testEnvelope(payload, 1, "event-fields", "nonce-fields"), payload, nil); err == nil {
		t.Fatal("expected authenticated message mismatch")
	}
	verifier.authenticatedMessageID = ""

	if _, err := guard.Reserve(context.Background(), testEnvelope(payload, 51, "event-amount", "nonce-amount"), payload, nil); !errors.Is(err, ErrAmountLimit) {
		t.Fatalf("amount limit: %v", err)
	}

	first := testEnvelope(payload, 50, "event-first", "nonce-first")
	if _, err := guard.Reserve(context.Background(), first, payload, nil); err != nil {
		t.Fatal(err)
	}
	second := testEnvelope(payload, 50, "event-second", "nonce-second")
	if _, err := guard.Reserve(context.Background(), second, payload, nil); err != nil {
		t.Fatal(err)
	}
	third := testEnvelope(payload, 1, "event-third", "nonce-third")
	if _, err := guard.Reserve(context.Background(), third, payload, nil); !errors.Is(err, ErrPendingLimit) {
		t.Fatalf("pending limit: %v", err)
	}
}

func TestConcurrentReplayReservationHasSingleWinner(t *testing.T) {
	guard := newTestGuard(t, &fakeVerifier{})
	payload := []byte("payload")
	envelope := testEnvelope(payload, 10, "event-race", "nonce-race")

	var (
		wg      sync.WaitGroup
		success atomic.Int32
	)
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := guard.Reserve(context.Background(), envelope, payload, nil); err == nil {
				success.Add(1)
			} else if !errors.Is(err, ErrReplay) {
				t.Errorf("Reserve: %v", err)
			}
		}()
	}
	wg.Wait()
	if success.Load() != 1 {
		t.Fatalf("successful reservations = %d, want 1", success.Load())
	}
}

func newTestGuard(t *testing.T, verifier Verifier) *Guard {
	t.Helper()
	guard, err := New(Policy{
		Route: Route{
			SourceDomain:      1,
			DestinationDomain: 2,
			SourceEmitter:     "0xsource",
			DestinationApp:    "0xdestination",
			Asset:             "USDC",
		},
		MaxAmount:        50,
		MaxPendingAmount: 100,
	}, verifier)
	if err != nil {
		t.Fatal(err)
	}
	return guard
}

func testEnvelope(payload []byte, amount uint64, eventID, nonce string) Envelope {
	return Envelope{
		Version:           1,
		SourceDomain:      1,
		DestinationDomain: 2,
		SourceEmitter:     "0xsource",
		DestinationApp:    "0xdestination",
		SourceEventID:     eventID,
		Nonce:             nonce,
		Asset:             "USDC",
		Recipient:         "alice",
		Amount:            amount,
		PayloadHash:       sha256.Sum256(payload),
	}
}
