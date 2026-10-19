// knowledge-demo runs deterministic fixtures only: no external model, key or database.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"time"

	"td-homework/examples/senior/agentmemory"
	"td-homework/examples/senior/llmclient"
	"td-homework/examples/senior/llmops"
	"td-homework/examples/senior/rag"
)

type Report struct {
	RAGRelevant           bool         `json:"rag_relevant"`
	NoEvidenceRefused     bool         `json:"no_evidence_refused"`
	ScopeDenied           bool         `json:"scope_denied"`
	MemoryApproved        bool         `json:"memory_approved"`
	DeletedHistoryBlocked bool         `json:"deleted_history_blocked"`
	CleanupRecovered      bool         `json:"cleanup_recovered"`
	HTTPAttempts          int32        `json:"http_attempts"`
	CacheHitBypassedModel bool         `json:"cache_hit_bypassed_model"`
	Costs                 llmops.Costs `json:"costs"`
}

func run(dir string) (Report, error) {
	var report Report
	a := agentmemory.NewAuthority()
	scope := agentmemory.Scope{Tenant: "demo", Bot: "support", Scene: "faq", SubjectType: "user", SubjectID: "customer"}
	contexts := map[string]context.Context{}
	for _, actor := range []string{"writer", "reviewer", "operator"} {
		a.Put(actor, agentmemory.Grant{Tenant: scope.Tenant, Subject: actor, Policy: "policy1", Scopes: []agentmemory.Scope{scope}, Roles: []string{"read", "write", "review", "activate", "delete"}, Expires: time.Now().Add(time.Hour)})
		ctx, err := a.Context(context.Background(), actor)
		if err != nil {
			return report, err
		}
		contexts[actor] = ctx
	}
	ctx := contexts["operator"]
	store := rag.NewStore()
	if err := store.Add(ctx, rag.Chunk{ID: "refund", Source: "policy.md", Text: "订单退款需要七天内申请", Policy: "policy1", Scope: scope, Expires: time.Now().Add(time.Hour)}); err != nil {
		return report, err
	}
	svc := rag.Service{Store: store, LLM: &llmclient.MockClient{Reply: "七天内申请退款 [policy.md]"}}
	_, hits, err := svc.Answer(ctx, scope, "怎么退款")
	if err != nil {
		return report, err
	}
	report.RAGRelevant = len(hits) == 1 && hits[0].Score > 0
	answer, hits, err := svc.Answer(ctx, scope, "火星轨道周期")
	if err != nil {
		return report, err
	}
	report.NoEvidenceRefused = answer == rag.Refusal && len(hits) == 0
	other := scope
	other.Tenant = "other"
	_, _, err = svc.Answer(ctx, other, "退款")
	report.ScopeDenied = errors.Is(err, agentmemory.ErrDenied)
	memory := agentmemory.Store{Dir: dir}
	rule, err := memory.Create(contexts["writer"], scope, "refund-rule", "保留退款条件", []string{"review:1"}, time.Now().Add(time.Hour))
	if err != nil {
		return report, err
	}
	digest, version, err := memory.ReviewDigest(contexts["reviewer"], scope, rule.ID)
	if err != nil {
		return report, err
	}
	if err = memory.Approve(contexts["reviewer"], scope, rule.ID, digest, version); err != nil {
		return report, err
	}
	if err = memory.Transition(ctx, scope, rule.ID, "active", version); err != nil {
		return report, err
	}
	if err = memory.Sync(); err != nil {
		return report, err
	}
	refs := []agentmemory.Ref{{ID: rule.ID, Version: version}}
	rules, err := memory.Resolve(ctx, scope, refs)
	if err != nil {
		return report, err
	}
	report.MemoryApproved = len(rules) == 1
	if err = memory.Delete(ctx, scope, rule.ID); err != nil {
		return report, err
	}
	memory = agentmemory.Store{Dir: dir}
	rules, err = memory.Resolve(ctx, scope, refs)
	if err != nil {
		return report, err
	}
	report.DeletedHistoryBlocked = len(rules) == 0
	more, err := memory.SyncOne(true)
	if !more || err == nil {
		return report, errors.New("lost ACK fault not reached")
	}
	if err = memory.Sync(); err != nil {
		return report, err
	}
	report.CleanupRecovered = true
	meter, err := llmops.NewMeter(llmops.Rates{Version: "teaching-v1", Input: 2, CachedInput: 1, Output: 3})
	if err != nil {
		return report, err
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		w.Header().Set("X-Request-ID", fmt.Sprintf("fixture-%d", n))
		if n == 1 {
			w.Header().Set("X-Fixture-Rejected", "before-generation")
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"Refund within seven days.\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"usage\":{\"input_tokens\":10,\"output_tokens\":2,\"input_tokens_details\":{\"cached_tokens\":4}}}}\n\n")
	}))
	defer server.Close()
	var meteringError error
	client, err := llmclient.NewLocalHTTP(server.URL, 3, func(attempt llmclient.Attempt) {
		if e := meter.Record(attempt); e != nil {
			meteringError = e
		}
	})
	if err != nil {
		return report, err
	}
	defer client.Close()
	engine, err := llmops.New(client, llmops.Config{PromptVersion: "faq1", ModelVersion: "fixture1", CorpusVersion: "refund1", TTL: time.Minute})
	if err != nil {
		return report, err
	}
	if _, _, err = engine.Answer(ctx, scope, "refund policy"); err != nil {
		return report, err
	}
	_, hit, err := engine.Answer(ctx, scope, "policy refund")
	if err != nil {
		return report, err
	}
	report.HTTPAttempts = calls.Load()
	report.CacheHitBypassedModel = hit && calls.Load() == 2
	report.Costs = meter.Snapshot()
	if meteringError != nil {
		return report, meteringError
	}
	if !report.RAGRelevant || !report.NoEvidenceRefused || !report.ScopeDenied || !report.MemoryApproved || !report.DeletedHistoryBlocked || !report.CleanupRecovered || !report.CacheHitBypassedModel || report.Costs.KnownMicroUSD != 22 {
		return report, errors.New("fixture invariant failed")
	}
	return report, nil
}
func execute() error {
	dir, err := os.MkdirTemp("", "knowledge-demo-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	report, err := run(dir)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(report)
}
func main() {
	if err := execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
