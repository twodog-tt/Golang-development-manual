package solanatx

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

	solana "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type solanaCompatibilityFixture struct {
	FixtureKind     string                     `json:"fixture_kind"`
	NodeVersion     string                     `json:"node_version"`
	GenesisHash     string                     `json:"genesis_hash"`
	ExpectedOutcome Outcome                    `json:"expected_outcome"`
	Responses       map[string]json.RawMessage `json:"responses"`
}

func TestSolanaCompatibilityFixturesNAndNMinusOne(t *testing.T) {
	for _, fixtureName := range []string{"n_minus_1.json", "n.json"} {
		t.Run(fixtureName, func(t *testing.T) {
			fixture := loadSolanaCompatibilityFixture(t, fixtureName)
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
			if !readiness.Ready {
				t.Fatalf("readiness = %+v", readiness)
			}
			if err = readiness.ValidateGenesisHash(fixture.GenesisHash); err != nil {
				t.Fatal(err)
			}

			signature := solanaCompatibilitySignature()
			status, err := adapter.TransactionStatus(context.Background(), signature, StatusOptions{
				RequiredCommitment: rpc.ConfirmationStatusConfirmed,
			})
			if err != nil {
				t.Fatal(err)
			}
			if status.Outcome != fixture.ExpectedOutcome {
				t.Fatalf("outcome = %s, want %s", status.Outcome, fixture.ExpectedOutcome)
			}
		})
	}
}

func TestSolanaIncompatibleSchemaIsRejected(t *testing.T) {
	fixture := loadSolanaCompatibilityFixture(t, "incompatible.json")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(fixture.Responses["getSignatureStatuses"])
	}))
	defer server.Close()

	adapter, err := NewRPCAdapter(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	status, err := adapter.TransactionStatus(context.Background(), solanaCompatibilitySignature(), StatusOptions{})
	if err == nil {
		t.Fatal("breaking confirmationStatus type was accepted")
	}
	if status.Outcome != "" {
		t.Fatalf("schema error was classified as transaction state %q", status.Outcome)
	}
}

func TestLocalnetSolanaCompatibilityGate(t *testing.T) {
	endpoint, expectedGenesis, client := solanaLocalnetConfiguration(t)
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
	if err = readiness.ValidateGenesisHash(expectedGenesis); err != nil {
		t.Fatal(err)
	}
	status, err := adapter.TransactionStatus(context.Background(), solanaCompatibilitySignature(), StatusOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if status.Outcome != OutcomeUnknown {
		t.Fatalf("unseen localnet signature = %s, want UNKNOWN", status.Outcome)
	}
}

func loadSolanaCompatibilityFixture(t *testing.T, name string) solanaCompatibilityFixture {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve compatibility fixture path")
	}
	payload, err := os.ReadFile(filepath.Join(filepath.Dir(currentFile), "..", "localnet", "fixtures", "solana", name))
	if err != nil {
		t.Fatal(err)
	}
	var fixture solanaCompatibilityFixture
	if err = json.Unmarshal(payload, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func solanaCompatibilitySignature() solana.Signature {
	var signature solana.Signature
	for index := range signature {
		signature[index] = 0xa1
	}
	return signature
}

func solanaLocalnetConfiguration(t *testing.T) (string, string, *http.Client) {
	t.Helper()
	if os.Getenv("NON_EVM_LOCALNET") != "1" {
		t.Skip("real localnet test is opt-in")
	}
	endpoint := os.Getenv("SOLANA_LOCALNET_RPC")
	expectedGenesis := os.Getenv("SOLANA_LOCALNET_EXPECTED_GENESIS")
	if endpoint == "" || expectedGenesis == "" {
		t.Fatal("SOLANA_LOCALNET_RPC and SOLANA_LOCALNET_EXPECTED_GENESIS are required")
	}
	timeout := 5 * time.Second
	if raw := os.Getenv("NON_EVM_LOCALNET_HTTP_TIMEOUT_MS"); raw != "" {
		milliseconds, err := strconv.Atoi(raw)
		if err != nil || milliseconds <= 0 {
			t.Fatalf("invalid NON_EVM_LOCALNET_HTTP_TIMEOUT_MS %q", raw)
		}
		timeout = time.Duration(milliseconds) * time.Millisecond
	}
	return endpoint, expectedGenesis, &http.Client{Timeout: timeout}
}
