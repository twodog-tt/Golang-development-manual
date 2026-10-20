package releaseadmission

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestFingerprintCoversActualInputs(t *testing.T) {
	for _, name := range []string{"vendor/dependency/code.go", "local/code.go", "workspace/module/code.go", "generated.go", "go.work", "binary.dat"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			file := filepath.Join(root, name)
			if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, []byte{0, 1, 2}, 0600); err != nil {
				t.Fatal(err)
			}
			before, err := Fingerprint(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, []byte{0, 1, 3}, 0600); err != nil {
				t.Fatal(err)
			}
			after, err := Fingerprint(context.Background(), root)
			if err != nil || before == after {
				t.Fatal("input change not detected", err)
			}
			if err := os.Chmod(file, 0700); err != nil {
				t.Fatal(err)
			}
			executable, err := Fingerprint(context.Background(), root)
			if err != nil || executable == after {
				t.Fatal("executable bit not bound", err)
			}
		})
	}
}
func TestFingerprintRejectsUnavailableOrUnboundedInput(t *testing.T) {
	for _, kind := range []string{"empty", "missing", "symlink", "symlink-root", "large-file", "too-many-files", "too-many-dirs", "total-bytes", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			ctx := context.Background()
			write := func(name string, b []byte) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(root, name), b, 0600); err != nil {
					t.Fatal(err)
				}
			}
			switch kind {
			case "empty":
			case "missing":
				root = filepath.Join(root, "absent")
			case "symlink":
				write("real", []byte("x"))
				if err := os.Symlink("real", filepath.Join(root, "link")); err != nil {
					t.Fatal(err)
				}
			case "symlink-root":
				write("real", []byte("x"))
				link := filepath.Join(t.TempDir(), "link")
				if err := os.Symlink(root, link); err != nil {
					t.Fatal(err)
				}
				root = link
			case "large-file":
				write("big", make([]byte, (2<<20)+1))
			case "too-many-files":
				for i := 0; i < 129; i++ {
					write(fmt.Sprint(i), []byte("x"))
				}
			case "too-many-dirs":
				write("file", []byte("x"))
				for i := 0; i < 256; i++ {
					if err := os.Mkdir(filepath.Join(root, fmt.Sprint(i)), 0700); err != nil {
						t.Fatal(err)
					}
				}
			case "total-bytes":
				for i := 0; i < 3; i++ {
					write(fmt.Sprint(i), make([]byte, 2<<20))
				}
			case "cancel":
				write("file", []byte("x"))
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if _, err := Fingerprint(ctx, root); err == nil {
				t.Fatal("invalid snapshot accepted")
			}
		})
	}
}
