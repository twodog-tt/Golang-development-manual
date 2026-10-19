package agentplatform

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

type LaunchSpec struct{ Tenant, ID, Chain, Account, TokenConfig, Liquidity, Runtime string }
type LaunchReceipt struct {
	Key, Parameters, Transaction, Block string
	Known, Canonical, Final             bool
}
type LaunchStep struct {
	State   string
	Receipt LaunchReceipt
}
type LaunchRecord struct {
	Spec     LaunchSpec
	Revision uint64
	State    string
	Steps    [3]LaunchStep
	Credited bool
}
type FeeEntry struct {
	Launch, Kind string
	Amount       int64
}
type launchDB struct {
	Format   int
	Launches map[string]LaunchRecord
	Remote   map[string]LaunchReceipt
	Effects  int
	Fees     []FeeEntry
}

// Unix-only local transactional store. Re-read under flock, then fsync/rename/
// directory fsync. Corruption is never interpreted as an empty database.
func transaction(path string, change func(*launchDB) error) error {
	if path == "" {
		return ErrDenied
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	l, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer l.Close()
	if err = syscall.Flock(int(l.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(l.Fd()), syscall.LOCK_UN)
	d := launchDB{Format: 1, Launches: map[string]LaunchRecord{}, Remote: map[string]LaunchReceipt{}, Fees: []FeeEntry{}}
	b, err := os.ReadFile(path)
	if err == nil {
		d = launchDB{}
		if err = json.Unmarshal(b, &d); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if d.Format != 1 || d.Launches == nil || d.Remote == nil || d.Fees == nil {
		return errors.New("invalid launch store")
	}
	if err = change(&d); err != nil {
		return err
	}
	b, err = json.Marshal(d)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".launch-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

type LaunchRemote interface {
	Submit(context.Context, string, string) (LaunchReceipt, error)
	Lookup(context.Context, string) (LaunchReceipt, error)
}
type Launcher struct {
	Path        string
	Remote      LaunchRemote
	AfterSubmit func()
}

func launchKey(s LaunchSpec) string         { return hash([]string{s.Tenant, s.ID}) }
func stepKey(s LaunchSpec, i int) string    { return hash([]any{s.Tenant, s.ID, i}) }
func stepParams(s LaunchSpec, i int) string { return hash([]any{"local-launch-v1", s, i}) }
func (l Launcher) Prepare(s LaunchSpec) error {
	if l.Remote == nil || s.Tenant == "" || s.ID == "" || s.Chain == "" || s.Account == "" || s.TokenConfig == "" || s.Liquidity == "" || s.Runtime == "" {
		return ErrDenied
	}
	return transaction(l.Path, func(d *launchDB) error {
		k := launchKey(s)
		if old, ok := d.Launches[k]; ok {
			if old.Spec != s {
				return ErrDenied
			}
			return nil
		}
		if len(d.Launches) >= 128 {
			return ErrDenied
		}
		r := LaunchRecord{Spec: s, Revision: 1, State: "running"}
		for i := range r.Steps {
			r.Steps[i].State = "ready"
		}
		d.Launches[k] = r
		return nil
	})
}
func (l Launcher) Snapshot(s LaunchSpec) (LaunchRecord, error) {
	var r LaunchRecord
	err := transaction(l.Path, func(d *launchDB) error {
		var ok bool
		r, ok = d.Launches[launchKey(s)]
		if !ok || r.Spec != s {
			return ErrDenied
		}
		return nil
	})
	return r, err
}
func (l Launcher) apply(s LaunchSpec, i int, revision uint64, e LaunchReceipt) error {
	return transaction(l.Path, func(d *launchDB) error {
		k := launchKey(s)
		r, ok := d.Launches[k]
		if !ok || r.Spec != s || r.Revision != revision || r.State == "review_chain" {
			return ErrDenied
		}
		if !e.Known {
			return ErrUnknown
		}
		if e.Key != stepKey(s, i) || e.Parameters != stepParams(s, i) || e.Transaction == "" || e.Block == "" {
			return ErrDenied
		}
		if !e.Final {
			return ErrUnknown
		}
		if !e.Canonical {
			// Only an explicit orphan observation for OUR previous receipt reverses
			// credited fees. RPC absence or another transaction is not that proof.
			old := r.Steps[i].Receipt
			if r.Steps[i].State != "done" || old.Transaction != e.Transaction || old.Block != e.Block {
				return ErrUnknown
			}
			r.State = "review_chain"
			if r.Credited {
				d.Fees = append(d.Fees, FeeEntry{k, "reversal", -10})
				r.Credited = false
			}
		} else {
			if r.Steps[i].State == "done" && r.Steps[i].Receipt != e {
				return ErrDenied
			}
			r.Steps[i] = LaunchStep{"done", e}
			if r.Steps[0].State == "done" && r.Steps[1].State == "done" && r.Steps[2].State == "done" {
				r.State = "completed"
				if !r.Credited {
					d.Fees = append(d.Fees, FeeEntry{k, "credit", 10})
					r.Credited = true
				}
			}
		}
		r.Revision++
		d.Launches[k] = r
		return nil
	})
}

// Reconcile ONLY queries. Even an unseen unknown operation cannot be retried:
// unseen is not a proof of non-execution. An unsupported lookup means manual review.
func (l Launcher) Reconcile(ctx context.Context, s LaunchSpec) error {
	if l.Remote == nil {
		return ErrDenied
	}
	for i := 0; i < 3; i++ {
		r, err := l.Snapshot(s)
		if err != nil {
			return err
		}
		if r.State == "review_chain" {
			return ErrDenied
		}
		if r.Steps[i].State == "ready" {
			continue
		}
		e, err := l.Remote.Lookup(ctx, stepKey(s, i))
		if err != nil || ctx.Err() != nil {
			return ErrUnknown
		}
		if err = l.apply(s, i, r.Revision, e); err != nil {
			return err
		}
		// apply must commit the quarantine/reversal before we report the block.
		// This must also hold for the final stage (there is no next loop iteration).
		if !e.Canonical {
			return ErrDenied
		}
	}
	return nil
}
func (l Launcher) Advance(ctx context.Context, s LaunchSpec) error {
	if err := l.Reconcile(ctx, s); err != nil {
		return err
	}
	var stage int
	var rev uint64
	claimed := false
	err := transaction(l.Path, func(d *launchDB) error {
		r, ok := d.Launches[launchKey(s)]
		if !ok || r.Spec != s || r.State == "review_chain" {
			return ErrDenied
		}
		if r.State == "completed" {
			return nil
		}
		for i, x := range r.Steps {
			if x.State == "done" {
				continue
			}
			if x.State != "ready" {
				return ErrUnknown
			}
			r.Steps[i].State = "unknown"
			r.Revision++
			rev = r.Revision
			stage = i
			claimed = true
			d.Launches[launchKey(s)] = r
			return nil
		}
		return ErrDenied
	})
	if err != nil || !claimed {
		return err
	}
	e, err := l.Remote.Submit(ctx, stepKey(s, stage), stepParams(s, stage))
	if l.AfterSubmit != nil {
		l.AfterSubmit()
	}
	if err != nil || ctx.Err() != nil {
		return ErrUnknown
	}
	return l.apply(s, stage, rev, e)
}
func (l Launcher) FeeBalance() (int64, error) {
	var n int64
	err := transaction(l.Path, func(d *launchDB) error {
		for _, e := range d.Fees {
			n += e.Amount
		}
		return nil
	})
	return n, err
}

// LocalRemote is an INDEPENDENT durable effect ledger, not an EVM, wallet or
// provider implementation. Submit is intentionally non-idempotent to expose
// accidental retries. Fault hooks belong to trusted local tests only.
type LocalRemote struct {
	Path      string
	LoseReply bool
}

func (r LocalRemote) Submit(ctx context.Context, key, params string) (LaunchReceipt, error) {
	if ctx.Err() != nil {
		return LaunchReceipt{}, ctx.Err()
	}
	var e LaunchReceipt
	err := transaction(r.Path, func(d *launchDB) error {
		d.Effects++
		e = LaunchReceipt{key, params, hash([]any{key, d.Effects}), hash([]any{"block", d.Effects}), true, true, true}
		d.Remote[key] = e
		return nil
	})
	if err == nil && r.LoseReply {
		err = ErrUnknown
	}
	return e, err
}
func (r LocalRemote) Lookup(ctx context.Context, key string) (LaunchReceipt, error) {
	if ctx.Err() != nil {
		return LaunchReceipt{}, ctx.Err()
	}
	var e LaunchReceipt
	err := transaction(r.Path, func(d *launchDB) error { e = d.Remote[key]; return nil })
	return e, err
}
func (r LocalRemote) Effects() (int, error) {
	n := 0
	err := transaction(r.Path, func(d *launchDB) error { n = d.Effects; return nil })
	return n, err
}
func (r LocalRemote) Orphan(key string) error {
	return transaction(r.Path, func(d *launchDB) error {
		e, ok := d.Remote[key]
		if !ok {
			return ErrDenied
		}
		e.Canonical = false
		d.Remote[key] = e
		return nil
	})
}
