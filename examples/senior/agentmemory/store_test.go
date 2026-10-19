package agentmemory

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func fixture(t *testing.T) (Store, *Authority, Scope, map[string]context.Context) {
	t.Helper()
	s := Store{t.TempDir()}
	scope := Scope{"a", "bot", "faq", "user", "u"}
	a := NewAuthority()
	contexts := map[string]context.Context{}
	for _, name := range []string{"writer", "reviewer", "operator"} {
		a.Put(name, Grant{Tenant: "a", Subject: name, Policy: "p1", Scopes: []Scope{scope}, Roles: []string{"read", "write", "review", "activate", "delete"}, Expires: time.Now().Add(time.Hour)})
		ctx, e := a.Context(context.Background(), name)
		if e != nil {
			t.Fatal(e)
		}
		contexts[name] = ctx
	}
	return s, a, scope, contexts
}
func create(t *testing.T, s Store, scope Scope, c map[string]context.Context) Rule {
	t.Helper()
	r, e := s.Create(c["writer"], scope, "r", "不夸大承诺", []string{"feedback:1"}, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func activate(t *testing.T, s Store, scope Scope, c map[string]context.Context, r Rule) {
	t.Helper()
	d, v, e := s.ReviewDigest(c["reviewer"], scope, r.ID)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Approve(c["reviewer"], scope, r.ID, d, v); e != nil {
		t.Fatal(e)
	}
	if e = s.Transition(c["operator"], scope, r.ID, "active", v); e != nil {
		t.Fatal(e)
	}
}
func TestApprovalRevisionAndIsolation(t *testing.T) {
	s, _, scope, c := fixture(t)
	r := create(t, s, scope, c)
	if e := s.Transition(c["operator"], scope, r.ID, "active", 1); e == nil {
		t.Fatal("activation without approval")
	}
	d, v, e := s.ReviewDigest(c["reviewer"], scope, r.ID)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Approve(c["writer"], scope, r.ID, d, v); e == nil {
		t.Fatal("self-review")
	}
	if e = s.Approve(c["reviewer"], scope, r.ID, "old", v); e == nil {
		t.Fatal("bad digest")
	}
	activate(t, s, scope, c, r)
	ref := Ref{r.ID, 1}
	if hits, e := s.Resolve(c["operator"], scope, []Ref{ref}); e != nil || len(hits) != 1 {
		t.Fatal(hits, e)
	}
	edited, e := s.Edit(c["writer"], scope, r.ID, "保留退款条件", 1)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Edit(c["writer"], scope, r.ID, "stale", 1); e == nil {
		t.Fatal("lost update")
	}
	if e = s.Approve(c["reviewer"], scope, r.ID, d, v); e == nil {
		t.Fatal("stale approval")
	}
	if hits, e := s.Resolve(c["operator"], scope, []Ref{ref}); e != nil || len(hits) != 0 {
		t.Fatal(hits, e)
	}
	activate(t, s, scope, c, edited)
	if e = s.Transition(c["operator"], scope, r.ID, "retired", edited.Version); e != nil {
		t.Fatal(e)
	}
	if e = s.Transition(c["operator"], scope, r.ID, "active", edited.Version+1); e == nil {
		t.Fatal("revived retired version")
	}
	scope.SubjectID = "other"
	if _, e = s.Get(c["operator"], scope, r.ID); !errors.Is(e, ErrDenied) {
		t.Fatal(e)
	}
	if _, e = s.Get(context.Background(), scope, r.ID); !errors.Is(e, ErrDenied) {
		t.Fatal(e)
	}
}
func TestRevocationExpiryPolicyAndAuthorityRestart(t *testing.T) {
	for _, mode := range []string{"reviewer-revoke", "caller-revoke", "policy", "expiry", "issuer-restart"} {
		t.Run(mode, func(t *testing.T) {
			s, a, scope, c := fixture(t)
			r := create(t, s, scope, c)
			d, v, e := s.ReviewDigest(c["reviewer"], scope, r.ID)
			if e != nil {
				t.Fatal(e)
			}
			if e = s.Approve(c["reviewer"], scope, r.ID, d, v); e != nil {
				t.Fatal(e)
			}
			switch mode {
			case "reviewer-revoke":
				a.Revoke("reviewer")
			case "caller-revoke":
				a.Revoke("operator")
			case "policy":
				a.Put("operator", Grant{Tenant: "a", Subject: "operator", Policy: "p2", Scopes: []Scope{scope}, Roles: []string{"activate"}, Expires: time.Now().Add(time.Hour)})
				c["operator"], e = a.Context(context.Background(), "operator")
				if e != nil {
					t.Fatal(e)
				}
			case "expiry":
				if e = s.tx(func(d *state) error {
					r := d.Rules[key(scope, "r")]
					r.Expires = time.Now().Add(-time.Second)
					r.ApprovedDigest = digest(r) // Keep the binding valid so only the expiry guard can reject.
					d.Rules[key(scope, "r")] = r
					return nil
				}); e != nil {
					t.Fatal(e)
				}
			case "issuer-restart":
				fresh := NewAuthority()
				for _, name := range []string{"writer", "reviewer", "operator"} {
					fresh.Put(name, Grant{Tenant: "a", Subject: name, Policy: "p1", Scopes: []Scope{scope}, Roles: []string{"activate", "review"}, Expires: time.Now().Add(time.Hour)})
				}
				c["operator"], e = fresh.Context(context.Background(), "operator")
				if e != nil {
					t.Fatal(e)
				}
			}
			if e = s.Transition(c["operator"], scope, r.ID, "active", v); e == nil {
				t.Fatal("unsafe activation")
			}
		})
	}
}
func TestDeleteFencesStaleIndexCacheHistoryAndLostACK(t *testing.T) {
	s, _, scope, c := fixture(t)
	r := create(t, s, scope, c)
	activate(t, s, scope, c, r)
	if e := s.Sync(); e != nil {
		t.Fatal(e)
	}
	if e := s.Delete(c["operator"], scope, r.ID); e != nil {
		t.Fatal(e)
	}
	if e := s.tx(func(d *state) error {
		if d.Index[key(scope, r.ID)].Text == "" || d.Cache[key(scope, r.ID)].Text == "" {
			t.Fatal("expected stale projections before cleanup")
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	restarted := Store{s.Dir}
	if hits, e := restarted.Resolve(c["operator"], scope, []Ref{{r.ID, 1}}); e != nil || len(hits) != 0 {
		t.Fatal(hits, e)
	}
	if _, e := restarted.Get(c["operator"], scope, r.ID); e == nil {
		t.Fatal("tombstone read")
	}
	if _, e := restarted.Create(c["writer"], scope, r.ID, "revive", []string{"old"}, time.Now().Add(time.Hour)); e == nil {
		t.Fatal("ID reused")
	}
	if more, e := restarted.SyncOne(true); !more || e == nil {
		t.Fatal("fault not injected")
	}
	if e := restarted.Sync(); e != nil {
		t.Fatal(e)
	}
	if e := restarted.Sync(); e != nil {
		t.Fatal(e)
	}
	if e := restarted.tx(func(d *state) error {
		if len(d.Index) != 0 || len(d.Cache) != 0 {
			t.Fatal("deletion not propagated")
		}
		for _, j := range d.Jobs {
			if !j.Done {
				t.Fatal("pending ACK")
			}
		}
		if d.Rules[key(scope, r.ID)].Text != "" {
			t.Fatal("primary not erased")
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
}
func TestStaleProjectionJobCannotEraseNewActiveRevision(t *testing.T) {
	s, _, scope, c := fixture(t)
	r := create(t, s, scope, c)
	activate(t, s, scope, c, r)
	next, e := s.Edit(c["writer"], scope, r.ID, "新规则", 1)
	if e != nil {
		t.Fatal(e)
	}
	activate(t, s, scope, c, next)
	if e = s.Sync(); e != nil {
		t.Fatal(e)
	}
	if e = s.tx(func(d *state) error {
		if d.Index[key(scope, r.ID)].Version != 2 {
			t.Fatal("new projection missing")
		}
		for id, j := range d.Jobs {
			if j.Version == 1 {
				j.Done = false
				d.Jobs[id] = j
			}
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	if e = s.Sync(); e != nil {
		t.Fatal(e)
	}
	if e = s.tx(func(d *state) error {
		if d.Index[key(scope, r.ID)].Version != 2 {
			t.Fatal("stale job erased active projection")
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
}
func TestConcurrentEditOnlyOneWins(t *testing.T) {
	s, _, scope, c := fixture(t)
	r := create(t, s, scope, c)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := s.Edit(c["writer"], scope, r.ID, "edit", r.Version); results <- e }()
	}
	wg.Wait()
	close(results)
	wins := 0
	for e := range results {
		if e == nil {
			wins++
		}
	}
	if wins != 1 {
		t.Fatal(wins)
	}
}
func TestProcessRestartAfterProjectionWriteBeforeACK(t *testing.T) {
	if dir := os.Getenv("MEMORY_CHILD_DIR"); dir != "" {
		_, e := (Store{dir}).SyncOne(true)
		if e == nil {
			os.Exit(3)
		}
		os.Exit(23)
	}
	s, _, scope, c := fixture(t)
	r := create(t, s, scope, c)
	activate(t, s, scope, c, r)
	cmd := exec.Command(os.Args[0], "-test.run=^TestProcessRestartAfterProjectionWriteBeforeACK$")
	cmd.Env = append(os.Environ(), "MEMORY_CHILD_DIR="+s.Dir)
	e := cmd.Run()
	var ex *exec.ExitError
	if !errors.As(e, &ex) || ex.ExitCode() != 23 {
		t.Fatal(e)
	}
	if e = s.tx(func(d *state) error {
		p, ok := d.Index[key(scope, r.ID)]
		if !ok || p.Version != r.Version {
			t.Fatal("child did not persist projection")
		}
		pending := false
		for _, j := range d.Jobs {
			pending = pending || !j.Done
		}
		if !pending {
			t.Fatal("child unexpectedly acknowledged job")
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	if e = (Store{s.Dir}).Sync(); e != nil {
		t.Fatal(e)
	}
	if hits, e := s.Resolve(c["operator"], scope, []Ref{{r.ID, 1}}); e != nil || len(hits) != 1 {
		t.Fatal(hits, e)
	}
}
func TestCorruptStateFailsClosed(t *testing.T) {
	for _, bad := range []string{`{"Rules":null}`, `{}`, `{"Rules":{},"Index":{},"Cache":{}}`, `{`} {
		t.Run(bad, func(t *testing.T) {
			s, _, scope, c := fixture(t)
			create(t, s, scope, c)
			if e := os.WriteFile(filepath.Join(s.Dir, "state.json"), []byte(bad), 0600); e != nil {
				t.Fatal(e)
			}
			if _, e := s.Resolve(c["operator"], scope, []Ref{{"r", 1}}); e == nil {
				t.Fatal("corrupt state accepted")
			}
		})
	}
}
func TestRoleDenialAndPolicyReadDoesNotReturnText(t *testing.T) {
	s, a, scope, c := fixture(t)
	create(t, s, scope, c)
	a.Put("limited", Grant{Tenant: scope.Tenant, Subject: "limited", Policy: "p2", Scopes: []Scope{scope}, Roles: []string{"read"}, Expires: time.Now().Add(time.Hour)})
	ctx, e := a.Context(context.Background(), "limited")
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.Get(ctx, scope, "r")
	if e == nil || r.Text != "" {
		t.Fatal("policy mismatch leaked rule", r, e)
	}
	if _, e = s.Create(ctx, scope, "new", "rule", []string{"source"}, time.Now().Add(time.Hour)); e == nil {
		t.Fatal("read granted write")
	}
	if e = s.Delete(ctx, scope, "r"); e == nil {
		t.Fatal("read granted delete")
	}
}

func TestReadRoleCannotApproveOrActivate(t *testing.T) {
	s, a, scope, c := fixture(t)
	r := create(t, s, scope, c)
	a.Put("readonly", Grant{Tenant: scope.Tenant, Subject: "reader", Policy: "p1", Scopes: []Scope{scope}, Roles: []string{"read"}, Expires: time.Now().Add(time.Hour)})
	ctx, e := a.Context(context.Background(), "readonly")
	if e != nil {
		t.Fatal(e)
	}
	d, v, e := s.ReviewDigest(ctx, scope, r.ID)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Approve(ctx, scope, r.ID, d, v); !errors.Is(e, ErrDenied) {
		t.Fatal("read role approved", e)
	}
	if e = s.Approve(c["reviewer"], scope, r.ID, d, v); e != nil {
		t.Fatal(e)
	}
	if e = s.Transition(ctx, scope, r.ID, "active", v); !errors.Is(e, ErrDenied) {
		t.Fatal("read role activated", e)
	}
}
