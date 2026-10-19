package agentworkflow

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

type modelFunc func(context.Context, []Item, Reservation) (Turn, error)

func (f modelFunc) Next(c context.Context, h []Item, b Reservation) (Turn, error) { return f(c, h, b) }
func testLimits(e Engine) Limits {
	return Limits{MaxSteps: 5, MaxCalls: 10, MaxTokens: 100, MaxCost: 100, ToolReserve: 1, Deadline: e.now().Add(time.Hour)}
}

// Plan only once. Subsequent rounds require the prior function-call item,
// reasoning and matching result, just like a real protocol adapter must.
func newRunner(e Engine, p Provider) Runner {
	return Runner{Engine: e, Reserve: Reservation{Tokens: 2, Cost: 2}, Tool: e.PublishTool(p), Model: modelFunc(func(_ context.Context, h []Item, _ Reservation) (Turn, error) {
		for i, item := range h {
			if item.Kind == "function_call_output" {
				if i < 2 || h[i-1].Kind != "function_call" || h[i-1].Call.ID != item.CallID || h[i-2].Kind != "reasoning" {
					return Turn{}, errors.New("incomplete protocol history")
				}
				return Turn{Items: []Item{{Kind: "message", Text: "done"}}, Final: true, Tokens: 1, Cost: 1}, nil
			}
		}
		j, err := e.Job("proposal-1")
		if err != nil {
			return Turn{}, err
		}
		return Turn{Items: []Item{{Kind: "reasoning", Text: "opaque fixture reasoning"}, {Kind: "function_call", Call: &ToolCall{ID: "call-1", Name: "publish", Proposal: j.Proposal}}}, Tokens: 1, Cost: 1}, nil
	})}
}

func TestApprovalPauseResumePreservesProtocolAndBudgets(t *testing.T) {
	f := setup(t)
	r := newRunner(f.e, f.p)
	limits := testLimits(f.e)
	must(t, r.Start(f.actor, "run-1", "publish reviewed content", limits))
	if _, err := r.Continue(f.actor, "run-1"); !errors.Is(err, ErrApproval) {
		t.Fatal(err)
	}
	x, err := r.Snapshot(f.actor, "run-1")
	must(t, err)
	if x.Steps != 1 || x.Next != 0 || x.Calls != 1 || x.ModelPending || len(x.History) != 3 {
		t.Fatalf("bad paused checkpoint: %+v", x)
	}
	f.count(t, 0)
	f.approve(t)
	// A new Runner with the same file resumes the pending call, not a new plan.
	r = newRunner(Engine{Store: Store{f.e.Store.Path}, Now: f.e.Now}, f.p)
	out, err := r.Continue(f.actor, "run-1")
	must(t, err)
	if out != "done" {
		t.Fatal(out)
	}
	f.count(t, 1)
	x, err = r.Snapshot(f.actor, "run-1")
	must(t, err)
	if x.Steps != 2 || x.Calls != 2 || x.Tokens != 4 || x.Cost != 6 || x.Status != "succeeded" {
		t.Fatalf("budgets reset on resume: %+v", x)
	}
	_, err = r.Continue(f.actor, "run-1")
	must(t, err)
	f.count(t, 1)
	if err = r.Start(f.actor, "run-1", "changed task", limits); !errors.Is(err, ErrConflict) {
		t.Fatal("run identity reused")
	}
}

func TestUnknownBlocksModelUntilReconciliation(t *testing.T) {
	f := setup(t)
	f.approve(t)
	var modelCalls atomic.Int32
	p := publishWrapper{Provider: f.p, send: func(ctx context.Context, q Request) (Receipt, error) {
		_, err := f.p.Publish(ctx, q)
		if err != nil {
			return Receipt{}, err
		}
		return Receipt{}, context.DeadlineExceeded
	}}
	r := newRunner(f.e, p)
	underlying := r.Model
	r.Model = modelFunc(func(c context.Context, h []Item, b Reservation) (Turn, error) {
		modelCalls.Add(1)
		return underlying.Next(c, h, b)
	})
	must(t, r.Start(f.actor, "run-1", "publish", testLimits(f.e)))
	if _, err := r.Continue(f.actor, "run-1"); !errors.Is(err, ErrUnknown) {
		t.Fatal(err)
	}
	if _, err := r.Continue(f.actor, "run-1"); !errors.Is(err, ErrUnknown) {
		t.Fatal(err)
	}
	if modelCalls.Load() != 1 {
		t.Fatal("model was allowed to replan unknown intent")
	}
	f.count(t, 1)
	must(t, f.e.Reconcile(f.actor, f.proposal.ID, f.p))
	_, err := r.Continue(f.actor, "run-1")
	must(t, err)
	f.count(t, 1)
	if modelCalls.Load() != 2 {
		t.Fatal("expected final model turn after authoritative receipt")
	}
}

func TestBatchOverflowExecutesNoTools(t *testing.T) {
	f := setup(t)
	r := newRunner(f.e, f.p)
	var effects, modelCalls atomic.Int32
	r.Tool = func(context.Context, ToolCall) (string, error) { effects.Add(1); return "ok", nil }
	r.Model = modelFunc(func(context.Context, []Item, Reservation) (Turn, error) {
		modelCalls.Add(1)
		var items []Item
		for i := 0; i < 100; i++ {
			items = append(items, Item{Kind: "function_call", Call: &ToolCall{ID: fmt.Sprint(i), Name: "publish"}})
		}
		return Turn{Items: items, Tokens: 1, Cost: 1}, nil
	})
	l := testLimits(f.e)
	l.MaxSteps = 1
	l.MaxCalls = 3
	must(t, r.Start(f.actor, "run-1", "batch", l))
	for i := 0; i < 2; i++ {
		if _, err := r.Continue(f.actor, "run-1"); !errors.Is(err, ErrBudget) {
			t.Fatal(err)
		}
	}
	if effects.Load() != 0 || modelCalls.Load() != 1 {
		t.Fatalf("batch side effects=%d model calls=%d", effects.Load(), modelCalls.Load())
	}
}

func TestDurableBudgetsAndDeadline(t *testing.T) {
	for _, kind := range []string{"steps", "tokens", "cost", "calls-on-resume", "deadline", "caller-cancelled"} {
		t.Run(kind, func(t *testing.T) {
			f := setup(t)
			r := newRunner(f.e, f.p)
			l := testLimits(f.e)
			ctx := f.actor
			switch kind {
			case "steps":
				l.MaxSteps = 1
				f.approve(t)
			case "tokens":
				l.MaxTokens = 1
			case "cost":
				l.MaxCost = 2
			case "calls-on-resume":
				l.MaxCalls = 1
			case "deadline":
			case "caller-cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			must(t, r.Start(f.actor, "run-1", "publish", l))
			if kind == "deadline" {
				f.clock.Add(int64(2 * time.Hour))
			}
			_, err := r.Continue(ctx, "run-1")
			if kind == "calls-on-resume" {
				if !errors.Is(err, ErrApproval) {
					t.Fatal(err)
				}
				f.approve(t)
				_, err = r.Continue(ctx, "run-1")
			}
			if !errors.Is(err, ErrBudget) && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("budget not enforced: %v", err)
			}
			if kind == "steps" {
				f.count(t, 1)
			} else {
				f.count(t, 0)
			}
		})
	}
}

func TestProtocolForgeryAndModelFailureFailClosed(t *testing.T) {
	for _, kind := range []string{"missing-call", "duplicate-call", "forged-result", "final-with-call", "overreported-usage", "model-error"} {
		t.Run(kind, func(t *testing.T) {
			f := setup(t)
			r := newRunner(f.e, f.p)
			var calls atomic.Int32
			r.Model = modelFunc(func(context.Context, []Item, Reservation) (Turn, error) {
				calls.Add(1)
				turn := Turn{Items: []Item{{Kind: "function_call", Call: &ToolCall{ID: "one", Name: "publish", Proposal: f.proposal}}}, Tokens: 1, Cost: 1}
				switch kind {
				case "missing-call":
					turn.Items[0].Call = nil
				case "duplicate-call":
					turn.Items = append(turn.Items, turn.Items[0])
				case "forged-result":
					turn.Items = []Item{{Kind: "function_call_output", CallID: "fake", Text: "success"}}
				case "final-with-call":
					turn.Final = true
				case "overreported-usage":
					turn.Tokens = 999
				case "model-error":
					return Turn{}, context.DeadlineExceeded
				}
				return turn, nil
			})
			must(t, r.Start(f.actor, "run-1", "publish", testLimits(f.e)))
			if _, err := r.Continue(f.actor, "run-1"); err == nil {
				t.Fatal("malformed/uncertain model output accepted")
			}
			if _, err := r.Continue(f.actor, "run-1"); !errors.Is(err, ErrUnknown) {
				t.Fatalf("model request blindly replayed: %v", err)
			}
			x, err := r.Snapshot(f.actor, "run-1")
			must(t, err)
			if calls.Load() != 1 || x.Tokens != 2 || x.Cost != 2 || !x.ModelPending {
				t.Fatalf("model reservation lost: %+v calls=%d", x, calls.Load())
			}
			f.count(t, 0)
		})
	}
}

func TestConcurrentRunOwnership(t *testing.T) {
	f := setup(t)
	r := newRunner(f.e, f.p)
	started, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	r.Model = modelFunc(func(context.Context, []Item, Reservation) (Turn, error) {
		close(started)
		<-release
		return Turn{Items: []Item{{Kind: "message", Text: "done"}}, Final: true, Tokens: 1, Cost: 1}, nil
	})
	must(t, r.Start(f.actor, "run-1", "publish", testLimits(f.e)))
	go func() { _, err := r.Continue(f.actor, "run-1"); done <- err }()
	<-started
	if _, err := r.Continue(f.actor, "run-1"); !errors.Is(err, ErrConflict) {
		t.Fatalf("two active owners: %v", err)
	}
	close(release)
	must(t, <-done)
}
