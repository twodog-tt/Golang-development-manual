package frostcluster

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
)

var sessionLedgerBucket = []byte("used-sessions-v1")

// SessionLedger durably burns every DKG/signing session ID before a Taurus
// handler is constructed. A failed or interrupted ceremony remains burned
// across participant restarts so signing nonces cannot be resumed or reused.
//
// bbolt supplies an exclusive process lock and synchronous commit semantics.
// The ledger must live on local storage whose file-lock and fsync guarantees
// are trusted; do not place it on an eventually consistent shared filesystem.
type SessionLedger struct {
	path      string
	db        *bolt.DB
	closeOnce sync.Once
	closeErr  error
}

func OpenSessionLedger(path string) (*SessionLedger, error) {
	if path == "" {
		return nil, errors.New("session ledger path is required")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create session ledger directory: %w", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("inspect session ledger directory: %w", err)
	}
	if !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return nil, errors.New("session ledger directory must be private mode 0700")
	}
	if existing, statErr := os.Lstat(path); statErr == nil {
		if !existing.Mode().IsRegular() {
			return nil, errors.New("session ledger must be a regular file")
		}
		if existing.Mode().Perm()&0o077 != 0 {
			return nil, errors.New("session ledger must not grant group/other access")
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect session ledger: %w", statErr)
	}

	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open session ledger: %w", err)
	}
	ledger := &SessionLedger{path: path, db: db}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, createErr := tx.CreateBucketIfNotExists(sessionLedgerBucket)
		return createErr
	}); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize session ledger: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set session ledger permissions: %w", err)
	}
	return ledger, nil
}

func (l *SessionLedger) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// Reserve commits one session ID and its operation kind. Existing IDs are
// always rejected, even when the kind matches.
func (l *SessionLedger) Reserve(sessionID string, kind SessionKind) error {
	if l == nil || l.db == nil {
		return errors.New("session ledger is closed")
	}
	if _, err := (SessionSpec{ID: sessionID}).sessionBytes(); err != nil {
		return err
	}
	if kind != SessionKindDKG && kind != SessionKindSign {
		return fmt.Errorf("invalid session kind %q", kind)
	}
	var previous SessionKind
	err := l.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(sessionLedgerBucket)
		if bucket == nil {
			return errors.New("session ledger bucket is missing")
		}
		if value := bucket.Get([]byte(sessionID)); value != nil {
			previous = SessionKind(string(value))
			return ErrSessionReplay
		}
		return bucket.Put([]byte(sessionID), []byte(kind))
	})
	if errors.Is(err, ErrSessionReplay) {
		return fmt.Errorf("%w: %s (%s)", ErrSessionReplay, sessionID, previous)
	}
	if err != nil {
		return fmt.Errorf("reserve session in ledger: %w", err)
	}
	return nil
}

func (l *SessionLedger) Close() error {
	if l == nil {
		return nil
	}
	l.closeOnce.Do(func() {
		if l.db != nil {
			l.closeErr = l.db.Close()
		}
	})
	return l.closeErr
}
