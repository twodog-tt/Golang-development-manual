package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadSecretFileRequiresPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pin")
	if err := os.WriteFile(path, []byte("user:secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	secret, err := readSecretFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if secret != "user:secret" {
		t.Fatalf("secret=%q, want trimmed credential", secret)
	}
	if err = os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err = readSecretFile(path); err == nil {
		t.Fatal("accepted a world-readable credential file")
	}
}

func TestWriteReportPublishesPrivateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	if err := writeReport(path, []byte("{\"ok\":true}\n")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("report mode=%04o, want 0600", got)
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != "{\"ok\":true}\n" {
		t.Fatalf("report=%q", encoded)
	}
}
