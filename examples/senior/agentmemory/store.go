package agentmemory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

type Rule struct {
	ID             string
	Scope          Scope
	Text           string
	Sources        []string
	Status         string
	Version        uint64
	Policy, Writer string
	Reviewer       Identity
	ApprovedDigest string
	Expires        time.Time
	Deleted        bool
}
type Ref struct {
	ID      string
	Version uint64
}
type projection struct {
	Version uint64
	Text    string
}
type job struct {
	ID      string
	Version uint64
	Done    bool
}
type state struct {
	Rules        map[string]Rule
	Index, Cache map[string]projection
	Jobs         map[string]job
}
type Store struct{ Dir string }

func (s Store) tx(fn func(*state) error) error {
	if s.Dir == "" {
		return errors.New("memory directory required")
	}
	if err := os.MkdirAll(s.Dir, 0700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(s.Dir, "lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	d := state{map[string]Rule{}, map[string]projection{}, map[string]projection{}, map[string]job{}}
	b, err := os.ReadFile(filepath.Join(s.Dir, "state.json"))
	if err == nil {
		d = state{} // Missing fields in an existing journal must fail closed.
		if err = json.Unmarshal(b, &d); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if d.Rules == nil || d.Index == nil || d.Cache == nil || d.Jobs == nil {
		return errors.New("incomplete memory journal")
	}
	if err = fn(&d); err != nil {
		return err
	}
	b, err = json.Marshal(d)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.Dir, ".memory-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(f.Name(), filepath.Join(s.Dir, "state.json")); err != nil {
		return err
	}
	dir, err := os.Open(s.Dir)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
func digest(r Rule) string {
	b, _ := json.Marshal(struct {
		Scope   Scope
		Text    string
		Sources []string
		Version uint64
		Policy  string
		Expires time.Time
	}{r.Scope, r.Text, r.Sources, r.Version, r.Policy, r.Expires})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func key(scope Scope, id string) string { b, _ := json.Marshal([]any{scope, id}); return string(b) }
func queue(d *state, k string, r Rule) {
	id := fmt.Sprintf("%s@%d", k, r.Version)
	if _, ok := d.Jobs[id]; !ok {
		d.Jobs[id] = job{k, r.Version, false}
	}
}
func (s Store) Create(ctx context.Context, scope Scope, id, text string, sources []string, expires time.Time) (Rule, error) {
	who, err := Check(ctx, scope, "write")
	if err != nil {
		return Rule{}, err
	}
	if id == "" || text == "" || len(text) > 4096 || len(sources) == 0 || !expires.After(time.Now()) {
		return Rule{}, errors.New("rule input or expiry invalid")
	}
	r := Rule{ID: id, Scope: scope, Text: text, Sources: append([]string(nil), sources...), Version: 1, Status: "candidate", Policy: who.Policy, Writer: who.Subject, Expires: expires}
	err = s.tx(func(d *state) error {
		if _, e := Check(ctx, scope, "write"); e != nil {
			return e
		}
		k := key(scope, id)
		if _, ok := d.Rules[k]; ok {
			return ErrConflict
		}
		d.Rules[k] = r
		return nil
	})
	return r, err
}
func (s Store) Get(ctx context.Context, scope Scope, id string) (Rule, error) {
	if _, err := Check(ctx, scope, "read"); err != nil {
		return Rule{}, err
	}
	var r Rule
	err := s.tx(func(d *state) error {
		var ok bool
		r, ok = d.Rules[key(scope, id)]
		who, e := Check(ctx, scope, "read")
		if e != nil {
			return e
		}
		if !ok || r.Deleted || r.Policy != who.Policy {
			return ErrDenied
		}
		return nil
	})
	if err != nil {
		return Rule{}, err
	}
	return r, nil
}
func (s Store) ReviewDigest(ctx context.Context, scope Scope, id string) (string, uint64, error) {
	r, err := s.Get(ctx, scope, id)
	if err != nil {
		return "", 0, err
	}
	return digest(r), r.Version, nil
}
func (s Store) Approve(ctx context.Context, scope Scope, id, reviewedDigest string, version uint64) error {
	who, err := Check(ctx, scope, "review")
	if err != nil {
		return err
	}
	return s.tx(func(d *state) error {
		if _, e := Check(ctx, scope, "review"); e != nil {
			return e
		}
		k := key(scope, id)
		r, ok := d.Rules[k]
		if !ok || r.Deleted {
			return ErrDenied
		}
		if r.Writer == who.Subject {
			return ErrDenied
		}
		if r.Status != "candidate" || r.Version != version || digest(r) != reviewedDigest || r.Policy != who.Policy || !r.Expires.After(time.Now()) {
			return ErrConflict
		}
		r.Status = "approved"
		r.Reviewer = who
		r.ApprovedDigest = reviewedDigest
		d.Rules[k] = r
		return nil
	})
}

// Transition activates only a reviewed revision; retire/reject revoke immediately.
func (s Store) Transition(ctx context.Context, scope Scope, id, to string, version uint64) error {
	who, err := Check(ctx, scope, "activate")
	if err != nil {
		return err
	}
	return s.tx(func(d *state) error {
		if _, e := Check(ctx, scope, "activate"); e != nil {
			return e
		}
		k := key(scope, id)
		r, ok := d.Rules[k]
		if !ok || r.Deleted {
			return ErrDenied
		}
		if r.Version != version {
			return ErrConflict
		}
		if to == "active" {
			if r.Status != "approved" || r.Policy != who.Policy || r.ApprovedDigest != digest(r) || !reviewerLive(ctx, r.Reviewer, scope) || !r.Expires.After(time.Now()) {
				return ErrConflict
			}
			r.Status = "active"
		} else if to == "retired" && (r.Status == "active" || r.Status == "approved" || r.Status == "candidate") {
			r.Status = to
			r.Version++
			r.ApprovedDigest = ""
		} else if to == "rejected" && r.Status == "candidate" {
			r.Status = to
			r.Version++
		} else {
			return ErrConflict
		}
		d.Rules[k] = r
		queue(d, k, r)
		return nil
	})
}
func (s Store) Edit(ctx context.Context, scope Scope, id, text string, version uint64) (Rule, error) {
	who, err := Check(ctx, scope, "write")
	if err != nil {
		return Rule{}, err
	}
	if text == "" || len(text) > 4096 {
		return Rule{}, errors.New("invalid rule text")
	}
	var out Rule
	err = s.tx(func(d *state) error {
		if _, e := Check(ctx, scope, "write"); e != nil {
			return e
		}
		k := key(scope, id)
		r, ok := d.Rules[k]
		if !ok || r.Deleted {
			return ErrDenied
		}
		if r.Version != version || r.Status == "retired" || r.Status == "rejected" {
			return ErrConflict
		}
		r.Text = text
		r.Version++
		r.Policy = who.Policy
		r.Writer = who.Subject
		r.Status = "candidate"
		r.ApprovedDigest = ""
		r.Reviewer = Identity{}
		d.Rules[k] = r
		queue(d, k, r)
		out = r
		return nil
	})
	return out, err
}
func (s Store) Delete(ctx context.Context, scope Scope, id string) error {
	if _, err := Check(ctx, scope, "delete"); err != nil {
		return err
	}
	return s.tx(func(d *state) error {
		if _, e := Check(ctx, scope, "delete"); e != nil {
			return e
		}
		k := key(scope, id)
		r, ok := d.Rules[k]
		if !ok {
			return ErrDenied
		}
		if r.Deleted {
			return nil
		}
		r.Version++
		r.Deleted = true
		r.Status = "retired"
		r.Text = ""
		r.Sources = nil
		r.ApprovedDigest = ""
		r.Reviewer = Identity{}
		d.Rules[k] = r
		queue(d, k, r)
		return nil
	})
}

// Resolve always rechecks the authoritative row. A cached/indexed old revision or
// historical run snapshot cannot restore deleted/retired/expired content.
func (s Store) Resolve(ctx context.Context, scope Scope, refs []Ref) ([]Rule, error) {
	who, err := Check(ctx, scope, "read")
	if err != nil {
		return nil, err
	}
	out := []Rule{}
	err = s.tx(func(d *state) error {
		if _, e := Check(ctx, scope, "read"); e != nil {
			return e
		}
		for _, ref := range refs {
			r, ok := d.Rules[key(scope, ref.ID)]
			if !ok || r.Deleted || r.Status != "active" || r.Version != ref.Version || r.Policy != who.Policy || !r.Expires.After(time.Now()) {
				continue
			}
			r.Sources = append([]string(nil), r.Sources...)
			out = append(out, r)
		}
		return nil
	})
	return out, err
}

// SyncOne simulates independent index/cache writes followed by an Outbox ACK.
// Both projections are local; the injected lost ACK leaves the job pending.
func (s Store) SyncOne(loseACK bool) (bool, error) {
	var chosen string
	err := s.tx(func(d *state) error {
		ids := []string{}
		for id, j := range d.Jobs {
			if !j.Done {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		if len(ids) == 0 {
			return nil
		}
		chosen = ids[0]
		j := d.Jobs[chosen]
		r, ok := d.Rules[j.ID]
		if !ok {
			return errors.New("missing source row")
		}
		if r.Version == j.Version && r.Status == "active" && !r.Deleted {
			p := projection{r.Version, r.Text}
			d.Index[j.ID] = p
			d.Cache[j.ID] = p
		} else if r.Deleted || r.Status != "active" {
			delete(d.Index, j.ID)
			delete(d.Cache, j.ID)
		}
		return nil
	})
	if err != nil || chosen == "" {
		return false, err
	}
	if loseACK {
		return true, errors.New("injected projection ACK loss")
	}
	err = s.tx(func(d *state) error { j := d.Jobs[chosen]; j.Done = true; d.Jobs[chosen] = j; return nil })
	return true, err
}
func (s Store) Sync() error {
	for {
		more, err := s.SyncOne(false)
		if err != nil {
			return err
		}
		if !more {
			return nil
		}
	}
}
