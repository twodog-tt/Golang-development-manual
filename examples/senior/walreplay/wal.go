// Package walreplay demonstrates framed WAL records, explicit torn-tail repair,
// checksummed snapshots, and deterministic matching-engine recovery.
package walreplay

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"sync"

	"td-homework/examples/senior/matchingengine"
)

const (
	recordVersion = 1
	maxRecordSize = 1 << 20
)

var castagnoli = crc32.MakeTable(crc32.Castagnoli)

type Durability int

const (
	NoSync Durability = iota
	SyncEveryRecord
)

type diskRecord struct {
	Version int                    `json:"version"`
	Command matchingengine.Command `json:"command"`
}

type WAL struct {
	mu         sync.Mutex
	file       *os.File
	durability Durability
}

func Open(path string, durability Durability) (*WAL, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	return &WAL{file: file, durability: durability}, nil
}

func (w *WAL) Append(command matchingengine.Command) error {
	if err := matchingengine.ValidateCommand(command, command.Sequence); err != nil {
		return fmt.Errorf("validate command: %w", err)
	}
	payload, err := json.Marshal(diskRecord{Version: recordVersion, Command: command})
	if err != nil {
		return err
	}
	if len(payload) > maxRecordSize {
		return fmt.Errorf("WAL record too large: %d", len(payload))
	}

	frame := make([]byte, 4+len(payload)+4)
	binary.BigEndian.PutUint32(frame[:4], uint32(len(payload)))
	copy(frame[4:], payload)
	binary.BigEndian.PutUint32(frame[4+len(payload):], crc32.Checksum(payload, castagnoli))

	w.mu.Lock()
	defer w.mu.Unlock()
	if err := writeFull(w.file, frame); err != nil {
		return err
	}
	if w.durability == SyncEveryRecord {
		return w.file.Sync()
	}
	return nil
}

func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

type TornTailError struct {
	Offset     int64
	ValidBytes int64
}

func (e *TornTailError) Error() string {
	return fmt.Sprintf("torn WAL tail at byte %d; valid prefix is %d bytes", e.Offset, e.ValidBytes)
}

type ChecksumError struct {
	Offset int64
}

func (e *ChecksumError) Error() string {
	return fmt.Sprintf("WAL checksum mismatch at byte %d", e.Offset)
}

// Scan returns every complete record and the length of the valid prefix.
// A partial final frame is reported as TornTailError. A complete frame with a
// bad checksum is treated as corruption and must never be silently truncated.
func Scan(path string) ([]matchingengine.Command, int64, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()

	var (
		commands   []matchingengine.Command
		offset     int64
		validBytes int64
		header     [4]byte
	)
	for {
		_, err = io.ReadFull(file, header[:])
		switch {
		case errors.Is(err, io.EOF):
			return commands, validBytes, nil
		case errors.Is(err, io.ErrUnexpectedEOF):
			return commands, validBytes, &TornTailError{Offset: offset, ValidBytes: validBytes}
		case err != nil:
			return commands, validBytes, err
		}

		length := int(binary.BigEndian.Uint32(header[:]))
		if length <= 0 || length > maxRecordSize {
			return commands, validBytes, fmt.Errorf("invalid WAL record length %d at byte %d", length, offset)
		}
		payload := make([]byte, length)
		if _, err = io.ReadFull(file, payload); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return commands, validBytes, &TornTailError{Offset: offset, ValidBytes: validBytes}
			}
			return commands, validBytes, err
		}
		var checksumBytes [4]byte
		if _, err = io.ReadFull(file, checksumBytes[:]); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return commands, validBytes, &TornTailError{Offset: offset, ValidBytes: validBytes}
			}
			return commands, validBytes, err
		}
		if got, want := crc32.Checksum(payload, castagnoli), binary.BigEndian.Uint32(checksumBytes[:]); got != want {
			return commands, validBytes, &ChecksumError{Offset: offset}
		}

		var record diskRecord
		if err = json.Unmarshal(payload, &record); err != nil {
			return commands, validBytes, fmt.Errorf("decode WAL record at byte %d: %w", offset, err)
		}
		if record.Version != recordVersion {
			return commands, validBytes, fmt.Errorf("unsupported WAL record version %d at byte %d", record.Version, offset)
		}
		if err = matchingengine.ValidateCommand(record.Command, record.Command.Sequence); err != nil {
			return commands, validBytes, fmt.Errorf("invalid command at byte %d: %w", offset, err)
		}
		commands = append(commands, record.Command)
		offset += int64(4 + length + 4)
		validBytes = offset
	}
}

func RepairTornTail(path string, validBytes int64) error {
	if validBytes < 0 {
		return errors.New("validBytes must not be negative")
	}
	return os.Truncate(path, validBytes)
}

func writeFull(writer io.Writer, payload []byte) error {
	for len(payload) > 0 {
		n, err := writer.Write(payload)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		payload = payload[n:]
	}
	return nil
}
