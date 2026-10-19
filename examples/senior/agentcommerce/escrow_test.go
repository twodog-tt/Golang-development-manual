package agentcommerce

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
)

func TestV2FlowOrderingStopsOnPrepaymentAndCaptureFailure(t *testing.T) {
	for _, flow := range []string{"authorization", "upfront", "escrow"} {
		want, _ := FlowStages(flow)
		got := []string{}
		if err := RunFlow(context.Background(), flow, func(_ context.Context, s string) error { got = append(got, s); return nil }); err != nil || !reflect.DeepEqual(got, want) {
			t.Fatal(flow, got, err)
		}
		for i := range want {
			got = nil
			err := RunFlow(context.Background(), flow, func(_ context.Context, s string) error {
				got = append(got, s)
				if s == want[i] {
					return errors.New("ambiguous stage")
				}
				return nil
			})
			if err == nil || !reflect.DeepEqual(got, want[:i+1]) {
				t.Fatal("continued after failed stage", flow, i, got)
			}
		}
	}
	if err := RunFlow(context.Background(), "unknown", func(context.Context, string) error { t.Fatal("unknown flow executed"); return nil }); err == nil {
		t.Fatal("unknown flow accepted")
	}
}
func job(t *testing.T) *Job {
	t.Helper()
	j, err := NewJob("client", "provider", "evaluator", 100, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = j.SetBudget("provider", 10); err != nil {
		t.Fatal(err)
	}
	return j
}
func TestEscrowExpectedBudgetProviderAndRoleBoundaries(t *testing.T) {
	j, err := NewJob("client", "", "evaluator", 100, 1)
	if err != nil {
		t.Fatal(err)
	}
	_ = j.SetBudget("client", 10)
	if err = j.Fund("client", 10, 10); err == nil {
		t.Fatal("funded before provider selection")
	}
	if err = j.SetProvider("outsider", "provider"); err == nil {
		t.Fatal("provider substituted")
	}
	if err = j.SetProvider("client", "provider"); err != nil {
		t.Fatal(err)
	}
	_ = j.SetBudget("provider", 20)
	if err = j.Fund("client", 10, 10); err == nil || j.Snapshot().Escrow != 0 {
		t.Fatal("price changed after approval")
	}
	if err = j.Fund("provider", 20, 10); err == nil {
		t.Fatal("wrong funder")
	}
	if err = j.Fund("client", 20, 10); err != nil {
		t.Fatal(err)
	}
	if err = j.Reject("client"); err == nil {
		t.Fatal("client unilaterally refunded funded job")
	}
	if err = j.Submit("client", "digest"); err == nil {
		t.Fatal("wrong provider")
	}
	if err = j.Complete("evaluator"); err == nil {
		t.Fatal("completed before submission")
	}
	if err = j.Submit("provider", "digest"); err != nil {
		t.Fatal(err)
	}
	if err = j.Complete("provider"); err == nil {
		t.Fatal("provider acted as evaluator")
	}
	if err = j.Complete("evaluator"); err != nil {
		t.Fatal(err)
	}
	if err = j.Refund("anyone", 100); err == nil {
		t.Fatal("terminal double refund")
	}
}
func TestExpiryBoundaryHookRevertRollbackAndUnhookableRefund(t *testing.T) {
	j := job(t)
	if err := j.Fund("client", 10, 100); err == nil {
		t.Fatal("funded at expiry")
	}
	j.InjectHook(HookFault{After: true})
	if err := j.Fund("client", 10, 99); err == nil || j.Snapshot().Escrow != 0 || j.Snapshot().Status != "open" {
		t.Fatal("after-hook left partial funding")
	}
	j.InjectHook(HookFault{})
	if err := j.Fund("client", 10, 99); err != nil {
		t.Fatal(err)
	}
	j.InjectHook(HookFault{Before: true, After: true})
	if err := j.Refund("anyone", 99); err == nil {
		t.Fatal("early refund")
	}
	if j.Snapshot().Status != "funded" {
		t.Fatal("clock auto-transitioned state")
	}
	if err := j.Refund("anyone", 100); err != nil {
		t.Fatal("hook blocked safety refund", err)
	}
	s := j.Snapshot()
	if s.Refunded != 10 || s.Paid != 0 || s.Escrow != 0 || s.Client != "client" {
		t.Fatal("refund redirected", s)
	}
}
func TestCompleteRefundRaceAndBothTransactionOrders(t *testing.T) {
	for _, order := range []string{"complete-first", "refund-first", "concurrent"} {
		t.Run(order, func(t *testing.T) {
			j := job(t)
			_ = j.Fund("client", 10, 10)
			_ = j.Submit("provider", "digest")
			switch order {
			case "complete-first":
				if err := j.Complete("evaluator"); err != nil {
					t.Fatal(err)
				}
				if err := j.Refund("anyone", 100); err == nil {
					t.Fatal("double payout")
				}
			case "refund-first":
				if err := j.Refund("anyone", 100); err != nil {
					t.Fatal(err)
				}
				if err := j.Complete("evaluator"); err == nil {
					t.Fatal("completed expired job")
				}
			case "concurrent":
				var wg sync.WaitGroup
				wg.Add(2)
				go func() { defer wg.Done(); _ = j.Complete("evaluator") }()
				go func() { defer wg.Done(); _ = j.Refund("anyone", 100) }()
				wg.Wait()
			}
			s := j.Snapshot()
			if s.Paid+s.Refunded != 10 || s.Escrow != 0 || (s.Paid != 0 && s.Refunded != 0) {
				t.Fatal("money invariant failed", s)
			}
		})
	}
}
func TestRejectPermissionsAndAfterHookAtomicity(t *testing.T) {
	for _, stage := range []string{"open", "funded", "submitted"} {
		j := job(t)
		caller := "client"
		if stage != "open" {
			_ = j.Fund("client", 10, 10)
			caller = "evaluator"
		}
		if stage == "submitted" {
			_ = j.Submit("provider", "digest")
		}
		if err := j.Reject("outsider"); err == nil {
			t.Fatal("unauthorized reject")
		}
		j.InjectHook(HookFault{After: true})
		before := j.Snapshot()
		if err := j.Reject(caller); err == nil || j.Snapshot() != before {
			t.Fatal("hook revert retained partial refund")
		}
		j.InjectHook(HookFault{})
		if err := j.Reject(caller); err != nil {
			t.Fatal(err)
		}
		s := j.Snapshot()
		if s.Status != "rejected" || s.Escrow != 0 || (stage != "open" && s.Refunded != 10) {
			t.Fatal(s)
		}
	}
}
