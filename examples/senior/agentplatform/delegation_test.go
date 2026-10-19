package agentplatform

import (
	"crypto/ed25519"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

func key(name string) ed25519.PrivateKey {
	return ed25519.NewKeyFromSeed([]byte(digest([]byte(name)))[:32])
} // public teaching seeds only
func delegation() (*Delegator, []SignedGrant) {
	keys := map[string]ed25519.PublicKey{}
	for _, n := range []string{"platform", "agent-a", "agent-b", "agent-c"} {
		keys[n] = key(n).Public().(ed25519.PublicKey)
	}
	d := NewDelegator("platform", keys)
	root := Grant{"root", "", "platform", "agent-a", "user", "tenant-a", "tool-api", "report", []string{"read", "invoke"}, 200, 8}
	child := Grant{"child", hash(root), "agent-a", "agent-b", "user", "tenant-a", "tool-api", "report", []string{"invoke"}, 180, 8}
	return d, []SignedGrant{SignGrant(root, key("platform")), SignGrant(child, key("agent-a"))}
}
func reserve(d *Delegator, chain []SignedGrant, id string, n int64) error {
	return d.Reserve(chain, Principal{"tenant-a", "agent-b"}, "tool-api", "report", "invoke", id, n, 100)
}

func TestDelegationSignaturesAudienceAttenuationAndParentRevocation(t *testing.T) {
	d, c := delegation()
	must(t, reserve(d, c, "ok", 1))
	must(t, reserve(d, c, "ok", 1))
	denied(t, reserve(d, c, "ok", 2))
	for _, name := range []string{"signature", "issuer", "parent", "tenant", "audience", "resource", "scope", "deadline", "budget", "owner", "cycle", "empty-id"} {
		t.Run(name, func(t *testing.T) {
			d, c := delegation()
			g := c[1].Grant
			switch name {
			case "signature":
				c[1].Signature[0] ^= 1
				denied(t, reserve(d, c, "bad", 1))
				return
			case "issuer":
				g.Issuer = "agent-c"
			case "parent":
				g.Parent = "wrong"
			case "tenant":
				g.Tenant = "other"
			case "audience":
				g.Audience = "wallet-api"
			case "resource":
				g.Resource = "private"
			case "scope":
				g.Scopes = []string{"sign"}
			case "deadline":
				g.Deadline = 201
			case "budget":
				g.Budget = 9
			case "owner":
				g.Owner = "other-user"
			case "cycle":
				g.Subject = "agent-a"
			case "empty-id":
				g.ID = ""
			}
			c[1] = SignGrant(g, key(g.Issuer))
			denied(t, reserve(d, c, "bad", 1))
			if d.Used(c[0].Grant) != 0 {
				t.Fatal("rejected chain charged budget")
			}
		})
	}
	d, c = delegation()
	denied(t, reserve(d, nil, "empty-chain", 1))
	denied(t, reserve(d, append(append([]SignedGrant(nil), c...), c...), "cycle", 1))
	tooDeep := append(append([]SignedGrant(nil), c...), c...)
	tooDeep = append(tooDeep, c[0])
	denied(t, reserve(d, tooDeep, "depth", 1))
	denied(t, d.Reserve(c, Principal{"tenant-a", "agent-b"}, "", "report", "invoke", "empty-aud", 1, 100))
	denied(t, d.Reserve(c, Principal{"tenant-a", "impostor"}, "tool-api", "report", "invoke", "x", 1, 100))
	denied(t, d.Reserve(c, Principal{"tenant-a", "agent-b"}, "tool-api", "report", "invoke", "x", 1, 180))
	d.Revoke("root")
	denied(t, reserve(d, c, "new", 1))
}

func TestDelegationConcurrentChildrenShareAncestorReservation(t *testing.T) {
	d, c := delegation()
	var won atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			chain := append([]SignedGrant(nil), c...)
			g := chain[1].Grant
			g.ID = fmt.Sprint("child-", i)
			chain[1] = SignGrant(g, key("agent-a"))
			if reserve(d, chain, fmt.Sprint("intent-", i), 1) == nil {
				won.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if won.Load() != 8 || d.Used(c[0].Grant) != 8 {
		t.Fatal("fan-out exceeded ancestor budget", won.Load())
	}
}

func TestDelegationCannotMoveAnIntentToAnotherLeaf(t *testing.T) {
	t.Run("same-leaf-different-scope", func(t *testing.T) {
		d, chain := delegation()
		g := chain[1].Grant
		g.Scopes = []string{"read", "invoke"}
		chain[1] = SignGrant(g, key("agent-a"))
		must(t, reserve(d, chain, "same-operation", 2))
		denied(t, d.Reserve(chain, Principal{"tenant-a", "agent-b"}, "tool-api", "report", "read", "same-operation", 2, 100))
		if d.Used(chain[0].Grant) != 2 {
			t.Fatal("scope substitution changed the reservation")
		}
	})
	d, c := delegation()
	must(t, reserve(d, c, "operation", 2))
	g := c[1].Grant
	g.ID = "another-child"
	c[1] = SignGrant(g, key("agent-a"))
	denied(t, reserve(d, c, "operation", 2))
	if d.Used(c[0].Grant) != 2 {
		t.Fatal("duplicate reservation")
	}
	d.Revoke("root")
	denied(t, reserve(d, c, "operation", 2))
}
