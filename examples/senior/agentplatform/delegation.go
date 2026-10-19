package agentplatform

import (
	"crypto/ed25519"
	"encoding/json"
	"sync"
)

type Grant struct {
	ID, Parent, Issuer, Subject, Owner, Tenant, Audience, Resource string
	Scopes                                                         []string
	Deadline, Budget                                               int64
}
type SignedGrant struct {
	Grant     Grant
	Signature []byte
}

func SignGrant(g Grant, key ed25519.PrivateKey) SignedGrant {
	b, _ := json.Marshal(g)
	return SignedGrant{g, ed25519.Sign(key, append([]byte("local-agent-delegation:v1\x00"), b...))}
}

type reservation struct {
	Leaf, Scope string
	Amount      int64
}
type Delegator struct {
	mu           sync.Mutex
	root         string
	keys         map[string]ed25519.PublicKey
	revoked      map[string]bool
	used         map[string]int64
	reservations map[string]reservation
}

func NewDelegator(root string, keys map[string]ed25519.PublicKey) *Delegator {
	copyKeys := map[string]ed25519.PublicKey{}
	for k, v := range keys {
		copyKeys[k] = append(ed25519.PublicKey(nil), v...)
	}
	return &Delegator{root: root, keys: copyKeys, revoked: map[string]bool{}, used: map[string]int64{}, reservations: map[string]reservation{}}
}
func (d *Delegator) Revoke(id string) { d.mu.Lock(); defer d.mu.Unlock(); d.revoked[id] = true }
func validScopes(s []string) bool {
	if len(s) == 0 || len(s) > 8 {
		return false
	}
	seen := map[string]bool{}
	for _, v := range s {
		if v == "" || seen[v] {
			return false
		}
		seen[v] = true
	}
	return true
}
func subset(a, b []string) bool {
	for _, x := range a {
		found := false
		for _, y := range b {
			if x == y {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Reserve revalidates the ENTIRE chain under the same lock as budget charging.
// An unknown effect keeps its reservation; no automatic timeout refund exists.
func (d *Delegator) Reserve(chain []SignedGrant, caller Principal, audience, resource, scope, intent string, amount, now int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.root == "" || caller.Tenant == "" || caller.Subject == "" || audience == "" || resource == "" || scope == "" || len(chain) == 0 || len(chain) > 4 || intent == "" || amount <= 0 {
		return ErrDenied
	}
	ids := map[string]bool{}
	actors := map[string]bool{}
	digests := []string{}
	for i, s := range chain {
		g := s.Grant
		key := d.keys[g.Issuer]
		b, _ := json.Marshal(g)
		if len(key) != ed25519.PublicKeySize || !ed25519.Verify(key, append([]byte("local-agent-delegation:v1\x00"), b...), s.Signature) || g.ID == "" || ids[g.ID] || actors[g.Subject] || g.Subject == "" || g.Owner == "" || d.revoked[g.ID] || g.Tenant != caller.Tenant || g.Audience != audience || g.Resource != resource || g.Deadline <= now || g.Budget <= 0 || !validScopes(g.Scopes) {
			return ErrDenied
		}
		if i == 0 {
			if g.Issuer != d.root || g.Parent != "" {
				return ErrDenied
			}
		} else {
			p := chain[i-1].Grant
			if g.Issuer != p.Subject || g.Parent != hash(p) || g.Owner != p.Owner || g.Deadline > p.Deadline || g.Budget > p.Budget || !subset(g.Scopes, p.Scopes) {
				return ErrDenied
			}
		}
		ids[g.ID] = true
		actors[g.Subject] = true
		digests = append(digests, hash(g))
	}
	leaf := chain[len(chain)-1].Grant
	if leaf.Subject != caller.Subject || !subset([]string{scope}, leaf.Scopes) {
		return ErrDenied
	}
	// Intent uniqueness is rooted in owner/tenant, so switching children cannot
	// recharge or reinterpret the same business operation with another binding.
	rk := hash([]string{caller.Tenant, chain[0].Grant.Owner, intent})
	if old, ok := d.reservations[rk]; ok {
		if old.Leaf == hash(leaf) && old.Scope == scope && old.Amount == amount {
			return nil
		}
		return ErrDenied
	}
	for i, k := range digests {
		if amount > chain[i].Grant.Budget-d.used[k] {
			return ErrDenied
		}
	}
	for _, k := range digests {
		d.used[k] += amount
	}
	d.reservations[rk] = reservation{hash(leaf), scope, amount}
	return nil
}
func (d *Delegator) Used(g Grant) int64 { d.mu.Lock(); defer d.mu.Unlock(); return d.used[hash(g)] }
