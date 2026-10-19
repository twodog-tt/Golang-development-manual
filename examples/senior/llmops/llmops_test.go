package llmops

import (
	"context"
	"errors"
	"math"
	"td-homework/examples/senior/agentmemory"
	"td-homework/examples/senior/llmclient"
	"testing"
	"time"
)

type model struct {
	calls int
	fail  bool
	hook  func()
}

func (m *model) Complete(context.Context, []llmclient.Message) (string, llmclient.Usage, error) {
	m.calls++
	if m.hook != nil {
		m.hook()
	}
	if m.fail {
		return "partial", llmclient.Usage{}, errors.New("drop")
	}
	return "FAQ answer", llmclient.Usage{Known: true, Source: "fixture"}, nil
}
func (m *model) StreamChat(context.Context, []llmclient.Message, llmclient.StreamHandler) (llmclient.Usage, error) {
	return llmclient.Usage{}, errors.New("unused")
}
func setup(t *testing.T) (*Engine, *model, *agentmemory.Authority, agentmemory.Scope, context.Context) {
	t.Helper()
	a := agentmemory.NewAuthority()
	scope := agentmemory.Scope{Tenant: "a", Bot: "bot", Scene: "faq", SubjectType: "user", SubjectID: "u"}
	a.Put("one", agentmemory.Grant{Tenant: "a", Subject: "alice", Policy: "p1", Scopes: []agentmemory.Scope{scope}, Roles: []string{"read"}, Expires: time.Now().Add(time.Hour)})
	ctx, e := a.Context(context.Background(), "one")
	if e != nil {
		t.Fatal(e)
	}
	m := &model{}
	engine, e := New(m, Config{"prompt1", "model1", "corpus1", time.Minute})
	if e != nil {
		t.Fatal(e)
	}
	return engine, m, a, scope, ctx
}
func TestCacheHitBypassesModel(t *testing.T) {
	e, m, _, scope, ctx := setup(t)
	if _, hit, err := e.Answer(ctx, scope, "refund policy"); err != nil || hit {
		t.Fatal(hit, err)
	}
	if _, hit, err := e.Answer(ctx, scope, "policy refund"); err != nil || !hit || m.calls != 1 {
		t.Fatal(hit, m.calls, err)
	}
	s := e.Snapshot()
	if s.Tasks != 2 || s.Hits != 1 || s.Generations != 1 {
		t.Fatal(s)
	}
}
func TestBoundaryBeforeSimilarity(t *testing.T) {
	for _, mode := range []string{"tenant", "bot", "scene", "subject", "identity", "policy", "revision", "prompt", "model", "corpus", "expiry"} {
		t.Run(mode, func(t *testing.T) {
			e, m, a, scope, ctx := setup(t)
			if _, _, err := e.Answer(ctx, scope, "refund policy"); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "tenant":
				scope.Tenant = "b"
			case "bot":
				scope.Bot = "other"
			case "scene":
				scope.Scene = "other"
			case "subject":
				scope.SubjectID = "other"
			case "prompt":
				e.config.PromptVersion = "p2"
			case "model":
				e.config.ModelVersion = "m2"
			case "corpus":
				e.config.CorpusVersion = "c2"
			case "expiry":
				e.entries[0].Until = time.Now().Add(-time.Second)
			}
			if mode == "tenant" || mode == "bot" || mode == "scene" || mode == "subject" || mode == "identity" || mode == "policy" || mode == "revision" {
				subject, policy := "alice", "p1"
				if mode == "identity" {
					subject = "bob"
				}
				if mode == "policy" {
					policy = "p2"
				}
				a.Put("one", agentmemory.Grant{Tenant: scope.Tenant, Subject: subject, Policy: policy, Scopes: []agentmemory.Scope{scope}, Roles: []string{"read"}, Expires: time.Now().Add(time.Hour)})
				var err error
				ctx, err = a.Context(context.Background(), "one")
				if err != nil {
					t.Fatal(err)
				}
			}
			compares := 0
			e.match = func(string, string) float64 { compares++; return 1 }
			if _, hit, err := e.Answer(ctx, scope, "refund policy"); err != nil || hit || compares != 0 || m.calls != 2 {
				t.Fatalf("hit=%v compare=%d calls=%d err=%v", hit, compares, m.calls, err)
			}
		})
	}
}
func TestFailureAndRevocationNotCached(t *testing.T) {
	for _, mode := range []string{"failed", "revoked", "cached-revoked"} {
		t.Run(mode, func(t *testing.T) {
			e, m, a, scope, ctx := setup(t)
			switch mode {
			case "failed":
				m.fail = true
			case "revoked":
				m.hook = func() { a.Revoke("one") }
			case "cached-revoked":
				if _, _, err := e.Answer(ctx, scope, "refund policy"); err != nil {
					t.Fatal(err)
				}
				a.Revoke("one")
			}
			v, hit, err := e.Answer(ctx, scope, "refund policy")
			if err == nil || hit || v != "" {
				t.Fatal(v, hit, err)
			}
			if mode != "cached-revoked" && len(e.entries) != 0 {
				t.Fatal("cached failed output")
			}
		})
	}
}
func TestCostSubsetUnknownFailuresAndOverflow(t *testing.T) {
	m, e := NewMeter(Rates{"fixture-v1", 2, 1, 3})
	if e != nil {
		t.Fatal(e)
	}
	if e = m.Record(llmclient.Attempt{Usage: llmclient.Usage{Known: true, Source: "reported", PromptTokens: 10, CachedInputTokens: 4, CompletionTokens: 2}}); e != nil {
		t.Fatal(e)
	}
	_ = m.Record(llmclient.Attempt{Err: errors.New("drop")})
	_ = m.Record(llmclient.Attempt{Usage: llmclient.Usage{Source: "mock-word-count", PromptTokens: 99}})
	s := m.Snapshot()
	if s.Attempts != 3 || s.KnownMicroUSD != 22 || s.Unknown != 2 || s.Failed != 1 {
		t.Fatal(s)
	}
	if e = m.Record(llmclient.Attempt{Usage: llmclient.Usage{Known: true, Source: "bad", PromptTokens: 1, CachedInputTokens: 2}}); e == nil {
		t.Fatal("invalid subset")
	}
	overflow, _ := NewMeter(Rates{"overflow", math.MaxInt64, 0, 0})
	if e = overflow.Record(llmclient.Attempt{Usage: llmclient.Usage{Known: true, Source: "reported", PromptTokens: 2}}); e == nil {
		t.Fatal("overflow")
	}
	if overflow.Snapshot().Unknown != 1 || overflow.Snapshot().KnownMicroUSD != 0 {
		t.Fatal(overflow.Snapshot())
	}
}

func TestKnownFailedUsageStillCharged(t *testing.T) {
	m, e := NewMeter(Rates{"test", 2, 1, 3})
	if e != nil {
		t.Fatal(e)
	}
	e = m.Record(llmclient.Attempt{Usage: llmclient.Usage{Known: true, Source: "reported", PromptTokens: 10, CompletionTokens: 2, CachedInputTokens: 4}, Err: errors.New("application rejected answer")})
	if e != nil {
		t.Fatal(e)
	}
	s := m.Snapshot()
	if s.Failed != 1 || s.KnownMicroUSD != 22 || s.Unknown != 0 {
		t.Fatal(s)
	}
}
