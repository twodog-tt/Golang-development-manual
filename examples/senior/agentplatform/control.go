// Package agentplatform is a local control-plane fixture, not a hosted platform.
// Administrative methods and Principal values are supplied by trusted ingress.
package agentplatform

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
)

var (
	ErrDenied  = errors.New("denied or conflicting binding")
	ErrUnknown = errors.New("unknown effect: query evidence, do not resubmit")
)

func hash(v any) string      { b, _ := json.Marshal(v); return digest(b) }
func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// Binding contains fingerprints computed by trusted build/deployment ingress,
// never hashes asserted by the model. Version is a logical immutable identifier.
type Binding struct {
	Tenant, Version, Artifact, Manifest, Runtime, Config, Permissions, Policy, Environment string
}

func (b Binding) valid() bool {
	if b.Tenant == "" || b.Version == "" || b.Environment == "" {
		return false
	}
	for _, s := range []string{b.Artifact, b.Manifest, b.Runtime, b.Config, b.Permissions, b.Policy} {
		x, e := hex.DecodeString(s)
		if e != nil || len(x) != 32 {
			return false
		}
	}
	return true
}

type release struct {
	Binding Binding
	State   string
}
type ticket struct {
	Binding    Binding
	Generation uint64
	State      string
	Receipt    string
}
type Control struct {
	mu         sync.Mutex
	versions   map[string]release
	approvals  map[string]Binding
	active     Binding
	generation uint64
	jobs       map[string]ticket
}

func NewControl() *Control {
	return &Control{versions: map[string]release{}, approvals: map[string]Binding{}, jobs: map[string]ticket{}}
}
func versionKey(b Binding) string { return hash([]string{b.Tenant, b.Version}) }
func sameVersion(a, b Binding) bool {
	return a.Tenant == b.Tenant && a.Version == b.Version && a.Artifact == b.Artifact && a.Manifest == b.Manifest && a.Runtime == b.Runtime && a.Permissions == b.Permissions
}
func (c *Control) Register(b Binding) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !b.valid() {
		return ErrDenied
	}
	k := versionKey(b)
	if old, ok := c.versions[k]; ok {
		if !sameVersion(old.Binding, b) {
			return ErrDenied
		}
		return nil
	}
	c.versions[k] = release{Binding: b, State: "reviewing"}
	return nil
}
func (c *Control) Approve(b Binding) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	r, ok := c.versions[versionKey(b)]
	if !b.valid() || !ok || !sameVersion(r.Binding, b) || (r.State != "reviewing" && r.State != "suspended" && r.State != "published") {
		return ErrDenied
	}
	c.approvals[hash(b)] = b
	r.State = "published"
	c.versions[versionKey(b)] = r
	return nil
}
func (c *Control) Deploy(b Binding) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	r, ok := c.versions[versionKey(b)]
	approved, hasApproval := c.approvals[hash(b)]
	if !ok || !sameVersion(r.Binding, b) || !hasApproval || approved != b || r.State != "published" {
		return ErrDenied
	}
	// Switching versions always fences old claims, even when switching back (ABA).
	c.generation++
	c.active = b
	return nil
}
func (c *Control) Stop(b Binding, revoke bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	r, ok := c.versions[versionKey(b)]
	if !ok || !sameVersion(r.Binding, b) {
		return ErrDenied
	}
	if r.State == "revoked" {
		return ErrDenied
	}
	r.State = "suspended"
	if revoke {
		r.State = "revoked"
	}
	for key, approved := range c.approvals {
		if versionKey(approved) == versionKey(b) {
			delete(c.approvals, key)
		}
	}
	c.versions[versionKey(b)] = r
	if versionKey(c.active) == versionKey(b) {
		c.generation++
		c.active = Binding{}
	}
	// Submitted effects cannot be undone by a control-plane stop.
	for id, j := range c.jobs {
		if versionKey(j.Binding) == versionKey(b) && j.State == "submitted" {
			j.State = "unknown"
			c.jobs[id] = j
		}
	}
	return nil
}
func (c *Control) Claim(id string, b Binding) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if id == "" || !b.valid() || c.active != b {
		return 0, ErrDenied
	}
	if _, ok := c.jobs[id]; ok {
		return 0, ErrDenied
	}
	c.jobs[id] = ticket{Binding: b, Generation: c.generation, State: "claimed"}
	return c.generation, nil
}

// Submit models the SIGNER-SIDE atomic authorization/effect-admission point.
// In production this generation must be enforced at the signer, not only worker.
func (c *Control) Submit(id string, epoch uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	j, ok := c.jobs[id]
	if !ok || j.State != "claimed" || epoch != j.Generation || epoch != c.generation || j.Binding != c.active {
		return ErrDenied
	}
	j.State = "submitted"
	c.jobs[id] = j
	return nil
}
func (c *Control) Cancel(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	j, ok := c.jobs[id]
	if !ok {
		return ErrDenied
	}
	if j.State == "submitted" || j.State == "unknown" {
		j.State = "unknown"
	} else if j.State == "claimed" {
		j.State = "canceled"
	}
	c.jobs[id] = j
	return nil
}
func (c *Control) JobState(id string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.jobs[id].State
}

// EffectEvidence is supplied ONLY by the trusted read-only reconciler, not by
// a worker's success flag. Revocation must not disable this recovery channel.
type EffectEvidence struct {
	Job                     string
	Binding                 Binding
	Generation              uint64
	Transaction             string
	Known, Canonical, Final bool
}

func (c *Control) Reconcile(e EffectEvidence) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	j, ok := c.jobs[e.Job]
	if !ok || j.Binding != e.Binding || j.Generation != e.Generation || !e.Known || !e.Canonical || !e.Final || e.Transaction == "" {
		return ErrDenied
	}
	if j.State == "confirmed" {
		if j.Receipt == e.Transaction {
			return nil
		}
		return ErrDenied
	}
	if j.State != "unknown" && j.State != "submitted" {
		return ErrDenied
	}
	j.State = "confirmed"
	j.Receipt = e.Transaction
	c.jobs[e.Job] = j
	return nil
}

// Plugin admission is deliberately closed: only a fixed declarative operation
// is interpreted. Arbitrary third-party native code is NEVER loaded in-process.
type Plugin struct{ Mode, Operation, Target string }

func Admit(p Plugin) error {
	if p.Mode != "declarative-v1" || p.Operation != "report" || p.Target != "local-report" {
		return ErrDenied
	}
	return nil
}
func Broker(p Plugin, capability string) (string, error) {
	if err := Admit(p); err != nil {
		return "", err
	}
	if capability != "read:local-report" {
		return "", ErrDenied
	}
	return "bounded local report", nil
}
