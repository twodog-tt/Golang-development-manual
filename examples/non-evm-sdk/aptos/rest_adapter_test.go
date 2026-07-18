package aptostx

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	aptos "github.com/aptos-labs/aptos-go-sdk"
)

func TestRESTAdapterReadinessContractAndChainIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1" {
			t.Errorf("request = %s %s, want GET /v1", request.Method, request.URL.Path)
		}
		if request.Header.Get("Accept") != "application/json" {
			t.Errorf("Accept = %q", request.Header.Get("Accept"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"chain_id":2,
			"epoch":"77",
			"ledger_version":"12345",
			"oldest_ledger_version":"100",
			"ledger_timestamp":"1700000000000000",
			"block_height":"456",
			"node_role":"full_node"
		}`))
	}))
	defer server.Close()

	adapter, err := NewRESTAdapter(server.URL+"/v1", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	readiness, err := adapter.Readiness(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !readiness.Ready || readiness.ChainID != 2 || readiness.LedgerVersion != 12345 || readiness.BlockHeight != 456 {
		t.Fatalf("readiness = %+v", readiness)
	}
	if err = readiness.ValidateChainID("2"); err != nil {
		t.Fatal(err)
	}
	if err = readiness.ValidateChainID("1"); err == nil {
		t.Fatal("expected chain ID mismatch")
	}
}

func TestRESTAdapterSubmitContract(t *testing.T) {
	signed := aptosTestSignedTransfer(t)
	signedBCS := signed.BCS
	hash := signed.Hash
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v1/transactions" {
			t.Errorf("request = %s %s, want POST /v1/transactions", request.Method, request.URL.Path)
		}
		if got := request.Header.Get("Content-Type"); got != aptosSignedTransactionBCS {
			t.Errorf("Content-Type = %q", got)
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		if !bytes.Equal(body, signedBCS) {
			t.Errorf("body = %x", body)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusAccepted)
		_, _ = writer.Write([]byte(`{"type":"pending_transaction","hash":"` + hash + `"}`))
	}))
	defer server.Close()
	adapter, err := NewRESTAdapter(server.URL+"/v1", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.Submit(context.Background(), signedBCS)
	if err != nil {
		t.Fatal(err)
	}
	if result.Hash != hash || result.Outcome != TxPending {
		t.Fatalf("result = %+v", result)
	}
}

func TestRESTAdapterQueryTransactionOutcomes(t *testing.T) {
	hash := aptosTestHash(0x42)
	tests := []struct {
		name        string
		response    string
		wantOutcome TxOutcome
		wantSuccess *bool
		wantVM      string
	}{
		{
			name:        "pending has no execution result",
			response:    `{"type":"pending_transaction","hash":"` + hash + `"}`,
			wantOutcome: TxPending,
		},
		{
			name:        "executed successfully",
			response:    `{"type":"user_transaction","hash":"` + hash + `","version":"99","success":true,"vm_status":"Executed successfully"}`,
			wantOutcome: TxSucceeded,
			wantSuccess: aptosBoolPointer(true),
			wantVM:      "Executed successfully",
		},
		{
			name:        "VM execution failed",
			response:    `{"type":"user_transaction","hash":"` + hash + `","version":"100","success":false,"vm_status":"Move abort in 0x1::coin"}`,
			wantOutcome: TxFailed,
			wantSuccess: aptosBoolPointer(false),
			wantVM:      "Move abort in 0x1::coin",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodGet || request.URL.Path != "/v1/transactions/by_hash/"+hash {
					t.Errorf("request = %s %s", request.Method, request.URL.Path)
				}
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(test.response))
			}))
			defer server.Close()
			adapter, err := NewRESTAdapter(server.URL+"/v1", server.Client())
			if err != nil {
				t.Fatal(err)
			}
			status, err := adapter.QueryTransaction(context.Background(), hash)
			if err != nil {
				t.Fatal(err)
			}
			if status.Outcome != test.wantOutcome || !equalOptionalBool(status.Success, test.wantSuccess) || status.VMStatus != test.wantVM {
				t.Fatalf("status = %+v", status)
			}
		})
	}
}

func TestRESTAdapterTransactionNotFoundIsUnknown(t *testing.T) {
	hash := aptosTestHash(0x53)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1/transactions/by_hash/"+hash {
			t.Errorf("request = %s %s", request.Method, request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusNotFound)
		_, _ = writer.Write([]byte(`{"message":"Transaction not found","error_code":"transaction_not_found"}`))
	}))
	defer server.Close()
	adapter, err := NewRESTAdapter(server.URL+"/v1", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	status, err := adapter.QueryTransaction(context.Background(), hash)
	if err != nil {
		t.Fatal(err)
	}
	if status.Hash != hash || status.Outcome != TxUnknown || status.Success != nil {
		t.Fatalf("status = %+v", status)
	}
}

func TestRESTAdapterErrorsAreStructured(t *testing.T) {
	signed := aptosTestSignedTransfer(t)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"message":"sequence number too old","error_code":"sequence_number_too_old","vm_error_code":"1001"}`))
	}))
	defer server.Close()
	adapter, err := NewRESTAdapter(server.URL+"/v1", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.Submit(context.Background(), signed.BCS)
	var restErr *RESTError
	if !errors.As(err, &restErr) || restErr.StatusCode != http.StatusBadRequest || restErr.ErrorCode != "sequence_number_too_old" || restErr.VMErrorCode != "1001" {
		t.Fatalf("error = %T %+v", err, err)
	}
}

func TestRESTAdapterRejectsMismatchedSubmitHash(t *testing.T) {
	signed := aptosTestSignedTransfer(t)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusAccepted)
		_, _ = writer.Write([]byte(`{"type":"pending_transaction","hash":"` + aptosTestHash(0x7f) + `"}`))
	}))
	defer server.Close()
	adapter, err := NewRESTAdapter(server.URL+"/v1", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.Submit(context.Background(), signed.BCS)
	if err == nil || !strings.Contains(err.Error(), "signed transaction hash") {
		t.Fatalf("hash mismatch error = %v", err)
	}
}

func aptosTestHash(value byte) string {
	return "0x" + hex.EncodeToString(bytes.Repeat([]byte{value}, 32))
}

func aptosBoolPointer(value bool) *bool {
	return &value
}

func equalOptionalBool(left, right *bool) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func aptosTestSignedTransfer(t *testing.T) SignedTransfer {
	t.Helper()
	var seed [32]byte
	copy(seed[:], bytes.Repeat([]byte{0x31}, len(seed)))
	signed, err := BuildSignedTransfer(TransferInput{
		PrivateKeySeed:    seed,
		Recipient:         aptos.AccountTwo,
		Amount:            1000,
		Sequence:          7,
		MaxGasAmount:      2000,
		GasUnitPrice:      100,
		ExpirationSeconds: 2_000_000_000,
		ChainID:           2,
	})
	if err != nil {
		t.Fatal(err)
	}
	return signed
}
