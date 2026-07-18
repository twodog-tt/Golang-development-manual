package frostcluster

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/taurusgroup/multi-party-sig/pkg/math/curve"
	"github.com/taurusgroup/multi-party-sig/pkg/party"
	"github.com/taurusgroup/multi-party-sig/pkg/taproot"
	upstreamfrost "github.com/taurusgroup/multi-party-sig/protocols/frost"
)

func TestShareStorePlaintextAndStaticEncryptionBoundaries(t *testing.T) {
	config := testTaprootConfig(t, "alice")

	plaintextStore, err := NewShareStore(filepath.Join(t.TempDir(), "plain", "share.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := plaintextStore.SaveNew("test-key", config); err != nil {
		t.Fatal(err)
	}
	if err := plaintextStore.SaveNew("test-key", config); !errors.Is(err, ErrShareExists) {
		t.Fatalf("second SaveNew error=%v, want ErrShareExists", err)
	}
	plainEnvelope := readShareEnvelope(t, plaintextStore.Path())
	if plainEnvelope.Protection != ProtectionPlaintext {
		t.Fatalf("plaintext protection=%q", plainEnvelope.Protection)
	}
	if !bytes.Contains(plainEnvelope.Payload, []byte(`"private_share"`)) {
		t.Fatal("plaintext envelope boundary is not explicit")
	}
	loaded, err := plaintextStore.Load("test-key")
	if err != nil {
		t.Fatal(err)
	}
	assertSameConfig(t, config, loaded)

	staticKey := make([]byte, 32)
	if _, err := rand.Read(staticKey); err != nil {
		t.Fatal(err)
	}
	encryptedStore, err := NewShareStore(filepath.Join(t.TempDir(), "encrypted", "share.json"), staticKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := encryptedStore.SaveNew("test-key", config); err != nil {
		t.Fatal(err)
	}
	encryptedEnvelope := readShareEnvelope(t, encryptedStore.Path())
	if encryptedEnvelope.Protection != ProtectionStaticAESGCM {
		t.Fatalf("encrypted protection=%q", encryptedEnvelope.Protection)
	}
	if bytes.Contains(encryptedEnvelope.Payload, []byte(`"private_share"`)) {
		t.Fatal("encrypted payload exposes plaintext share JSON")
	}
	loaded, err = encryptedStore.Load("test-key")
	if err != nil {
		t.Fatal(err)
	}
	assertSameConfig(t, config, loaded)

	wrongKey := append([]byte(nil), staticKey...)
	wrongKey[0] ^= 1
	wrongStore, err := NewShareStore(encryptedStore.Path(), wrongKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wrongStore.Load("test-key"); err == nil {
		t.Fatal("wrong static key unexpectedly decrypted share")
	}
}

func TestShareStoreRejectsLoosePermissionsAndTampering(t *testing.T) {
	config := testTaprootConfig(t, "alice")
	store, err := NewShareStore(filepath.Join(t.TempDir(), "private", "share.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveNew("test-key", config); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(store.Path(), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load("test-key"); err == nil {
		t.Fatal("loosely permissioned share unexpectedly loaded")
	}
	if err := os.Chmod(store.Path(), 0o600); err != nil {
		t.Fatal(err)
	}
	envelope := readShareEnvelope(t, store.Path())
	envelope.PartyID = "bob"
	tampered, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.Path(), tampered, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load("test-key"); err == nil {
		t.Fatal("tampered envelope unexpectedly loaded")
	}
}

func testTaprootConfig(t *testing.T, self party.ID) *upstreamfrost.TaprootConfig {
	t.Helper()
	group := curve.Secp256k1{}
	secret := randomTestScalar(t)
	coefficient := randomTestScalar(t)
	publicPoint := secret.ActOnBase().(*curve.Secp256k1Point)
	if !publicPoint.HasEvenY() {
		secret.Negate()
		coefficient.Negate()
		publicPoint = secret.ActOnBase().(*curve.Secp256k1Point)
	}
	private := make(map[party.ID]*curve.Secp256k1Scalar)
	verification := make(map[party.ID]*curve.Secp256k1Point)
	for _, id := range []party.ID{"alice", "bob", "carol"} {
		scalar := group.NewScalar().
			Set(coefficient).
			Mul(id.Scalar(group)).
			Add(secret).(*curve.Secp256k1Scalar)
		private[id] = scalar
		verification[id] = scalar.ActOnBase().(*curve.Secp256k1Point)
	}
	chainKey := make([]byte, 32)
	if _, err := rand.Read(chainKey); err != nil {
		t.Fatal(err)
	}
	return &upstreamfrost.TaprootConfig{
		ID:                 self,
		Threshold:          1,
		PrivateShare:       private[self],
		PublicKey:          taproot.PublicKey(publicPoint.XBytes()),
		ChainKey:           chainKey,
		VerificationShares: verification,
	}
}

func randomTestScalar(t *testing.T) *curve.Secp256k1Scalar {
	t.Helper()
	scalar := new(curve.Secp256k1Scalar)
	for scalar.IsZero() {
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			t.Fatal(err)
		}
		if err := scalar.UnmarshalBinary(raw); err != nil {
			continue
		}
	}
	return scalar
}

func readShareEnvelope(t *testing.T, path string) shareEnvelope {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var envelope shareEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope
}

func assertSameConfig(t *testing.T, expected, actual *upstreamfrost.TaprootConfig) {
	t.Helper()
	if expected.ID != actual.ID ||
		expected.Threshold != actual.Threshold ||
		!bytes.Equal(expected.PublicKey, actual.PublicKey) ||
		!bytes.Equal(expected.ChainKey, actual.ChainKey) {
		t.Fatalf("config metadata changed after reload")
	}
	expectedPrivate, err := expected.PrivateShare.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	actualPrivate, err := actual.PrivateShare.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(expectedPrivate, actualPrivate) {
		t.Fatal("private share changed after reload")
	}
}
