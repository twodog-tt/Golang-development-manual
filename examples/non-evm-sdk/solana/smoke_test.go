package solanatx

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go/rpc"
)

func TestSmokeReadOnly(t *testing.T) {
	endpoint := os.Getenv("SOLANA_RPC_URL")
	if endpoint == "" {
		t.Skip("set SOLANA_RPC_URL to run the read-only Solana smoke test")
	}
	adapter, err := NewRPCAdapter(endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	readiness, err := adapter.Readiness(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !readiness.Ready {
		t.Fatalf("Solana node is not ready: %+v", readiness)
	}
	if err = readiness.ValidateGenesisHash(os.Getenv("SOLANA_EXPECTED_GENESIS_HASH")); err != nil {
		t.Fatal(err)
	}
	t.Logf("Solana health=%s genesis_hash=%s", readiness.Health, readiness.GenesisHash)
}

func TestSmokeBroadcastSignedTransaction(t *testing.T) {
	endpoint := os.Getenv("SOLANA_RPC_URL")
	signedTransaction := os.Getenv("SOLANA_SIGNED_TX_BASE64")
	if endpoint == "" || signedTransaction == "" {
		t.Skip("set SOLANA_RPC_URL and SOLANA_SIGNED_TX_BASE64 to opt in to broadcast")
	}
	adapter, err := NewRPCAdapter(endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	signature, err := adapter.Submit(ctx, signedTransaction, SubmitOptions{
		PreflightCommitment: rpc.CommitmentConfirmed,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Solana RPC accepted signature %s; this is not confirmation", signature)
}
