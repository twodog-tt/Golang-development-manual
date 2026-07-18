package walreplay

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"td-homework/examples/senior/matchingengine"
)

func TestSnapshotAndIncrementalReplay(t *testing.T) {
	dir := t.TempDir()
	walPath := filepath.Join(dir, "orders.wal")
	snapshotPath := filepath.Join(dir, "orders.snapshot")

	wal, err := Open(walPath, SyncEveryRecord)
	if err != nil {
		t.Fatal(err)
	}
	engine := matchingengine.New()
	processor := &Processor{Engine: engine, WAL: wal}

	mustProcess(t, processor, orderCommand(1, "ask", matchingengine.Sell, 100, 10))
	mustProcess(t, processor, orderCommand(2, "buy", matchingengine.Buy, 100, 4))
	if err = WriteSnapshot(snapshotPath, engine.Snapshot()); err != nil {
		t.Fatal(err)
	}
	mustProcess(t, processor, orderCommand(3, "bid", matchingengine.Buy, 90, 8))
	mustProcess(t, processor, matchingengine.Command{
		Sequence:    4,
		Type:        matchingengine.CommandCancelOrder,
		CancelOrder: &matchingengine.CancelOrder{OrderID: "bid"},
	})
	wantHash, err := engine.StateHash()
	if err != nil {
		t.Fatal(err)
	}
	if err = wal.Close(); err != nil {
		t.Fatal(err)
	}

	recovered, events, err := Recover(snapshotPath, walPath, false)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.LastSequence() != 4 {
		t.Fatalf("last sequence = %d, want 4", recovered.LastSequence())
	}
	if len(events) == 0 {
		t.Fatal("expected events regenerated from WAL suffix")
	}
	gotHash, err := recovered.StateHash()
	if err != nil {
		t.Fatal(err)
	}
	if gotHash != wantHash {
		t.Fatalf("recovered state mismatch: got %s want %s", gotHash, wantHash)
	}
}

func TestTornTailRequiresExplicitRepair(t *testing.T) {
	dir := t.TempDir()
	walPath := filepath.Join(dir, "orders.wal")
	snapshotPath := filepath.Join(dir, "missing.snapshot")

	wal, err := Open(walPath, NoSync)
	if err != nil {
		t.Fatal(err)
	}
	if err = wal.Append(orderCommand(1, "bid", matchingengine.Buy, 90, 2)); err != nil {
		t.Fatal(err)
	}
	if err = wal.Close(); err != nil {
		t.Fatal(err)
	}

	file, err := os.OpenFile(walPath, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write([]byte{0, 0}); err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}

	_, _, err = Recover(snapshotPath, walPath, false)
	var torn *TornTailError
	if !errors.As(err, &torn) {
		t.Fatalf("got %v, want TornTailError", err)
	}

	recovered, _, err := Recover(snapshotPath, walPath, true)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.LastSequence() != 1 {
		t.Fatalf("last sequence = %d, want 1", recovered.LastSequence())
	}
	_, _, err = Scan(walPath)
	if err != nil {
		t.Fatalf("repaired WAL still invalid: %v", err)
	}
}

func TestChecksumCorruptionIsNotSilentlyTruncated(t *testing.T) {
	dir := t.TempDir()
	walPath := filepath.Join(dir, "orders.wal")

	wal, err := Open(walPath, NoSync)
	if err != nil {
		t.Fatal(err)
	}
	if err = wal.Append(orderCommand(1, "bid", matchingengine.Buy, 90, 2)); err != nil {
		t.Fatal(err)
	}
	if err = wal.Close(); err != nil {
		t.Fatal(err)
	}

	payload, err := os.ReadFile(walPath)
	if err != nil {
		t.Fatal(err)
	}
	payload[8] ^= 0x01
	if err = os.WriteFile(walPath, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err = Scan(walPath)
	var checksum *ChecksumError
	if !errors.As(err, &checksum) {
		t.Fatalf("got %v, want ChecksumError", err)
	}
}

func TestInvalidCommandIsNotAppended(t *testing.T) {
	dir := t.TempDir()
	walPath := filepath.Join(dir, "orders.wal")
	wal, err := Open(walPath, NoSync)
	if err != nil {
		t.Fatal(err)
	}
	processor := &Processor{Engine: matchingengine.New(), WAL: wal}
	_, err = processor.Apply(orderCommand(2, "gap", matchingengine.Buy, 90, 2))
	if err == nil {
		t.Fatal("expected sequence validation error")
	}
	if err = wal.Close(); err != nil {
		t.Fatal(err)
	}
	stat, err := os.Stat(walPath)
	if err != nil {
		t.Fatal(err)
	}
	if stat.Size() != 0 {
		t.Fatalf("invalid command was appended: size=%d", stat.Size())
	}
}

func orderCommand(seq uint64, id string, side matchingengine.Side, price, qty int64) matchingengine.Command {
	return matchingengine.Command{
		Sequence: seq,
		Type:     matchingengine.CommandNewOrder,
		NewOrder: &matchingengine.NewOrder{
			OrderID:       id,
			ClientOrderID: "client-" + id,
			AccountID:     "account-" + id,
			Side:          side,
			Price:         price,
			Quantity:      qty,
			TimeInForce:   matchingengine.GTC,
			STP:           matchingengine.STPCancelTaker,
		},
	}
}

func mustProcess(t *testing.T, processor *Processor, command matchingengine.Command) {
	t.Helper()
	if _, err := processor.Apply(command); err != nil {
		t.Fatalf("process sequence %d: %v", command.Sequence, err)
	}
}
