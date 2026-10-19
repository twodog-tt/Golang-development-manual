package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Authority is a local pre-provisioned token fixture, not an OAuth issuer.
// Production must validate issuer/signature or introspection, audience, expiry and grants.
type Authority struct {
	mu       sync.RWMutex
	Resource string
	tokens   map[string]Grant
}
type Grant struct {
	Principal Principal
	Audience  string
	Expires   time.Time
}

func NewAuthority(resource string) *Authority {
	return &Authority{Resource: resource, tokens: map[string]Grant{}}
}
func (a *Authority) Put(token string, g Grant) {
	a.mu.Lock()
	defer a.mu.Unlock()
	g.Principal.Scopes = append([]string(nil), g.Principal.Scopes...)
	a.tokens[token] = g
}
func (a *Authority) Revoke(token string) { a.mu.Lock(); defer a.mu.Unlock(); delete(a.tokens, token) }
func (a *Authority) Resolve(token string) (Principal, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	g, ok := a.tokens[token]
	if !ok || g.Audience != a.Resource || !g.Expires.After(time.Now()) || g.Principal.Subject == "" || g.Principal.Tenant == "" {
		return Principal{}, auth.ErrInvalidToken
	}
	p := g.Principal
	p.Scopes = append([]string(nil), p.Scopes...)
	return p, nil
}
func HTTP(c Config, origins []string, idle time.Duration) http.Handler {
	if c.Authority == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "authority required", http.StatusUnauthorized)
		})
	}

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return New(c) }, &mcp.StreamableHTTPOptions{JSONResponse: true, SessionTimeout: idle})
	protected := auth.RequireBearerToken(func(_ context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
		if c.Authority == nil {
			return nil, auth.ErrInvalidToken
		}
		p, err := c.Authority.Resolve(token)
		if err != nil {
			return nil, err
		}
		// Length-prefix identity avoids tenant/subject concatenation collisions.
		return &auth.TokenInfo{UserID: fmt.Sprintf("%d:%s%s", len(p.Tenant), p.Tenant, p.Subject), Scopes: p.Scopes, Expiration: time.Now().Add(time.Minute)}, nil
	}, &auth.RequireBearerTokenOptions{ResourceMetadataURL: c.Authority.Resource + "/.well-known/oauth-protected-resource"})(handler)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			allowed := false
			for _, o := range origins {
				if o == origin {
					allowed = true
				}
			}
			if !allowed {
				http.Error(w, "origin denied", 403)
				return
			}
		}
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		if strings.HasSuffix(r.URL.Path, "/.well-known/oauth-protected-resource") {
			if r.Method != http.MethodGet {
				http.Error(w, "method", 405)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"resource": c.Authority.Resource, "authorization_servers": []string{"https://issuer.example.invalid"}, "scopes_supported": []string{"orders:read", "greet"}})
			return
		}
		if r.URL.Path != "/mcp" {
			http.NotFound(w, r)
			return
		}
		protected.ServeHTTP(w, r)
	})
}
