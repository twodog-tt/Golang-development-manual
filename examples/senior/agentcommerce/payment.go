package agentcommerce

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"td-homework/examples/senior/agentmemory"
)

type Principal struct{ Tenant, Subject, Policy string }

func principal(w agentmemory.Identity) Principal { return Principal{w.Tenant, w.Subject, w.Policy} }

type Record struct {
	Revision      uint64
	Scope         agentmemory.Scope
	Who           Principal
	Binding       Binding
	Mechanism     Mechanism
	Payment       Payment
	Digest        common.Hash
	State, Result string
	Receipt       Observation
}
type Entry struct {
	Intent, Kind  string
	BlockHash     common.Hash
	Debit, Credit string
	Amount        int64
}

// Observation is obtained from a trusted adapter, not an HTTP client's success flag.
// Known=false is NOT evidence of non-payment. Canceled is distinct from consumed.
type Observation struct {
	Digest                                common.Hash
	Network                               string
	Asset, From, To                       common.Address
	Value                                 string
	Nonce, Transaction, BlockHash         common.Hash
	Known, Canonical, Finalized, Canceled bool
}
type Chain interface {
	Settle(context.Context, Mechanism, Payment) (Observation, error)
	Lookup(context.Context, Mechanism, Payment) (Observation, error)
}
type Engine struct {
	Dir   string
	Payer common.Address
	Chain Chain
	// Now and AfterSettle are trusted local fault-test seams, never request fields.
	Now         func() time.Time
	AfterSettle func()
}

func (e Engine) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}
func (e Engine) db() store { return store{filepath.Join(e.Dir, "payments.json")} }
func recordKey(scope agentmemory.Scope, id string) string {
	b, _ := json.Marshal([]any{scope, id})
	return string(b)
}
func nonceKey(m Mechanism, p Payment) string {
	b, _ := json.Marshal([]string{m.Network, m.Asset, p.From.Hex(), p.Nonce.Hex()})
	return string(b)
}
func authorized(ctx context.Context, scope agentmemory.Scope, r Record) error {
	who, err := agentmemory.Check(ctx, scope, "pay")
	if err != nil || r.Scope != scope || r.Who != principal(who) {
		return ErrDenied
	}
	return nil
}
func (e Engine) Prepare(ctx context.Context, scope agentmemory.Scope, b Binding, m Mechanism, p Payment) error {
	who, err := agentmemory.Check(ctx, scope, "pay")
	if err != nil || b.Tenant != scope.Tenant || b.Subject != who.Subject || b.Subject != scope.SubjectID || b.Intent == "" || e.Dir == "" || e.Chain == nil || e.Payer == (common.Address{}) || p.From != e.Payer {
		return ErrDenied
	}
	// Reconstruct all non-body binding constraints; BodyHash is computed by trusted
	// ingress from actual request bytes. A model cannot author the stored binding.
	checked, err := NewBinding(b.Tenant, b.Subject, b.Intent, b.Method, b.Resource, nil)
	if err != nil {
		return err
	}
	checked.BodyHash = b.BodyHash
	if checked != b {
		return ErrDenied
	}
	m = m.Normalized()
	digest, err := VerifyPayment(m, p, e.now().Unix())
	if err != nil {
		return err
	}
	p.Signature = append([]byte(nil), p.Signature...)
	return e.db().tx(func(d *database) error {
		if _, err := agentmemory.Check(ctx, scope, "pay"); err != nil {
			return err
		}
		key := recordKey(scope, b.Intent)
		if old, ok := d.Payments[key]; ok {
			if old.Binding == b && old.Digest == digest && old.Mechanism == m && old.Who == principal(who) {
				return nil
			}
			return ErrDenied
		}
		nk := nonceKey(m, p)
		if _, exists := d.Nonces[nk]; exists {
			return ErrDenied
		}
		if len(d.Payments) >= 128 {
			return errors.New("fixture intent limit reached")
		}
		d.Nonces[nk] = key
		d.Payments[key] = Record{Revision: 1, Scope: scope, Who: principal(who), Binding: b, Mechanism: m, Payment: p, Digest: digest, State: "ready"}
		return nil
	})
}

// Execute takes an authenticated ingress binding for the ACTUAL request. A valid
// token-transfer signature alone is insufficient authority for this operation.
// Before work/settle, persist unknown. Repeated callers cannot take over unknown.
func (e Engine) Execute(ctx context.Context, scope agentmemory.Scope, b Binding) (string, error) {
	if e.Dir == "" || e.Chain == nil {
		return "", ErrDenied
	}
	if _, err := agentmemory.Check(ctx, scope, "pay"); err != nil {
		return "", err
	}
	var r Record
	claimed := false
	err := e.db().tx(func(d *database) error {
		var ok bool
		r, ok = d.Payments[recordKey(scope, b.Intent)]
		if !ok || r.Binding != b || r.Payment.From != e.Payer {
			return ErrDenied
		}
		if err := authorized(ctx, scope, r); err != nil {
			return err
		}
		if r.State == "paid" {
			return nil
		}
		if r.State != "ready" {
			return ErrUnknown
		}
		if _, err := VerifyPayment(r.Mechanism, r.Payment, e.now().Unix()); err != nil {
			return err
		}
		r.State = "unknown"
		r.Revision++
		r.Result = "local-report:" + bindingHash(b) // bounded deterministic work fixture, not external execution
		d.Payments[recordKey(scope, b.Intent)] = r
		claimed = true
		return nil
	})
	if err != nil {
		return "", err
	}
	if claimed {
		callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		obs, err := e.Chain.Settle(callCtx, r.Mechanism, r.Payment)
		if e.AfterSettle != nil {
			e.AfterSettle()
		}
		if err != nil || callCtx.Err() != nil {
			return "", ErrUnknown
		}
		if err = e.accept(ctx, scope, b.Intent, r, obs); err != nil {
			return "", err
		}
	}
	return e.Result(ctx, scope, b.Intent)
}
func (e Engine) Recover(ctx context.Context, scope agentmemory.Scope, id string) error {
	if e.Dir == "" || e.Chain == nil {
		return ErrDenied
	}
	r, err := e.Snapshot(ctx, scope, id)
	if err != nil {
		return err
	}
	if r.State != "unknown" && r.State != "paid" {
		return ErrDenied
	}
	callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	obs, err := e.Chain.Lookup(callCtx, r.Mechanism, r.Payment)
	if err != nil || callCtx.Err() != nil {
		return ErrUnknown
	}
	return e.accept(ctx, scope, id, r, obs)
}
func (e Engine) accept(ctx context.Context, scope agentmemory.Scope, id string, expected Record, obs Observation) error {
	unresolved := false
	err := e.db().tx(func(d *database) error {
		key := recordKey(scope, id)
		r, ok := d.Payments[key]
		if !ok {
			return ErrDenied
		}
		if err := authorized(ctx, scope, r); err != nil {
			return err
		}
		if r.State != "unknown" && r.State != "paid" {
			return ErrDenied
		}
		if r.Revision == 0 || r.Revision != expected.Revision || r.State != expected.State || r.Receipt != expected.Receipt {
			return ErrDenied
		}
		if obs.Digest != r.Digest || obs.Network != r.Mechanism.Network || obs.Asset.Hex() != r.Mechanism.Asset || obs.From != r.Payment.From || obs.To != r.Payment.To || obs.Value != r.Payment.Value || obs.Nonce != r.Payment.Nonce {
			return ErrDenied
		}
		// Absence, pending data, or a lagging node is not proof that an
		// already finalized payment was orphaned. Preserve its ledger evidence.
		if !obs.Known || !obs.Finalized || obs.Transaction == (common.Hash{}) || obs.BlockHash == (common.Hash{}) {
			if r.State == "unknown" {
				r.Receipt = obs
				r.Revision++
				d.Payments[key] = r
			}
			unresolved = true
			return nil
		}
		if !obs.Canonical {
			if r.State == "paid" {
				// An orphan report about another block cannot revoke this receipt.
				if obs.Transaction != r.Receipt.Transaction || obs.BlockHash != r.Receipt.BlockHash {
					return ErrDenied
				}
				d.Entries = append(d.Entries, Entry{key, "reversal", r.Receipt.BlockHash, "merchant", "payer", 10})
			}
			r.State = "unknown"
			r.Receipt = obs
			r.Revision++
			d.Payments[key] = r
			unresolved = true
			return nil
		}
		if r.State == "paid" && (r.Receipt.BlockHash != obs.BlockHash || r.Receipt.Transaction != obs.Transaction || obs.Canceled) {
			d.Entries = append(d.Entries, Entry{key, "reversal", r.Receipt.BlockHash, "merchant", "payer", 10})
			r.State = "unknown"
		}
		if obs.Canceled {
			r.State = "failed"
		} else if r.State != "paid" {
			d.Entries = append(d.Entries, Entry{key, "settlement", obs.BlockHash, "payer", "merchant", 10})
			r.State = "paid"
		}
		r.Receipt = obs
		r.Revision++
		d.Payments[key] = r
		return nil
	})
	if err != nil {
		return err
	}
	if unresolved {
		return ErrUnknown
	}
	return nil
}
func (e Engine) Snapshot(ctx context.Context, scope agentmemory.Scope, id string) (Record, error) {
	if e.Dir == "" || e.Payer == (common.Address{}) {
		return Record{}, ErrDenied
	}
	if _, err := agentmemory.Check(ctx, scope, "pay"); err != nil {
		return Record{}, err
	}
	var r Record
	err := e.db().tx(func(d *database) error {
		var ok bool
		r, ok = d.Payments[recordKey(scope, id)]
		if !ok || r.Payment.From != e.Payer {
			return ErrDenied
		}
		return authorized(ctx, scope, r)
	})
	if err != nil {
		return Record{}, err
	}
	return r, nil
}
func (e Engine) Result(ctx context.Context, scope agentmemory.Scope, id string) (string, error) {
	r, err := e.Snapshot(ctx, scope, id)
	if err != nil {
		return "", err
	}
	if r.State != "paid" || r.Result == "" {
		return "", ErrUnknown
	}
	return r.Result, nil
}
func (e Engine) Entries(ctx context.Context, scope agentmemory.Scope, id string) ([]Entry, error) {
	if _, err := e.Snapshot(ctx, scope, id); err != nil {
		return nil, err
	}
	var out []Entry
	err := e.db().tx(func(d *database) error {
		if err := authorized(ctx, scope, d.Payments[recordKey(scope, id)]); err != nil {
			return err
		}
		for _, entry := range d.Entries {
			if entry.Intent == recordKey(scope, id) {
				out = append(out, entry)
			}
		}
		return nil
	})
	return out, err
}

// LocalChain is a durable, independently stored ledger fixture. It never calls
// RPC, sends a transaction or uses a production key. LostReply models
// settlement_pending after a broadcast; the persisted receipt remains queryable.
type LocalChain struct {
	Dir       string
	LostReply bool
	Now       func() time.Time
}

func (c LocalChain) db() store { return store{filepath.Join(c.Dir, "chain.json")} }
func (c LocalChain) Settle(ctx context.Context, m Mechanism, p Payment) (Observation, error) {
	var out Observation
	now := time.Now()
	if c.Now != nil {
		now = c.Now()
	}
	digest, err := VerifyPayment(m, p, now.Unix())
	if err != nil {
		return out, err
	}
	err = c.db().tx(func(d *database) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		nk := nonceKey(m, p)
		if old, ok := d.Chain[nk]; ok {
			if old.Digest != digest || old.Canceled {
				return ErrDenied
			}
			out = old
			return nil
		}
		out = Observation{Digest: digest, Network: m.Network, Asset: FixtureAsset, From: p.From, To: p.To, Value: p.Value, Nonce: p.Nonce, Transaction: crypto.Keccak256Hash(digest[:], []byte("fixture-tx")), BlockHash: crypto.Keccak256Hash([]byte("fixture-block-1")), Known: true, Canonical: true, Finalized: true}
		d.Chain[nk] = out
		d.ChainEffects++
		return nil
	})
	if err != nil {
		return out, err
	}
	if c.LostReply {
		return out, ErrUnknown
	}
	return out, nil
}
func (c LocalChain) Lookup(ctx context.Context, m Mechanism, p Payment) (Observation, error) {
	digest, err := PaymentDigest(m, p)
	if err != nil {
		return Observation{}, err
	}
	out := Observation{Digest: digest, Network: m.Network, Asset: FixtureAsset, From: p.From, To: p.To, Value: p.Value, Nonce: p.Nonce}
	err = c.db().tx(func(d *database) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if v, ok := d.Chain[nonceKey(m, p)]; ok {
			out = v
		}
		return nil
	})
	return out, err
}
func (c LocalChain) Effects() (int, error) {
	n := 0
	err := c.db().tx(func(d *database) error { n = d.ChainEffects; return nil })
	return n, err
}
