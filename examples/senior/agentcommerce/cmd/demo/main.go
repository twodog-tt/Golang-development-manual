package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	c "td-homework/examples/senior/agentcommerce"
	"td-homework/examples/senior/agentmemory"
)

func run() error {
	dir, err := os.MkdirTemp("", "agent-commerce-demo-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	scope := agentmemory.Scope{Tenant: "demo", Bot: "buyer", Scene: "reports", SubjectType: "user", SubjectID: "alice"}
	a := agentmemory.NewAuthority()
	a.Put("local-token", agentmemory.Grant{Tenant: "demo", Subject: "alice", Policy: "commerce-v1", Scopes: []agentmemory.Scope{scope}, Roles: []string{"pay"}, Expires: time.Now().Add(time.Hour)})
	ctx, err := a.Context(context.Background(), "local-token")
	if err != nil {
		return err
	}
	b, err := c.NewBinding("demo", "alice", "report-1", "POST", "https://reports.fixture/report", []byte(`{"topic":"go"}`))
	if err != nil {
		return err
	}
	seed := sha256.Sum256([]byte("public teaching payment key; NEVER fund"))
	key, err := crypto.ToECDSA(seed[:])
	if err != nil {
		return err
	}
	now := time.Now()
	payment := c.Payment{From: crypto.PubkeyToAddress(key.PublicKey), To: c.FixtureMerchant, Value: "10", ValidAfter: now.Add(-time.Minute).Unix(), ValidBefore: now.Add(time.Minute).Unix(), Nonce: crypto.Keccak256Hash([]byte("local-nonce"))}
	mechanism := c.FixtureMechanism()
	digest, err := c.PaymentDigest(mechanism, payment)
	if err != nil {
		return err
	}
	payment.Signature, err = crypto.Sign(digest[:], key)
	if err != nil {
		return err
	}
	chain := c.LocalChain{Dir: dir, LostReply: true}
	engine := c.Engine{Dir: dir, Payer: payment.From, Chain: chain}
	if err = engine.Prepare(ctx, scope, b, mechanism, payment); err != nil {
		return err
	}
	swapped := b
	swapped.BodyHash = sha256.Sum256([]byte("other request"))
	_, swapErr := engine.Execute(ctx, scope, swapped)
	_, lost := engine.Execute(ctx, scope, b)
	_, retry := engine.Execute(ctx, scope, b)
	engine = c.Engine{Dir: dir, Payer: payment.From, Chain: c.LocalChain{Dir: dir}}
	if err = engine.Recover(ctx, scope, b.Intent); err != nil {
		return err
	}
	result, err := engine.Execute(ctx, scope, b)
	if err != nil {
		return err
	}
	effects, err := chain.Effects()
	if err != nil {
		return err
	}
	entries, err := engine.Entries(ctx, scope, b.Intent)
	if err != nil {
		return err
	}
	flows := map[string][]string{}
	for _, flow := range []string{"authorization", "upfront", "escrow"} {
		name := flow
		if err = c.RunFlow(ctx, flow, func(_ context.Context, stage string) error { flows[name] = append(flows[name], stage); return nil }); err != nil {
			return err
		}
	}
	job, err := c.NewJob("client", "provider", "evaluator", 100, 1)
	if err != nil {
		return err
	}
	if err = job.SetBudget("provider", 20); err != nil {
		return err
	}
	staleBudget := job.Fund("client", 10, 10)
	if err = job.Fund("client", 20, 10); err != nil {
		return err
	}
	if err = job.Submit("provider", "content-digest"); err != nil {
		return err
	}
	job.InjectHook(c.HookFault{Before: true, After: true})
	if err = job.Refund("watcher", 100); err != nil {
		return err
	}
	lateComplete := job.Complete("evaluator")
	state := job.Snapshot()
	pass := swapErr != nil && errors.Is(lost, c.ErrUnknown) && errors.Is(retry, c.ErrUnknown) && result != "" && effects == 1 && len(entries) == 1 && staleBudget != nil && lateComplete != nil && state.Refunded == 20 && state.Paid == 0
	if !pass {
		return fmt.Errorf("commerce invariant failed")
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"passed": pass, "request_swap_rejected": swapErr != nil, "unknown_blocked_retry": errors.Is(retry, c.ErrUnknown), "settlements": effects, "ledger_entries": len(entries), "flow_stages": flows, "budget_change_rejected": staleBudget != nil, "expiry_refund": state.Refunded, "hook_cannot_block_refund": true})
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
