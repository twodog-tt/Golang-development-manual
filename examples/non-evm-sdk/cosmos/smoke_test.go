package cosmostx

import (
	"context"
	"encoding/base64"
	"os"
	"testing"
	"time"
)

func TestSmokeReadOnly(t *testing.T) {
	endpoint := os.Getenv("COSMOS_RPC_URL")
	if endpoint == "" {
		t.Skip("set COSMOS_RPC_URL to run the read-only CometBFT smoke test")
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
		t.Fatalf("CometBFT node is still catching up: %+v", readiness)
	}
	if err = readiness.ValidateChainID(os.Getenv("COSMOS_EXPECTED_CHAIN_ID")); err != nil {
		t.Fatal(err)
	}
	t.Logf("CometBFT chain_id=%s height=%d version=%s", readiness.ChainID, readiness.LatestBlockHeight, readiness.Version)
}

func TestSmokeBroadcastSignedTransaction(t *testing.T) {
	endpoint := os.Getenv("COSMOS_RPC_URL")
	encodedTransaction := os.Getenv("COSMOS_SIGNED_TX_BASE64")
	if endpoint == "" || encodedTransaction == "" {
		t.Skip("set COSMOS_RPC_URL and COSMOS_SIGNED_TX_BASE64 to opt in to broadcast")
	}
	txBytes, err := base64.StdEncoding.DecodeString(encodedTransaction)
	if err != nil {
		t.Fatalf("decode COSMOS_SIGNED_TX_BASE64: %v", err)
	}
	adapter, err := NewRPCAdapter(endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := adapter.BroadcastSync(ctx, txBytes)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome == TxRejected {
		t.Fatalf("CheckTx rejected transaction: code=%d codespace=%s log=%s", result.CheckTxCode, result.Codespace, result.Log)
	}
	t.Logf("CheckTx accepted hash %s; transaction is still pending inclusion", result.Hash)
}
