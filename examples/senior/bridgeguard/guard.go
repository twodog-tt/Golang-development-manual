// Package bridgeguard demonstrates application-layer controls around a
// protocol-specific cross-chain proof verifier.
//
// It does not replace light-client, quorum-signature, or validity-proof
// verification. The Verifier interface is the mandatory chain/protocol boundary.
package bridgeguard

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
)

var (
	ErrReplay          = errors.New("cross-chain message already reserved or completed")
	ErrRouteMismatch   = errors.New("cross-chain route mismatch")
	ErrPayloadMismatch = errors.New("payload hash mismatch")
	ErrAmountLimit     = errors.New("per-message amount limit exceeded")
	ErrPendingLimit    = errors.New("route pending amount limit exceeded")
)

type Envelope struct {
	Version           uint16   `json:"version"`
	SourceDomain      uint32   `json:"source_domain"`
	DestinationDomain uint32   `json:"destination_domain"`
	SourceEmitter     string   `json:"source_emitter"`
	DestinationApp    string   `json:"destination_app"`
	SourceEventID     string   `json:"source_event_id"`
	Nonce             string   `json:"nonce"`
	Asset             string   `json:"asset"`
	Recipient         string   `json:"recipient"`
	Amount            uint64   `json:"amount"`
	PayloadHash       [32]byte `json:"payload_hash"`
}

type Route struct {
	SourceDomain      uint32
	DestinationDomain uint32
	SourceEmitter     string
	DestinationApp    string
	Asset             string
}

type Policy struct {
	Route            Route
	MaxAmount        uint64
	MaxPendingAmount uint64
}

type VerifiedSource struct {
	// CanonicalEventID is assigned by the protocol adapter after verifying that
	// the proof/attestation authenticates this exact source-chain event.
	CanonicalEventID string
	// AuthenticatedMessageID proves that the protocol adapter decoded and
	// authenticated every field represented by MessageID, not merely that a
	// source transaction or event with CanonicalEventID exists.
	AuthenticatedMessageID string
}

type Verifier interface {
	Verify(ctx context.Context, envelope Envelope, proof []byte) (VerifiedSource, error)
}

type State string

const (
	StateReserved  State = "RESERVED"
	StateCompleted State = "COMPLETED"
)

type Reservation struct {
	MessageID string
	Amount    uint64
	State     State
}

type Guard struct {
	mu            sync.Mutex
	policy        Policy
	verifier      Verifier
	reservations  map[string]Reservation
	pendingAmount uint64
}

func New(policy Policy, verifier Verifier) (*Guard, error) {
	if verifier == nil {
		return nil, errors.New("verifier is required")
	}
	if policy.Route.SourceDomain == policy.Route.DestinationDomain {
		return nil, errors.New("source and destination domains must differ")
	}
	if policy.Route.SourceEmitter == "" || policy.Route.DestinationApp == "" || policy.Route.Asset == "" {
		return nil, errors.New("route emitter, destination app, and asset are required")
	}
	if policy.MaxAmount == 0 || policy.MaxPendingAmount < policy.MaxAmount {
		return nil, errors.New("invalid amount limits")
	}
	return &Guard{
		policy:       policy,
		verifier:     verifier,
		reservations: make(map[string]Reservation),
	}, nil
}

// Reserve verifies the route, payload binding, and protocol proof before
// atomically consuming replay and exposure-limit capacity.
func (g *Guard) Reserve(ctx context.Context, envelope Envelope, payload, proof []byte) (Reservation, error) {
	if err := g.validateEnvelope(envelope, payload); err != nil {
		return Reservation{}, err
	}
	messageID := MessageID(envelope)
	g.mu.Lock()
	_, alreadyReserved := g.reservations[messageID]
	g.mu.Unlock()
	if alreadyReserved {
		return Reservation{}, ErrReplay
	}

	verified, err := g.verifier.Verify(ctx, envelope, proof)
	if err != nil {
		return Reservation{}, fmt.Errorf("verify source proof: %w", err)
	}
	if verified.CanonicalEventID == "" || verified.CanonicalEventID != envelope.SourceEventID {
		return Reservation{}, errors.New("verified source event does not match envelope")
	}
	if verified.AuthenticatedMessageID == "" || verified.AuthenticatedMessageID != messageID {
		return Reservation{}, errors.New("verified source message fields do not match envelope")
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.reservations[messageID]; exists {
		return Reservation{}, ErrReplay
	}
	if envelope.Amount > g.policy.MaxPendingAmount-g.pendingAmount {
		return Reservation{}, ErrPendingLimit
	}

	reservation := Reservation{
		MessageID: messageID,
		Amount:    envelope.Amount,
		State:     StateReserved,
	}
	g.reservations[messageID] = reservation
	g.pendingAmount += envelope.Amount
	return reservation, nil
}

// Complete marks destination execution as final for this application. The
// replay record remains forever (or for the protocol's full replay horizon).
func (g *Guard) Complete(messageID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	reservation, exists := g.reservations[messageID]
	if !exists {
		return errors.New("reservation not found")
	}
	if reservation.State == StateCompleted {
		return nil
	}
	reservation.State = StateCompleted
	g.reservations[messageID] = reservation
	g.pendingAmount -= reservation.Amount
	return nil
}

func (g *Guard) Reservation(messageID string) (Reservation, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	reservation, exists := g.reservations[messageID]
	return reservation, exists
}

func (g *Guard) PendingAmount() uint64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.pendingAmount
}

func (g *Guard) validateEnvelope(envelope Envelope, payload []byte) error {
	if envelope.Version == 0 || envelope.SourceEventID == "" || envelope.Nonce == "" || envelope.Recipient == "" {
		return errors.New("version, source event, nonce, and recipient are required")
	}
	if envelope.Amount > g.policy.MaxAmount {
		return ErrAmountLimit
	}
	route := g.policy.Route
	if envelope.SourceDomain != route.SourceDomain ||
		envelope.DestinationDomain != route.DestinationDomain ||
		envelope.SourceEmitter != route.SourceEmitter ||
		envelope.DestinationApp != route.DestinationApp ||
		envelope.Asset != route.Asset {
		return ErrRouteMismatch
	}
	hash := sha256.Sum256(payload)
	if !bytes.Equal(hash[:], envelope.PayloadHash[:]) {
		return ErrPayloadMismatch
	}
	return nil
}

// MessageID is an internal idempotency key, not a substitute for the bridge
// protocol's own message digest. Length prefixes prevent concatenation ambiguity.
func MessageID(envelope Envelope) string {
	hasher := sha256.New()
	writeUint16(hasher, envelope.Version)
	writeUint32(hasher, envelope.SourceDomain)
	writeUint32(hasher, envelope.DestinationDomain)
	writeString(hasher, envelope.SourceEmitter)
	writeString(hasher, envelope.DestinationApp)
	writeString(hasher, envelope.SourceEventID)
	writeString(hasher, envelope.Nonce)
	writeString(hasher, envelope.Asset)
	writeString(hasher, envelope.Recipient)
	writeUint64(hasher, envelope.Amount)
	_, _ = hasher.Write(envelope.PayloadHash[:])
	return hex.EncodeToString(hasher.Sum(nil))
}

type writer interface {
	Write([]byte) (int, error)
}

func writeUint16(w writer, value uint16) {
	var encoded [2]byte
	binary.BigEndian.PutUint16(encoded[:], value)
	_, _ = w.Write(encoded[:])
}

func writeUint32(w writer, value uint32) {
	var encoded [4]byte
	binary.BigEndian.PutUint32(encoded[:], value)
	_, _ = w.Write(encoded[:])
}

func writeUint64(w writer, value uint64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	_, _ = w.Write(encoded[:])
}

func writeString(w writer, value string) {
	writeUint32(w, uint32(len(value)))
	_, _ = w.Write([]byte(value))
}
