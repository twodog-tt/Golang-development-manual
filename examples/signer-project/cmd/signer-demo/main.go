package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/twodog-tt/Golang-development-manual/examples/signer-project/backend/software"
	"github.com/twodog-tt/Golang-development-manual/examples/signer-project/fence"
)

func main() {
	dbPath := flag.String("db", ".signer-demo/fence.db", "path to the bbolt fence database")
	keyID := flag.String("key", "software-demo-key", "logical signing key ID")
	owner := flag.String("owner", "worker-a", "authenticated owner (demo input)")
	epoch := flag.Uint64("epoch", 1, "fencing epoch")
	requestID := flag.String("request", "request-1", "idempotency request ID")
	message := flag.String("message", "hello durable signer", "payload to sign")
	flag.Parse()

	if err := os.MkdirAll(filepath.Dir(*dbPath), 0o700); err != nil {
		log.Fatal(err)
	}
	seedHash := sha256.Sum256([]byte("signer-project deterministic software demo key; never use in production"))
	var seed [ed25519.SeedSize]byte
	copy(seed[:], seedHash[:])
	backend, err := software.NewFromSeed(*keyID, seed)
	if err != nil {
		log.Fatal(err)
	}
	signer, err := fence.Open(*dbPath, backend)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := signer.Close(); err != nil {
			log.Printf("close signer: %v", err)
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
	if !software.Verify(receipt) {
		log.Fatal("software signature verification failed")
	}

	fmt.Printf("backend calls in this run: %d\n", backend.Calls())
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(receipt); err != nil {
		log.Fatal(err)
	}
}
