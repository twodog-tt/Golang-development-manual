package frostsandbox_test

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	frostsandbox "github.com/twodog-tt/Golang-development-manual/examples/signer-project/backend/frost"
	"github.com/twodog-tt/Golang-development-manual/examples/signer-project/fence"
)

func TestThreePartyDKGAndTwoPartyTaprootSigning(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	backend, err := frostsandbox.New(ctx, "frost-demo-key")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := backend.Parties(), []string{"alice", "bob", "carol"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("DKG parties=%v, want %v", got, want)
	}
	if got, want := backend.SigningParties(), []string{"alice", "bob"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("signing parties=%v, want %v", got, want)
	}
	if frostsandbox.Threshold != 1 || frostsandbox.RequiredSigners != 2 {
		t.Fatalf("threshold=%d required=%d, want 1/2", frostsandbox.Threshold, frostsandbox.RequiredSigners)
	}

	signer, err := fence.Open(filepath.Join(t.TempDir(), "fence.db"), backend)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := signer.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	receipt, err := signer.Sign(ctx, fence.Request{
		KeyID:     "frost-demo-key",
		Owner:     "coordinator-a",
		Epoch:     1,
		RequestID: "frost-request-1",
		Payload:   []byte("Taproot threshold signing sandbox"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !frostsandbox.VerifyReceipt(receipt) {
		t.Fatal("FROST Taproot/BIP-340 receipt did not verify")
	}

	tampered := receipt
	tampered.PayloadDigest[0] ^= 1
	if frostsandbox.VerifyReceipt(tampered) {
		t.Fatal("tampered digest unexpectedly verified")
	}
}
