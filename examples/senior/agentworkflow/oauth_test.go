package agentworkflow

import (
	"context"
	"errors"
	"testing"
)

type rotateFunc func(context.Context, string) (string, error)

func (f rotateFunc) Refresh(c context.Context, t string) (string, error) { return f(c, t) }

func TestRefreshAckLossQuarantinesCredential(t *testing.T) {
	f := setup(t)
	f.approve(t)
	p := FixtureRotator{Store: f.p.Store}
	b := f.proposal.Binding
	lost := rotateFunc(func(c context.Context, token string) (string, error) {
		_, err := p.Refresh(c, token)
		if err != nil {
			return "", err
		}
		return "", context.DeadlineExceeded
	})
	if err := f.e.Refresh(f.actor, b, lost); !errors.Is(err, ErrUnknown) {
		t.Fatal(err)
	}
	f.e.Store = Store{f.e.Store.Path}
	c, err := f.e.Credential(b)
	must(t, err)
	if c.Status != "reauth_required" {
		t.Fatal(c)
	}
	if err = f.e.Refresh(f.actor, b, p); err == nil {
		t.Fatal("old token replayed")
	}
	if _, err = f.e.Claim(f.actor, f.proposal.ID, 1); !errors.Is(err, ErrApproval) {
		t.Fatalf("quarantine bypass: %v", err)
	}
	n, err := p.Calls()
	must(t, err)
	if n != 1 {
		t.Fatalf("refresh attempts=%d", n)
	}
	must(t, f.e.SetCredential(b, Credential{Subject: "different-subject", Scope: b.Scope, Token: "NONSECRET-reauthorized", Status: "active"}))
	if _, err = f.e.Claim(f.actor, f.proposal.ID, 1); !errors.Is(err, ErrApproval) {
		t.Fatal("old approval survived identity change")
	}
}
func TestRefreshCrashRequiresReauthorization(t *testing.T) {
	f := setup(t)
	crash(t, f, "refresh")
	c, err := f.e.Credential(f.proposal.Binding)
	must(t, err)
	if c.Status != "refreshing" {
		t.Fatal(c)
	}
	must(t, f.e.RecoverRefresh(f.review, f.proposal.Binding))
	if err = f.e.Refresh(f.actor, f.proposal.Binding, FixtureRotator{Store: f.p.Store}); err == nil {
		t.Fatal("crashed refresh replayed")
	}
	n, err := (FixtureRotator{Store: f.p.Store}).Calls()
	must(t, err)
	if n != 1 {
		t.Fatalf("refresh attempts=%d", n)
	}
}
func TestConcurrentRefreshAndStaleCompletion(t *testing.T) {
	for _, reauthorize := range []bool{false, true} {
		t.Run(map[bool]string{false: "single-refresh", true: "stale-response"}[reauthorize], func(t *testing.T) {
			f := setup(t)
			b := f.proposal.Binding
			p := FixtureRotator{Store: f.p.Store}
			started, release := make(chan struct{}), make(chan struct{})
			done := make(chan error, 1)
			blocked := rotateFunc(func(c context.Context, token string) (string, error) {
				next, err := p.Refresh(c, token)
				close(started)
				<-release
				return next, err
			})
			go func() { done <- f.e.Refresh(f.actor, b, blocked) }()
			<-started
			if err := f.e.Refresh(f.actor, b, p); err == nil {
				t.Fatal("concurrent rotation accepted")
			}
			if reauthorize {
				must(t, f.e.SetCredential(b, Credential{Subject: b.CredentialSubject, Scope: b.Scope, Token: "NONSECRET-new-grant", Status: "active"}))
			}
			close(release)
			err := <-done
			if reauthorize {
				if !errors.Is(err, ErrConflict) {
					t.Fatal(err)
				}
			} else {
				must(t, err)
			}
			c, err := f.e.Credential(b)
			must(t, err)
			if c.Status != "active" {
				t.Fatal(c)
			}
			if reauthorize && c.Token != "NONSECRET-new-grant" {
				t.Fatal("stale rotation overwrote new grant")
			}
			n, err := p.Calls()
			must(t, err)
			if n != 1 {
				t.Fatalf("provider refresh calls=%d", n)
			}
		})
	}
}
