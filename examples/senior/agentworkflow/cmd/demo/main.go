// Demo uses non-secret identities, an offline model and an independent file
// provider. It performs no network/model request and no real publication.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	aw "td-homework/examples/senior/agentworkflow"
)

type plan struct{ Proposal aw.Proposal }

func (p plan) Next(_ context.Context, h []aw.Item, _ aw.Reservation) (aw.Turn, error) {
	for _, v := range h {
		if v.Kind == "function_call_output" {
			return aw.Turn{Items: []aw.Item{{Kind: "message", Text: "done"}}, Final: true, Tokens: 1, Cost: 1}, nil
		}
	}
	return aw.Turn{Items: []aw.Item{{Kind: "reasoning", Text: "NONSECRET fixture planning"}, {Kind: "function_call", Call: &aw.ToolCall{ID: "call-1", Name: "publish", Proposal: p.Proposal}}}, Tokens: 1, Cost: 1}, nil
}

type loseReply struct{ aw.FixtureProvider }

func (p loseReply) Publish(ctx context.Context, q aw.Request) (aw.Receipt, error) {
	_, err := p.FixtureProvider.Publish(ctx, q)
	if err != nil {
		return aw.Receipt{}, err
	}
	return aw.Receipt{}, context.DeadlineExceeded
}

func run() error {
	dir := flag.String("dir", "", "required disposable state directory")
	step := flag.String("step", "demo", "demo | init | plan | approve | run | reconcile | inspect")
	lost := flag.Bool("lose-reply", false, "accept publication then lose its reply (run only)")
	flag.Parse()
	if *dir == "" || flag.NArg() != 0 {
		return errors.New("use -dir DISPOSABLE_DIRECTORY")
	}
	e := aw.Engine{Store: aw.Store{Path: filepath.Join(*dir, "control.json")}}
	p := aw.FixtureProvider{Store: aw.Store{Path: filepath.Join(*dir, "external.json")}, Deduplicate: true}
	auth := aw.Authority{"worker-token": {ID: "alice", Tenant: "tenant-a"}, "review-token": {ID: "bob", Tenant: "tenant-a"}}
	actor, err := auth.Context(context.Background(), "worker-token")
	if err != nil {
		return err
	}
	reviewer, err := auth.Context(context.Background(), "review-token")
	if err != nil {
		return err
	}
	proposal := aw.Proposal{ID: "proposal-1", Intent: "intent-1", Version: 1, Binding: aw.Binding{Tenant: "tenant-a", Provider: "fixture", Account: "account-a", CredentialSubject: "subject-a", Scope: "publish", Tool: "publish", ToolVersion: "v1", WorkflowVersion: "v1", PolicyVersion: "v1"}, Text: "reviewed content"}
	var provider aw.Provider = p
	if *lost {
		if *step != "run" {
			return errors.New("-lose-reply applies only to -step run")
		}
		provider = loseReply{p}
	}
	r := aw.Runner{Engine: e, Model: plan{proposal}, Reserve: aw.Reservation{Tokens: 2, Cost: 2}, Tool: e.PublishTool(provider)}
	init := func() error {
		if _, err := os.Stat(e.Store.Path); err == nil {
			return errors.New("state already exists; use a fresh disposable directory")
		} else if !os.IsNotExist(err) {
			return err
		}
		b := proposal.Binding
		for _, g := range []aw.Grant{{Actor: "alice", Tenant: b.Tenant, Provider: b.Provider, Account: b.Account, Tool: b.Tool, Execute: true}, {Actor: "bob", Tenant: b.Tenant, Provider: b.Provider, Account: b.Account, Tool: b.Tool, Review: true}} {
			if err := e.SetGrant(g); err != nil {
				return err
			}
		}
		if err := e.SetCredential(b, aw.Credential{Subject: b.CredentialSubject, Scope: b.Scope, Token: "NONSECRET-initial", Status: "active"}); err != nil {
			return err
		}
		if err := e.Propose(actor, proposal); err != nil {
			return err
		}
		return r.Start(actor, "run-1", "publish reviewed content", aw.Limits{MaxSteps: 5, MaxCalls: 10, MaxTokens: 100, MaxCost: 100, ToolReserve: 1, Deadline: time.Now().Add(time.Hour)})
	}
	approve := func() error { return e.Approve(reviewer, proposal.ID, proposal.Digest(), time.Now().Add(time.Hour)) }
	continueRun := func() error { _, err := r.Continue(actor, "run-1"); return err }
	switch *step {
	case "demo":
		if err = init(); err != nil {
			return err
		}
		if err = continueRun(); !errors.Is(err, aw.ErrApproval) {
			return fmt.Errorf("expected approval pause, got %v", err)
		}
		if err = approve(); err != nil {
			return err
		}
		if err = continueRun(); err != nil {
			return err
		}
	case "init":
		err = init()
	case "plan":
		err = continueRun()
		if errors.Is(err, aw.ErrApproval) {
			err = nil
		}
	case "approve":
		err = approve()
	case "run":
		err = continueRun()
		if errors.Is(err, aw.ErrUnknown) {
			err = nil
		}
	case "reconcile":
		err = e.Reconcile(actor, proposal.ID, p)
	case "inspect":
	default:
		return errors.New("unknown step")
	}
	if err != nil {
		return err
	}
	j, err := e.Job(proposal.ID)
	if err != nil {
		return err
	}
	snapshot, err := r.Snapshot(actor, "run-1")
	if err != nil {
		return err
	}
	count, err := p.Count()
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"job_status": j.Status, "run_status": snapshot.Status, "external_effects": count, "model_requests": snapshot.Steps, "tool_attempts": snapshot.Calls, "reserved_tokens": snapshot.Tokens, "reserved_cost": snapshot.Cost, "receipt": j.Receipt})
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
