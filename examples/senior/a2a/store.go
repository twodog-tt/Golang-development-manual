// Package a2a is a local HTTP+JSON 1.0 subset and durable recovery fixture.
// It does not claim full A2A conformance. File locking requires Unix.
package a2a

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

type store struct{ dir string }

func (s store) tx(fn func(*database) error) error {
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(s.dir, "lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	d := database{Tasks: map[string]Task{}, Messages: map[string]savedReply{}, Intents: map[string]Intent{}, Deliveries: map[string]string{}}
	b, err := os.ReadFile(filepath.Join(s.dir, "state.json"))
	if err == nil {
		if err = json.Unmarshal(b, &d); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if d.Tasks == nil || d.Messages == nil || d.Intents == nil || d.Deliveries == nil {
		return errors.New("incomplete journal")
	}
	if err = fn(&d); err != nil {
		return err
	}
	b, err = json.Marshal(d)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.dir, ".state-*")
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
	if err = os.Rename(f.Name(), filepath.Join(s.dir, "state.json")); err != nil {
		return err
	}
	dir, err := os.Open(s.dir)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

type database struct {
	Deduplicate *bool
	Tasks       map[string]Task
	Messages    map[string]savedReply
	Intents     map[string]Intent
	Deliveries  map[string]string
	Effects     int
}
