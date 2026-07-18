package chainmerge

import (
	"crypto/sha256"
	"errors"
	"reflect"
	"testing"
)

func TestBackfillRealtimeOverlapAndCanonicalAdoption(t *testing.T) {
	merger := newMerger(t)
	b101 := block(101, "b101", "anchor")
	b102 := block(102, "b102", "b101")
	b103 := block(103, "b103", "b102")

	observe(t, merger, SourceBackfill, b101, b102)
	observe(t, merger, SourceBackfill, b102)
	observe(t, merger, SourceRealtime, b102, b103)
	change, err := merger.Adopt("b103")
	if err != nil {
		t.Fatal(err)
	}
	if got := hashes(change.Applied); !reflect.DeepEqual(got, []string{"b101", "b102", "b103"}) {
		t.Fatalf("applied=%v", got)
	}
	if err = merger.VerifyOverlap(102, 102, SourceBackfill, SourceRealtime); err != nil {
		t.Fatal(err)
	}
	if err = merger.VerifyOverlap(101, 103, SourceBackfill, SourceRealtime); !errors.Is(err, ErrIncompleteOverlap) {
		t.Fatalf("full overlap: got %v, want ErrIncompleteOverlap", err)
	}
	if got := merger.Sources("b102"); !reflect.DeepEqual(got, []Source{SourceBackfill, SourceRealtime}) {
		t.Fatalf("sources=%v", got)
	}
}

func TestAdoptReorgRollsBackDescendingAndReplaysAscending(t *testing.T) {
	merger := newMerger(t)
	a101 := block(101, "a101", "anchor")
	a102 := block(102, "a102", "a101")
	a103 := block(103, "a103", "a102")
	observe(t, merger, SourceRealtime, a101, a102, a103)
	if _, err := merger.Adopt("a103"); err != nil {
		t.Fatal(err)
	}
	if err := merger.Finalize(101, "a101"); err != nil {
		t.Fatal(err)
	}

	b102 := block(102, "b102", "a101")
	b103 := block(103, "b103", "b102")
	observe(t, merger, SourceBackfill, b102, b103)
	change, err := merger.Adopt("b103")
	if err != nil {
		t.Fatal(err)
	}
	if got := hashes(change.Orphaned); !reflect.DeepEqual(got, []string{"a103", "a102"}) {
		t.Fatalf("orphaned=%v", got)
	}
	if got := hashes(change.Applied); !reflect.DeepEqual(got, []string{"b102", "b103"}) {
		t.Fatalf("applied=%v", got)
	}
	if canonical, ok := merger.Canonical(102); !ok || canonical.Hash != "b102" {
		t.Fatalf("canonical=%+v ok=%v", canonical, ok)
	}
	if got := merger.Sources("a102"); !reflect.DeepEqual(got, []Source{SourceRealtime}) {
		t.Fatalf("orphan evidence was lost: %v", got)
	}
}

func TestFinalizedHistoryCannotBeRewritten(t *testing.T) {
	merger := newMerger(t)
	a101 := block(101, "a101", "anchor")
	a102 := block(102, "a102", "a101")
	a103 := block(103, "a103", "a102")
	observe(t, merger, SourceRealtime, a101, a102, a103)
	if _, err := merger.Adopt("a103"); err != nil {
		t.Fatal(err)
	}
	if err := merger.Finalize(102, "a102"); err != nil {
		t.Fatal(err)
	}

	b102 := block(102, "b102", "a101")
	b103 := block(103, "b103", "b102")
	observe(t, merger, SourceBackfill, b102, b103)
	if _, err := merger.Adopt("b103"); !errors.Is(err, ErrFinalizedConflict) {
		t.Fatalf("finalized reorg: got %v, want ErrFinalizedConflict", err)
	}
	if merger.Head().Hash != "a103" {
		t.Fatal("failed adoption mutated canonical head")
	}
}

func TestGapAndEvidenceConflictFailClosed(t *testing.T) {
	merger := newMerger(t)
	b102 := block(102, "b102", "missing-101")
	observe(t, merger, SourceRealtime, b102)
	if _, err := merger.Adopt("b102"); !errors.Is(err, ErrGap) {
		t.Fatalf("gap: got %v, want ErrGap", err)
	}

	conflict := b102
	conflict.ParentHash = "other-parent"
	if _, err := merger.Observe(SourceBackfill, conflict); !errors.Is(err, ErrEvidenceConflict) {
		t.Fatalf("conflicting evidence: got %v, want ErrEvidenceConflict", err)
	}
}

func TestInvalidEvidenceIsRejected(t *testing.T) {
	merger := newMerger(t)
	valid := block(101, "b101", "anchor")
	if _, err := merger.Observe(Source("unknown"), valid); !errors.Is(err, ErrInvalidBlock) {
		t.Fatalf("unknown source: got %v, want ErrInvalidBlock", err)
	}
	valid.PayloadHash = [32]byte{}
	if _, err := merger.Observe(SourceRealtime, valid); !errors.Is(err, ErrInvalidBlock) {
		t.Fatalf("empty payload digest: got %v, want ErrInvalidBlock", err)
	}
}

func newMerger(t *testing.T) *Merger {
	t.Helper()
	merger, err := New(block(100, "anchor", "parent-99"))
	if err != nil {
		t.Fatal(err)
	}
	return merger
}

func observe(t *testing.T, merger *Merger, source Source, blocks ...Block) {
	t.Helper()
	for _, item := range blocks {
		if _, err := merger.Observe(source, item); err != nil {
			t.Fatal(err)
		}
	}
}

func block(height uint64, hash, parent string) Block {
	return Block{
		Height:      height,
		Hash:        hash,
		ParentHash:  parent,
		PayloadHash: sha256.Sum256([]byte("payload:" + hash)),
	}
}

func hashes(blocks []Block) []string {
	result := make([]string, 0, len(blocks))
	for _, item := range blocks {
		result = append(result, item.Hash)
	}
	return result
}
