package cosmostx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"
)

type cosmosCompatibilityFixture struct {
	FixtureKind     string                     `json:"fixture_kind"`
	NodeVersion     string                     `json:"node_version"`
	ReportedVersion string                     `json:"reported_version"`
	ChainID         string                     `json:"chain_id"`
	Hash            string                     `json:"hash"`
	ExpectedOutcome TxOutcome                  `json:"expected_outcome"`
	Responses       map[string]json.RawMessage `json:"responses"`
}

func TestCosmosCompatibilityFixturesNAndNMinusOne(t *testing.T) {
	for _, fixtureName := range []string{"n_minus_1.json", "n.json"} {
		t.Run(fixtureName, func(t *testing.T) {
			fixture := loadCosmosCompatibilityFixture(t, fixtureName)
			if fixture.FixtureKind != "minimized-rpc-contract" || fixture.NodeVersion == "" {
				t.Fatalf("fixture metadata = %+v", fixture)
			}

			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				var body struct {
					Method string `json:"method"`
				}
				if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
					t.Errorf("decode request: %v", err)
					http.Error(writer, "bad request", http.StatusBadRequest)
					return
				}
				response, ok := fixture.Responses[body.Method]
				if !ok {
					t.Errorf("fixture %s has no response for %s", fixtureName, body.Method)
					http.Error(writer, "missing fixture response", http.StatusInternalServerError)
					return
				}
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write(response)
			}))
			defer server.Close()

			adapter, err := NewRPCAdapter(server.URL, server.Client())
			if err != nil {
				t.Fatal(err)
			}
			readiness, err := adapter.Readiness(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if !readiness.Ready || readiness.Version != fixture.ReportedVersion {
				t.Fatalf("readiness = %+v", readiness)
			}
			if err = readiness.ValidateChainID(fixture.ChainID); err != nil {
				t.Fatal(err)
			}

			status, err := adapter.QueryTransaction(context.Background(), fixture.Hash)
			if err != nil {
				t.Fatal(err)
			}
			if status.Outcome != fixture.ExpectedOutcome {
				t.Fatalf("outcome = %s, want %s", status.Outcome, fixture.ExpectedOutcome)
			}
		})
	}
}

func TestCosmosIncompatibleSchemaIsRejected(t *testing.T) {
	fixture := loadCosmosCompatibilityFixture(t, "incompatible.json")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(fixture.Responses["status"])
	}))
	defer server.Close()

	adapter, err := NewRPCAdapter(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	readiness, err := adapter.Readiness(context.Background())
	if err == nil {
		t.Fatal("breaking latest_block_height type was accepted")
	}
	if readiness.Ready || readiness.ChainID != "" {
		t.Fatalf("schema error returned partial readiness %+v", readiness)
	}
}

func TestLocalnetCosmosCompatibilityGate(t *testing.T) {
	endpoint, expectedChainID, client := cosmosLocalnetConfiguration(t)
	adapter, err := NewRPCAdapter(endpoint, client)
	if err != nil {
		t.Fatal(err)
	}
	readiness, err := adapter.Readiness(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !readiness.Ready {
		t.Fatalf("localnet is not ready: %+v", readiness)
	}
	if err = readiness.ValidateChainID(expectedChainID); err != nil {
		t.Fatal(err)
	}
	status, err := adapter.QueryTransaction(context.Background(), cosmosCompatibilityUnknownHash())
	if err != nil {
		t.Fatal(err)
	}
	if status.Outcome != TxUnknown || status.Found {
		t.Fatalf("unseen localnet transaction = %+v, want UNKNOWN", status)
	}
}

func loadCosmosCompatibilityFixture(t *testing.T, name string) cosmosCompatibilityFixture {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve compatibility fixture path")
	}
	payload, err := os.ReadFile(filepath.Join(filepath.Dir(currentFile), "..", "localnet", "fixtures", "cosmos", name))
	if err != nil {
		t.Fatal(err)
	}
	var fixture cosmosCompatibilityFixture
	if err = json.Unmarshal(payload, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func cosmosCompatibilityUnknownHash() string {
	return "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC"
}

func cosmosLocalnetConfiguration(t *testing.T) (string, string, *http.Client) {
	t.Helper()
	if os.Getenv("NON_EVM_LOCALNET") != "1" {
		t.Skip("real localnet test is opt-in")
	}
	endpoint := os.Getenv("COSMOS_LOCALNET_RPC")
	expectedChainID := os.Getenv("COSMOS_LOCALNET_EXPECTED_CHAIN_ID")
	if endpoint == "" || expectedChainID == "" {
		t.Fatal("COSMOS_LOCALNET_RPC and COSMOS_LOCALNET_EXPECTED_CHAIN_ID are required")
	}
	timeout := 5 * time.Second
	if raw := os.Getenv("NON_EVM_LOCALNET_HTTP_TIMEOUT_MS"); raw != "" {
		milliseconds, err := strconv.Atoi(raw)
		if err != nil || milliseconds <= 0 {
			t.Fatalf("invalid NON_EVM_LOCALNET_HTTP_TIMEOUT_MS %q", raw)
		}
		timeout = time.Duration(milliseconds) * time.Millisecond
	}
	return endpoint, expectedChainID, &http.Client{Timeout: timeout}
}
