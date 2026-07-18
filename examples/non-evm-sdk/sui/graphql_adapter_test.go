package suiadapter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

type capturedGraphQLRequest struct {
	OperationName string                     `json:"operationName"`
	Query         string                     `json:"query"`
	Variables     map[string]json.RawMessage `json:"variables"`
}

func TestGraphQLReadinessContractAndChainIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/graphql" {
			t.Errorf("request = %s %s, want POST /graphql", request.Method, request.URL.Path)
		}
		if request.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", request.Header.Get("Content-Type"))
		}
		body := decodeCapturedGraphQLRequest(t, request)
		if body.OperationName != "Readiness" || len(body.Variables) != 0 {
			t.Errorf("request = %+v", body)
		}
		if !strings.Contains(body.Query, "chainIdentifier") || !strings.Contains(body.Query, "checkpoint") || !strings.Contains(body.Query, "sequenceNumber") {
			t.Errorf("query = %s", body.Query)
		}
		writeGraphQLData(writer, `{"chainIdentifier":"sui:testnet","checkpoint":{"sequenceNumber":123,"digest":"checkpoint-digest"}}`)
	}))
	defer server.Close()

	adapter, err := NewGraphQLAdapter(server.URL+"/graphql", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	readiness, err := adapter.Readiness(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !readiness.Ready || readiness.ChainIdentifier != "sui:testnet" || readiness.CheckpointSequenceNumber != 123 {
		t.Fatalf("readiness = %+v", readiness)
	}
	if err = readiness.ValidateChainIdentifier("sui:testnet"); err != nil {
		t.Fatal(err)
	}
	if err = readiness.ValidateChainIdentifier("sui:mainnet"); err == nil {
		t.Fatal("expected chain identifier mismatch")
	}
	support := adapter.EndpointSupport()
	if !support.ExecuteTransaction || support.DeprecatedJSONRPC || support.Transport != "GraphQL" {
		t.Fatalf("support = %+v", support)
	}
}

func TestGraphQLQueryTransactionContractAndOutcomes(t *testing.T) {
	digest := "11111111111111111111111111111111"
	tests := []struct {
		name           string
		transaction    string
		wantOutcome    TxOutcome
		wantFound      bool
		wantExecution  string
		wantCheckpoint *uint64
	}{
		{
			name:        "not indexed is unknown",
			transaction: `null`,
			wantOutcome: TxUnknown,
		},
		{
			name:           "success",
			transaction:    `{"digest":"` + digest + `","effects":{"status":"SUCCESS","executionError":null,"transaction":{"digest":"` + digest + `"},"checkpoint":{"sequenceNumber":"55"}}}`,
			wantOutcome:    TxSucceeded,
			wantFound:      true,
			wantCheckpoint: suiUint64Pointer(55),
		},
		{
			name:           "execution failure",
			transaction:    `{"digest":"` + digest + `","effects":{"status":"FAILURE","executionError":{"message":"MoveAbort(MoveLocation...)"},"transaction":{"digest":"` + digest + `"},"checkpoint":{"sequenceNumber":56}}}`,
			wantOutcome:    TxFailed,
			wantFound:      true,
			wantExecution:  "MoveAbort(MoveLocation...)",
			wantCheckpoint: suiUint64Pointer(56),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				body := decodeCapturedGraphQLRequest(t, request)
				if body.OperationName != "Transaction" {
					t.Errorf("operation = %q", body.OperationName)
				}
				var gotDigest string
				if err := json.Unmarshal(body.Variables["digest"], &gotDigest); err != nil || gotDigest != digest {
					t.Errorf("digest variable = %q err=%v", gotDigest, err)
				}
				if !strings.Contains(body.Query, "transaction(digest: $digest)") || !strings.Contains(body.Query, "executionError { message }") {
					t.Errorf("query = %s", body.Query)
				}
				writeGraphQLData(writer, `{"transaction":`+test.transaction+`}`)
			}))
			defer server.Close()
			adapter, err := NewGraphQLAdapter(server.URL, server.Client())
			if err != nil {
				t.Fatal(err)
			}
			status, err := adapter.QueryTransaction(context.Background(), digest)
			if err != nil {
				t.Fatal(err)
			}
			if status.Outcome != test.wantOutcome || status.Found != test.wantFound || status.ExecutionError != test.wantExecution || !reflect.DeepEqual(status.CheckpointSequenceNumber, test.wantCheckpoint) {
				t.Fatalf("status = %+v", status)
			}
		})
	}
}

func TestGraphQLExecuteTransactionContractAndDigestSemantics(t *testing.T) {
	transactionData := base64.StdEncoding.EncodeToString([]byte{1, 2, 3})
	signatures := []string{
		base64.StdEncoding.EncodeToString([]byte{4, 5}),
		base64.StdEncoding.EncodeToString([]byte{6, 7}),
	}
	transactionDigest := "tx-digest-from-effects-transaction"

	tests := []struct {
		name          string
		effects       string
		wantOutcome   TxOutcome
		wantExecution string
	}{
		{
			name:        "success",
			effects:     `{"status":"SUCCESS","executionError":null,"transaction":{"digest":"` + transactionDigest + `"},"checkpoint":{"sequenceNumber":77}}`,
			wantOutcome: TxSucceeded,
		},
		{
			name:          "on-chain failure",
			effects:       `{"status":"FAILURE","executionError":{"message":"InsufficientGas"},"transaction":{"digest":"` + transactionDigest + `"},"checkpoint":{"sequenceNumber":"78"}}`,
			wantOutcome:   TxFailed,
			wantExecution: "InsufficientGas",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodPost || request.URL.Path != "/graphql" {
					t.Errorf("request = %s %s", request.Method, request.URL.Path)
				}
				body := decodeCapturedGraphQLRequest(t, request)
				if body.OperationName != "ExecuteTransaction" {
					t.Errorf("operation = %q", body.OperationName)
				}
				if !strings.Contains(body.Query, "$transactionDataBcs: Base64!") ||
					!strings.Contains(body.Query, "$signatures: [Base64!]!") ||
					!strings.Contains(body.Query, "executionError { message }") ||
					!strings.Contains(body.Query, "transaction { digest }") {
					t.Errorf("mutation = %s", body.Query)
				}
				if strings.Contains(body.Query, "effectsDigest") || strings.Contains(body.Query, "effects {\n      digest") || strings.Contains(body.Query, "\n    errors") {
					t.Errorf("mutation requests deprecated/wrong fields: %s", body.Query)
				}
				var gotTransactionData string
				var gotSignatures []string
				_ = json.Unmarshal(body.Variables["transactionDataBcs"], &gotTransactionData)
				_ = json.Unmarshal(body.Variables["signatures"], &gotSignatures)
				if gotTransactionData != transactionData || !reflect.DeepEqual(gotSignatures, signatures) {
					t.Errorf("variables = %+v", body.Variables)
				}
				writeGraphQLData(writer, `{"executeTransaction":{"effects":`+test.effects+`}}`)
			}))
			defer server.Close()
			adapter, err := NewGraphQLAdapter(server.URL+"/graphql", server.Client())
			if err != nil {
				t.Fatal(err)
			}
			paddedSignatures := []string{" \n" + signatures[0] + "\t", signatures[1] + "  "}
			status, err := adapter.Submit(context.Background(), " \t"+transactionData+"\n", paddedSignatures)
			if err != nil {
				t.Fatal(err)
			}
			if status.Digest != transactionDigest || status.Outcome != test.wantOutcome || status.ExecutionError != test.wantExecution {
				t.Fatalf("status = %+v", status)
			}
		})
	}
}

func TestGraphQLTopLevelAndHTTPErrors(t *testing.T) {
	t.Run("GraphQL errors", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"data":{"executeTransaction":null},"errors":[{"message":"Invalid user signature","path":["executeTransaction"],"extensions":{"code":"BAD_USER_INPUT"}}]}`))
		}))
		defer server.Close()
		adapter, err := NewGraphQLAdapter(server.URL, server.Client())
		if err != nil {
			t.Fatal(err)
		}
		_, err = adapter.Submit(
			context.Background(),
			base64.StdEncoding.EncodeToString([]byte{1}),
			[]string{base64.StdEncoding.EncodeToString([]byte{2})},
		)
		var graphQLErrors *GraphQLErrors
		if !errors.As(err, &graphQLErrors) || len(graphQLErrors.Errors) != 1 || graphQLErrors.Errors[0].Message != "Invalid user signature" {
			t.Fatalf("error = %T %v", err, err)
		}
	})

	t.Run("HTTP error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			http.Error(writer, "rate limited", http.StatusTooManyRequests)
		}))
		defer server.Close()
		adapter, err := NewGraphQLAdapter(server.URL, server.Client())
		if err != nil {
			t.Fatal(err)
		}
		_, err = adapter.Readiness(context.Background())
		var httpErr *HTTPError
		if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusTooManyRequests {
			t.Fatalf("error = %T %v", err, err)
		}
	})
}

func decodeCapturedGraphQLRequest(t *testing.T, request *http.Request) capturedGraphQLRequest {
	t.Helper()
	defer request.Body.Close()
	var body capturedGraphQLRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		t.Errorf("decode request: %v", err)
	}
	return body
}

func writeGraphQLData(writer http.ResponseWriter, data string) {
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write([]byte(`{"data":` + data + `}`))
}

func suiUint64Pointer(value uint64) *uint64 {
	return &value
}
