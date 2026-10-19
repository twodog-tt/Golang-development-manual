// Package agentmemory demonstrates local memory governance, not a production IAM service.
package agentmemory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"time"
)

var ErrDenied = errors.New("knowledge: permission denied")
var ErrConflict = errors.New("knowledge: stale revision or invalid transition")

type Scope struct{ Tenant, Bot, Scene, SubjectType, SubjectID string }

func (s Scope) Valid() bool {
	return s.Tenant != "" && s.Bot != "" && s.Scene != "" && s.SubjectType != "" && s.SubjectID != ""
}

type Identity struct {
	Tenant, Subject, Policy string
	Revision                uint64
	Issuer                  string
}
type Grant struct {
	Tenant, Subject, Policy string
	Scopes                  []Scope
	Roles                   []string
	Expires                 time.Time
}
type Authority struct {
	mu        sync.RWMutex
	grants    map[string]Grant
	revisions map[string]uint64
	issuer    string
}

func NewAuthority() *Authority {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return &Authority{grants: map[string]Grant{}, revisions: map[string]uint64{}, issuer: hex.EncodeToString(b[:])}
}
func actorKey(tenant, subject string) string {
	b, _ := json.Marshal([]string{tenant, subject})
	return string(b)
}
func (a *Authority) Put(token string, g Grant) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if prev, ok := a.grants[token]; ok {
		a.revisions[actorKey(prev.Tenant, prev.Subject)]++
	}
	g.Scopes = slices.Clone(g.Scopes)
	g.Roles = slices.Clone(g.Roles)
	a.revisions[actorKey(g.Tenant, g.Subject)]++
	a.grants[token] = g
}
func (a *Authority) Revoke(token string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	g, ok := a.grants[token]
	if ok {
		a.revisions[actorKey(g.Tenant, g.Subject)]++
		delete(a.grants, token)
	}
}

type authKey struct{}
type lease struct {
	a        *Authority
	token    string
	identity Identity
}

func (a *Authority) resolve(token string) (Identity, Grant, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	g, ok := a.grants[token]
	if token == "" || !ok || g.Tenant == "" || g.Subject == "" || g.Policy == "" || !g.Expires.After(time.Now()) {
		return Identity{}, Grant{}, ErrDenied
	}
	return Identity{g.Tenant, g.Subject, g.Policy, a.revisions[actorKey(g.Tenant, g.Subject)], a.issuer}, g, nil
}

// Context is called by the trusted transport/launcher, never with a model-supplied token.
func (a *Authority) Context(ctx context.Context, token string) (context.Context, error) {
	i, _, err := a.resolve(token)
	if err != nil {
		return nil, err
	}
	return context.WithValue(ctx, authKey{}, lease{a, token, i}), nil
}
func Check(ctx context.Context, scope Scope, role string) (Identity, error) {
	if err := ctx.Err(); err != nil {
		return Identity{}, err
	}
	l, ok := ctx.Value(authKey{}).(lease)
	if !ok || !scope.Valid() {
		return Identity{}, ErrDenied
	}
	i, g, err := l.a.resolve(l.token)
	if err != nil || i != l.identity || i.Tenant != scope.Tenant || !slices.Contains(g.Scopes, scope) || !slices.Contains(g.Roles, role) {
		return Identity{}, ErrDenied
	}
	return i, nil
}
func reviewerLive(ctx context.Context, who Identity, scope Scope) bool {
	l, ok := ctx.Value(authKey{}).(lease)
	if !ok {
		return false
	}
	a := l.a
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.issuer != who.Issuer || a.revisions[actorKey(who.Tenant, who.Subject)] != who.Revision {
		return false
	}
	for _, g := range a.grants {
		if g.Tenant == who.Tenant && g.Subject == who.Subject && g.Policy == who.Policy && g.Expires.After(time.Now()) && slices.Contains(g.Scopes, scope) && slices.Contains(g.Roles, "review") {
			return true
		}
	}
	return false
}
