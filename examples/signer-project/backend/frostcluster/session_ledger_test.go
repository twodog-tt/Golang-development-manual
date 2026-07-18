package frostcluster

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSessionLedgerRejectsReplayAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "participant", "sessions.db")
	sessionID, err := NewSessionID()
	if err != nil {
		t.Fatal(err)
	}

	first, err := OpenSessionLedger(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Reserve(sessionID, SessionKindSign); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := OpenSessionLedger(path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if err := second.Reserve(sessionID, SessionKindSign); !errors.Is(err, ErrSessionReplay) {
		t.Fatalf("replayed session error=%v, want ErrSessionReplay", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("session ledger mode=%#o, want 0600", info.Mode().Perm())
	}
}

func TestDecodeProtocolMessageRejectsMalformedCBOR(t *testing.T) {
	if _, _, err := decodeProtocolMessage([]byte{0xff}); err == nil {
		t.Fatal("malformed CBOR was accepted")
	}
}
