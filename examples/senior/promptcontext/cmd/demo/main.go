package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"td-homework/examples/senior/agentmemory"
	"td-homework/examples/senior/llmclient"
	pc "td-homework/examples/senior/promptcontext"
	"time"
)

func run() error {
	scope := agentmemory.Scope{Tenant: "demo", Bot: "orders", Scene: "query", SubjectType: "user", SubjectID: "alice"}
	auth := agentmemory.NewAuthority()
	auth.Put("fixture", agentmemory.Grant{Tenant: scope.Tenant, Subject: "alice", Policy: "v1", Scopes: []agentmemory.Scope{scope}, Roles: []string{"read"}, Expires: time.Now().Add(time.Minute)})
	ctx, e := auth.Context(context.Background(), "fixture")
	if e != nil {
		return e
	}
	groups := []pc.Group{{Scope: scope, ID: "policy", Required: true, Items: []pc.Item{{Kind: "system", Text: pc.SystemPrompt}}}, {Scope: scope, ID: "old-tool", Items: []pc.Item{{Kind: "reasoning", Text: "opaque fixture"}, {Kind: "call", CallID: "call1", Text: "lookup ORD-1"}, {Kind: "result", CallID: "call1", Text: "old paid order"}}}, {Scope: scope, ID: "current", Required: true, Items: []pc.Item{{Kind: "current_user", Text: "query ORD-1"}}}}
	counter := pc.FixtureCounter{}
	required := append(append([]pc.Item(nil), groups[0].Items...), groups[2].Items...)
	n, e := counter.Count(ctx, required)
	if e != nil {
		return e
	}
	b := pc.Budget{Window: n + 40, Output: 20, Reasoning: 10, MaxOutput: 30, Margin: 10}
	assembly, e := pc.Assemble(ctx, scope, groups, b, counter)
	if e != nil {
		return e
	}
	model := pc.LocalFixture{Model: &llmclient.MockClient{Reply: `{"action":"query","order_id":"ORD-1"}`}, Version: pc.PromptVersion, Status: "completed"}
	intent, e := pc.ExtractAssembled(ctx, scope, model, assembly)
	if e != nil {
		return e
	}
	model.Status = "incomplete"
	_, incomplete := pc.ExtractAssembled(ctx, scope, model, assembly)
	_, bad := pc.ParseIntent(`{"action":"query","action":"refund","order_id":"ORD-1"}`)
	pass := len(assembly.Dropped) == 1 && assembly.Dropped[0] == "old-tool" && intent.OrderID == "ORD-1" && incomplete != nil && bad != nil
	if !pass {
		return fmt.Errorf("prompt fixture invariant failed")
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"passed": pass, "counter": assembly.CounterVersion, "input_units": assembly.Input, "dropped": assembly.Dropped, "strict_intent": intent, "incomplete_rejected": incomplete != nil, "duplicate_rejected": bad != nil, "effects_executed": 0})
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
