// Package frostsandbox runs a real FROST Taproot/BIP-340 protocol from
// multi-party-sig with an intentionally non-production in-memory transport.
package frostsandbox

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"sync"

	"github.com/taurusgroup/multi-party-sig/pkg/party"
	"github.com/taurusgroup/multi-party-sig/pkg/protocol"
	"github.com/taurusgroup/multi-party-sig/pkg/taproot"
	upstreamfrost "github.com/taurusgroup/multi-party-sig/protocols/frost"
	"github.com/twodog-tt/Golang-development-manual/examples/signer-project/fence"
)

const (
	Algorithm       = "FROST_SECP256K1_BIP340_2_OF_3"
	Threshold       = 1
	RequiredSigners = Threshold + 1
)

var defaultParties = party.IDSlice{"alice", "bob", "carol"}

// Backend holds all three shares in one process so the sandbox can run without
// external infrastructure. Production deployments must isolate them.
type Backend struct {
	keyID   string
	parties party.IDSlice
	signers party.IDSlice
	configs map[party.ID]*upstreamfrost.TaprootConfig
	public  taproot.PublicKey
	mu      sync.Mutex
}

// New runs a three-party distributed key generation protocol. In the upstream
// API, threshold is the maximum tolerated corruptions, so threshold=1 requires
// threshold+1 (two) parties for signing.
func New(ctx context.Context, keyID string) (*Backend, error) {
	if ctx == nil {
		return nil, errors.New("frost sandbox: context is nil")
	}
	if keyID == "" {
		return nil, errors.New("frost sandbox: key ID is required")
	}
	parties := defaultParties.Copy()
	sessionID, err := randomSessionID()
	if err != nil {
		return nil, fmt.Errorf("frost sandbox: create DKG session ID: %w", err)
	}
	handlers := make(map[party.ID]*protocol.MultiHandler, len(parties))
	for _, id := range parties {
		handler, err := protocol.NewMultiHandler(upstreamfrost.KeygenTaproot(id, parties, Threshold), sessionID)
		if err != nil {
			return nil, fmt.Errorf("frost sandbox: start DKG for %q: %w", id, err)
		}
		handlers[id] = handler
	}
	results, err := runHandlers(ctx, handlers)
	if err != nil {
		return nil, fmt.Errorf("frost sandbox: DKG: %w", err)
	}

	configs := make(map[party.ID]*upstreamfrost.TaprootConfig, len(parties))
	var public taproot.PublicKey
	for _, id := range parties {
		config, ok := results[id].(*upstreamfrost.TaprootConfig)
		if !ok || config == nil {
			return nil, fmt.Errorf("frost sandbox: DKG returned %T for %q", results[id], id)
		}
		if config.ID != id || config.Threshold != Threshold {
			return nil, fmt.Errorf("frost sandbox: invalid DKG config for %q", id)
		}
		if public == nil {
			public = append(taproot.PublicKey(nil), config.PublicKey...)
		} else if !bytes.Equal(public, config.PublicKey) {
			return nil, errors.New("frost sandbox: DKG participants disagree on public key")
		}
		configs[id] = config
	}
	return &Backend{
		keyID:   keyID,
		parties: parties,
		signers: parties[:RequiredSigners].Copy(),
		configs: configs,
		public:  public,
	}, nil
}

// Sign runs the interactive signing protocol with alice and bob only.
func (b *Backend) Sign(ctx context.Context, keyID string, digest fence.Digest) (fence.BackendResult, error) {
	if ctx == nil {
		return fence.BackendResult{}, errors.New("frost sandbox: context is nil")
	}
	if err := ctx.Err(); err != nil {
		return fence.BackendResult{}, err
	}
	if keyID != b.keyID {
		return fence.BackendResult{}, fmt.Errorf("frost sandbox: unknown key %q", keyID)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	sessionID, err := randomSessionID()
	if err != nil {
		return fence.BackendResult{}, fmt.Errorf("frost sandbox: create signing session ID: %w", err)
	}
	handlers := make(map[party.ID]*protocol.MultiHandler, len(b.signers))
	for _, id := range b.signers {
		handler, err := protocol.NewMultiHandler(
			upstreamfrost.SignTaproot(b.configs[id], b.signers, digest[:]),
			sessionID,
		)
		if err != nil {
			return fence.BackendResult{}, fmt.Errorf("frost sandbox: start signing for %q: %w", id, err)
		}
		handlers[id] = handler
	}
	results, err := runHandlers(ctx, handlers)
	if err != nil {
		return fence.BackendResult{}, fmt.Errorf("frost sandbox: sign: %w", err)
	}

	var signature taproot.Signature
	for _, id := range b.signers {
		candidate, ok := results[id].(taproot.Signature)
		if !ok {
			return fence.BackendResult{}, fmt.Errorf("frost sandbox: signing returned %T for %q", results[id], id)
		}
		if signature == nil {
			signature = append(taproot.Signature(nil), candidate...)
		} else if !bytes.Equal(signature, candidate) {
			return fence.BackendResult{}, errors.New("frost sandbox: signers disagree on signature")
		}
	}
	if !b.public.Verify(signature, digest[:]) {
		return fence.BackendResult{}, errors.New("frost sandbox: BIP-340 verification failed")
	}
	return fence.BackendResult{
		Algorithm: Algorithm,
		PublicKey: append([]byte(nil), b.public...),
		Signature: append([]byte(nil), signature...),
	}, nil
}

// Parties returns the three DKG participants.
func (b *Backend) Parties() []string {
	result := make([]string, len(b.parties))
	for i, id := range b.parties {
		result[i] = string(id)
	}
	return result
}

// SigningParties returns the two participants used for this sandbox's signing.
func (b *Backend) SigningParties() []string {
	result := make([]string, len(b.signers))
	for i, id := range b.signers {
		result[i] = string(id)
	}
	return result
}

// VerifyResult verifies a backend result as a BIP-340 signature.
func VerifyResult(result fence.BackendResult, digest fence.Digest) bool {
	return result.Algorithm == Algorithm &&
		len(result.PublicKey) == 32 &&
		len(result.Signature) == taproot.SignatureLen &&
		taproot.PublicKey(result.PublicKey).Verify(taproot.Signature(result.Signature), digest[:])
}

// VerifyReceipt verifies a receipt produced by this backend.
func VerifyReceipt(receipt fence.Receipt) bool {
	return VerifyResult(fence.BackendResult{
		Algorithm: receipt.Algorithm,
		PublicKey: receipt.PublicKey,
		Signature: receipt.Signature,
	}, receipt.PayloadDigest)
}

func randomSessionID() ([]byte, error) {
	sessionID := make([]byte, 32)
	if _, err := rand.Read(sessionID); err != nil {
		return nil, err
	}
	return sessionID, nil
}
