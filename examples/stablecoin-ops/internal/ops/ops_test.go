package ops

import (
	"context"
	"encoding/json"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestChainMonitorBoundsHistoryWithContinueAsNew(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	var audits atomic.Int32
	env.RegisterActivityWithOptions(func(context.Context, string) (string, error) { audits.Add(1); return "settled", nil }, activity.RegisterOptions{Name: "AuditChain"})
	env.ExecuteWorkflow(ChainPaymentWorkflow, ChainInput{Input: Input{Payment: Payment{ID: "bounded-monitor"}}, Monitor: true})
	if !workflow.IsContinueAsNewError(env.GetWorkflowError()) || audits.Load() != 50 {
		t.Fatalf("unbounded audit history: count=%d error=%v", audits.Load(), env.GetWorkflowError())
	}
}

func TestAmountAndWebhookBoundaries(t *testing.T) {
	for _, v := range []string{"0", "-1", "1.1", "01", "1e6", "9223372036854775808"} {
		if _, err := Amount(v); err == nil {
			t.Fatalf("accepted %q", v)
		}
	}
	if n, err := Amount("9007199254740993"); err != nil || n != 9007199254740993 {
		t.Fatal("lost integer precision")
	}
	now := time.Unix(1000, 0)
	body := []byte(`{"amount_atomic":"1"}`)
	sig := Signature("secret", "1000", body)
	if !VerifyWebhook("secret", "1000", sig, body, now) {
		t.Fatal("valid signature rejected")
	}
	if VerifyWebhook("secret", "900", sig, body, now) || VerifyWebhook("secret", "1000", sig, []byte("changed"), now) {
		t.Fatal("accepted stale or tampered webhook")
	}
}
func TestEOASignaturesBindEveryMessageByte(t *testing.T) {
	key, _ := crypto.HexToECDSA(strings.Repeat("0", 63) + "1")
	address := crypto.PubkeyToAddress(key.PublicKey).Hex()
	msg := "localhost wants you to sign in\nnonce: 12345678"
	sig, _ := crypto.Sign(accounts.TextHash([]byte(msg)), key)
	sig[64] += 27
	if verifyEOA(address, msg, hexutil.Encode(sig)) != nil {
		t.Fatal("valid EIP-191 signature rejected")
	}
	for _, bad := range []string{msg + "x", strings.ReplaceAll(msg, "localhost", "evil.example"), strings.ReplaceAll(msg, "12345678", "87654321")} {
		if verifyEOA(address, bad, hexutil.Encode(sig)) == nil {
			t.Fatal("accepted altered signed scope")
		}
	}
	sig[64] = 255
	if verifyEOA(address, msg, hexutil.Encode(sig)) == nil {
		t.Fatal("accepted invalid recovery ID")
	}
}
func TestContractWalletPinnedRPCAndFailClosed(t *testing.T) {
	for _, mode := range []string{"ok", "wrong_chain", "wrong_magic", "rpc_error"} {
		t.Run(mode, func(t *testing.T) {
			called := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					ID     any               `json:"id"`
					Method string            `json:"method"`
					Params []json.RawMessage `json:"params"`
				}
				json.NewDecoder(r.Body).Decode(&req)
				result := any("0x7a69")
				switch req.Method {
				case "eth_chainId":
					if mode == "wrong_chain" {
						result = "0x1"
					}
				case "eth_blockNumber":
					result = "0x10"
				case "eth_call":
					called = true
					if string(req.Params[1]) != `"0x10"` {
						t.Error("un-pinned verification block")
					}
					result = "0x1626ba7e" + strings.Repeat("0", 56)
					if mode == "wrong_magic" {
						result = "0xffffffff" + strings.Repeat("0", 56)
					}
				}
				if mode == "rpc_error" {
					json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32000, "message": "unavailable"}})
					return
				}
				json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
			}))
			defer server.Close()
			err := verify1271(context.Background(), server.URL, "0x1111111111111111111111111111111111111111", "message", "0x1234")
			if mode == "ok" && (err != nil || !called) {
				t.Fatal("valid contract fixture rejected", err)
			}
			if mode != "ok" && err == nil {
				t.Fatal("did not fail closed")
			}
		})
	}
}
func TestWorkflowApprovalTimerAndActivityRetry(t *testing.T) {
	for _, approve := range []bool{false, true} {
		t.Run(map[bool]string{false: "timeout", true: "approved_with_retry"}[approve], func(t *testing.T) {
			var suite testsuite.WorkflowTestSuite
			env := suite.NewTestWorkflowEnvironment()
			states := make(chan string, 10)
			attempts := make(chan bool, 5)
			env.RegisterActivityWithOptions(func(context.Context, string, string) error { states <- "expired"; return nil }, activity.RegisterOptions{Name: "SetState"})
			env.RegisterActivityWithOptions(func(context.Context, string) error { return nil }, activity.RegisterOptions{Name: "AuditApproval"})
			env.RegisterActivityWithOptions(func(context.Context, Payment) error {
				attempts <- true
				if len(attempts) == 1 {
					return context.DeadlineExceeded
				}
				return nil
			}, activity.RegisterOptions{Name: "Submit"})
			env.RegisterActivityWithOptions(func(context.Context, string) (string, error) { return "confirmed", nil }, activity.RegisterOptions{Name: "Observe"})
			env.RegisterActivityWithOptions(func(context.Context, string) error { states <- "settled"; return nil }, activity.RegisterOptions{Name: "Settle"})
			env.RegisterActivityWithOptions(func(context.Context, string) error { return nil }, activity.RegisterOptions{Name: "Notify"})
			if approve {
				env.RegisterDelayedCallback(func() { env.SignalWorkflow("approve", true) }, time.Second)
			}
			env.ExecuteWorkflow(PaymentWorkflow, Input{Payment: Payment{ID: "fixture"}, ApprovalSeconds: 5, ObserveSeconds: 5})
			if err := env.GetWorkflowError(); err != nil {
				t.Fatal(err)
			}
			want := "expired"
			if approve {
				want = "settled"
			}
			if got := <-states; got != want {
				t.Fatal(got, want)
			}
			if approve && len(attempts) != 2 {
				t.Fatal("Activity was not retried")
			}
		})
	}
}

func TestChainMonitorSurvivesNotificationFailure(t *testing.T) {
	for _, reconcile := range []bool{false, true} {
		t.Run(map[bool]string{false: "payment", true: "reconcile"}[reconcile], func(t *testing.T) {
			for _, legacy := range []bool{false, true} {
				t.Run(map[bool]string{false: "new_monitor", true: "legacy_history"}[legacy], func(t *testing.T) {
					var suite testsuite.WorkflowTestSuite
					env := suite.NewTestWorkflowEnvironment()
					var audits atomic.Int32
					if legacy {
						marker := "chain-monitor-after-notify"
						if reconcile {
							marker = "chain-reconcile-after-notify"
						}
						env.OnGetVersion(marker, workflow.DefaultVersion, 1).Return(workflow.DefaultVersion)
					}
					env.RegisterActivityWithOptions(func(context.Context, string) error { return nil }, activity.RegisterOptions{Name: "AuditApproval"})
					env.RegisterActivityWithOptions(func(context.Context, Payment) error { return nil }, activity.RegisterOptions{Name: "Submit"})
					env.RegisterActivityWithOptions(func(context.Context, string) (string, error) { return "confirmed", nil }, activity.RegisterOptions{Name: "Observe"})
					env.RegisterActivityWithOptions(func(context.Context, string) error { return nil }, activity.RegisterOptions{Name: "Settle"})
					env.RegisterActivityWithOptions(func(context.Context, string) error { return context.DeadlineExceeded }, activity.RegisterOptions{Name: "Notify"})
					env.RegisterActivityWithOptions(func(context.Context, string) (string, error) { audits.Add(1); return "settled", nil }, activity.RegisterOptions{Name: "AuditChain"})
					env.RegisterDelayedCallback(func() { env.SignalWorkflow("approve", true) }, time.Second)
					if reconcile {
						env.ExecuteWorkflow(ChainReconcileWorkflow, Payment{ID: "notify-outage"})
					} else {
						env.ExecuteWorkflow(ChainPaymentWorkflow, ChainInput{Input: Input{Payment: Payment{ID: "notify-outage"}, ApprovalSeconds: 5, ObserveSeconds: 5}})
					}
					if legacy {
						if env.GetWorkflowError() == nil || audits.Load() != 0 {
							t.Fatalf("changed legacy command path: %d %v", audits.Load(), env.GetWorkflowError())
						}
					} else if !workflow.IsContinueAsNewError(env.GetWorkflowError()) || audits.Load() != 50 {
						t.Fatalf("notification outage stopped audit: %d %v", audits.Load(), env.GetWorkflowError())
					}
				})
			}
		})
	}
}

func TestChainMonitorDoesNotMaskSettlementFailure(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	var audits atomic.Int32
	env.RegisterActivityWithOptions(func(context.Context, string) (string, error) { return "confirmed", nil }, activity.RegisterOptions{Name: "Observe"})
	env.RegisterActivityWithOptions(func(context.Context, string) error { return context.DeadlineExceeded }, activity.RegisterOptions{Name: "Settle"})
	env.RegisterActivityWithOptions(func(context.Context, string) (string, error) { audits.Add(1); return "pending", nil }, activity.RegisterOptions{Name: "AuditChain"})
	env.ExecuteWorkflow(ChainReconcileWorkflow, Payment{ID: "failed-settlement"})
	if env.GetWorkflowError() == nil || audits.Load() != 0 {
		t.Fatalf("masked ledger failure: audits=%d error=%v", audits.Load(), env.GetWorkflowError())
	}
}

func TestRecoveryAuditCannotAcknowledgeFailedAudit(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	var marked atomic.Int32
	env.RegisterActivityWithOptions(func(context.Context, string) (string, error) { return "", context.DeadlineExceeded }, activity.RegisterOptions{Name: "AuditChain"})
	env.RegisterActivityWithOptions(func(context.Context, string) error { marked.Add(1); return nil }, activity.RegisterOptions{Name: "MarkRecoveryAudited"})
	env.ExecuteWorkflow(RecoveryAuditWorkflow, "plan", "payment")
	if env.GetWorkflowError() == nil || marked.Load() != 0 {
		t.Fatal("failed audit acknowledged recovery")
	}
}
