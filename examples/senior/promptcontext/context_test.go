package promptcontext

import (
	"context"
	"errors"
	"strings"
	"td-homework/examples/senior/agentmemory"
	"td-homework/examples/senior/llmclient"
	"testing"
	"time"
)

func fixture(t *testing.T) (*agentmemory.Authority, context.Context, agentmemory.Scope, []Group) {
	t.Helper()
	scope := agentmemory.Scope{Tenant: "demo", Bot: "support", Scene: "orders", SubjectType: "user", SubjectID: "alice"}
	a := agentmemory.NewAuthority()
	a.Put("reader", agentmemory.Grant{Tenant: scope.Tenant, Subject: "alice", Policy: "v1", Scopes: []agentmemory.Scope{scope}, Roles: []string{"read"}, Expires: time.Now().Add(time.Hour)})
	ctx, e := a.Context(context.Background(), "reader")
	if e != nil {
		t.Fatal(e)
	}
	return a, ctx, scope, []Group{{Scope: scope, ID: "policy", Required: true, Items: []Item{{Kind: "system", Text: "No tools without authority."}}}, {Scope: scope, ID: "old-tool", Items: []Item{{Kind: "reasoning", Text: "opaque fixture reasoning"}, {Kind: "call", CallID: "call1", Text: "lookup"}, {Kind: "result", CallID: "call1", Text: strings.Repeat("data", 20)}}}, {Scope: scope, ID: "task", Required: true, Items: []Item{{Kind: "current_user", Text: "query ORD-1"}}}}
}
func TestBudgetBoundaryAndAtomicToolGroup(t *testing.T) {
	_, ctx, scope, g := fixture(t)
	counter := FixtureCounter{}
	all := []Item{}
	for _, v := range g {
		all = append(all, v.Items...)
	}
	n, e := counter.Count(ctx, all)
	if e != nil {
		t.Fatal(e)
	}
	budget := Budget{Window: n + 30, Output: 10, Reasoning: 15, MaxOutput: 25, Margin: 5}
	got, e := Assemble(ctx, scope, g, budget, counter)
	if e != nil || len(got.Dropped) != 0 {
		t.Fatal(got, e)
	}
	budget.Window--
	got, e = Assemble(ctx, scope, g, budget, counter)
	if e != nil || len(got.Dropped) != 1 || got.Dropped[0] != "old-tool" {
		t.Fatal(got, e)
	}
	for _, i := range got.Items {
		if i.Kind == "call" || i.Kind == "result" || i.Kind == "reasoning" {
			t.Fatal("split tool group")
		}
	}
	if len(g[1].Items) != 3 {
		t.Fatal("mutated original history")
	}
}
func TestRequiredGroupsAndOutputLimitsFailClosed(t *testing.T) {
	_, ctx, scope, g := fixture(t)
	for _, b := range []Budget{{Window: 1, Output: 1, MaxOutput: 1}, {Window: 100, Output: 30, Reasoning: 30, MaxOutput: 50}, {Window: 100, Output: 30, MaxOutput: 40, Margin: 80}, {Window: 100, Output: -1, MaxOutput: 40}, {Window: 1<<30 + 1, Output: 1, MaxOutput: 1}} {
		if _, e := Assemble(ctx, scope, g, b, FixtureCounter{}); !errors.Is(e, ErrBudget) {
			t.Fatal(b, e)
		}
	}
	g[0].Required = false
	if _, e := Assemble(ctx, scope, g, Budget{Window: 1000, Output: 10, MaxOutput: 10}, FixtureCounter{}); e == nil {
		t.Fatal("optional policy accepted")
	}
}
func TestBrokenToolHistoryAndPendingCallsRejected(t *testing.T) {
	for _, mode := range []string{"orphan", "pending", "duplicate", "cross-group", "unknown-kind"} {
		t.Run(mode, func(t *testing.T) {
			_, ctx, scope, g := fixture(t)
			switch mode {
			case "orphan":
				g[1].Items = g[1].Items[2:]
			case "pending":
				g[1].Items = g[1].Items[:2]
			case "duplicate":
				g[1].Items = append(g[1].Items, Item{Kind: "call", CallID: "call1"})
			case "cross-group":
				g = append(g, Group{Scope: scope, ID: "result", Items: g[1].Items[2:]})
				g[1].Items = g[1].Items[:2]
			case "unknown-kind":
				g[1].Items[0].Kind = "developer-from-model"
			}
			if _, e := Assemble(ctx, scope, g, Budget{Window: 1000, Output: 10, MaxOutput: 10}, FixtureCounter{}); e == nil {
				t.Fatal("invalid history accepted")
			}
		})
	}
}

type badCounter struct {
	err  error
	n    int
	hook func()
}

func (b badCounter) Version() string { return "test-counter" }
func (b badCounter) Count(context.Context, []Item) (int, error) {
	if b.hook != nil {
		b.hook()
	}
	return b.n, b.err
}
func TestAuthorizationAndCounterFailures(t *testing.T) {
	a, ctx, scope, g := fixture(t)
	b := Budget{Window: 1000, Output: 10, MaxOutput: 10}
	g[1].Scope.Tenant = "other"
	if _, e := Assemble(ctx, scope, g, b, FixtureCounter{}); !errors.Is(e, agentmemory.ErrDenied) {
		t.Fatal(e)
	}
	g[1].Scope = scope
	for _, counter := range []Counter{nil, badCounter{err: errors.New("tokenizer unavailable")}, badCounter{n: -1}, badCounter{hook: func() { a.Revoke("reader") }}} {
		if _, e := Assemble(ctx, scope, g, b, counter); e == nil {
			t.Fatal("unsafe fallback")
		}
	}
}

type captureFixture struct {
	calls int
	items int
	hook  func()
}

func (f *captureFixture) CompleteStructured(_ context.Context, m []llmclient.Message) (Envelope, error) {
	f.calls++
	f.items = len(m)
	if f.hook != nil {
		f.hook()
	}
	return Envelope{Version: PromptVersion, Status: "completed", Content: `{"action":"query","order_id":"ORD-1"}`}, nil
}
func TestBudgetAssemblyFeedsActualModelAndUnsupportedHistoryFailsClosed(t *testing.T) {
	a, ctx, scope, groups := fixture(t)
	counter := FixtureCounter{}
	required, _ := counter.Count(ctx, append(append([]Item{}, groups[0].Items...), groups[2].Items...))
	budget := Budget{Window: required + 20, Output: 10, Reasoning: 5, MaxOutput: 15, Margin: 5}
	assembly, e := Assemble(ctx, scope, groups, budget, counter)
	if e != nil {
		t.Fatal(e)
	}
	spy := &captureFixture{}
	if _, e = ExtractAssembled(ctx, scope, spy, assembly); e != nil || spy.calls != 1 || spy.items != 2 {
		t.Fatal(spy, e)
	}
	budget.Window = 1000
	full, e := Assemble(ctx, scope, groups, budget, counter)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ExtractAssembled(ctx, scope, spy, full); e == nil || spy.calls != 1 {
		t.Fatal("opaque tool history silently flattened", e)
	}
	spy.hook = func() { a.Revoke("reader") }
	if got, e := ExtractAssembled(ctx, scope, spy, assembly); e == nil || got.Action != "" {
		t.Fatal("revoked output returned", got, e)
	}
}

func TestDispatchRechecksFinalFormattingAndAssemblyIdentity(t *testing.T) {
	auth, ctx, scope, g := fixture(t)
	g[1].Items = []Item{{Kind: "data", Text: strings.Repeat(`"`, 40)}}
	g[1].Required = true
	all := []Item{}
	for _, group := range g {
		all = append(all, group.Items...)
	}
	counter := FixtureCounter{}
	n, e := counter.Count(ctx, all)
	if e != nil {
		t.Fatal(e)
	}
	a, e := Assemble(ctx, scope, g, Budget{Window: n + 10, Output: 10, MaxOutput: 10}, counter)
	if e != nil {
		t.Fatal(e)
	}
	model := &captureFixture{}
	if _, e = ExtractAssembled(ctx, scope, model, a); !errors.Is(e, ErrBudget) || model.calls != 0 {
		t.Fatal("final quoting/framing exceeded window", e, model.calls)
	}
	auth.Put("second", agentmemory.Grant{Tenant: scope.Tenant, Subject: "bob", Policy: "v1", Scopes: []agentmemory.Scope{scope}, Roles: []string{"read"}, Expires: time.Now().Add(time.Hour)})
	other, e := auth.Context(context.Background(), "second")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ExtractAssembled(other, scope, model, a); !errors.Is(e, agentmemory.ErrDenied) || model.calls != 0 {
		t.Fatal("assembly reused by a different identity", e)
	}
}

type changingCounter struct {
	version string
	change  bool
}

func (c *changingCounter) Version() string { return c.version }
func (c *changingCounter) Count(ctx context.Context, items []Item) (int, error) {
	if c.change {
		c.version = "v2"
	}
	return (FixtureCounter{}).Count(ctx, items)
}
func TestCounterVersionChangesFailBeforeModelDispatch(t *testing.T) {
	_, ctx, scope, groups := fixture(t)
	groups = []Group{groups[0], groups[2]}
	budget := Budget{Window: 1000, Output: 10, MaxOutput: 10}
	counter := &changingCounter{version: "v1", change: true}
	if _, err := Assemble(ctx, scope, groups, budget, counter); err == nil {
		t.Fatal("assembly accepted changing tokenizer")
	}
	counter = &changingCounter{version: "v1"}
	a, err := Assemble(ctx, scope, groups, budget, counter)
	if err != nil {
		t.Fatal(err)
	}
	counter.change = true
	model := &captureFixture{}
	if _, err = ExtractAssembled(ctx, scope, model, a); err == nil || model.calls != 0 {
		t.Fatal("dispatch accepted changing tokenizer", err, model.calls)
	}
}
