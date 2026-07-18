package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	pkcs11backend "github.com/twodog-tt/Golang-development-manual/examples/signer-project/backend/pkcs11"
)

func main() {
	module := flag.String("module", os.Getenv("PKCS11_MODULE_PATH"), "absolute path to the vendor PKCS#11 module")
	tokenLabel := flag.String("token-label", os.Getenv("PKCS11_TOKEN_LABEL"), "exact token label (exclusive with serial and slot)")
	tokenSerial := flag.String("token-serial", os.Getenv("PKCS11_TOKEN_SERIAL"), "exact token serial (exclusive with label and slot)")
	slot := flag.Int("slot", envInt("PKCS11_SLOT", -1), "numeric slot ID (exclusive with label and serial)")
	pinFile := flag.String("pin-file", os.Getenv("PKCS11_PIN_FILE"), "0600 regular file containing the user/CU credential")
	logicalKeyID := flag.String("logical-key", os.Getenv("SIGNER_KEY_ID"), "logical signer key ID")
	objectIDHex := flag.String("object-id-hex", os.Getenv("PKCS11_OBJECT_ID_HEX"), "hex-encoded exact CKA_ID bytes")
	objectLabel := flag.String("object-label", os.Getenv("PKCS11_OBJECT_LABEL"), "exact CKA_LABEL")
	expectedFingerprint := flag.String(
		"expected-spki-sha256",
		os.Getenv("PKCS11_EXPECTED_SPKI_SHA256"),
		"independently recorded SHA-256 of SubjectPublicKeyInfo DER",
	)
	requireFingerprint := flag.Bool(
		"require-fingerprint",
		true,
		"fail unless expected-spki-sha256 is supplied",
	)
	requireAttributes := flag.Bool(
		"require-attribute-evidence",
		true,
		"fail when a required CKA_* safety attribute is unreadable",
	)
	maxSessions := flag.Int("max-sessions", envInt("PKCS11_MAX_SESSIONS", 8), "maximum PKCS#11 sessions (at least two)")
	poolWait := flag.Duration("pool-wait-timeout", 10*time.Second, "maximum wait for a PKCS#11 session")
	concurrency := flag.Int("concurrency", 4, "parallel signing workers")
	signatures := flag.Int("signatures", 16, "total challenge signatures")
	timeout := flag.Duration("timeout", 2*time.Minute, "overall acceptance timeout")
	output := flag.String("output", "", "optional 0600 JSON report path; defaults to stdout")
	flag.Parse()

	pin, err := readSecretFile(*pinFile)
	if err != nil {
		log.Fatal(err)
	}
	objectID, err := decodeHex("object-id-hex", *objectIDHex)
	if err != nil {
		log.Fatal(err)
	}
	var fingerprint []byte
	if strings.TrimSpace(*expectedFingerprint) != "" {
		fingerprint, err = decodeHex("expected-spki-sha256", *expectedFingerprint)
		if err != nil {
			log.Fatal(err)
		}
	}
	var slotNumber *int
	if *slot >= 0 {
		value := *slot
		slotNumber = &value
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	report, err := pkcs11backend.RunAcceptance(ctx, pkcs11backend.Config{
		ModulePath:              *module,
		TokenLabel:              *tokenLabel,
		TokenSerial:             *tokenSerial,
		SlotNumber:              slotNumber,
		PIN:                     pin,
		LogicalKeyID:            *logicalKeyID,
		ObjectID:                objectID,
		ObjectLabel:             []byte(*objectLabel),
		MaxSessions:             *maxSessions,
		PoolWaitTimeout:         *poolWait,
		CreateIfMissing:         false,
		ExpectedPublicKeySHA256: fingerprint,
	}, pkcs11backend.AcceptanceOptions{
		Concurrency:                *concurrency,
		Signatures:                 *signatures,
		RequireAttributeEvidence:   *requireAttributes,
		RequireExpectedFingerprint: *requireFingerprint,
	})
	if err != nil {
		log.Fatal(err)
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	encoded = append(encoded, '\n')
	if *output == "" {
		if _, err = os.Stdout.Write(encoded); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err = writeReport(*output, encoded); err != nil {
		log.Fatal(err)
	}
}

func readSecretFile(path string) (string, error) {
	if path == "" {
		return "", errors.New("hsm acceptance: pin-file is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("hsm acceptance: open pin-file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()
	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("hsm acceptance: stat pin-file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("hsm acceptance: pin-file must be a regular file")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return "", fmt.Errorf(
			"hsm acceptance: pin-file permissions %04o expose the credential",
			info.Mode().Perm(),
		)
	}
	encoded, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil {
		return "", fmt.Errorf("hsm acceptance: read pin-file: %w", err)
	}
	if len(encoded) > 4096 {
		return "", errors.New("hsm acceptance: pin-file exceeds 4096 bytes")
	}
	secret := strings.TrimSuffix(string(encoded), "\n")
	secret = strings.TrimSuffix(secret, "\r")
	if secret == "" {
		return "", errors.New("hsm acceptance: pin-file is empty")
	}
	if strings.IndexByte(secret, 0) >= 0 {
		return "", errors.New("hsm acceptance: pin-file contains NUL")
	}
	return secret, nil
}

func decodeHex(name, value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("hsm acceptance: %s is required", name)
	}
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("hsm acceptance: decode %s: %w", name, err)
	}
	return decoded, nil
}

func writeReport(path string, data []byte) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("hsm acceptance: create report: %w", err)
	}
	temporaryPath := temporary.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err = temporary.Chmod(0o600); err == nil {
		_, err = temporary.Write(data)
	}
	if err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err != nil {
		return fmt.Errorf("hsm acceptance: write report: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("hsm acceptance: close report: %w", closeErr)
	}
	if err = os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("hsm acceptance: publish report: %w", err)
	}
	cleanup = false
	return nil
}

func envInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		log.Fatalf("parse %s: %v", name, err)
	}
	return parsed
}
