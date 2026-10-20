package releaseadmission

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"
)

type Ticket struct{ ID string }
type Receipt struct {
	Intent, TargetDigest, Policy string
	Sequence                     int
}
type pending struct {
	Intent  string
	Epoch   uint64
	Bundle  Bundle
	Binding string
}
type Gate struct {
	mu                                     sync.Mutex
	policy                                 Policy
	target                                 Target
	root                                   string
	now                                    func() time.Time
	epoch                                  uint64
	keys, approvals, exceptions, artifacts map[string]bool
	versions                               map[string]bool
	tickets                                map[string]pending
	intents                                map[string]string
	receipts                               map[string]Receipt
}

func copyValue[T any](v T) T {
	b, _ := json.Marshal(v)
	var out T
	_ = json.Unmarshal(b, &out)
	return out
}
func NewGate(p Policy, t Target, inputRoot string, now func() time.Time) (*Gate, error) {
	if !p.valid() {
		return nil, ErrDenied
	}
	if _, ok := p.artifact(t); !ok {
		return nil, ErrDenied
	}
	if now == nil {
		now = time.Now
	}
	return &Gate{policy: copyValue(p), target: t, root: inputRoot, now: now, epoch: 1, keys: map[string]bool{}, approvals: map[string]bool{}, exceptions: map[string]bool{}, artifacts: map[string]bool{}, versions: map[string]bool{p.Version: true}, tickets: map[string]pending{}, intents: map[string]string{}, receipts: map[string]Receipt{}}, nil
}

// Administration is trusted host input, NOT an HTTP API or model tool.
// Every change fences outstanding tickets, including change-away-and-back (ABA).
func (g *Gate) SetTarget(t Target) { g.mu.Lock(); defer g.mu.Unlock(); g.target = t; g.epoch++ }
func (g *Gate) SetPolicy(p Policy) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !p.valid() || g.versions[p.Version] {
		return ErrDenied
	}
	g.policy = copyValue(p)
	g.versions[p.Version] = true
	g.epoch++
	return nil
}
func (g *Gate) RevokeKey(id string) { g.mu.Lock(); defer g.mu.Unlock(); g.keys[id] = true; g.epoch++ }
func (g *Gate) RevokeApproval(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.approvals[id] = true
	g.epoch++
}
func (g *Gate) RevokeException(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.exceptions[id] = true
	g.epoch++
}
func (g *Gate) RevokeArtifact(digest string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.artifacts[digest] = true
	g.epoch++
}
func bounded(b Bundle) bool {
	if len(b.Artifact) == 0 || len(b.Artifact) > 1<<20 || len(b.SBOMReport) > 128<<10 || len(b.ScanReport) > 128<<10 {
		return false
	}
	envelopes := []Envelope{b.Provenance, b.Tests, b.SBOM, b.Scan, b.Approval}
	if b.Exception != nil {
		envelopes = append(envelopes, *b.Exception)
	}
	for _, e := range envelopes {
		if len(e.Payload) > 128<<10 || len(e.Signatures) > 1 {
			return false
		}
	}
	return true
}

// validate runs every hard check before considering a narrowly scoped CVE waiver.
// Reports are prefetched, authenticated offline evidence. Missing bytes or a failed
// fetch/scan cannot be represented as a successful empty report.
func (g *Gate) validate(ctx context.Context, b Bundle) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !bounded(b) {
		return ErrDenied
	}
	p, t, now := g.policy, g.target, g.now().Unix()
	d, ok := p.artifact(t)
	if !ok || Digest(b.Artifact) != d || g.artifacts[d] {
		return ErrDenied
	}
	inputs, err := Fingerprint(ctx, g.root)
	if err != nil || inputs != t.Inputs {
		return ErrDenied
	}
	run, err := p.provenance(b.Provenance, t, now, g.keys)
	if err != nil {
		return err
	}
	if err = p.tests(b.Tests, t, run, now, g.keys); err != nil {
		return err
	}
	if err = p.sbom(b, t, now, g.keys); err != nil {
		return err
	}
	risks, err := p.scan(b, t, now, g.keys)
	if err != nil {
		return err
	}
	if err = p.approval(b, t, now, g.keys, g.approvals); err != nil {
		return err
	}
	if err = p.exception(b, t, risks, now, g.keys, g.exceptions); err != nil {
		return err
	}
	return ctx.Err()
}
func (g *Gate) Prepare(ctx context.Context, intent string, b Bundle) (Ticket, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if intent == "" || len(intent) > 128 || len(g.tickets) >= 128 {
		return Ticket{}, ErrDenied
	}
	if err := g.validate(ctx, b); err != nil {
		return Ticket{}, err
	}
	x := binding([]any{g.target, g.policy.Version, b})
	if old, ok := g.intents[intent]; ok && old != x {
		return Ticket{}, ErrDenied
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return Ticket{}, err
	}
	id := hex.EncodeToString(nonce[:])
	g.tickets[id] = pending{intent, g.epoch, copyValue(b), x}
	g.intents[intent] = x
	return Ticket{id}, nil
}

// Commit models an atomic LAST-MILE dry-run admission, not kubectl, a registry
// push or a blockchain transaction. The same rechecks apply to retries/rollback.
// A production executor must enforce the binding/fence at its own effect boundary.
func (g *Gate) Commit(ctx context.Context, t Ticket) (Receipt, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	x, ok := g.tickets[t.ID]
	if !ok || x.Epoch != g.epoch {
		return Receipt{}, ErrDenied
	}
	if err := g.validate(ctx, x.Bundle); err != nil {
		return Receipt{}, err
	}
	if binding([]any{g.target, g.policy.Version, x.Bundle}) != x.Binding {
		return Receipt{}, ErrDenied
	}
	if r, ok := g.receipts[x.Intent]; ok {
		return r, nil
	}
	r := Receipt{x.Intent, TargetDigest(g.target), g.policy.Version, len(g.receipts) + 1}
	g.receipts[x.Intent] = r
	return r, nil
}
func (g *Gate) Effects() int { g.mu.Lock(); defer g.mu.Unlock(); return len(g.receipts) }
