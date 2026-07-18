package suiadapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

type suiCompatibilityFixture struct {
	FixtureKind     string                     `json:"fixture_kind"`
	NodeVersion     string                     `json:"node_version"`
	ChainIdentifier string                     `json:"chain_identifier"`
	Digest          string                     `json:"digest"`
	ExpectedOutcome TxOutcome                  `json:"expected_outcome"`
	Responses       map[string]json.RawMessage `json:"responses"`
}

func TestSuiCompatibilityFixturesNAndNMinusOne(t *testing.T) {
	for _, fixtureName := range []string{"n_minus_1.json", "n.json"} {
		t.Run(fixtureName, func(t *testing.T) {
			fixture := loadSuiCompatibilityFixture(t, fixtureName)
			if fixture.FixtureKind != "minimized-graphql-contract" || fixture.NodeVersion == "" {
				t.Fatalf("fixture metadata = %+v", fixture)
			}

			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				var body struct {
					OperationName string `json:"operationName"`
					Query         string `json:"query"`
				}
				if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
					t.Errorf("decode request: %v", err)
					http.Error(writer, "bad request", http.StatusBadRequest)
					return
				}
				if strings.Contains(body.Query, "sui_get") || strings.Contains(body.Query, `"jsonrpc"`) {
					t.Errorf("deprecated JSON-RPC appeared in GraphQL request: %s", body.Query)
				}
				response, ok := fixture.Responses[body.OperationName]
				if !ok {
					t.Errorf("fixture %s has no response for %s", fixtureName, body.OperationName)
					http.Error(writer, "missing fixture response", http.StatusInternalServerError)
					return
				}
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write(response)
			}))
			defer server.Close()

			adapter, err := NewGraphQLAdapter(server.URL, server.Client())
			if err != nil {
				t.Fatal(err)
			}
			support := adapter.EndpointSupport()
			if support.Transport != "GraphQL" || support.DeprecatedJSONRPC || !support.QueryTransaction {
				t.Fatalf("transport boundary = %+v", support)
			}
			readiness, err := adapter.Readiness(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if !readiness.Ready {
				t.Fatalf("readiness = %+v", readiness)
			}
			if err = readiness.ValidateChainIdentifier(fixture.ChainIdentifier); err != nil {
				t.Fatal(err)
			}

			status, err := adapter.QueryTransaction(context.Background(), fixture.Digest)
			if err != nil {
				t.Fatal(err)
			}
			if status.Outcome != fixture.ExpectedOutcome {
				t.Fatalf("outcome = %s, want %s", status.Outcome, fixture.ExpectedOutcome)
			}
		})
	}
}

func TestSuiIncompatibleSchemaIsRejected(t *testing.T) {
	fixture := loadSuiCompatibilityFixture(t, "incompatible.json")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(fixture.Responses["Transaction"])
	}))
	defer server.Close()

	adapter, err := NewGraphQLAdapter(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	status, err := adapter.QueryTransaction(context.Background(), fixture.Digest)
	if err == nil {
		t.Fatal("unknown execution status enum was accepted")
	}
	if status.Outcome != "" {
		t.Fatalf("schema error was classified as transaction state %q", status.Outcome)
	}
}

func TestLocalnetSuiCompatibilityGate(t *testing.T) {
	endpoint, expectedChainIdentifier, client := suiLocalnetConfiguration(t)
	adapter, err := NewGraphQLAdapter(endpoint, client)
	if err != nil {
		t.Fatal(err)
	}
	support := adapter.EndpointSupport()
	if support.Transport != "GraphQL" || support.DeprecatedJSONRPC {
		t.Fatalf("localnet transport boundary = %+v", support)
	}
	readiness, err := adapter.Readiness(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !readiness.Ready {
		t.Fatalf("localnet is not ready: %+v", readiness)
	}
	if err = readiness.ValidateChainIdentifier(expectedChainIdentifier); err != nil {
		t.Fatal(err)
	}
	status, err := adapter.QueryTransaction(context.Background(), suiCompatibilityUnknownDigest())
	if err != nil {
		t.Fatal(err)
	}
	if status.Outcome != TxUnknown || status.Found {
		t.Fatalf("unseen localnet transaction = %+v, want UNKNOWN", status)
	}
}

func loadSuiCompatibilityFixture(t *testing.T, name string) suiCompatibilityFixture {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve compatibility fixture path")
	}
	payload, err := os.ReadFile(filepath.Join(filepath.Dir(currentFile), "..", "localnet", "fixtures", "sui", name))
	if err != nil {
		t.Fatal(err)
	}
	var fixture suiCompatibilityFixture
	if err = json.Unmarshal(payload, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func suiCompatibilityUnknownDigest() string {
	return "11111111111111111111111111111111"
}

func suiLocalnetConfiguration(t *testing.T) (string, string, *http.Client) {
	t.Helper()
	if os.Getenv("NON_EVM_LOCALNET") != "1" {
		t.Skip("real localnet test is opt-in")
	}
	endpoint := os.Getenv("SUI_LOCALNET_GRAPHQL")
	expectedChainIdentifier := os.Getenv("SUI_LOCALNET_EXPECTED_CHAIN_IDENTIFIER")
	if endpoint == "" || expectedChainIdentifier == "" {
		t.Fatal("SUI_LOCALNET_GRAPHQL and SUI_LOCALNET_EXPECTED_CHAIN_IDENTIFIER are required")
	}
	timeout := 5 * time.Second
	if raw := os.Getenv("NON_EVM_LOCALNET_HTTP_TIMEOUT_MS"); raw != "" {
		milliseconds, err := strconv.Atoi(raw)
		if err != nil || milliseconds <= 0 {
			t.Fatalf("invalid NON_EVM_LOCALNET_HTTP_TIMEOUT_MS %q", raw)
		}
		timeout = time.Duration(milliseconds) * time.Millisecond
	}
	return endpoint, expectedChainIdentifier, &http.Client{Timeout: timeout}
}
