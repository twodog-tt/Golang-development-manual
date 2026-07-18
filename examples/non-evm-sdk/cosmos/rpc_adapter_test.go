package cosmostx

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type capturedRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

func TestRPCAdapterReadinessContractAndChainIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/comet" {
			t.Errorf("request = %s %s, want POST /comet", request.Method, request.URL.Path)
		}
		body := decodeCapturedRPCRequest(t, request)
		if body.JSONRPC != "2.0" || body.ID != 1 || body.Method != "status" || len(body.Params) != 0 {
			t.Errorf("request body = %+v params=%s", body, body.Params)
		}
		writeRPCResult(writer, `{
			"node_info":{"id":"node-1","network":"theta-testnet-001","version":"0.38.21"},
			"sync_info":{"latest_block_height":"12345","catching_up":false}
		}`)
	}))
	defer server.Close()

	adapter, err := NewRPCAdapter(server.URL+"/comet", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	readiness, err := adapter.Readiness(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !readiness.Ready || readiness.ChainID != "theta-testnet-001" || readiness.LatestBlockHeight != 12345 {
		t.Fatalf("readiness = %+v", readiness)
	}
	if err = readiness.ValidateChainID("theta-testnet-001"); err != nil {
		t.Fatal(err)
	}
	if err = readiness.ValidateChainID("provider"); err == nil {
		t.Fatal("expected chain ID mismatch")
	}
}

func TestBroadcastSyncContractAndCheckTxSemantics(t *testing.T) {
	txBytes := []byte{1, 2, 3, 4}
	hash := sha256.Sum256(txBytes)
	hashHex := stringsUpperHex(hash[:])

	tests := []struct {
		name        string
		code        string
		wantOutcome TxOutcome
		wantCode    uint32
	}{
		{name: "CheckTx accepted remains pending", code: "0", wantOutcome: TxPending},
		{name: "CheckTx rejection is not execution failure", code: "7", wantOutcome: TxRejected, wantCode: 7},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				body := decodeCapturedRPCRequest(t, request)
				if body.Method != "broadcast_tx_sync" {
					t.Errorf("method = %q", body.Method)
				}
				var params map[string]string
				if err := json.Unmarshal(body.Params, &params); err != nil {
					t.Errorf("decode params: %v", err)
				}
				if params["tx"] != base64.StdEncoding.EncodeToString(txBytes) {
					t.Errorf("params = %s", body.Params)
				}
				writeRPCResult(writer, `{"code":"`+test.code+`","codespace":"bank","log":"check log","hash":"`+hashHex+`"}`)
			}))
			defer server.Close()
			adapter, err := NewRPCAdapter(server.URL, server.Client())
			if err != nil {
				t.Fatal(err)
			}
			result, err := adapter.BroadcastSync(context.Background(), txBytes)
			if err != nil {
				t.Fatal(err)
			}
			if result.Outcome != test.wantOutcome || result.Hash != hashHex || result.CheckTxCode != test.wantCode {
				t.Fatalf("result = %+v", result)
			}
		})
	}
}

func TestBroadcastSyncRejectsMismatchedHash(t *testing.T) {
	txBytes := []byte{1, 2, 3, 4}
	wrongHash := stringsUpperHex(stringsOfByte(0x44, 32))
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeRPCResult(writer, `{"code":"0","hash":"`+wrongHash+`"}`)
	}))
	defer server.Close()
	adapter, err := NewRPCAdapter(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.BroadcastSync(context.Background(), txBytes)
	if err == nil || !strings.Contains(err.Error(), "signed transaction hash") {
		t.Fatalf("hash mismatch error = %v", err)
	}
}

func TestQueryTransactionContractAndOutcomes(t *testing.T) {
	hashBytes := stringsOfByte(0x7a, 32)
	hashHex := stringsUpperHex(hashBytes)

	tests := []struct {
		name        string
		response    string
		wantOutcome TxOutcome
		wantFound   bool
		wantCode    uint32
	}{
		{
			name:        "committed success",
			response:    `{"jsonrpc":"2.0","id":1,"result":{"hash":"` + hashHex + `","height":"88","index":2,"tx_result":{"code":0,"log":"ok"}}}`,
			wantOutcome: TxSucceeded,
			wantFound:   true,
		},
		{
			name:        "committed execution failure",
			response:    `{"jsonrpc":"2.0","id":1,"result":{"hash":"` + hashHex + `","height":"89","index":"3","tx_result":{"code":"12","codespace":"bank","log":"insufficient funds"}}}`,
			wantOutcome: TxFailed,
			wantFound:   true,
			wantCode:    12,
		},
		{
			name:        "not indexed yet",
			response:    `{"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"Internal error","data":"tx (` + hashHex + `) not found"}}`,
			wantOutcome: TxUnknown,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodPost || request.URL.Path != "/rpc" {
					t.Errorf("request = %s %s", request.Method, request.URL.Path)
				}
				body := decodeCapturedRPCRequest(t, request)
				if body.Method != "tx" {
					t.Errorf("method = %q", body.Method)
				}
				var params struct {
					Hash  string `json:"hash"`
					Prove bool   `json:"prove"`
				}
				if err := json.Unmarshal(body.Params, &params); err != nil {
					t.Errorf("decode params: %v", err)
				}
				if params.Hash != base64.StdEncoding.EncodeToString(hashBytes) || params.Prove {
					t.Errorf("params = %+v", params)
				}
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(test.response))
			}))
			defer server.Close()
			adapter, err := NewRPCAdapter(server.URL+"/rpc", server.Client())
			if err != nil {
				t.Fatal(err)
			}
			status, err := adapter.QueryTransaction(context.Background(), "0x"+stringsLowerHex(hashBytes))
			if err != nil {
				t.Fatal(err)
			}
			if status.Hash != hashHex || status.Outcome != test.wantOutcome || status.Found != test.wantFound || status.Code != test.wantCode {
				t.Fatalf("status = %+v", status)
			}
		})
	}
}

func TestQueryTransactionDoesNotHideIndexingErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"transaction indexing is disabled"}}`))
	}))
	defer server.Close()
	adapter, err := NewRPCAdapter(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.QueryTransaction(context.Background(), stringsUpperHex(stringsOfByte(1, 32)))
	var rpcErr *RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("error = %T %v", err, err)
	}
}

func TestRPCAdapterHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "upstream timeout", http.StatusGatewayTimeout)
	}))
	defer server.Close()
	adapter, err := NewRPCAdapter(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.Readiness(context.Background())
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusGatewayTimeout {
		t.Fatalf("error = %T %v", err, err)
	}
}

func decodeCapturedRPCRequest(t *testing.T, request *http.Request) capturedRPCRequest {
	t.Helper()
	defer request.Body.Close()
	var body capturedRPCRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		t.Errorf("decode request: %v", err)
	}
	return body
}

func writeRPCResult(writer http.ResponseWriter, result string) {
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":` + result + `}`))
}

func stringsOfByte(value byte, count int) []byte {
	return bytes.Repeat([]byte{value}, count)
}

func stringsUpperHex(value []byte) string {
	return strings.ToUpper(hex.EncodeToString(value))
}

func stringsLowerHex(value []byte) string {
	return hex.EncodeToString(value)
}
