package agentworkflow

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fixture struct {
	e             Engine
	p             FixtureProvider
	auth          Authority
	actor, review context.Context
	proposal      Proposal
	clock         *atomic.Int64
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func setup(t *testing.T) *fixture {
	t.Helper()
	dir := t.TempDir()
	clock := new(atomic.Int64)
	clock.Store(time.Now().UnixNano())
	f := &fixture{e: Engine{Store: Store{filepath.Join(dir, "control.json")}, Now: func() time.Time { return time.Unix(0, clock.Load()) }}, p: FixtureProvider{Store: Store{filepath.Join(dir, "external.json")}, Deduplicate: true}, auth: Authority{"worker-token": {ID: "alice", Tenant: "tenant-a"}, "review-token": {ID: "bob", Tenant: "tenant-a"}, "other-token": {ID: "mallory", Tenant: "tenant-b"}}, clock: clock}
	var err error
	f.actor, err = f.auth.Context(context.Background(), "worker-token")
	must(t, err)
	f.review, err = f.auth.Context(context.Background(), "review-token")
	must(t, err)
	f.proposal = Proposal{ID: "proposal-1", Intent: "intent-1", Version: 1, Binding: Binding{Tenant: "tenant-a", Provider: "fixture", Account: "account-a", CredentialSubject: "subject-a", Scope: "publish", Tool: "publish", ToolVersion: "v1", WorkflowVersion: "v1", PolicyVersion: "v1"}, Text: "reviewed content"}
	b := f.proposal.Binding
	must(t, f.e.SetGrant(Grant{Actor: "alice", Tenant: b.Tenant, Provider: b.Provider, Account: b.Account, Tool: b.Tool, Execute: true}))
	must(t, f.e.SetGrant(Grant{Actor: "bob", Tenant: b.Tenant, Provider: b.Provider, Account: b.Account, Tool: b.Tool, Review: true}))
	must(t, f.e.SetCredential(b, Credential{Subject: b.CredentialSubject, Scope: b.Scope, Token: "NONSECRET-initial", Status: "active"}))
	must(t, f.e.Propose(f.actor, f.proposal))
	return f
}
func (f *fixture) approve(t *testing.T) {
	t.Helper()
	must(t, f.e.Approve(f.review, f.proposal.ID, f.proposal.Digest(), f.e.now().Add(time.Hour)))
}
func (f *fixture) claim(t *testing.T) Claim {
	t.Helper()
	c, err := f.e.Claim(f.actor, f.proposal.ID, time.Minute)
	must(t, err)
	return c
}
func (f *fixture) count(t *testing.T, want int) {
	t.Helper()
	n, err := f.p.Count()
	must(t, err)
	if n != want {
		t.Fatalf("effects=%d want=%d", n, want)
	}
}
func (f *fixture) status(t *testing.T, want string) {
	t.Helper()
	j, err := f.e.Job(f.proposal.ID)
	must(t, err)
	if j.Status != want {
		t.Fatalf("job=%+v want=%s", j, want)
	}
}
func (f *fixture) advance() { f.clock.Add(int64(2 * time.Minute)) }

func TestApprovalAuthorizationAndBinding(t *testing.T) {
	for _, kind := range []string{"unauthenticated", "fake-bearer", "cross-tenant", "unprivileged", "self-review", "wrong-digest", "expired", "account", "credential", "tool-version", "workflow-version", "policy-version", "proposal-version", "provider", "scope"} {
		t.Run(kind, func(t *testing.T) {
			f := setup(t)
			ctx := f.review
			p := f.proposal
			expires := f.e.now().Add(time.Hour)
			switch kind {
			case "unauthenticated":
				ctx = context.Background()
			case "fake-bearer":
				if _, err := f.auth.Context(context.Background(), "bob"); !errors.Is(err, ErrDenied) {
					t.Fatal("forged identity accepted")
				}
				return
			case "cross-tenant":
				var err error
				ctx, err = f.auth.Context(context.Background(), "other-token")
				must(t, err)
			case "unprivileged":
				ctx = f.actor
			case "self-review":
				ctx = f.actor
				g := Grant{Actor: "alice", Tenant: p.Binding.Tenant, Provider: p.Binding.Provider, Account: p.Binding.Account, Tool: p.Binding.Tool, Review: true, Execute: true}
				must(t, f.e.SetGrant(g))
			case "wrong-digest":
				p.Text = "changed"
			case "expired":
				expires = f.e.now()
			case "account":
				p.Binding.Account = "account-b"
			case "credential":
				p.Binding.CredentialSubject = "subject-b"
			case "tool-version":
				p.Binding.ToolVersion = "v2"
			case "workflow-version":
				p.Binding.WorkflowVersion = "v2"
			case "policy-version":
				p.Binding.PolicyVersion = "v2"
			case "proposal-version":
				p.Version++
			case "provider":
				p.Binding.Provider = "another-provider"
			case "scope":
				p.Binding.Scope = "admin"
			}
			if err := f.e.Approve(ctx, p.ID, p.Digest(), expires); err == nil {
				t.Fatal("unauthorized/stale approval accepted")
			}
			f.status(t, "pending")
			f.count(t, 0)
		})
	}
}

func TestConcurrentApproveEditAndClaim(t *testing.T) {
	t.Run("approve-versus-edit", func(t *testing.T) {
		f := setup(t)
		var wg sync.WaitGroup
		p := f.proposal
		p.Version++
		p.Text = "new content"
		for i := 0; i < 16; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = f.e.Approve(f.review, p.ID, f.proposal.Digest(), f.e.now().Add(time.Hour))
			}()
		}
		must(t, f.e.Edit(f.actor, p.ID, p))
		wg.Wait()
		f.status(t, "pending")
		if _, err := f.e.Claim(f.actor, p.ID, time.Minute); err == nil {
			t.Fatal("old approval survived edit")
		}
		f.count(t, 0)
	})
	t.Run("one-approval-one-claim", func(t *testing.T) {
		f := setup(t)
		var approvals, claims atomic.Int32
		var wg sync.WaitGroup
		for i := 0; i < 16; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if f.e.Approve(f.review, f.proposal.ID, f.proposal.Digest(), f.e.now().Add(time.Hour)) == nil {
					approvals.Add(1)
				}
			}()
		}
		wg.Wait()
		for i := 0; i < 16; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := f.e.Claim(f.actor, f.proposal.ID, time.Minute); err == nil {
					claims.Add(1)
				}
			}()
		}
		wg.Wait()
		if approvals.Load() != 1 || claims.Load() != 1 {
			t.Fatalf("approvals=%d claims=%d", approvals.Load(), claims.Load())
		}
	})
}

func TestExecutionRechecksApprovalAndCredential(t *testing.T) {
	for _, kind := range []string{"revoke", "reviewer-role-removed", "executor-role-removed", "credential-subject-changed", "approval-expired", "payload-change", "unauthenticated-worker"} {
		t.Run(kind, func(t *testing.T) {
			f := setup(t)
			if kind == "approval-expired" {
				must(t, f.e.Approve(f.review, f.proposal.ID, f.proposal.Digest(), f.e.now().Add(30*time.Second)))
			} else {
				f.approve(t)
			}
			c := f.claim(t)
			ctx := f.actor
			b := f.proposal.Binding
			switch kind {
			case "revoke":
				must(t, f.e.Revoke(f.review, c.ID))
			case "reviewer-role-removed":
				must(t, f.e.SetGrant(Grant{Actor: "bob", Tenant: b.Tenant, Provider: b.Provider, Account: b.Account, Tool: b.Tool}))
			case "executor-role-removed":
				must(t, f.e.SetGrant(Grant{Actor: "alice", Tenant: b.Tenant, Provider: b.Provider, Account: b.Account, Tool: b.Tool}))
			case "credential-subject-changed":
				must(t, f.e.SetCredential(b, Credential{Subject: "new-subject", Scope: b.Scope, Token: "NONSECRET-new", Status: "active"}))
			case "approval-expired":
				f.clock.Add(int64(45 * time.Second)) // approval expired, lease still valid
			case "payload-change":
				p := f.proposal
				p.Text = "malicious"
				if _, err := f.e.PublishTool(f.p)(f.actor, ToolCall{ID: "new-call", Name: "publish", Proposal: p}); !errors.Is(err, ErrConflict) {
					t.Fatalf("payload substitution: %v", err)
				}
				f.count(t, 0)
				return
			case "unauthenticated-worker":
				ctx = context.Background()
			}
			if err := f.e.Dispatch(ctx, c, f.p); err == nil || (kind == "approval-expired" && !errors.Is(err, ErrApproval)) {
				t.Fatalf("execution escaped authorization or wrong guard fired: %v", err)
			}
			f.count(t, 0)
		})
	}
}

type publishWrapper struct {
	Provider
	send   func(context.Context, Request) (Receipt, error)
	lookup func(context.Context, Request) (Receipt, bool, error)
}

func (p publishWrapper) Publish(c context.Context, q Request) (Receipt, error) {
	if p.send != nil {
		return p.send(c, q)
	}
	return p.Provider.Publish(c, q)
}
func (p publishWrapper) Lookup(c context.Context, q Request) (Receipt, bool, error) {
	if p.lookup != nil {
		return p.lookup(c, q)
	}
	return p.Provider.Lookup(c, q)
}

func TestLateWorkerCannotTriggerAutomaticDuplicate(t *testing.T) {
	f := setup(t)
	f.p.Deduplicate = false
	f.approve(t)
	claim := f.claim(t)
	started, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	p := publishWrapper{Provider: f.p, send: func(ctx context.Context, q Request) (Receipt, error) {
		close(started)
		<-release
		return f.p.Publish(ctx, q)
	}}
	go func() { done <- f.e.Dispatch(f.actor, claim, p) }()
	<-started
	f.advance()
	must(t, f.e.RecoverExpired(f.actor, claim.ID))
	must(t, f.e.Reconcile(f.actor, claim.ID, p))
	f.status(t, "manual")
	f.count(t, 0)
	if err := f.e.RetryUnknown(f.actor, claim.ID, p, time.Minute); !errors.Is(err, ErrUnknown) {
		t.Fatalf("unsafe retry: %v", err)
	}
	close(release)
	if err := <-done; !errors.Is(err, ErrLease) {
		t.Fatalf("stale worker wrote completion: %v", err)
	}
	f.status(t, "manual")
	must(t, f.e.Reconcile(f.actor, claim.ID, f.p))
	f.status(t, "succeeded")
	f.count(t, 1)
}

func TestUnknownSameIntentReplayAndReceiptValidation(t *testing.T) {
	f := setup(t)
	f.approve(t)
	c := f.claim(t)
	lost := publishWrapper{Provider: f.p, send: func(ctx context.Context, q Request) (Receipt, error) {
		_, err := f.p.Publish(ctx, q)
		if err != nil {
			return Receipt{}, err
		}
		return Receipt{}, context.DeadlineExceeded
	}}
	if err := f.e.Dispatch(f.actor, c, lost); !errors.Is(err, ErrUnknown) {
		t.Fatalf("want unknown: %v", err)
	}
	f.count(t, 1)
	bad := publishWrapper{Provider: f.p, lookup: func(ctx context.Context, q Request) (Receipt, bool, error) {
		r, found, err := f.p.Lookup(ctx, q)
		r.Digest = "wrong"
		return r, found, err
	}}
	if err := f.e.Reconcile(f.actor, c.ID, bad); !errors.Is(err, ErrConflict) {
		t.Fatalf("bad receipt accepted: %v", err)
	}
	f.status(t, "unknown")
	f.advance()
	must(t, f.e.RecoverExpired(f.actor, c.ID))
	hidden := publishWrapper{Provider: f.p, lookup: func(context.Context, Request) (Receipt, bool, error) { return Receipt{}, false, nil }}
	must(t, f.e.Reconcile(f.actor, c.ID, hidden))
	f.status(t, "unknown")
	must(t, f.e.RetryUnknown(f.actor, c.ID, f.p, time.Minute))
	f.count(t, 1)
	f.status(t, "succeeded")
	if _, err := f.e.PublishTool(f.p)(f.actor, ToolCall{ID: "another-model-call", Name: "publish", Proposal: f.proposal}); err != nil {
		t.Fatal(err)
	}
	f.count(t, 1)
	p := f.proposal
	p.ID = "new-job"
	if err := f.e.Propose(f.actor, p); !errors.Is(err, ErrConflict) {
		t.Fatal("duplicate business intent accepted")
	}
}

func TestUndispatchedLeaseRecoveryAndRevokedInFlight(t *testing.T) {
	t.Run("undispatched", func(t *testing.T) {
		f := setup(t)
		f.approve(t)
		old := f.claim(t)
		f.advance()
		must(t, f.e.RecoverExpired(f.actor, old.ID))
		f.status(t, "ready")
		fresh := f.claim(t)
		if err := f.e.Dispatch(f.actor, old, f.p); !errors.Is(err, ErrLease) {
			t.Fatal(err)
		}
		must(t, f.e.Dispatch(f.actor, fresh, f.p))
		f.count(t, 1)
	})
	t.Run("revoked-in-flight", func(t *testing.T) {
		f := setup(t)
		f.approve(t)
		c := f.claim(t)
		p := publishWrapper{Provider: f.p, send: func(ctx context.Context, q Request) (Receipt, error) {
			r, err := f.p.Publish(ctx, q)
			must(t, f.e.Revoke(f.review, c.ID))
			return r, err
		}}
		must(t, f.e.Dispatch(f.actor, c, p))
		f.status(t, "succeeded")
		f.count(t, 1)
		j, err := f.e.Job(c.ID)
		must(t, err)
		if !j.Approval.Revoked {
			t.Fatal("revocation audit lost")
		}
	})
}

func TestDefinitiveRejectionAndUnsupportedVersion(t *testing.T) {
	f := setup(t)
	f.approve(t)
	c := f.claim(t)
	p := publishWrapper{Provider: f.p, send: func(context.Context, Request) (Receipt, error) { return Receipt{}, ErrRejected }}
	must(t, f.e.Dispatch(f.actor, c, p))
	f.status(t, "failed")
	f.count(t, 0)
	p2 := f.proposal
	p2.ID = "proposal-2"
	p2.Intent = "intent-2"
	p2.Binding.ToolVersion = "unknown-version"
	if err := f.e.Propose(f.actor, p2); !errors.Is(err, ErrConflict) {
		t.Fatalf("unsupported executor version accepted: %v", err)
	}
}

func TestToolResourceIsolationAndTerminalMonotonicity(t *testing.T) {
	for _, kind := range []string{"tenant", "account", "credential", "scope", "tool"} {
		t.Run(kind, func(t *testing.T) {
			f := setup(t)
			f.approve(t)
			c := ToolCall{ID: "model-call", Name: "publish", Proposal: f.proposal}
			switch kind {
			case "tenant":
				c.Proposal.Binding.Tenant = "tenant-b"
			case "account":
				c.Proposal.Binding.Account = "account-b"
			case "credential":
				c.Proposal.Binding.CredentialSubject = "other-subject"
			case "scope":
				c.Proposal.Binding.Scope = "admin"
			case "tool":
				c.Name = "delete_account"
			}
			if _, err := f.e.PublishTool(f.p)(f.actor, c); err == nil {
				t.Fatal("tool escaped approved binding")
			}
			f.count(t, 0)
			f.status(t, "ready")
		})
	}
	t.Run("late-attempt-failure", func(t *testing.T) {
		f := setup(t)
		f.approve(t)
		c := f.claim(t)
		p := publishWrapper{Provider: f.p, send: func(ctx context.Context, q Request) (Receipt, error) {
			_, err := f.p.Publish(ctx, q)
			if err != nil {
				return Receipt{}, err
			}
			must(t, f.e.Reconcile(f.actor, c.ID, f.p))
			return Receipt{}, ErrRejected
		}}
		must(t, f.e.Dispatch(f.actor, c, p))
		f.status(t, "succeeded")
		f.count(t, 1)
	})
}

// The test subprocess exits after the external durable effect, bypassing all
// Go defers. The parent reopens both independent files and exercises recovery.
func TestCrashWorker(t *testing.T) {
	mode := os.Getenv("AGENT_WORKFLOW_CRASH_MODE")
	if mode == "" {
		return
	}
	n, err := strconv.ParseInt(os.Getenv("AGENT_WORKFLOW_CLOCK"), 10, 64)
	must(t, err)
	e := Engine{Store: Store{os.Getenv("AGENT_WORKFLOW_CONTROL")}, Now: func() time.Time { return time.Unix(0, n) }}
	a := Authority{"worker-token": {ID: "alice", Tenant: "tenant-a"}}
	ctx, err := a.Context(context.Background(), "worker-token")
	must(t, err)
	p := FixtureProvider{Store: Store{os.Getenv("AGENT_WORKFLOW_EXTERNAL")}, Deduplicate: true}
	if mode == "refresh" {
		j, err := e.Job("proposal-1")
		must(t, err)
		_ = e.Refresh(ctx, j.Proposal.Binding, rotateFunc(func(c context.Context, old string) (string, error) {
			_, err := (FixtureRotator{Store: p.Store}).Refresh(c, old)
			must(t, err)
			os.Exit(23)
			return "", nil
		}))
	} else {
		crash := publishWrapper{Provider: p, send: func(c context.Context, q Request) (Receipt, error) {
			if mode != "before-send" {
				_, err := p.Publish(c, q)
				must(t, err)
			}
			os.Exit(23)
			return Receipt{}, nil
		}}
		if mode == "run" {
			r := newRunner(e, crash)
			_, _ = r.Continue(ctx, "run-1")
		} else {
			claim, err := e.Claim(ctx, "proposal-1", time.Minute)
			must(t, err)
			_ = e.Dispatch(ctx, claim, crash)
		}
	}
	t.Fatal("crash boundary not reached")
}
func crash(t *testing.T, f *fixture, mode string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestCrashWorker$")
	cmd.Env = append(os.Environ(), "AGENT_WORKFLOW_CRASH_MODE="+mode, "AGENT_WORKFLOW_CLOCK="+strconv.FormatInt(f.clock.Load(), 10), "AGENT_WORKFLOW_CONTROL="+f.e.Store.Path, "AGENT_WORKFLOW_EXTERNAL="+f.p.Store.Path)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 23 {
		t.Fatalf("subprocess=%v %s", err, out)
	}
}
func TestProcessCrashAndRestartRecovery(t *testing.T) {
	t.Run("incomplete-state", func(t *testing.T) {
		f := setup(t)
		must(t, os.WriteFile(f.e.Store.Path, []byte(`{}`), 0600))
		if _, err := f.e.Job(f.proposal.ID); err == nil {
			t.Fatal("incomplete existing state treated as new empty store")
		}
		if err := f.e.Propose(f.actor, f.proposal); err == nil {
			t.Fatal("incomplete facts silently reconstructed")
		}
		f.count(t, 0)
	})
	for _, mode := range []string{"after-accept", "before-send", "run"} {
		t.Run(mode, func(t *testing.T) {
			f := setup(t)
			f.approve(t)
			if mode == "run" {
				r := newRunner(f.e, f.p)
				must(t, r.Start(f.actor, "run-1", "publish reviewed content", testLimits(f.e)))
			}
			crash(t, f, mode)
			f.status(t, "unknown")
			f.advance()
			f.e.Store = Store{f.e.Store.Path}
			f.p.Store = Store{f.p.Store.Path}
			must(t, f.e.RecoverExpired(f.actor, f.proposal.ID))
			if mode == "before-send" {
				f.p.Deduplicate = false
				must(t, f.e.Reconcile(f.actor, f.proposal.ID, f.p))
				f.status(t, "manual")
				f.count(t, 0)
				return
			}
			must(t, f.e.Reconcile(f.actor, f.proposal.ID, f.p))
			f.status(t, "succeeded")
			f.count(t, 1)
			if mode == "run" {
				r := newRunner(f.e, f.p)
				out, err := r.Continue(f.actor, "run-1")
				must(t, err)
				if out != "done" {
					t.Fatal(out)
				}
				f.count(t, 1)
			}
		})
	}
}
