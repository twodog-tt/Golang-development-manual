package pkcs11backend_test

import (
	"context"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	pkcs11backend "github.com/twodog-tt/Golang-development-manual/examples/signer-project/backend/pkcs11"
	"github.com/twodog-tt/Golang-development-manual/examples/signer-project/fence"
)

func TestOpenRejectsIncompleteConfig(t *testing.T) {
	if _, err := pkcs11backend.Open(pkcs11backend.Config{}); err == nil {
		t.Fatal("Open accepted an empty config")
	}
}

func TestOpenRequiresExactlyOneTokenSelector(t *testing.T) {
	slot := 7
	config := pkcs11backend.Config{
		ModulePath:   "/does/not/matter",
		TokenLabel:   "token-a",
		TokenSerial:  "serial-a",
		SlotNumber:   &slot,
		PIN:          "not-a-real-pin",
		LogicalKeyID: "key-a",
		ObjectID:     []byte("key-a"),
		ObjectLabel:  []byte("key-a"),
	}
	if _, err := pkcs11backend.Open(config); err == nil {
		t.Fatal("Open accepted multiple token selectors")
	}
}

func TestOpenRejectsInvalidExpectedFingerprintBeforeLoadingModule(t *testing.T) {
	config := pkcs11backend.Config{
		ModulePath:              "/does/not/matter",
		TokenLabel:              "token-a",
		PIN:                     "not-a-real-pin",
		LogicalKeyID:            "key-a",
		ObjectID:                []byte("key-a"),
		ObjectLabel:             []byte("key-a"),
		ExpectedPublicKeySHA256: []byte{1, 2, 3},
	}
	if _, err := pkcs11backend.Open(config); err == nil {
		t.Fatal("Open accepted an invalid public-key fingerprint length")
	}
}

func TestSoftHSMAcceptance(t *testing.T) {
	module := os.Getenv("SIGNER_PROJECT_PKCS11_MODULE")
	if module == "" {
		t.Skip("set SIGNER_PROJECT_PKCS11_MODULE to run the SoftHSM2 acceptance path")
	}
	config := pkcs11backend.Config{
		ModulePath:      module,
		TokenLabel:      envOr("SIGNER_PROJECT_PKCS11_TOKEN", "signer-project"),
		PIN:             envOr("SIGNER_PROJECT_PKCS11_PIN", "123456"),
		LogicalKeyID:    "softhsm-acceptance-p256",
		ObjectID:        []byte("signer-acceptance-p256"),
		ObjectLabel:     []byte("signer-acceptance-p256"),
		MaxSessions:     4,
		CreateIfMissing: true,
	}
	existingOnly := config
	existingOnly.CreateIfMissing = false
	existingOnly.ObjectID = []byte("missing-" + filepath.Base(t.TempDir()))
	existingOnly.ObjectLabel = existingOnly.ObjectID
	if _, err := pkcs11backend.Open(existingOnly); !errors.Is(err, pkcs11backend.ErrKeyNotFound) {
		t.Fatalf("existing-key-only open: got %v, want ErrKeyNotFound", err)
	}
	bootstrap, err := pkcs11backend.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	identity := bootstrap.Identity()
	if err = bootstrap.Close(); err != nil {
		t.Fatal(err)
	}
	config.CreateIfMissing = false
	config.ExpectedPublicKeySHA256, err = hex.DecodeString(identity.PublicKeySHA256)
	if err != nil {
		t.Fatal(err)
	}
	report, err := pkcs11backend.RunAcceptance(
		context.Background(),
		config,
		pkcs11backend.AcceptanceOptions{
			Concurrency:                2,
			Signatures:                 4,
			RequireAttributeEvidence:   true,
			RequireExpectedFingerprint: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !report.ReopenIdentityStable || !report.AllSignaturesVerified {
		t.Fatalf("incomplete acceptance report: %+v", report)
	}
	if !report.ExistingKeyOnly || !report.FingerprintPinned ||
		report.AttributeEvidenceMode != "required" {
		t.Fatalf("unsafe acceptance evidence mode: %+v", report)
	}
}

func TestSoftHSMIntegration(t *testing.T) {
	module := os.Getenv("SIGNER_PROJECT_PKCS11_MODULE")
	if module == "" {
		t.Skip("set SIGNER_PROJECT_PKCS11_MODULE to run against an initialized SoftHSM2 token")
	}
	tokenLabel := envOr("SIGNER_PROJECT_PKCS11_TOKEN", "signer-project")
	pin := envOr("SIGNER_PROJECT_PKCS11_PIN", "123456")
	config := pkcs11backend.Config{
		ModulePath:      module,
		TokenLabel:      tokenLabel,
		PIN:             pin,
		LogicalKeyID:    "softhsm-p256",
		ObjectID:        []byte("signer-project-p256"),
		ObjectLabel:     []byte("signer-project-p256"),
		CreateIfMissing: true,
	}
	dbPath := filepath.Join(t.TempDir(), "fence.db")
	request := fence.Request{
		KeyID:     config.LogicalKeyID,
		Owner:     "hsm-owner-a",
		Epoch:     5,
		RequestID: "hsm-request-1",
		Payload:   []byte("SoftHSM2 durable fence integration test"),
	}

	backend, err := pkcs11backend.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := fence.Open(dbPath, backend)
	if err != nil {
		_ = backend.Close()
		t.Fatal(err)
	}
	first, err := signer.Sign(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !pkcs11backend.VerifyReceipt(first) {
		t.Fatal("SoftHSM2 P-256 receipt did not verify")
	}
	if identity := backend.Identity(); identity.PublicKeySHA256 == "" || len(identity.PublicKeyDER) == 0 {
		t.Fatalf("incomplete SoftHSM2 identity: %+v", identity)
	}
	if err = signer.Close(); err != nil {
		t.Fatal(err)
	}
	if err = backend.Close(); err != nil {
		t.Fatal(err)
	}

	reopenedBackend, err := pkcs11backend.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reopenedBackend.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	reopenedSigner, err := fence.Open(dbPath, reopenedBackend)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reopenedSigner.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	retried, err := reopenedSigner.Sign(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, retried) {
		t.Fatal("restart retry did not return the persisted HSM receipt")
	}

	higher := request
	higher.Owner = "hsm-owner-b"
	higher.Epoch = 6
	higher.RequestID = "hsm-request-2"
	higher.Payload = []byte("SoftHSM2 key reuse after restart")
	second, err := reopenedSigner.Sign(context.Background(), higher)
	if err != nil {
		t.Fatal(err)
	}
	if !pkcs11backend.VerifyReceipt(second) {
		t.Fatal("reopened SoftHSM2 key produced an invalid receipt")
	}
	if _, err = reopenedSigner.Sign(context.Background(), request); !errors.Is(err, fence.ErrStaleEpoch) {
		t.Fatalf("old epoch after HSM restart: got %v, want ErrStaleEpoch", err)
	}
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
