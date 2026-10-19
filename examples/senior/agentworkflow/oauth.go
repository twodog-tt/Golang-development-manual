package agentworkflow

import (
	"context"
	"errors"
	"fmt"
)

// Token contains NON-SECRET fixture markers only. This file store is not a
// production encrypted credential vault; never put live access/refresh tokens here.
type Credential struct {
	Subject, Scope, Token, Status string
	Version, Attempt              uint64
}
type Rotator interface {
	Refresh(context.Context, string) (string, error)
}

var ErrReplay = errors.New("rotated refresh token replay rejected")

// SetCredential models a trusted OAuth callback/operator supplying a newly
// verified grant. It is not an end-user API and must not accept model claims.
func (e Engine) SetCredential(b Binding, c Credential) error {
	return e.Store.transaction(func(s *state) error {
		if c.Subject == "" || c.Scope == "" || c.Token == "" || c.Status != "active" {
			return ErrConflict
		}
		if old := s.Credentials[credentialKey(b)]; old != nil {
			c.Version = old.Version + 1
			c.Attempt = old.Attempt
		} else {
			c.Version = 1
		}
		s.Credentials[credentialKey(b)] = &c
		return nil
	})
}
func (e Engine) Credential(b Binding) (Credential, error) {
	var c Credential
	err := e.Store.transaction(func(s *state) error {
		old := s.Credentials[credentialKey(b)]
		if old == nil {
			return ErrConflict
		}
		c = *old
		return nil
	})
	return c, err
}
func (e Engine) Refresh(ctx context.Context, b Binding, p Rotator) error {
	a, err := identity(ctx)
	if err != nil {
		return err
	}
	var old Credential
	err = e.Store.transaction(func(s *state) error {
		if !allowed(s, a, b, false) {
			return ErrDenied
		}
		c := s.Credentials[credentialKey(b)]
		if c == nil || !credentialOK(s, b) {
			return ErrConflict
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		c.Attempt++
		c.Status = "refreshing"
		old = *c
		return nil
	})
	if err != nil {
		return err
	}
	token, callErr := p.Refresh(ctx, old.Token)
	err = e.Store.transaction(func(s *state) error {
		c := s.Credentials[credentialKey(b)]
		if c == nil || c.Version != old.Version || c.Attempt != old.Attempt || c.Status != "refreshing" {
			return ErrConflict
		}
		if callErr != nil || token == "" {
			c.Status = "reauth_required"
			return nil
		}
		c.Token = token
		c.Version++
		c.Status = "active"
		return nil
	})
	if err != nil {
		return err
	}
	if callErr != nil || token == "" {
		return ErrUnknown
	}
	return nil
}

// RecoverRefresh never replays the old token after an ambiguous refresh.
// A process crash leaves refreshing durable; an operator must reauthorize.
func (e Engine) RecoverRefresh(ctx context.Context, b Binding) error {
	a, err := identity(ctx)
	if err != nil {
		return err
	}
	return e.Store.transaction(func(s *state) error {
		if !allowed(s, a, b, true) {
			return ErrDenied
		}
		c := s.Credentials[credentialKey(b)]
		if c == nil || c.Status != "refreshing" {
			return ErrConflict
		}
		c.Status = "reauth_required"
		return nil
	})
}

type FixtureRotator struct{ Store Store }

func (p FixtureRotator) Refresh(ctx context.Context, old string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var token string
	var replay bool
	err := p.Store.transaction(func(s *state) error {
		s.RefreshCalls++
		if s.RefreshUsed[old] {
			replay = true
			return nil
		}
		s.RefreshUsed[old] = true
		token = fmt.Sprint("NONSECRET-rotation-", s.RefreshCalls)
		return nil
	})
	if err != nil {
		return "", err
	}
	if replay {
		return "", ErrReplay
	}
	return token, nil
}
func (p FixtureRotator) Calls() (int, error) {
	var n int
	err := p.Store.transaction(func(s *state) error { n = s.RefreshCalls; return nil })
	return n, err
}
