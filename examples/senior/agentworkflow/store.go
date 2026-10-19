// Package agentworkflow is an offline teaching fixture for authorized tool
// execution and durable recovery. It never calls a model or a real provider.
package agentworkflow

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// Store uses an OS file lock plus atomic replacement on Unix (Linux/macOS).
// Each transaction reloads the file, so separate workers cannot overwrite one
// another's state. No network operation runs while holding this lock.
// Use a transactional database for a production multi-host control plane.
type Store struct{ Path string }

type state struct {
	Jobs         map[string]*Job        `json:"jobs"`
	Grants       map[string]Grant       `json:"grants"`
	Credentials  map[string]*Credential `json:"credentials"`
	Runs         map[string]*Run        `json:"runs"`
	Effects      map[string]Receipt     `json:"effects"`
	EffectCount  int                    `json:"effect_count"`
	RefreshUsed  map[string]bool        `json:"refresh_used"`
	RefreshCalls int                    `json:"refresh_calls"`
}

func (s Store) transaction(change func(*state) error) error {
	if s.Path == "" {
		return errors.New("state path required")
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0700); err != nil {
		return err
	}
	lock, err := os.OpenFile(s.Path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	x := state{Jobs: map[string]*Job{}, Grants: map[string]Grant{}, Credentials: map[string]*Credential{}, Runs: map[string]*Run{}, Effects: map[string]Receipt{}, RefreshUsed: map[string]bool{}}
	b, err := os.ReadFile(s.Path)
	if err == nil {
		x = state{} // existing but incomplete files must not look like a fresh store
		if err = json.Unmarshal(b, &x); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if x.Jobs == nil || x.Grants == nil || x.Credentials == nil || x.Runs == nil || x.Effects == nil || x.RefreshUsed == nil {
		return errors.New("incomplete state")
	}
	if err = change(&x); err != nil {
		return err
	} // rejected transactions never replace state
	b, err = json.MarshalIndent(x, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.Path), ".workflow-*")
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
	if err = os.Rename(f.Name(), s.Path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(s.Path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
