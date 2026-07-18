package suiadapter

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestSmokeReadOnly(t *testing.T) {
	endpoint := os.Getenv("SUI_GRAPHQL_URL")
	if endpoint == "" {
		t.Skip("set SUI_GRAPHQL_URL to run the read-only Sui GraphQL smoke test")
	}
	adapter, err := NewGraphQLAdapter(endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	readiness, err := adapter.Readiness(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !readiness.Ready {
		t.Fatalf("Sui GraphQL endpoint has no indexed checkpoint: %+v", readiness)
	}
	if err = readiness.ValidateChainIdentifier(os.Getenv("SUI_EXPECTED_CHAIN_IDENTIFIER")); err != nil {
		t.Fatal(err)
	}
	t.Logf("Sui chain_identifier=%s checkpoint=%d", readiness.ChainIdentifier, readiness.CheckpointSequenceNumber)
}

func TestSmokeBroadcastSignedTransaction(t *testing.T) {
	endpoint := os.Getenv("SUI_GRAPHQL_URL")
	encodedInput := os.Getenv("SUI_SIGNED_TRANSACTION_JSON")
	if endpoint == "" || encodedInput == "" {
		t.Skip("set SUI_GRAPHQL_URL and SUI_SIGNED_TRANSACTION_JSON to opt in to execution")
	}
	var input struct {
		TransactionDataBCS string   `json:"transactionDataBcs"`
		Signatures         []string `json:"signatures"`
	}
	if err := json.Unmarshal([]byte(encodedInput), &input); err != nil {
		t.Fatalf("decode SUI_SIGNED_TRANSACTION_JSON: %v", err)
	}
	adapter, err := NewGraphQLAdapter(endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	status, err := adapter.Submit(ctx, input.TransactionDataBCS, input.Signatures)
	if err != nil {
		t.Fatal(err)
	}
	if status.Outcome == TxFailed {
		t.Fatalf("Sui execution failed: digest=%s error=%s", status.Digest, status.ExecutionError)
	}
	t.Logf("Sui transaction executed: digest=%s checkpoint=%v", status.Digest, status.CheckpointSequenceNumber)
}
