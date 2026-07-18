package aptostx

import (
	"context"
	"encoding/base64"
	"os"
	"testing"
	"time"
)

func TestSmokeReadOnly(t *testing.T) {
	endpoint := os.Getenv("APTOS_REST_URL")
	if endpoint == "" {
		t.Skip("set APTOS_REST_URL to run the read-only Aptos smoke test")
	}
	adapter, err := NewRESTAdapter(endpoint, nil)
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
		t.Fatalf("Aptos node is not ready: %+v", readiness)
	}
	if err = readiness.ValidateChainID(os.Getenv("APTOS_EXPECTED_CHAIN_ID")); err != nil {
		t.Fatal(err)
	}
	t.Logf("Aptos chain_id=%d ledger_version=%d block_height=%d", readiness.ChainID, readiness.LedgerVersion, readiness.BlockHeight)
}

func TestSmokeBroadcastSignedTransaction(t *testing.T) {
	endpoint := os.Getenv("APTOS_REST_URL")
	encodedTransaction := os.Getenv("APTOS_SIGNED_TX_BCS_BASE64")
	if endpoint == "" || encodedTransaction == "" {
		t.Skip("set APTOS_REST_URL and APTOS_SIGNED_TX_BCS_BASE64 to opt in to broadcast")
	}
	signedBCS, err := base64.StdEncoding.DecodeString(encodedTransaction)
	if err != nil {
		t.Fatalf("decode APTOS_SIGNED_TX_BCS_BASE64: %v", err)
	}
	adapter, err := NewRESTAdapter(endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := adapter.Submit(ctx, signedBCS)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Aptos REST accepted hash %s; transaction is still pending execution", result.Hash)
}
