package agentworkflow

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	ErrDenied   = errors.New("authorization denied")
	ErrApproval = errors.New("approval required or stale")
	ErrConflict = errors.New("state or payload conflict")
	ErrLease    = errors.New("stale or expired lease")
	ErrUnknown  = errors.New("unknown external outcome: reconcile before continuing")
	ErrBudget   = errors.New("run budget exhausted")
)

type identityKey struct{}
type Principal struct{ ID, Tenant string }

// Authority is a fixture authenticator. Bearer values and identities are
// server-owned configuration, never model args or reviewer fields in a request.
type Authority map[string]Principal

func (a Authority) Context(ctx context.Context, bearer string) (context.Context, error) {
	p, ok := a[bearer]
	if !ok || p.ID == "" || p.Tenant == "" {
		return nil, ErrDenied
	}
	return context.WithValue(ctx, identityKey{}, p), nil
}
func identity(ctx context.Context) (Principal, error) {
	p, ok := ctx.Value(identityKey{}).(Principal)
	if !ok {
		return p, ErrDenied
	}
	return p, nil
}

type Grant struct {
	Actor, Tenant, Provider, Account, Tool string
	Review, Execute                        bool
}

func grantKey(actor, tenant, provider, account, tool string) string {
	b, _ := json.Marshal([]string{actor, tenant, provider, account, tool})
	return string(b)
}
func allowed(s *state, p Principal, b Binding, review bool) bool {
	g := s.Grants[grantKey(p.ID, p.Tenant, b.Provider, b.Account, b.Tool)]
	return p.Tenant == b.Tenant && g.Actor == p.ID && ((review && g.Review) || (!review && g.Execute))
}

// Binding is immutable after dispatch. Token generation is not identity: normal
// refresh can keep the same subject, whereas reauthorization changes Subject.
type Binding struct {
	Tenant, Provider, Account, CredentialSubject, Scope string
	Tool, ToolVersion, WorkflowVersion, PolicyVersion   string
}
type Proposal struct {
	ID, Intent string
	Version    int
	Binding    Binding
	Text       string
}

func (p Proposal) Digest() string {
	b, _ := json.Marshal(p)
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

type Approval struct {
	Digest, Reviewer string
	Expires          time.Time
	Revoked          bool
}
type Job struct {
	Proposal   Proposal
	Requester  string
	Approval   *Approval
	Status     string
	Owner      string
	Epoch      uint64
	LeaseUntil time.Time
	Dispatched bool
	Receipt    string
	Audit      []string
}
type Claim struct {
	ID, Owner string
	Epoch     uint64
}
type Engine struct {
	Store Store
	Now   func() time.Time
}

func (e Engine) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now().UTC()
}

// Configuration methods are operator-only fixture setup, not public endpoints.
func (e Engine) SetGrant(g Grant) error {
	return e.Store.transaction(func(s *state) error {
		s.Grants[grantKey(g.Actor, g.Tenant, g.Provider, g.Account, g.Tool)] = g
		return nil
	})
}
func credentialKey(b Binding) string {
	v, _ := json.Marshal([]string{b.Tenant, b.Provider, b.Account})
	return string(v)
}
func credentialOK(s *state, b Binding) bool {
	c := s.Credentials[credentialKey(b)]
	return c != nil && c.Status == "active" && c.Subject == b.CredentialSubject && c.Scope == b.Scope
}
func (e Engine) Propose(ctx context.Context, p Proposal) error {
	a, err := identity(ctx)
	if err != nil {
		return err
	}
	return e.Store.transaction(func(s *state) error {
		if p.ID == "" || p.Intent == "" || p.Version != 1 || p.Text == "" || p.Binding.Tool != "publish" || p.Binding.ToolVersion != "v1" || p.Binding.WorkflowVersion != "v1" || p.Binding.PolicyVersion != "v1" {
			return ErrConflict
		}
		if !allowed(s, a, p.Binding, false) || !credentialOK(s, p.Binding) {
			return ErrDenied
		}
		if old := s.Jobs[p.ID]; old != nil {
			if old.Proposal == p && old.Requester == a.ID {
				return nil
			}
			return ErrConflict
		}
		for _, old := range s.Jobs {
			if old.Proposal.Binding.Tenant == p.Binding.Tenant && old.Proposal.Binding.Provider == p.Binding.Provider && old.Proposal.Binding.Account == p.Binding.Account && old.Proposal.Intent == p.Intent {
				return ErrConflict
			}
		}
		s.Jobs[p.ID] = &Job{Proposal: p, Requester: a.ID, Status: "pending", Audit: []string{"proposed"}}
		return nil
	})
}
func (e Engine) Job(id string) (Job, error) {
	var out Job
	err := e.Store.transaction(func(s *state) error {
		j := s.Jobs[id]
		if j == nil {
			return ErrConflict
		}
		out = *j
		return nil
	})
	return out, err
}
func (e Engine) Approve(ctx context.Context, id, digest string, expires time.Time) error {
	a, err := identity(ctx)
	if err != nil {
		return err
	}
	return e.Store.transaction(func(s *state) error {
		j := s.Jobs[id]
		if j == nil {
			return ErrConflict
		}
		if !allowed(s, a, j.Proposal.Binding, true) || a.ID == j.Requester {
			return ErrDenied
		}
		if j.Status != "pending" || digest != j.Proposal.Digest() || !expires.After(e.now()) {
			return ErrApproval
		}
		j.Approval = &Approval{Digest: digest, Reviewer: a.ID, Expires: expires}
		j.Status = "ready"
		j.Audit = append(j.Audit, "approved:"+a.ID)
		return nil
	})
}
func (e Engine) Edit(ctx context.Context, id string, p Proposal) error {
	a, err := identity(ctx)
	if err != nil {
		return err
	}
	return e.Store.transaction(func(s *state) error {
		j := s.Jobs[id]
		if j == nil {
			return ErrConflict
		}
		if !allowed(s, a, j.Proposal.Binding, false) || a.ID != j.Requester {
			return ErrDenied
		}
		if (j.Status != "pending" && j.Status != "ready") || j.Dispatched || p.ID != id || p.Intent != j.Proposal.Intent || p.Binding != j.Proposal.Binding || p.Version != j.Proposal.Version+1 || p.Text == "" {
			return ErrConflict
		}
		j.Proposal = p
		j.Approval = nil
		j.Status = "pending"
		j.Audit = append(j.Audit, "edited:approval-invalidated")
		return nil
	})
}
func (e Engine) Revoke(ctx context.Context, id string) error {
	a, err := identity(ctx)
	if err != nil {
		return err
	}
	return e.Store.transaction(func(s *state) error {
		j := s.Jobs[id]
		if j == nil {
			return ErrConflict
		}
		if !allowed(s, a, j.Proposal.Binding, true) {
			return ErrDenied
		}
		if j.Approval != nil {
			j.Approval.Revoked = true
		}
		if j.Dispatched && j.Status != "succeeded" && j.Status != "failed" {
			j.Status = "unknown"
		} else if !j.Dispatched {
			j.Status = "pending"
		}
		j.Audit = append(j.Audit, "revoked:"+a.ID+":in-flight-effects-not-undone")
		return nil
	})
}
func (e Engine) approvalOK(s *state, j *Job) bool {
	a := j.Approval
	b := j.Proposal.Binding
	return a != nil && !a.Revoked && a.Digest == j.Proposal.Digest() && a.Expires.After(e.now()) && allowed(s, Principal{a.Reviewer, b.Tenant}, b, true) && credentialOK(s, b)
}
func (e Engine) Claim(ctx context.Context, id string, lease time.Duration) (Claim, error) {
	a, err := identity(ctx)
	if err != nil {
		return Claim{}, err
	}
	var token Claim
	err = e.Store.transaction(func(s *state) error {
		j := s.Jobs[id]
		if j == nil {
			return ErrConflict
		}
		if !allowed(s, a, j.Proposal.Binding, false) {
			return ErrDenied
		}
		if j.Status != "ready" || lease <= 0 {
			return ErrConflict
		}
		if !e.approvalOK(s, j) {
			return ErrApproval
		}
		j.Epoch++
		j.Owner = a.ID
		j.LeaseUntil = e.now().Add(lease)
		j.Status = "claimed"
		token = Claim{id, a.ID, j.Epoch}
		return nil
	})
	return token, err
}
func (e Engine) validClaim(j *Job, c Claim) bool {
	return j != nil && j.Owner == c.Owner && j.Epoch == c.Epoch && j.LeaseUntil.After(e.now())
}

type Request struct {
	Intent, Digest string
	Binding        Binding
	Text           string
}
type Receipt struct {
	ID, Intent, Digest string
	Binding            Binding
}
type Provider interface {
	Publish(context.Context, Request) (Receipt, error)
	Lookup(context.Context, Request) (Receipt, bool, error)
	Idempotent() bool // fixture contract: retention covers the entire recovery window
}

var ErrRejected = errors.New("provider proved non-execution and closed the request")

func request(j Job) Request {
	return Request{j.Proposal.Intent, j.Proposal.Digest(), j.Proposal.Binding, j.Proposal.Text}
}
func matches(q Request, r Receipt) bool {
	return r.ID != "" && r.Intent == q.Intent && r.Digest == q.Digest && r.Binding == q.Binding
}

// Dispatch durably records uncertainty BEFORE sending. A crash at either side
// of the call is recoverable without assuming it never reached the provider.
func (e Engine) Dispatch(ctx context.Context, c Claim, p Provider) error {
	a, err := identity(ctx)
	if err != nil {
		return err
	}
	var q Request
	err = e.Store.transaction(func(s *state) error {
		j := s.Jobs[c.ID]
		if !e.validClaim(j, c) || a.ID != c.Owner {
			return ErrLease
		}
		if j.Status != "claimed" {
			return ErrConflict
		}
		if !allowed(s, a, j.Proposal.Binding, false) {
			return ErrDenied
		}
		if !e.approvalOK(s, j) {
			return ErrApproval
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		j.Status = "unknown"
		j.Dispatched = true
		j.Audit = append(j.Audit, "dispatch-started")
		q = request(*j)
		return nil
	})
	if err != nil {
		return err
	}
	r, callErr := p.Publish(ctx, q)
	return e.Store.transaction(func(s *state) error {
		j := s.Jobs[c.ID]
		if !e.validClaim(j, c) {
			return ErrLease
		}
		// Reconciliation may already have confirmed success while this request
		// was in flight. A late transport/attempt failure must not downgrade it.
		if j.Status == "succeeded" {
			return nil
		}
		if callErr != nil {
			if errors.Is(callErr, ErrRejected) {
				j.Status = "failed"
				return nil
			}
			return ErrUnknown
		}
		if !matches(q, r) {
			return ErrUnknown
		}
		j.Status = "succeeded"
		j.Receipt = r.ID
		j.Audit = append(j.Audit, "provider-receipt")
		return nil
	})
}

// RecoverExpired fences local completion. It does not fence the provider.
// An undispached claim can return to ready; any dispatched claim stays unknown.
func (e Engine) RecoverExpired(ctx context.Context, id string) error {
	a, err := identity(ctx)
	if err != nil {
		return err
	}
	return e.Store.transaction(func(s *state) error {
		j := s.Jobs[id]
		if j == nil {
			return ErrConflict
		}
		if !allowed(s, a, j.Proposal.Binding, false) {
			return ErrDenied
		}
		if (j.Status != "claimed" && j.Status != "unknown") || j.LeaseUntil.After(e.now()) {
			return ErrConflict
		}
		j.Epoch++
		j.Owner = ""
		j.LeaseUntil = time.Time{}
		if j.Dispatched {
			j.Status = "unknown"
		} else {
			j.Status = "ready"
		}
		j.Audit = append(j.Audit, "lease-recovered")
		return nil
	})
}

// Reconcile may record a historical success even after approval revocation.
// Not-found is never proof that an older in-flight request cannot still succeed.
func (e Engine) Reconcile(ctx context.Context, id string, p Provider) error {
	a, err := identity(ctx)
	if err != nil {
		return err
	}
	j, err := e.Job(id)
	if err != nil {
		return err
	}
	var q Request
	err = e.Store.transaction(func(s *state) error {
		current := s.Jobs[id]
		if !allowed(s, a, current.Proposal.Binding, false) {
			return ErrDenied
		}
		if current.Status == "succeeded" {
			return nil
		}
		if current.Status != "unknown" && current.Status != "manual" {
			return ErrConflict
		}
		q = request(*current)
		return nil
	})
	if err != nil {
		return err
	}
	if q.Intent == "" {
		return nil
	}
	r, found, err := p.Lookup(ctx, q)
	if err != nil {
		return ErrUnknown
	}
	if found && !matches(q, r) {
		return ErrConflict
	}
	return e.Store.transaction(func(s *state) error {
		current := s.Jobs[id]
		if current.Proposal != j.Proposal {
			return ErrConflict
		}
		if current.Status == "succeeded" {
			return nil
		}
		if current.Status != "unknown" && current.Status != "manual" {
			return ErrConflict
		}
		if found {
			current.Status = "succeeded"
			current.Receipt = r.ID
			current.Audit = append(current.Audit, "reconciled-receipt")
			return nil
		}
		if !p.Idempotent() {
			current.Status = "manual"
		}
		return nil
	})
}

// RetryUnknown requires explicit recovery and a stable provider idempotency
// contract. It reuses the SAME intent, binding and payload, and rechecks approval.
func (e Engine) RetryUnknown(ctx context.Context, id string, p Provider, lease time.Duration) error {
	if !p.Idempotent() {
		return ErrUnknown
	}
	a, err := identity(ctx)
	if err != nil {
		return err
	}
	var c Claim
	err = e.Store.transaction(func(s *state) error {
		j := s.Jobs[id]
		if j == nil {
			return ErrConflict
		}
		if !allowed(s, a, j.Proposal.Binding, false) {
			return ErrDenied
		}
		if j.Status != "unknown" || j.Owner != "" || lease <= 0 {
			return ErrConflict
		}
		if !e.approvalOK(s, j) {
			return ErrApproval
		}
		j.Status = "ready"
		return nil
	})
	if err != nil {
		return err
	}
	c, err = e.Claim(ctx, id, lease)
	if err != nil {
		return err
	}
	return e.Dispatch(ctx, c, p)
}
