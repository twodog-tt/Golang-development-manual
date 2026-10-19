package agentworkflow

import (
	"context"
	"encoding/json"
	"fmt"
)

// FixtureProvider has its own durable file, independent from the control plane.
// Real providers must enforce their documented key scope, payload binding and
// retention. The local lease epoch is deliberately not sent as remote fencing.
type FixtureProvider struct {
	Store       Store
	Deduplicate bool
}

func (p FixtureProvider) Idempotent() bool { return p.Deduplicate }
func effectKey(q Request) string {
	b, _ := json.Marshal([]string{q.Binding.Tenant, q.Binding.Provider, q.Binding.Account, q.Intent})
	return string(b)
}
func (p FixtureProvider) Publish(ctx context.Context, q Request) (Receipt, error) {
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	var receipt Receipt
	err := p.Store.transaction(func(s *state) error {
		key := effectKey(q)
		if old, ok := s.Effects[key]; ok && p.Deduplicate {
			if !matches(q, old) {
				return ErrConflict
			}
			receipt = old
			return nil
		}
		s.EffectCount++
		receipt = Receipt{fmt.Sprint("object-", s.EffectCount), q.Intent, q.Digest, q.Binding}
		if !p.Deduplicate {
			key = fmt.Sprintf("%s/%d", key, s.EffectCount)
		}
		s.Effects[key] = receipt
		return nil
	})
	return receipt, err
}
func (p FixtureProvider) Lookup(ctx context.Context, q Request) (Receipt, bool, error) {
	if err := ctx.Err(); err != nil {
		return Receipt{}, false, err
	}
	var out Receipt
	found := false
	err := p.Store.transaction(func(s *state) error {
		for _, r := range s.Effects {
			if r.Intent == q.Intent && r.Binding == q.Binding {
				if found || !matches(q, r) {
					return ErrConflict
				}
				out = r
				found = true
			}
		}
		return nil
	})
	return out, found, err
}
func (p FixtureProvider) Count() (int, error) {
	var n int
	err := p.Store.transaction(func(s *state) error { n = s.EffectCount; return nil })
	return n, err
}
