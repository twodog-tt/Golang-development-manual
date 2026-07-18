// Package frostcluster runs Taurus FROST Taproot DKG and signing across
// independently deployable participant processes.
//
// The coordinator in this package is a message router. It validates session
// metadata and protocol.Message headers, but it never creates or loads a FROST
// TaprootConfig and therefore never obtains a private share.
package frostcluster

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/taurusgroup/multi-party-sig/pkg/party"
)

const (
	// TaprootDKGProtocol and TaprootSignProtocol are the protocol identifiers
	// emitted by the pinned Taurus multi-party-sig version.
	TaprootDKGProtocol  = "frost/keygen-threshold-taproot"
	TaprootSignProtocol = "frost/sign-threshold-taproot"

	// DefaultMaxBodyBytes limits every JSON control request and every marshaled
	// Taurus protocol message accepted by the HTTP services.
	DefaultMaxBodyBytes int64 = 1 << 20

	DefaultMaxQueueMessages   = 4096
	DefaultMaxSessionMessages = 16384
	DefaultSessionTTL         = 10 * time.Minute
	DefaultProtocolTimeout    = 30 * time.Second
)

type SessionKind string

const (
	SessionKindDKG  SessionKind = "taproot-dkg"
	SessionKindSign SessionKind = "taproot-sign"
)

var safeIdentifier = regexp.MustCompile(`\A[A-Za-z0-9][A-Za-z0-9._-]{0,63}\z`)

// SessionSpec is public routing metadata. DigestHex is not secret. No field in
// this type contains a private share or signing nonce.
type SessionSpec struct {
	ID        string      `json:"id"`
	Kind      SessionKind `json:"kind"`
	KeyID     string      `json:"key_id"`
	Parties   []string    `json:"parties"`
	Threshold int         `json:"threshold"`
	DigestHex string      `json:"digest_hex,omitempty"`
	ExpiresAt time.Time   `json:"expires_at,omitempty"`
}

func (s SessionSpec) protocolID() string {
	switch s.Kind {
	case SessionKindDKG:
		return TaprootDKGProtocol
	case SessionKindSign:
		return TaprootSignProtocol
	default:
		return ""
	}
}

func (s SessionSpec) sessionBytes() ([]byte, error) {
	raw, err := hex.DecodeString(s.ID)
	if err != nil {
		return nil, fmt.Errorf("decode session ID: %w", err)
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("session ID must encode 32 bytes, got %d", len(raw))
	}
	return raw, nil
}

func (s SessionSpec) partyIDs() (party.IDSlice, error) {
	rawIDs := make([]party.ID, len(s.Parties))
	for i, value := range s.Parties {
		if !validIdentifier(value) {
			return nil, fmt.Errorf("invalid party ID %q", value)
		}
		rawIDs[i] = party.ID(value)
	}
	ids := party.NewIDSlice(rawIDs)
	if !ids.Valid() || len(ids) != len(s.Parties) {
		return nil, errors.New("party IDs must be unique")
	}
	return ids, nil
}

// Validate checks routing metadata and requires Parties to be in canonical
// lexical order. Requiring one representation prevents participants from
// accidentally deriving different Taurus session hashes.
func (s SessionSpec) Validate(now time.Time) error {
	if _, err := s.sessionBytes(); err != nil {
		return err
	}
	if !validIdentifier(s.KeyID) {
		return fmt.Errorf("invalid key ID %q", s.KeyID)
	}
	if s.protocolID() == "" {
		return fmt.Errorf("unsupported session kind %q", s.Kind)
	}
	ids, err := s.partyIDs()
	if err != nil {
		return err
	}
	if len(ids) < 2 {
		return errors.New("at least two parties are required")
	}
	for i, id := range ids {
		if string(id) != s.Parties[i] {
			return errors.New("parties must be in canonical lexical order")
		}
	}
	if s.Threshold < 0 || s.Threshold >= len(ids) {
		return fmt.Errorf("threshold %d is invalid for %d parties", s.Threshold, len(ids))
	}
	if s.Kind == SessionKindDKG {
		if s.DigestHex != "" {
			return errors.New("DKG session must not contain a digest")
		}
	} else {
		digest, err := hex.DecodeString(s.DigestHex)
		if err != nil || len(digest) != 32 {
			return errors.New("signing digest must be 32-byte lowercase hexadecimal")
		}
		if s.DigestHex != strings.ToLower(s.DigestHex) {
			return errors.New("signing digest must use lowercase hexadecimal")
		}
		if len(ids) < s.Threshold+1 {
			return errors.New("signing party count is below threshold+1")
		}
	}
	if !s.ExpiresAt.IsZero() && !s.ExpiresAt.After(now) {
		return errors.New("session already expired")
	}
	return nil
}

func (s SessionSpec) equal(other SessionSpec) bool {
	if s.ID != other.ID ||
		s.Kind != other.Kind ||
		s.KeyID != other.KeyID ||
		s.Threshold != other.Threshold ||
		s.DigestHex != other.DigestHex ||
		!s.ExpiresAt.Equal(other.ExpiresAt) ||
		len(s.Parties) != len(other.Parties) {
		return false
	}
	for i := range s.Parties {
		if s.Parties[i] != other.Parties[i] {
			return false
		}
	}
	return true
}

func (s SessionSpec) contains(id string) bool {
	i := sort.SearchStrings(s.Parties, id)
	return i < len(s.Parties) && s.Parties[i] == id
}

func validIdentifier(value string) bool {
	return len(value) <= 32 && safeIdentifier.MatchString(value)
}

// NewSessionID returns a cryptographically random, canonical 32-byte session
// identifier encoded as lowercase hexadecimal.
func NewSessionID() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate session ID: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

type DKGRequest struct {
	SessionID string   `json:"session_id"`
	KeyID     string   `json:"key_id"`
	Parties   []string `json:"parties"`
	Threshold int      `json:"threshold"`
}

type DKGResponse struct {
	SessionID string `json:"session_id"`
	KeyID     string `json:"key_id"`
	PartyID   string `json:"party_id"`
	PublicKey string `json:"public_key"`
	Threshold int    `json:"threshold"`
}

type SignRequest struct {
	SessionID string   `json:"session_id"`
	KeyID     string   `json:"key_id"`
	Signers   []string `json:"signers"`
	DigestHex string   `json:"digest_hex"`
}

type SignResponse struct {
	SessionID string `json:"session_id"`
	KeyID     string `json:"key_id"`
	PartyID   string `json:"party_id"`
	PublicKey string `json:"public_key"`
	Signature string `json:"signature"`
}

type deliveryResult struct {
	Status string `json:"status"`
}

type errorResponse struct {
	Error string `json:"error"`
}
