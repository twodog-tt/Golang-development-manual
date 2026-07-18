package frostcluster

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/taurusgroup/multi-party-sig/pkg/math/curve"
	"github.com/taurusgroup/multi-party-sig/pkg/math/polynomial"
	"github.com/taurusgroup/multi-party-sig/pkg/party"
	"github.com/taurusgroup/multi-party-sig/pkg/taproot"
	upstreamfrost "github.com/taurusgroup/multi-party-sig/protocols/frost"
)

const (
	maxShareFileBytes = 1 << 20

	ProtectionPlaintext    = "plaintext"
	ProtectionStaticAESGCM = "aes-256-gcm-static"
)

var ErrShareExists = errors.New("participant share already exists")

type shareEnvelope struct {
	Version    int    `json:"version"`
	Protection string `json:"protection"`
	KeyID      string `json:"key_id"`
	PartyID    string `json:"party_id"`
	Nonce      []byte `json:"nonce,omitempty"`
	Payload    []byte `json:"payload"`
}

type storedTaprootConfig struct {
	ID                 string            `json:"id"`
	Threshold          int               `json:"threshold"`
	PrivateShare       []byte            `json:"private_share"`
	PublicKey          []byte            `json:"public_key"`
	ChainKey           []byte            `json:"chain_key"`
	Parties            []string          `json:"parties"`
	VerificationShares map[string][]byte `json:"verification_shares"`
}

// ShareStore owns exactly one participant's persisted TaprootConfig file.
//
// With a nil staticKey, the share is stored as plainly encoded JSON inside the
// envelope. With a 32-byte staticKey, the payload is encrypted with AES-256-GCM.
// A static key in the same host/process security domain is only at-rest
// obfuscation against accidental file disclosure; it is not HSM/KMS-backed
// key isolation and does not protect a compromised participant process.
type ShareStore struct {
	path      string
	staticKey []byte
	mu        sync.Mutex
}

func NewShareStore(path string, staticKey []byte) (*ShareStore, error) {
	if path == "" {
		return nil, errors.New("share path is required")
	}
	if len(staticKey) != 0 && len(staticKey) != 32 {
		return nil, errors.New("static share-encryption key must be exactly 32 bytes")
	}
	return &ShareStore{
		path:      path,
		staticKey: append([]byte(nil), staticKey...),
	}, nil
}

func (s *ShareStore) Path() string {
	return s.path
}

func (s *ShareStore) Protection() string {
	if len(s.staticKey) == 32 {
		return ProtectionStaticAESGCM
	}
	return ProtectionPlaintext
}

// Exists reports whether any filesystem entry already occupies the configured
// share path. DKG uses this as a fail-fast preflight; SaveNew remains the
// authoritative no-replace operation for races.
func (s *ShareStore) Exists() (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := os.Lstat(s.path)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	default:
		return false, fmt.Errorf("inspect share path: %w", err)
	}
}

func (s *ShareStore) Save(keyID string, config *upstreamfrost.TaprootConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.save(keyID, config, true)
}

// SaveNew refuses to replace an existing share. DKG callers should use this
// method so an accidental second ceremony cannot destroy the only local share.
func (s *ShareStore) SaveNew(keyID string, config *upstreamfrost.TaprootConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.save(keyID, config, false)
}

func (s *ShareStore) save(keyID string, config *upstreamfrost.TaprootConfig, replace bool) error {
	if !validIdentifier(keyID) {
		return fmt.Errorf("invalid key ID %q", keyID)
	}
	stored, err := encodeTaprootConfig(config)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(stored)
	if err != nil {
		return fmt.Errorf("marshal share payload: %w", err)
	}
	envelope := shareEnvelope{
		Version:    1,
		Protection: s.Protection(),
		KeyID:      keyID,
		PartyID:    stored.ID,
	}
	if len(s.staticKey) == 32 {
		block, err := aes.NewCipher(s.staticKey)
		if err != nil {
			return fmt.Errorf("initialize share cipher: %w", err)
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			return fmt.Errorf("initialize share AEAD: %w", err)
		}
		envelope.Nonce = make([]byte, aead.NonceSize())
		if _, err := rand.Read(envelope.Nonce); err != nil {
			return fmt.Errorf("generate share nonce: %w", err)
		}
		envelope.Payload = aead.Seal(nil, envelope.Nonce, payload, shareAAD(keyID, stored.ID))
	} else {
		envelope.Payload = payload
	}
	encoded, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal share envelope: %w", err)
	}
	encoded = append(encoded, '\n')
	if len(encoded) > maxShareFileBytes {
		return errors.New("share envelope exceeds size limit")
	}
	if err := atomicWritePrivateFile(s.path, encoded, replace); err != nil {
		return fmt.Errorf("persist share: %w", err)
	}
	return nil
}

func (s *ShareStore) Load(expectedKeyID string) (*upstreamfrost.TaprootConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !validIdentifier(expectedKeyID) {
		return nil, fmt.Errorf("invalid key ID %q", expectedKeyID)
	}
	data, err := readPrivateFile(s.path, maxShareFileBytes)
	if err != nil {
		return nil, fmt.Errorf("load share: %w", err)
	}
	var envelope shareEnvelope
	if err := decodeJSONStrict(data, &envelope); err != nil {
		return nil, fmt.Errorf("decode share envelope: %w", err)
	}
	if envelope.Version != 1 {
		return nil, fmt.Errorf("unsupported share version %d", envelope.Version)
	}
	if envelope.KeyID != expectedKeyID {
		return nil, fmt.Errorf("share key ID %q does not match %q", envelope.KeyID, expectedKeyID)
	}
	if !validIdentifier(envelope.PartyID) {
		return nil, errors.New("share envelope contains invalid party ID")
	}

	var payload []byte
	switch envelope.Protection {
	case ProtectionPlaintext:
		if len(s.staticKey) != 0 {
			return nil, errors.New("share is plaintext but store requires static encryption")
		}
		if len(envelope.Nonce) != 0 {
			return nil, errors.New("plaintext share unexpectedly contains a nonce")
		}
		payload = envelope.Payload
	case ProtectionStaticAESGCM:
		if len(s.staticKey) != 32 {
			return nil, errors.New("share requires a 32-byte static decryption key")
		}
		block, err := aes.NewCipher(s.staticKey)
		if err != nil {
			return nil, fmt.Errorf("initialize share cipher: %w", err)
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			return nil, fmt.Errorf("initialize share AEAD: %w", err)
		}
		if len(envelope.Nonce) != aead.NonceSize() {
			return nil, errors.New("share nonce has invalid length")
		}
		payload, err = aead.Open(nil, envelope.Nonce, envelope.Payload, shareAAD(envelope.KeyID, envelope.PartyID))
		if err != nil {
			return nil, errors.New("decrypt share: authentication failed")
		}
	default:
		return nil, fmt.Errorf("unsupported share protection %q", envelope.Protection)
	}

	var stored storedTaprootConfig
	if err := decodeJSONStrict(payload, &stored); err != nil {
		return nil, fmt.Errorf("decode share payload: %w", err)
	}
	if stored.ID != envelope.PartyID {
		return nil, errors.New("share envelope and payload party IDs disagree")
	}
	config, err := decodeTaprootConfig(stored)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func encodeTaprootConfig(config *upstreamfrost.TaprootConfig) (storedTaprootConfig, error) {
	if config == nil || config.PrivateShare == nil {
		return storedTaprootConfig{}, errors.New("Taproot config or private share is nil")
	}
	if !validIdentifier(string(config.ID)) {
		return storedTaprootConfig{}, errors.New("Taproot config has invalid participant ID")
	}
	if len(config.PublicKey) != 32 {
		return storedTaprootConfig{}, errors.New("Taproot public key must be 32 bytes")
	}
	if _, err := (curve.Secp256k1{}).LiftX(config.PublicKey); err != nil {
		return storedTaprootConfig{}, fmt.Errorf("invalid Taproot public key: %w", err)
	}
	if len(config.ChainKey) != 0 && len(config.ChainKey) != 32 {
		return storedTaprootConfig{}, errors.New("FROST chain key must be empty or 32 bytes")
	}
	if len(config.VerificationShares) < 2 {
		return storedTaprootConfig{}, errors.New("at least two verification shares are required")
	}
	if config.Threshold < 0 || config.Threshold >= len(config.VerificationShares) {
		return storedTaprootConfig{}, errors.New("Taproot config threshold is invalid")
	}
	privateShare, err := config.PrivateShare.MarshalBinary()
	if err != nil {
		return storedTaprootConfig{}, fmt.Errorf("marshal private share: %w", err)
	}
	stored := storedTaprootConfig{
		ID:                 string(config.ID),
		Threshold:          config.Threshold,
		PrivateShare:       privateShare,
		PublicKey:          append([]byte(nil), config.PublicKey...),
		ChainKey:           append([]byte(nil), config.ChainKey...),
		VerificationShares: make(map[string][]byte, len(config.VerificationShares)),
	}
	for id, point := range config.VerificationShares {
		if !validIdentifier(string(id)) || point == nil {
			return storedTaprootConfig{}, errors.New("invalid verification share entry")
		}
		encoded, err := point.MarshalBinary()
		if err != nil {
			return storedTaprootConfig{}, fmt.Errorf("marshal verification share %q: %w", id, err)
		}
		stored.Parties = append(stored.Parties, string(id))
		stored.VerificationShares[string(id)] = encoded
	}
	sort.Strings(stored.Parties)
	if !containsString(stored.Parties, stored.ID) {
		return storedTaprootConfig{}, errors.New("participant verification share is missing")
	}
	selfPoint := config.PrivateShare.ActOnBase()
	if !selfPoint.Equal(config.VerificationShares[config.ID]) {
		return storedTaprootConfig{}, errors.New("private share does not match participant verification share")
	}
	if err := validatePublicConsistency(
		config.PublicKey,
		partyIDsFromStrings(stored.Parties),
		config.VerificationShares,
	); err != nil {
		return storedTaprootConfig{}, err
	}
	return stored, nil
}

func decodeTaprootConfig(stored storedTaprootConfig) (*upstreamfrost.TaprootConfig, error) {
	if !validIdentifier(stored.ID) {
		return nil, errors.New("stored participant ID is invalid")
	}
	if len(stored.PublicKey) != 32 {
		return nil, errors.New("stored Taproot public key must be 32 bytes")
	}
	if _, err := (curve.Secp256k1{}).LiftX(stored.PublicKey); err != nil {
		return nil, fmt.Errorf("stored Taproot public key is invalid: %w", err)
	}
	if len(stored.ChainKey) != 0 && len(stored.ChainKey) != 32 {
		return nil, errors.New("stored FROST chain key must be empty or 32 bytes")
	}
	if len(stored.Parties) < 2 || len(stored.VerificationShares) != len(stored.Parties) {
		return nil, errors.New("stored verification-share set is incomplete")
	}
	if !sort.StringsAreSorted(stored.Parties) {
		return nil, errors.New("stored parties are not in canonical order")
	}
	for i, id := range stored.Parties {
		if !validIdentifier(id) || (i > 0 && stored.Parties[i-1] == id) {
			return nil, errors.New("stored parties contain an invalid or duplicate ID")
		}
	}
	if stored.Threshold < 0 || stored.Threshold >= len(stored.Parties) {
		return nil, errors.New("stored threshold is invalid")
	}
	if !containsString(stored.Parties, stored.ID) {
		return nil, errors.New("stored participant is absent from party set")
	}

	privateShare := new(curve.Secp256k1Scalar)
	if err := privateShare.UnmarshalBinary(stored.PrivateShare); err != nil {
		return nil, fmt.Errorf("decode private share: %w", err)
	}
	verificationShares := make(map[party.ID]*curve.Secp256k1Point, len(stored.Parties))
	for _, id := range stored.Parties {
		encoded, ok := stored.VerificationShares[id]
		if !ok {
			return nil, fmt.Errorf("verification share %q is missing", id)
		}
		point := new(curve.Secp256k1Point)
		if err := point.UnmarshalBinary(encoded); err != nil {
			return nil, fmt.Errorf("decode verification share %q: %w", id, err)
		}
		verificationShares[party.ID(id)] = point
	}
	selfPoint := privateShare.ActOnBase()
	if !selfPoint.Equal(verificationShares[party.ID(stored.ID)]) {
		return nil, errors.New("stored private share does not match verification share")
	}
	if err := validatePublicConsistency(
		stored.PublicKey,
		partyIDsFromStrings(stored.Parties),
		verificationShares,
	); err != nil {
		return nil, err
	}
	return &upstreamfrost.TaprootConfig{
		ID:                 party.ID(stored.ID),
		Threshold:          stored.Threshold,
		PrivateShare:       privateShare,
		PublicKey:          append([]byte(nil), stored.PublicKey...),
		ChainKey:           append([]byte(nil), stored.ChainKey...),
		VerificationShares: verificationShares,
	}, nil
}

func shareAAD(keyID, partyID string) []byte {
	return []byte("frostcluster/share/v1\x00" + keyID + "\x00" + partyID)
}

func atomicWritePrivateFile(path string, data []byte, replace bool) (returnErr error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create share directory: %w", err)
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("inspect share directory: %w", err)
	}
	if !dirInfo.IsDir() {
		return errors.New("share parent path is not a directory")
	}
	if dirInfo.Mode().Perm()&0o077 != 0 {
		return errors.New("share directory must not grant group/other access")
	}

	temp, err := os.CreateTemp(dir, ".frost-share-*")
	if err != nil {
		return fmt.Errorf("create temporary share: %w", err)
	}
	tempName := temp.Name()
	defer func() {
		_ = temp.Close()
		if returnErr != nil {
			_ = os.Remove(tempName)
		}
	}()
	if err := temp.Chmod(0o600); err != nil {
		return fmt.Errorf("set temporary share permissions: %w", err)
	}
	if _, err := io.Copy(temp, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("write temporary share: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync temporary share: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary share: %w", err)
	}
	if replace {
		if err := os.Rename(tempName, path); err != nil {
			return fmt.Errorf("replace share file: %w", err)
		}
	} else {
		if err := os.Link(tempName, path); err != nil {
			if errors.Is(err, os.ErrExist) {
				return ErrShareExists
			}
			return fmt.Errorf("install new share without replacement: %w", err)
		}
		if err := os.Remove(tempName); err != nil {
			return fmt.Errorf("remove temporary share link: %w", err)
		}
	}
	dirHandle, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open share directory for sync: %w", err)
	}
	defer dirHandle.Close()
	if err := dirHandle.Sync(); err != nil {
		return fmt.Errorf("sync share directory: %w", err)
	}
	return nil
}

func decodeJSONStrict(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func containsString(sorted []string, value string) bool {
	index := sort.SearchStrings(sorted, value)
	return index < len(sorted) && sorted[index] == value
}

func partyIDsFromStrings(values []string) party.IDSlice {
	ids := make(party.IDSlice, len(values))
	for i, value := range values {
		ids[i] = party.ID(value)
	}
	return ids
}

func validatePublicConsistency(
	publicKey taproot.PublicKey,
	parties party.IDSlice,
	verificationShares map[party.ID]*curve.Secp256k1Point,
) error {
	group := curve.Secp256k1{}
	coefficients := polynomial.Lagrange(group, parties)
	reconstructed := group.NewPoint()
	for _, id := range parties {
		point := verificationShares[id]
		if point == nil {
			return fmt.Errorf("verification share %q is missing", id)
		}
		reconstructed = reconstructed.Add(coefficients[id].Act(point))
	}
	expected, err := group.LiftX(publicKey)
	if err != nil {
		return fmt.Errorf("lift Taproot public key: %w", err)
	}
	if !reconstructed.Equal(expected) {
		return errors.New("verification shares do not reconstruct the Taproot public key")
	}
	return nil
}
