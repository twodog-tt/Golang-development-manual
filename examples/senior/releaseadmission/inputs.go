package releaseadmission

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

// Fingerprint covers ALL regular files in a dedicated immutable source snapshot,
// including vendor, local replace/workspace modules and generated sources.
// The caller must include every workspace root; omitted inputs cannot be inferred.
// It rejects symlinks/special files and bounds file count/total bytes. It does not
// turn a concurrently mutable checkout into an atomic snapshot or authenticate it.
func Fingerprint(ctx context.Context, root string) (string, error) {
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", ErrDenied
	}
	type entry struct {
		Path, Digest string
		Executable   bool
	}
	entries := []entry{}
	total := int64(0)
	visited := 0
	err = filepath.WalkDir(root, func(path string, item os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		visited++
		if visited > 256 {
			return ErrDenied
		}
		if item.IsDir() {
			return nil
		}
		if item.Type()&os.ModeSymlink != 0 {
			return ErrDenied
		}
		i, err := item.Info()
		if err != nil || !i.Mode().IsRegular() || len(entries) >= 128 {
			return ErrDenied
		}
		if i.Size() > 2<<20 || i.Size() < 0 {
			return ErrDenied
		}
		total += i.Size()
		if total > 4<<20 {
			return ErrDenied
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		actual, err := f.Stat()
		if err != nil || !actual.Mode().IsRegular() || !os.SameFile(i, actual) {
			f.Close()
			return ErrDenied
		}
		b, err := io.ReadAll(io.LimitReader(f, (2<<20)+1))
		closeErr := f.Close()
		if err != nil || closeErr != nil || int64(len(b)) != i.Size() {
			return ErrDenied
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		entries = append(entries, entry{filepath.ToSlash(rel), Digest(b), i.Mode()&0111 != 0})
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", ErrDenied
	}
	return binding(entries), nil
}
