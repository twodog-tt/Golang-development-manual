// Package software provides an Ed25519 backend for tests and local demos.
// It is not a protected-key backend.
package software

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/twodog-tt/Golang-development-manual/examples/signer-project/fence"
)

const Algorithm = "ED25519"

// Backend keeps one Ed25519 private key in process memory.
type Backend struct {
	keyID   string
	private ed25519.PrivateKey
	calls   atomic.Uint64
}

// New creates a software backend with a random test key.
func New(keyID string) (*Backend, error) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return NewFromPrivateKey(keyID, private)
}

// NewFromSeed creates a reproducible software backend for tests and demos.
func NewFromSeed(keyID string, seed [ed25519.SeedSize]byte) (*Backend, error) {
	return NewFromPrivateKey(keyID, ed25519.NewKeyFromSeed(seed[:]))
}

// NewFromPrivateKey wraps an Ed25519 private key after copying it.
func NewFromPrivateKey(keyID string, private ed25519.PrivateKey) (*Backend, error) {
	if keyID == "" {
		return nil, errors.New("software backend: key ID is required")
	}
	if len(private) != ed25519.PrivateKeySize {
		return nil, errors.New("software backend: invalid Ed25519 private key")
	}
	return &Backend{keyID: keyID, private: append(ed25519.PrivateKey(nil), private...)}, nil
}

// Sign implements fence.Backend.
func (b *Backend) Sign(ctx context.Context, keyID string, digest fence.Digest) (fence.BackendResult, error) {
	if err := ctx.Err(); err != nil {
		return fence.BackendResult{}, err
	}
	if keyID != b.keyID {
		return fence.BackendResult{}, fmt.Errorf("software backend: unknown key %q", keyID)
	}
	b.calls.Add(1)
	public := b.private.Public().(ed25519.PublicKey)
	return fence.BackendResult{
		Algorithm: Algorithm,
		PublicKey: append([]byte(nil), public...),
		Signature: ed25519.Sign(b.private, digest[:]),
	}, nil
}

// Calls is the number of Sign invocations observed by this process.
func (b *Backend) Calls() uint64 {
	return b.calls.Load()
}

// Verify checks a software receipt against its persisted payload digest.
func Verify(receipt fence.Receipt) bool {
	return receipt.Algorithm == Algorithm &&
		len(receipt.PublicKey) == ed25519.PublicKeySize &&
		ed25519.Verify(ed25519.PublicKey(receipt.PublicKey), receipt.PayloadDigest[:], receipt.Signature)
}
