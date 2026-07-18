package walreplay

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"

	"td-homework/examples/senior/matchingengine"
)

type snapshotEnvelope struct {
	Version  int             `json:"version"`
	Payload  json.RawMessage `json:"payload"`
	Checksum uint32          `json:"checksum_crc32c"`
}

func WriteSnapshot(path string, snapshot matchingengine.Snapshot) error {
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	envelope := snapshotEnvelope{
		Version:  1,
		Payload:  payload,
		Checksum: crc32.Checksum(payload, castagnoli),
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')

	dir := filepath.Dir(path)
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	cleanup := func() {
		_ = temp.Close()
		_ = os.Remove(tempName)
	}

	if err = temp.Chmod(0o600); err != nil {
		cleanup()
		return err
	}
	if err = writeFull(temp, encoded); err != nil {
		cleanup()
		return err
	}
	if err = temp.Sync(); err != nil {
		cleanup()
		return err
	}
	if err = temp.Close(); err != nil {
		_ = os.Remove(tempName)
		return err
	}
	if err = os.Rename(tempName, path); err != nil {
		_ = os.Remove(tempName)
		return err
	}

	// Persist the rename itself on filesystems that support directory fsync.
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func ReadSnapshot(path string) (matchingengine.Snapshot, bool, error) {
	encoded, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return matchingengine.Snapshot{}, false, nil
	}
	if err != nil {
		return matchingengine.Snapshot{}, false, err
	}
	var envelope snapshotEnvelope
	if err = json.Unmarshal(encoded, &envelope); err != nil {
		return matchingengine.Snapshot{}, false, fmt.Errorf("decode snapshot envelope: %w", err)
	}
	if envelope.Version != 1 {
		return matchingengine.Snapshot{}, false, fmt.Errorf("unsupported snapshot envelope version %d", envelope.Version)
	}
	if got := crc32.Checksum(envelope.Payload, castagnoli); got != envelope.Checksum {
		return matchingengine.Snapshot{}, false, errors.New("snapshot checksum mismatch")
	}
	var snapshot matchingengine.Snapshot
	if err = json.Unmarshal(envelope.Payload, &snapshot); err != nil {
		return matchingengine.Snapshot{}, false, fmt.Errorf("decode snapshot payload: %w", err)
	}
	return snapshot, true, nil
}
