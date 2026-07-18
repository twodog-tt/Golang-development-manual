package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"time"

	frostsandbox "github.com/twodog-tt/Golang-development-manual/examples/signer-project/backend/frost"
	"github.com/twodog-tt/Golang-development-manual/examples/signer-project/fence"
)

func main() {
	keyID := flag.String("key", "frost-demo-key", "logical FROST key ID")
	message := flag.String("message", "hello 2-of-3 FROST", "message whose domain-separated digest is signed")
	timeout := flag.Duration("timeout", 30*time.Second, "DKG and signing timeout")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	backend, err := frostsandbox.New(ctx, *keyID)
	if err != nil {
		log.Fatal(err)
	}
	digest := fence.DigestPayload([]byte(*message))
	result, err := backend.Sign(ctx, *keyID, digest)
	if err != nil {
		log.Fatal(err)
	}
	if !frostsandbox.VerifyResult(result, digest) {
		log.Fatal("BIP-340 verification failed")
	}

	fmt.Printf("DKG parties: %v\n", backend.Parties())
	fmt.Printf("threshold parameter: %d; required signers: %d; signing parties: %v\n",
		frostsandbox.Threshold, frostsandbox.RequiredSigners, backend.SigningParties())
	fmt.Printf("public key: %s\n", hex.EncodeToString(result.PublicKey))
	fmt.Printf("signature:  %s\n", hex.EncodeToString(result.Signature))
	fmt.Println("BIP-340 verification: ok")
}
