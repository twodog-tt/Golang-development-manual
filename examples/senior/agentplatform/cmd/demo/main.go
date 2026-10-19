package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"

	p "td-homework/examples/senior/agentplatform"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func fingerprint(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

type reader struct{ evidence p.PaymentEvidence }

func (r reader) Lookup(string) (p.PaymentEvidence, error) { return r.evidence, nil }

func main() {
	ctx := context.Background()
	dir, err := os.MkdirTemp("", "agentplatform-demo-")
	must(err)
	defer os.RemoveAll(dir)
	b := p.Binding{Tenant: "tenant-a", Version: "v1", Artifact: fingerprint("artifact"), Manifest: fingerprint("manifest"), Runtime: fingerprint("runtime"), Config: fingerprint("config"), Permissions: fingerprint("read"), Policy: fingerprint("policy"), Environment: "prod"}
	c := p.NewControl()
	must(c.Register(b))
	must(c.Approve(b))
	must(c.Deploy(b))
	changed := b
	changed.Config = fingerprint("changed")
	approvalBound := c.Deploy(changed) != nil
	epoch, err := c.Claim("old-worker", b)
	must(err)
	must(c.Stop(b, false))
	oldWorkerBlocked := c.Submit("old-worker", epoch) != nil
	nativeRejected := p.Admit(p.Plugin{Mode: "native", Operation: "report", Target: "local-report"}) != nil
	_, err = p.Broker(p.Plugin{Mode: "declarative-v1", Operation: "report", Target: "local-report"}, "read:local-report")
	must(err)
	seed := sha256.Sum256([]byte("public local delegation seed"))
	private := ed25519.NewKeyFromSeed(seed[:])
	d := p.NewDelegator("platform", map[string]ed25519.PublicKey{"platform": private.Public().(ed25519.PublicKey)})
	g := p.Grant{ID: "root", Issuer: "platform", Subject: "agent", Owner: "buyer", Tenant: "tenant-a", Audience: "report-api", Resource: "report", Scopes: []string{"invoke"}, Deadline: 200, Budget: 8}
	chain := []p.SignedGrant{p.SignGrant(g, private)}
	actor := p.Principal{Tenant: "tenant-a", Subject: "agent"}
	must(d.Reserve(chain, actor, "report-api", "report", "invoke", "first", 8, 100))
	budgetBlocked := d.Reserve(chain, actor, "report-api", "report", "invoke", "second", 1, 100) != nil
	o := p.Offer{ID: "report", Revision: "v1", Seller: "seller", Capability: fingerprint("capability"), Permissions: b.Permissions, Asset: "local-chain/usd", Unit: "call", Terms: "terms-v1", Price: 10, Quota: 1}
	buyer := p.Principal{Tenant: "tenant-a", Subject: "buyer"}
	m := p.NewMarket(nil)
	must(m.Publish(o))
	q, err := m.Quote("q", buyer, o, 200, 100)
	must(err)
	// Trusted local receipt adapter binds the exact server-side quote snapshot.
	m = p.NewMarket(reader{p.PaymentEvidence{Domain: o.Asset, Transaction: "tx", Event: "0", QuoteDigest: p.QuoteDigest(q), Block: "block", Canonical: true, Final: true}})
	must(m.Publish(o))
	q, err = m.Quote("q", buyer, o, 200, 100)
	must(err)
	must(m.Buy(buyer, q.ID, "receipt", 100))
	must(m.Invoke(buyer, q.ID, "call", o))
	must(m.Invoke(buyer, q.ID, "call", o))
	newOffer := o
	newOffer.Revision = "v2"
	newOffer.Permissions = "sign"
	offerBound := m.Invoke(buyer, q.ID, "call-new", newOffer) != nil
	remote := p.LocalRemote{Path: filepath.Join(dir, "remote.json"), LoseReply: true}
	launch := p.Launcher{Path: filepath.Join(dir, "launch.json"), Remote: remote}
	s := p.LaunchSpec{Tenant: buyer.Tenant, ID: "launch", Chain: "local-chain", Account: "creator", TokenConfig: "config", Liquidity: "pool", Runtime: "runtime"}
	must(launch.Prepare(s))
	replyLost := launch.Advance(ctx, s) != nil
	remote.LoseReply = false
	launch = p.Launcher{Path: launch.Path, Remote: remote}
	must(launch.Reconcile(ctx, s))
	for i := 0; i < 3; i++ {
		must(launch.Advance(ctx, s))
	}
	n, err := remote.Effects()
	must(err)
	balance, err := launch.FeeBalance()
	must(err)
	snap, err := launch.Snapshot(s)
	must(err)
	must(remote.Orphan(snap.Steps[0].Receipt.Key))
	_ = launch.Reconcile(ctx, s)
	after, err := launch.FeeBalance()
	must(err)
	snap, err = launch.Snapshot(s)
	must(err)
	r := p.NewReputation()
	e := p.ReviewEvidence{Job: "job", BuyerActor: "buyer-actor", SellerActor: "seller-actor", PaymentKey: "chain/tx/0", Paid: true, Delivered: true, Canonical: true, Final: true, Revision: 1}
	must(r.Observe(e))
	must(r.Add(p.Review{ID: "review", ReviewerActor: e.BuyerActor, Job: e.Job, Value: 5}))
	e.Refunded = true
	e.Revision++
	must(r.Observe(e))
	refundedExcluded := r.Count() == 0
	passed := approvalBound && oldWorkerBlocked && nativeRejected && budgetBlocked && offerBound && replyLost && n == 3 && balance == 10 && after == 0 && snap.State == "review_chain" && refundedExcluded
	must(json.NewEncoder(os.Stdout).Encode(map[string]any{"passed": passed, "approval_bound": approvalBound, "old_worker_blocked": oldWorkerBlocked, "native_plugin_rejected": nativeRejected, "budget_blocked": budgetBlocked, "offer_bound": offerBound, "lost_reply_recovered": replyLost, "launch_effects": n, "fee_before_reorg": balance, "fee_after_reorg": after, "launch_state": snap.State, "refunded_review_excluded": refundedExcluded}))
	if !passed {
		os.Exit(1)
	}
}
