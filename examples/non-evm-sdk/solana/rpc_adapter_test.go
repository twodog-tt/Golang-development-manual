package solanatx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	solana "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type capturedRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

func TestRPCAdapterReadinessContractAndIdentity(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if request.Method != http.MethodPost || request.URL.Path != "/rpc" {
			t.Errorf("request = %s %s, want POST /rpc", request.Method, request.URL.Path)
		}
		if got := request.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}
		body := decodeCapturedRPCRequest(t, request)
		if body.JSONRPC != "2.0" || body.ID != 1 || len(body.Params) != 0 {
			t.Errorf("unexpected envelope: %+v params=%s", body, body.Params)
		}
		writer.Header().Set("Content-Type", "application/json")
		switch requests {
		case 1:
			if body.Method != "getHealth" {
				t.Errorf("method = %q", body.Method)
			}
			writeRPCResult(writer, `"ok"`)
		case 2:
			if body.Method != "getGenesisHash" {
				t.Errorf("method = %q", body.Method)
			}
			writeRPCResult(writer, `"genesis-testnet"`)
		default:
			t.Errorf("unexpected request %d", requests)
		}
	}))
	defer server.Close()

	adapter, err := NewRPCAdapter(server.URL+"/rpc", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	readiness, err := adapter.Readiness(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !readiness.Ready || readiness.Health != "ok" || readiness.GenesisHash != "genesis-testnet" {
		t.Fatalf("readiness = %+v", readiness)
	}
	if err = readiness.ValidateGenesisHash("genesis-testnet"); err != nil {
		t.Fatal(err)
	}
	if err = readiness.ValidateGenesisHash("mainnet-genesis"); err == nil {
		t.Fatal("expected genesis hash mismatch")
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
}

func TestRPCAdapterSubmitContract(t *testing.T) {
	signed := testSignedSolanaTransfer(t)
	encodedTransaction := signed.Base64
	expectedSignature := signed.Signature
	maxRetries := uint(3)
	minContextSlot := uint64(55)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/solana" {
			t.Errorf("request = %s %s", request.Method, request.URL.Path)
		}
		body := decodeCapturedRPCRequest(t, request)
		if body.Method != "sendTransaction" {
			t.Errorf("method = %q", body.Method)
		}
		var params []json.RawMessage
		if err := json.Unmarshal(body.Params, &params); err != nil {
			t.Errorf("decode params: %v", err)
		}
		if len(params) != 2 {
			t.Errorf("params = %s", body.Params)
		} else {
			var gotTransaction string
			if err := json.Unmarshal(params[0], &gotTransaction); err != nil {
				t.Errorf("decode transaction: %v", err)
			}
			if gotTransaction != encodedTransaction {
				t.Errorf("transaction = %q", gotTransaction)
			}
			var config map[string]any
			if err := json.Unmarshal(params[1], &config); err != nil {
				t.Errorf("decode config: %v", err)
			}
			want := map[string]any{
				"encoding":            "base64",
				"skipPreflight":       false,
				"preflightCommitment": "confirmed",
				"maxRetries":          float64(3),
				"minContextSlot":      float64(55),
			}
			if !reflect.DeepEqual(config, want) {
				t.Errorf("config = %#v, want %#v", config, want)
			}
		}
		writeRPCResult(writer, `"`+expectedSignature.String()+`"`)
	}))
	defer server.Close()

	adapter, err := NewRPCAdapter(server.URL+"/solana", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	signature, err := adapter.Submit(context.Background(), encodedTransaction, SubmitOptions{
		PreflightCommitment: rpc.CommitmentConfirmed,
		MaxRetries:          &maxRetries,
		MinContextSlot:      &minContextSlot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if signature != expectedSignature {
		t.Fatalf("signature = %s, want %s", signature, expectedSignature)
	}
}

func TestRPCAdapterTransactionStatusSemantics(t *testing.T) {
	var signature solana.Signature
	copy(signature[:], bytes.Repeat([]byte{0x52}, len(signature)))
	var recentBlockhash solana.Hash
	copy(recentBlockhash[:], bytes.Repeat([]byte{0x63}, len(recentBlockhash)))

	tests := []struct {
		name              string
		statusResult      string
		blockhashResult   string
		required          rpc.ConfirmationStatusType
		wantOutcome       Outcome
		wantBlockhash     *bool
		wantMethods       []string
		wantExecutionFail bool
		includeBlockhash  bool
	}{
		{
			name:         "unknown signature without blockhash evidence is unknown",
			statusResult: `{"context":{"slot":10},"value":[null]}`,
			wantOutcome:  OutcomeUnknown,
			wantMethods:  []string{"getSignatureStatuses"},
		},
		{
			name:             "unknown signature with valid blockhash is pending",
			statusResult:     `{"context":{"slot":10},"value":[null]}`,
			blockhashResult:  `{"context":{"slot":11},"value":true}`,
			wantOutcome:      OutcomePending,
			wantBlockhash:    boolPointer(true),
			wantMethods:      []string{"getSignatureStatuses", "isBlockhashValid"},
			includeBlockhash: true,
		},
		{
			name:             "unknown signature with expired blockhash is expired",
			statusResult:     `{"context":{"slot":10},"value":[null]}`,
			blockhashResult:  `{"context":{"slot":11},"value":false}`,
			wantOutcome:      OutcomeExpired,
			wantBlockhash:    boolPointer(false),
			wantMethods:      []string{"getSignatureStatuses", "isBlockhashValid"},
			includeBlockhash: true,
		},
		{
			name:         "processed does not satisfy confirmed",
			statusResult: `{"context":{"slot":10},"value":[{"slot":9,"confirmations":1,"err":null,"confirmationStatus":"processed"}]}`,
			wantOutcome:  OutcomePending,
			wantMethods:  []string{"getSignatureStatuses"},
		},
		{
			name:         "finalized satisfies confirmed",
			statusResult: `{"context":{"slot":10},"value":[{"slot":9,"confirmations":null,"err":null,"confirmationStatus":"finalized"}]}`,
			wantOutcome:  OutcomeSucceeded,
			wantMethods:  []string{"getSignatureStatuses"},
		},
		{
			name:              "processed execution error remains pending",
			statusResult:      `{"context":{"slot":10},"value":[{"slot":9,"confirmations":1,"err":{"InstructionError":[0,"Custom"]},"confirmationStatus":"processed"}]}`,
			wantOutcome:       OutcomePending,
			wantMethods:       []string{"getSignatureStatuses"},
			wantExecutionFail: true,
		},
		{
			name:              "confirmed execution error is terminal failure",
			statusResult:      `{"context":{"slot":10},"value":[{"slot":9,"confirmations":0,"err":{"InstructionError":[0,"Custom"]},"confirmationStatus":"confirmed"}]}`,
			wantOutcome:       OutcomeFailed,
			wantMethods:       []string{"getSignatureStatuses"},
			wantExecutionFail: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var methods []string
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				body := decodeCapturedRPCRequest(t, request)
				methods = append(methods, body.Method)
				switch body.Method {
				case "getSignatureStatuses":
					assertSignatureStatusParams(t, body.Params, signature.String())
					writeRPCResult(writer, test.statusResult)
				case "isBlockhashValid":
					assertBlockhashParams(t, body.Params, recentBlockhash.String())
					writeRPCResult(writer, test.blockhashResult)
				default:
					t.Errorf("unexpected method %q", body.Method)
				}
			}))
			defer server.Close()

			adapter, err := NewRPCAdapter(server.URL, server.Client())
			if err != nil {
				t.Fatal(err)
			}
			var statusOptions StatusOptions
			statusOptions.RequiredCommitment = test.required
			if test.includeBlockhash {
				statusOptions.RecentBlockhash = &recentBlockhash
			}
			status, err := adapter.TransactionStatus(context.Background(), signature, statusOptions)
			if err != nil {
				t.Fatal(err)
			}
			if status.Outcome != test.wantOutcome || !reflect.DeepEqual(status.BlockhashValid, test.wantBlockhash) {
				t.Fatalf("status = %+v, want outcome=%s blockhash=%v", status, test.wantOutcome, test.wantBlockhash)
			}
			if test.wantExecutionFail != (len(status.ExecutionError) != 0) {
				t.Fatalf("execution error = %s", status.ExecutionError)
			}
			if !reflect.DeepEqual(methods, test.wantMethods) {
				t.Fatalf("methods = %v, want %v", methods, test.wantMethods)
			}
		})
	}
}

func TestRPCAdapterErrorsArePreserved(t *testing.T) {
	t.Run("JSON-RPC error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32005,"message":"Node is unhealthy","data":{"numSlotsBehind":42}}}`))
		}))
		defer server.Close()
		adapter, err := NewRPCAdapter(server.URL, server.Client())
		if err != nil {
			t.Fatal(err)
		}
		_, err = adapter.Readiness(context.Background())
		var rpcErr *RPCError
		if !errors.As(err, &rpcErr) || rpcErr.Code != -32005 || !strings.Contains(string(rpcErr.Data), "numSlotsBehind") {
			t.Fatalf("error = %T %v", err, err)
		}
	})

	t.Run("HTTP error", func(t *testing.T) {
		signed := testSignedSolanaTransfer(t)
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			http.Error(writer, "provider unavailable", http.StatusServiceUnavailable)
		}))
		defer server.Close()
		adapter, err := NewRPCAdapter(server.URL, server.Client())
		if err != nil {
			t.Fatal(err)
		}
		_, err = adapter.Submit(context.Background(), signed.Base64, SubmitOptions{})
		var httpErr *HTTPError
		if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("error = %T %v", err, err)
		}
	})

	t.Run("returned signature must match signed bytes", func(t *testing.T) {
		signed := testSignedSolanaTransfer(t)
		var wrong solana.Signature
		copy(wrong[:], bytes.Repeat([]byte{0x7f}, len(wrong)))
		if wrong == signed.Signature {
			t.Fatal("test signature collision")
		}
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writeRPCResult(writer, `"`+wrong.String()+`"`)
		}))
		defer server.Close()
		adapter, err := NewRPCAdapter(server.URL, server.Client())
		if err != nil {
			t.Fatal(err)
		}
		_, err = adapter.Submit(context.Background(), signed.Base64, SubmitOptions{})
		if err == nil || !strings.Contains(err.Error(), "signed transaction ID") {
			t.Fatalf("signature mismatch error = %v", err)
		}
	})
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

func assertSignatureStatusParams(t *testing.T, raw json.RawMessage, signature string) {
	t.Helper()
	var params []json.RawMessage
	if err := json.Unmarshal(raw, &params); err != nil || len(params) != 2 {
		t.Errorf("status params = %s, err=%v", raw, err)
		return
	}
	var signatures []string
	var config map[string]bool
	if err := json.Unmarshal(params[0], &signatures); err != nil {
		t.Errorf("decode signatures: %v", err)
	}
	if err := json.Unmarshal(params[1], &config); err != nil {
		t.Errorf("decode status config: %v", err)
	}
	if !reflect.DeepEqual(signatures, []string{signature}) || !config["searchTransactionHistory"] {
		t.Errorf("status params = %s", raw)
	}
}

func assertBlockhashParams(t *testing.T, raw json.RawMessage, blockhash string) {
	t.Helper()
	var params []json.RawMessage
	if err := json.Unmarshal(raw, &params); err != nil || len(params) != 2 {
		t.Errorf("blockhash params = %s, err=%v", raw, err)
		return
	}
	var gotBlockhash string
	var config map[string]string
	_ = json.Unmarshal(params[0], &gotBlockhash)
	_ = json.Unmarshal(params[1], &config)
	if gotBlockhash != blockhash || config["commitment"] != "processed" {
		t.Errorf("blockhash params = %s", raw)
	}
}

func boolPointer(value bool) *bool {
	return &value
}

func testSignedSolanaTransfer(t *testing.T) SignedTransfer {
	t.Helper()
	var seed [32]byte
	copy(seed[:], bytes.Repeat([]byte{0x11}, len(seed)))
	recipient := solana.PublicKeyFromBytes(bytes.Repeat([]byte{0x22}, 32))
	blockhash := solana.HashFromBytes(bytes.Repeat([]byte{0x33}, 32))
	signed, err := BuildSignedTransfer(seed, recipient, 12345, blockhash)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}
