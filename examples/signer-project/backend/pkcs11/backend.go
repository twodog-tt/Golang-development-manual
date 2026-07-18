// Package pkcs11backend adapts a crypto11 ECDSA P-256 key to fence.Backend.
package pkcs11backend

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ThalesGroup/crypto11"
	"github.com/twodog-tt/Golang-development-manual/examples/signer-project/fence"
)

const Algorithm = "ECDSA_P256_SHA256_ASN1"

var (
	ErrKeyNotFound      = errors.New("pkcs11 backend: configured key was not found")
	ErrKeyAmbiguous     = errors.New("pkcs11 backend: configured key is ambiguous")
	ErrIdentityMismatch = errors.New("pkcs11 backend: public key identity mismatch")
)

// Config selects a PKCS#11 token and the object used for one logical key.
// Exactly one of TokenLabel, TokenSerial, or SlotNumber must be set.
//
// CreateIfMissing is intentionally false by default. Production signer
// processes should consume a key created by a separately audited HSM ceremony,
// not acquire key-management authority merely because an object is absent.
type Config struct {
	ModulePath              string
	TokenLabel              string
	TokenSerial             string
	SlotNumber              *int
	PIN                     string
	LogicalKeyID            string
	ObjectID                []byte
	ObjectLabel             []byte
	MaxSessions             int
	PoolWaitTimeout         time.Duration
	CreateIfMissing         bool
	ExpectedPublicKeySHA256 []byte
}

// Identity is the stable, non-secret identity of the configured HSM key.
type Identity struct {
	Algorithm       string `json:"algorithm"`
	PublicKeyDER    []byte `json:"public_key_der,omitempty"`
	PublicKeySHA256 string `json:"public_key_sha256"`
}

// Backend owns a crypto11 context and one P-256 key object. Open verifies the
// algorithm and public identity, but a normal signing process does not claim
// that a vendor exposed every private-key attribute. RunAcceptance performs
// the separate attribute and reconnect checks required at deployment time.
type Backend struct {
	logicalKeyID string
	context      *crypto11.Context
	signer       crypto11.Signer
	public       *ecdsa.PublicKey
	publicDER    []byte
	closeOnce    sync.Once
	closeErr     error
}

// Open connects to the token and finds the configured key. Only sandbox callers
// that explicitly set CreateIfMissing may generate one; crypto11 requests
// sensitive=true and extractable=false for that generated private object.
func Open(config Config) (*Backend, error) {
	if err := validateConfig(config); err != nil {
		return nil, err
	}
	maxSessions := config.MaxSessions
	if maxSessions == 0 {
		maxSessions = 4
	}
	ctx, err := crypto11.Configure(&crypto11.Config{
		Path:            config.ModulePath,
		TokenLabel:      config.TokenLabel,
		TokenSerial:     config.TokenSerial,
		SlotNumber:      config.SlotNumber,
		Pin:             config.PIN,
		MaxSessions:     maxSessions,
		PoolWaitTimeout: config.PoolWaitTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("pkcs11 backend: configure token: %w", err)
	}
	fail := func(err error) (*Backend, error) {
		_ = ctx.Close()
		return nil, err
	}

	keys, err := ctx.FindKeyPairs(config.ObjectID, config.ObjectLabel)
	if err != nil {
		return fail(fmt.Errorf("pkcs11 backend: find key: %w", err))
	}
	if len(keys) == 0 {
		keysByID, err := ctx.FindKeyPairs(config.ObjectID, nil)
		if err != nil {
			return fail(fmt.Errorf("pkcs11 backend: check key label: %w", err))
		}
		if len(keysByID) != 0 {
			return fail(fmt.Errorf(
				"pkcs11 backend: object ID %x exists but does not match label %q",
				config.ObjectID,
				config.ObjectLabel,
			))
		}
	}
	var signer crypto11.Signer
	switch len(keys) {
	case 0:
		if !config.CreateIfMissing {
			return fail(fmt.Errorf(
				"%w: CKA_ID=%x CKA_LABEL=%q",
				ErrKeyNotFound,
				config.ObjectID,
				config.ObjectLabel,
			))
		}
		signer, err = ctx.GenerateECDSAKeyPairWithLabel(config.ObjectID, config.ObjectLabel, elliptic.P256())
		if err != nil {
			return fail(fmt.Errorf("pkcs11 backend: generate P-256 key: %w", err))
		}
	case 1:
		signer = keys[0]
	default:
		return fail(fmt.Errorf(
			"%w: CKA_ID=%x CKA_LABEL=%q matched %d key pairs",
			ErrKeyAmbiguous,
			config.ObjectID,
			config.ObjectLabel,
			len(keys),
		))
	}

	public, ok := signer.Public().(*ecdsa.PublicKey)
	if !ok || public.Curve == nil || public.Curve.Params().Name != elliptic.P256().Params().Name {
		return fail(errors.New("pkcs11 backend: configured key is not ECDSA P-256"))
	}
	publicDER, err := x509.MarshalPKIXPublicKey(public)
	if err != nil {
		return fail(fmt.Errorf("pkcs11 backend: marshal public key: %w", err))
	}
	fingerprint := sha256.Sum256(publicDER)
	if len(config.ExpectedPublicKeySHA256) != 0 &&
		!bytes.Equal(config.ExpectedPublicKeySHA256, fingerprint[:]) {
		return fail(fmt.Errorf(
			"%w: expected=%s actual=%s",
			ErrIdentityMismatch,
			hex.EncodeToString(config.ExpectedPublicKeySHA256),
			hex.EncodeToString(fingerprint[:]),
		))
	}
	return &Backend{
		logicalKeyID: config.LogicalKeyID,
		context:      ctx,
		signer:       signer,
		public:       public,
		publicDER:    publicDER,
	}, nil
}

// Identity returns a defensive copy of the public identity pinned by this
// Backend. The SHA-256 value is over the exact SubjectPublicKeyInfo DER bytes.
func (b *Backend) Identity() Identity {
	publicDER := append([]byte(nil), b.publicDER...)
	fingerprint := sha256.Sum256(publicDER)
	return Identity{
		Algorithm:       Algorithm,
		PublicKeyDER:    publicDER,
		PublicKeySHA256: hex.EncodeToString(fingerprint[:]),
	}
}

// Sign asks the PKCS#11 token to produce a DER-encoded ECDSA signature and
// verifies it before returning it to the fence layer. PKCS#11 calls cannot be
// interrupted by context cancellation once dispatched.
func (b *Backend) Sign(ctx context.Context, keyID string, digest fence.Digest) (fence.BackendResult, error) {
	if ctx == nil {
		return fence.BackendResult{}, errors.New("pkcs11 backend: context is nil")
	}
	if err := ctx.Err(); err != nil {
		return fence.BackendResult{}, err
	}
	if keyID != b.logicalKeyID {
		return fence.BackendResult{}, fmt.Errorf("pkcs11 backend: unknown key %q", keyID)
	}
	signature, err := b.signer.Sign(rand.Reader, digest[:], crypto.SHA256)
	if err != nil {
		return fence.BackendResult{}, fmt.Errorf("pkcs11 backend: ECDSA sign: %w", err)
	}
	if !ecdsa.VerifyASN1(b.public, digest[:], signature) {
		return fence.BackendResult{}, errors.New("pkcs11 backend: token returned an invalid ECDSA signature")
	}
	return fence.BackendResult{
		Algorithm: Algorithm,
		PublicKey: append([]byte(nil), b.publicDER...),
		Signature: append([]byte(nil), signature...),
	}, nil
}

// Close logs out, releases sessions, and unloads the PKCS#11 module when this
// is its final crypto11 context.
func (b *Backend) Close() error {
	if b == nil || b.context == nil {
		return nil
	}
	b.closeOnce.Do(func() {
		b.closeErr = b.context.Close()
	})
	return b.closeErr
}

// VerifyResult verifies the public key and ASN.1 ECDSA signature returned by
// this backend.
func VerifyResult(result fence.BackendResult, digest fence.Digest) bool {
	if result.Algorithm != Algorithm || len(result.PublicKey) == 0 || len(result.Signature) == 0 {
		return false
	}
	parsed, err := x509.ParsePKIXPublicKey(result.PublicKey)
	if err != nil {
		return false
	}
	public, ok := parsed.(*ecdsa.PublicKey)
	return ok && public.Curve != nil && public.Curve.Params().Name == elliptic.P256().Params().Name &&
		ecdsa.VerifyASN1(public, digest[:], result.Signature)
}

// VerifyReceipt verifies a persisted receipt from the PKCS#11 backend.
func VerifyReceipt(receipt fence.Receipt) bool {
	return VerifyResult(fence.BackendResult{
		Algorithm: receipt.Algorithm,
		PublicKey: receipt.PublicKey,
		Signature: receipt.Signature,
	}, receipt.PayloadDigest)
}

func validateConfig(config Config) error {
	selectors := 0
	if config.TokenLabel != "" {
		selectors++
	}
	if config.TokenSerial != "" {
		selectors++
	}
	if config.SlotNumber != nil {
		selectors++
	}
	switch {
	case config.ModulePath == "":
		return errors.New("pkcs11 backend: module path is required")
	case selectors != 1:
		return fmt.Errorf("pkcs11 backend: exactly one token selector is required, got %d", selectors)
	case config.SlotNumber != nil && *config.SlotNumber < 0:
		return errors.New("pkcs11 backend: slot number cannot be negative")
	case config.PIN == "":
		return errors.New("pkcs11 backend: user PIN is required")
	case config.LogicalKeyID == "":
		return errors.New("pkcs11 backend: logical key ID is required")
	case len(config.ObjectID) == 0:
		return errors.New("pkcs11 backend: object ID is required")
	case len(config.ObjectLabel) == 0:
		return errors.New("pkcs11 backend: object label is required")
	case config.MaxSessions == 1 || config.MaxSessions < 0:
		return errors.New("pkcs11 backend: max sessions must be zero or at least two")
	case config.PoolWaitTimeout < 0:
		return errors.New("pkcs11 backend: pool wait timeout cannot be negative")
	case len(config.ExpectedPublicKeySHA256) != 0 &&
		len(config.ExpectedPublicKeySHA256) != sha256.Size:
		return fmt.Errorf(
			"pkcs11 backend: expected public key SHA-256 must be %d bytes",
			sha256.Size,
		)
	default:
		return nil
	}
}
