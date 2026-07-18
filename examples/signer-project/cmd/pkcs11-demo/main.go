package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	pkcs11backend "github.com/twodog-tt/Golang-development-manual/examples/signer-project/backend/pkcs11"
	"github.com/twodog-tt/Golang-development-manual/examples/signer-project/fence"
)

func main() {
	modulePath := flag.String("module", envOr("PKCS11_MODULE_PATH", ""), "path to the PKCS#11 module")
	tokenLabel := flag.String("token", envOr("PKCS11_TOKEN_LABEL", "signer-project"), "PKCS#11 token label")
	pin := flag.String("pin", envOr("PKCS11_USER_PIN", "123456"), "PKCS#11 user PIN (prefer environment variable)")
	keyID := flag.String("key", envOr("SIGNER_KEY_ID", "softhsm-p256"), "logical signing key ID")
	objectID := flag.String("object-id", envOr("PKCS11_OBJECT_ID", "signer-project-p256"), "PKCS#11 CKA_ID")
	objectLabel := flag.String("object-label", envOr("PKCS11_OBJECT_LABEL", "signer-project-p256"), "PKCS#11 CKA_LABEL")
	dbPath := flag.String("db", envOr("SIGNER_DB", "/var/lib/signer/fence.db"), "path to the bbolt fence database")
	owner := flag.String("owner", envOr("SIGNER_OWNER", "worker-hsm"), "authenticated owner (demo input)")
	epoch := flag.Uint64("epoch", envUint64("SIGNER_EPOCH", 1), "fencing epoch")
	requestID := flag.String("request", envOr("SIGNER_REQUEST_ID", "hsm-request-1"), "idempotency request ID")
	message := flag.String("message", envOr("SIGNER_MESSAGE", "hello SoftHSM2"), "payload to sign")
	flag.Parse()

	if err := os.MkdirAll(filepath.Dir(*dbPath), 0o700); err != nil {
		log.Fatal(err)
	}
	backend, err := pkcs11backend.Open(pkcs11backend.Config{
		ModulePath:   *modulePath,
		TokenLabel:   *tokenLabel,
		PIN:          *pin,
		LogicalKeyID: *keyID,
		ObjectID:     []byte(*objectID),
		ObjectLabel:  []byte(*objectLabel),
		// This executable is only the SoftHSM sandbox bootstrap. Production
		// HSM processes must use the default existing-key-only behavior.
		CreateIfMissing: true,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := backend.Close(); err != nil {
			log.Printf("close PKCS#11 backend: %v", err)
		}
	}()
	signer, err := fence.Open(*dbPath, backend)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := signer.Close(); err != nil {
			log.Printf("close fence store: %v", err)
		}
	}()

	receipt, err := signer.Sign(context.Background(), fence.Request{
		KeyID:     *keyID,
		Owner:     *owner,
		Epoch:     *epoch,
		RequestID: *requestID,
		Payload:   []byte(*message),
	})
	if err != nil {
		log.Fatal(err)
	}
	if !pkcs11backend.VerifyReceipt(receipt) {
		log.Fatal("P-256 ECDSA receipt verification failed")
	}

	fmt.Println("P-256 ECDSA verification: ok")
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(receipt); err != nil {
		log.Fatal(err)
	}
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envUint64(name string, fallback uint64) uint64 {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	var parsed uint64
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		log.Fatalf("parse %s: %v", name, err)
	}
	return parsed
}
