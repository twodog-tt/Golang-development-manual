package aptostx

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

type aptosCompatibilityFixture struct {
	FixtureKind     string          `json:"fixture_kind"`
	NodeVersion     string          `json:"node_version"`
	ChainID         string          `json:"chain_id"`
	Hash            string          `json:"hash"`
	ExpectedOutcome TxOutcome       `json:"expected_outcome"`
	Readiness       json.RawMessage `json:"readiness"`
	Transaction     json.RawMessage `json:"transaction"`
}

func TestAptosCompatibilityFixturesNAndNMinusOne(t *testing.T) {
	for _, fixtureName := range []string{"n_minus_1.json", "n.json"} {
		t.Run(fixtureName, func(t *testing.T) {
			fixture := loadAptosCompatibilityFixture(t, fixtureName)
			if fixture.FixtureKind != "minimized-rest-contract" || fixture.NodeVersion == "" {
				t.Fatalf("fixture metadata = %+v", fixture)
			}

			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Set("Content-Type", "application/json")
				switch request.URL.Path {
				case "/v1":
					_, _ = writer.Write(fixture.Readiness)
				case "/v1/transactions/by_hash/" + fixture.Hash:
					_, _ = writer.Write(fixture.Transaction)
				default:
					t.Errorf("unexpected path %s", request.URL.Path)
					http.Error(writer, "missing fixture response", http.StatusInternalServerError)
				}
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
			if !readiness.Ready {
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

func TestAptosIncompatibleSchemaIsRejected(t *testing.T) {
	fixture := loadAptosCompatibilityFixture(t, "incompatible.json")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(fixture.Transaction)
	}))
	defer server.Close()

	adapter, err := NewRESTAdapter(server.URL+"/v1", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	status, err := adapter.QueryTransaction(context.Background(), fixture.Hash)
	if err == nil {
		t.Fatal("breaking success field type was accepted")
	}
	if status.Outcome != "" {
		t.Fatalf("schema error was classified as transaction state %q", status.Outcome)
	}
}

func TestLocalnetAptosCompatibilityGate(t *testing.T) {
	endpoint, expectedChainID, client := aptosLocalnetConfiguration(t)
	adapter, err := NewRESTAdapter(endpoint, client)
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
	status, err := adapter.QueryTransaction(context.Background(), aptosCompatibilityUnknownHash())
	if err != nil {
		t.Fatal(err)
	}
	if status.Outcome != TxUnknown {
		t.Fatalf("unseen localnet transaction = %+v, want UNKNOWN", status)
	}
}

func loadAptosCompatibilityFixture(t *testing.T, name string) aptosCompatibilityFixture {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve compatibility fixture path")
	}
	payload, err := os.ReadFile(filepath.Join(filepath.Dir(currentFile), "..", "localnet", "fixtures", "aptos", name))
	if err != nil {
		t.Fatal(err)
	}
	var fixture aptosCompatibilityFixture
	if err = json.Unmarshal(payload, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func aptosCompatibilityUnknownHash() string {
	return "0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
}

func aptosLocalnetConfiguration(t *testing.T) (string, string, *http.Client) {
	t.Helper()
	if os.Getenv("NON_EVM_LOCALNET") != "1" {
		t.Skip("real localnet test is opt-in")
	}
	endpoint := os.Getenv("APTOS_LOCALNET_REST")
	expectedChainID := os.Getenv("APTOS_LOCALNET_EXPECTED_CHAIN_ID")
	if endpoint == "" || expectedChainID == "" {
		t.Fatal("APTOS_LOCALNET_REST and APTOS_LOCALNET_EXPECTED_CHAIN_ID are required")
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
