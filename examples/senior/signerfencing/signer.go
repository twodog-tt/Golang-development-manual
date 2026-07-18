// Package signerfencing demonstrates a signer-side fencing boundary.
//
// It uses Ed25519 only to make grants and receipts independently verifiable;
// it is not a blockchain transaction signer, HSM integration, or durable state
// store. A production signer must persist the highest epoch and request records
// before releasing a signature.
package signerfencing

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrInvalidGrant         = errors.New("signerfencing: invalid grant")
	ErrUnauthorizedGrant    = errors.New("signerfencing: grant signature is invalid")
	ErrGrantNotYetValid     = errors.New("signerfencing: grant is not yet valid")
	ErrGrantExpired         = errors.New("signerfencing: grant has expired")
	ErrCallerMismatch       = errors.New("signerfencing: authenticated caller does not match grant owner")
	ErrGrantRequestMismatch = errors.New("signerfencing: request ID does not match grant")
	ErrIntentMismatch       = errors.New("signerfencing: request intent does not match grant")
	ErrPolicyMismatch       = errors.New("signerfencing: request policy does not match grant")
	ErrStaleEpoch           = errors.New("signerfencing: stale fencing epoch")
	ErrOwnerConflict        = errors.New("signerfencing: owner conflicts at the same epoch")
	ErrRequestConflict      = errors.New("signerfencing: request ID was reused for different content")
)

const grantVersion uint16 = 1

var (
	grantDomain   = []byte("interview/signer-fencing/grant/v1")
	receiptDomain = []byte("interview/signer-fencing/receipt/v1")
)

// Grant is issued by a control plane and authenticated with its signing key.
// Epoch must increase whenever ownership of a key is reassigned.
type Grant struct {
	Version      uint16
	KeyID        string
	Owner        string
	Epoch        uint64
	NotBefore    int64
	ExpiresAt    int64
	RequestID    string
	IntentDigest [32]byte
	PolicyDigest [32]byte
	Signature    []byte
}

// Request binds an idempotency key to the exact intent and policy evaluated by
// the caller. The signer does not accept an opaque "please sign these bytes"
// request without these bindings.
type Request struct {
	ID           string
	IntentDigest [32]byte
	PolicyDigest [32]byte
}

// Receipt is the signer-authenticated result. Epoch is included so an auditor
// can prove which control-plane ownership generation authorized the operation.
type Receipt struct {
	KeyID        string
	Owner        string
	RequestID    string
	Epoch        uint64
	IntentDigest [32]byte
	PolicyDigest [32]byte
	Signature    []byte
}

type requestRecord struct {
	intent  [32]byte
	policy  [32]byte
	receipt Receipt
}

type keyState struct {
	epoch uint64
	owner string
	seen  map[string]requestRecord
}

// Signer keeps state in memory for the example. Production code must put the
// state transition and signature release behind one durable, crash-safe
// boundary; otherwise a restart can forget a fencing epoch or idempotency key.
type Signer struct {
	mu             sync.Mutex
	controlPublic  ed25519.PublicKey
	signingPrivate ed25519.PrivateKey
	states         map[string]*keyState
}

func New(controlPublic ed25519.PublicKey, signingPrivate ed25519.PrivateKey) (*Signer, error) {
	if len(controlPublic) != ed25519.PublicKeySize || len(signingPrivate) != ed25519.PrivateKeySize {
		return nil, errors.New("signerfencing: valid control public and signing private keys are required")
	}
	return &Signer{
		controlPublic:  append(ed25519.PublicKey(nil), controlPublic...),
		signingPrivate: append(ed25519.PrivateKey(nil), signingPrivate...),
		states:         make(map[string]*keyState),
	}, nil
}

// IssueGrant is a small control-plane helper. It signs all grant fields with a
// domain-separated canonical encoding.
func IssueGrant(controlPrivate ed25519.PrivateKey, grant Grant) (Grant, error) {
	if len(controlPrivate) != ed25519.PrivateKeySize {
		return Grant{}, errors.New("signerfencing: valid control private key is required")
	}
	if grant.Version == 0 {
		grant.Version = grantVersion
	}
	if err := validateGrantFields(grant); err != nil {
		return Grant{}, err
	}
	grant.Signature = ed25519.Sign(controlPrivate, grantPayload(grant))
	return grant, nil
}

// Sign verifies control-plane authority and then enforces caller identity,
// exact-intent authorization, fencing, and request idempotency inside the
// signer boundary. authenticatedCaller must come from an authenticated
// transport/workload identity, not from an untrusted request field.
func (s *Signer) Sign(now time.Time, authenticatedCaller string, grant Grant, request Request) (Receipt, error) {
	if err := validateGrantFields(grant); err != nil {
		return Receipt{}, err
	}
	if !ed25519.Verify(s.controlPublic, grantPayload(grant), grant.Signature) {
		return Receipt{}, ErrUnauthorizedGrant
	}
	unix := now.Unix()
	if unix < grant.NotBefore {
		return Receipt{}, ErrGrantNotYetValid
	}
	if unix >= grant.ExpiresAt {
		return Receipt{}, ErrGrantExpired
	}
	if authenticatedCaller == "" || authenticatedCaller != grant.Owner {
		return Receipt{}, ErrCallerMismatch
	}
	if request.ID == "" || isZeroDigest(request.IntentDigest) || isZeroDigest(request.PolicyDigest) {
		return Receipt{}, errors.New("signerfencing: request ID, intent digest, and policy digest are required")
	}
	if request.ID != grant.RequestID {
		return Receipt{}, ErrGrantRequestMismatch
	}
	if request.IntentDigest != grant.IntentDigest {
		return Receipt{}, ErrIntentMismatch
	}
	if request.PolicyDigest != grant.PolicyDigest {
		return Receipt{}, ErrPolicyMismatch
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.states[grant.KeyID]
	if state == nil {
		state = &keyState{seen: make(map[string]requestRecord)}
		s.states[grant.KeyID] = state
	}
	switch {
	case grant.Epoch < state.epoch:
		return Receipt{}, fmt.Errorf("%w: got=%d highest=%d", ErrStaleEpoch, grant.Epoch, state.epoch)
	case grant.Epoch == state.epoch && state.owner != "" && grant.Owner != state.owner:
		return Receipt{}, fmt.Errorf("%w: epoch=%d current=%q requested=%q", ErrOwnerConflict, grant.Epoch, state.owner, grant.Owner)
	case grant.Epoch > state.epoch:
		state.epoch = grant.Epoch
		state.owner = grant.Owner
	case state.owner == "":
		state.epoch = grant.Epoch
		state.owner = grant.Owner
	}

	if existing, ok := state.seen[request.ID]; ok {
		if existing.intent != request.IntentDigest || existing.policy != request.PolicyDigest {
			return Receipt{}, ErrRequestConflict
		}
		return cloneReceipt(existing.receipt), nil
	}

	receipt := Receipt{
		KeyID:        grant.KeyID,
		Owner:        grant.Owner,
		RequestID:    request.ID,
		Epoch:        grant.Epoch,
		IntentDigest: request.IntentDigest,
		PolicyDigest: request.PolicyDigest,
	}
	receipt.Signature = ed25519.Sign(s.signingPrivate, receiptPayload(receipt))
	state.seen[request.ID] = requestRecord{
		intent:  request.IntentDigest,
		policy:  request.PolicyDigest,
		receipt: cloneReceipt(receipt),
	}
	return cloneReceipt(receipt), nil
}

func VerifyReceipt(publicKey ed25519.PublicKey, receipt Receipt) bool {
	if len(publicKey) != ed25519.PublicKeySize || receipt.KeyID == "" || receipt.Owner == "" ||
		receipt.RequestID == "" || receipt.Epoch == 0 || isZeroDigest(receipt.IntentDigest) ||
		isZeroDigest(receipt.PolicyDigest) {
		return false
	}
	return ed25519.Verify(publicKey, receiptPayload(receipt), receipt.Signature)
}

func validateGrantFields(grant Grant) error {
	if grant.Version != grantVersion || grant.KeyID == "" || grant.Owner == "" || grant.Epoch == 0 ||
		grant.NotBefore <= 0 || grant.ExpiresAt <= grant.NotBefore || grant.RequestID == "" ||
		isZeroDigest(grant.IntentDigest) || isZeroDigest(grant.PolicyDigest) {
		return ErrInvalidGrant
	}
	return nil
}

func grantPayload(grant Grant) []byte {
	var buf bytes.Buffer
	writeBytes(&buf, grantDomain)
	writeUint16(&buf, grant.Version)
	writeString(&buf, grant.KeyID)
	writeString(&buf, grant.Owner)
	writeUint64(&buf, grant.Epoch)
	writeInt64(&buf, grant.NotBefore)
	writeInt64(&buf, grant.ExpiresAt)
	writeString(&buf, grant.RequestID)
	writeBytes(&buf, grant.IntentDigest[:])
	writeBytes(&buf, grant.PolicyDigest[:])
	return buf.Bytes()
}

func receiptPayload(receipt Receipt) []byte {
	var buf bytes.Buffer
	writeBytes(&buf, receiptDomain)
	writeString(&buf, receipt.KeyID)
	writeString(&buf, receipt.Owner)
	writeString(&buf, receipt.RequestID)
	writeUint64(&buf, receipt.Epoch)
	writeBytes(&buf, receipt.IntentDigest[:])
	writeBytes(&buf, receipt.PolicyDigest[:])
	return buf.Bytes()
}

func cloneReceipt(receipt Receipt) Receipt {
	receipt.Signature = append([]byte(nil), receipt.Signature...)
	return receipt
}

func isZeroDigest(digest [32]byte) bool {
	return digest == [32]byte{}
}

func writeString(buf *bytes.Buffer, value string) {
	writeBytes(buf, []byte(value))
}

func writeBytes(buf *bytes.Buffer, value []byte) {
	writeUint64(buf, uint64(len(value)))
	_, _ = buf.Write(value)
}

func writeUint16(buf *bytes.Buffer, value uint16) {
	var encoded [2]byte
	binary.BigEndian.PutUint16(encoded[:], value)
	_, _ = buf.Write(encoded[:])
}

func writeUint64(buf *bytes.Buffer, value uint64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	_, _ = buf.Write(encoded[:])
}

func writeInt64(buf *bytes.Buffer, value int64) {
	writeUint64(buf, uint64(value))
}
