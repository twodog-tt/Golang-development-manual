package agentcommerce

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// Single-host Linux/macOS fixture: OS lock, reload per transaction, fsync +
// atomic rename + directory sync. Network calls never execute under this lock.
type store struct{ path string }
type database struct {
	Payments     map[string]Record
	Nonces       map[string]string
	Entries      []Entry
	Chain        map[string]Observation
	ChainEffects int
}

func (s store) tx(change func(*database) error) error {
	if s.path == "" {
		return errors.New("state path required")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	lock, err := os.OpenFile(s.path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	x := database{Payments: map[string]Record{}, Nonces: map[string]string{}, Entries: []Entry{}, Chain: map[string]Observation{}}
	f, err := os.Open(s.path)
	if err == nil {
		defer f.Close()
		raw, e := io.ReadAll(io.LimitReader(f, (2<<20)+1))
		if e != nil {
			return e
		}
		if len(raw) > 2<<20 {
			return errors.New("state too large")
		}
		x = database{}
		if e = json.Unmarshal(raw, &x); e != nil {
			return e
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if x.Payments == nil || x.Nonces == nil || x.Entries == nil || x.Chain == nil {
		return errors.New("incomplete fixture state")
	}
	if err = change(&x); err != nil {
		return err
	}
	b, err := json.Marshal(x)
	if err != nil {
		return err
	}
	if len(b) > 2<<20 {
		return errors.New("state budget reached")
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".commerce-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err = tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err = tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmp.Name(), s.path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(s.path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
