package marketdatarecovery

import (
	"errors"
	"reflect"
	"testing"
)

func TestSnapshotBridgeAndLiveUpdates(t *testing.T) {
	book := New()
	book.BeginResync()

	mustDelta(t, book, Delta{
		Epoch:         "engine-7",
		FirstSequence: 101,
		LastSequence:  103,
		Bids:          []Level{{Price: 99, Quantity: 8}},
		Asks:          []Level{{Price: 101, Quantity: 0}, {Price: 102, Quantity: 4}},
	})
	mustDelta(t, book, Delta{
		Epoch:         "engine-7",
		FirstSequence: 104,
		LastSequence:  104,
		Bids:          []Level{{Price: 100, Quantity: 6}},
	})

	err := book.InstallSnapshot(Snapshot{
		Epoch:    "engine-7",
		Sequence: 102,
		Bids:     []Level{{Price: 100, Quantity: 5}, {Price: 99, Quantity: 2}},
		Asks:     []Level{{Price: 101, Quantity: 3}},
	})
	if err != nil {
		t.Fatal(err)
	}

	want := View{
		State:    StateLive,
		Epoch:    "engine-7",
		Sequence: 104,
		Bids:     []Level{{Price: 100, Quantity: 6}, {Price: 99, Quantity: 8}},
		Asks:     []Level{{Price: 102, Quantity: 4}},
	}
	if got := book.View(); !reflect.DeepEqual(got, want) {
		t.Fatalf("view mismatch:\n got=%+v\nwant=%+v", got, want)
	}

	// A fully stale duplicate can be ignored.
	mustDelta(t, book, Delta{
		Epoch:         "engine-7",
		FirstSequence: 103,
		LastSequence:  104,
		Bids:          []Level{{Price: 1, Quantity: 1}},
	})
	if got := book.View(); !reflect.DeepEqual(got, want) {
		t.Fatalf("stale delta mutated the book: %+v", got)
	}

	// A range overlapping the next expected sequence is applicable because
	// each level carries its absolute quantity as of LastSequence.
	mustDelta(t, book, Delta{
		Epoch:         "engine-7",
		FirstSequence: 104,
		LastSequence:  106,
		Asks:          []Level{{Price: 102, Quantity: 7}},
	})
	if got := book.View().Sequence; got != 106 {
		t.Fatalf("sequence=%d, want 106", got)
	}
}

func TestBufferedGapRequiresFreshSnapshot(t *testing.T) {
	book := New()
	book.BeginResync()
	mustDelta(t, book, Delta{
		Epoch:         "engine-1",
		FirstSequence: 12,
		LastSequence:  12,
		Bids:          []Level{{Price: 100, Quantity: 1}},
	})

	err := book.InstallSnapshot(Snapshot{
		Epoch:    "engine-1",
		Sequence: 10,
		Bids:     []Level{{Price: 99, Quantity: 1}},
	})
	if !errors.Is(err, ErrGap) {
		t.Fatalf("err=%v, want ErrGap", err)
	}
	if book.State() != StateResyncRequired {
		t.Fatalf("state=%s, want %s", book.State(), StateResyncRequired)
	}
	if view := book.View(); len(view.Bids) != 0 || view.Sequence != 0 {
		t.Fatalf("invalid partial state survived gap: %+v", view)
	}
}

func TestLiveGapAndEpochChangeFailClosed(t *testing.T) {
	tests := []struct {
		name  string
		delta Delta
		want  error
	}{
		{
			name: "gap",
			delta: Delta{
				Epoch:         "engine-1",
				FirstSequence: 12,
				LastSequence:  12,
				Bids:          []Level{{Price: 100, Quantity: 2}},
			},
			want: ErrGap,
		},
		{
			name: "epoch",
			delta: Delta{
				Epoch:         "engine-2",
				FirstSequence: 11,
				LastSequence:  11,
				Bids:          []Level{{Price: 100, Quantity: 2}},
			},
			want: ErrEpochChanged,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			book := liveBook(t)
			err := book.OnDelta(tt.delta)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err=%v, want %v", err, tt.want)
			}
			if book.State() != StateResyncRequired {
				t.Fatalf("state=%s, want resync", book.State())
			}
		})
	}
}

func TestInvalidDeltaFailsClosed(t *testing.T) {
	book := liveBook(t)
	err := book.OnDelta(Delta{
		Epoch:         "engine-1",
		FirstSequence: 11,
		LastSequence:  11,
		Bids: []Level{
			{Price: 100, Quantity: 2},
			{Price: 100, Quantity: 3},
		},
	})
	if err == nil {
		t.Fatal("expected duplicate price validation error")
	}
	if book.State() != StateResyncRequired {
		t.Fatalf("state=%s, want %s", book.State(), StateResyncRequired)
	}
	if got := book.View(); got.Sequence != 0 || len(got.Bids) != 0 || len(got.Asks) != 0 {
		t.Fatalf("invalid delta left a publishable book: %+v", got)
	}
}

func liveBook(t *testing.T) *Book {
	t.Helper()
	book := New()
	book.BeginResync()
	if err := book.InstallSnapshot(Snapshot{
		Epoch:    "engine-1",
		Sequence: 10,
		Bids:     []Level{{Price: 100, Quantity: 1}},
		Asks:     []Level{{Price: 101, Quantity: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	return book
}

func mustDelta(t *testing.T, book *Book, delta Delta) {
	t.Helper()
	if err := book.OnDelta(delta); err != nil {
		t.Fatal(err)
	}
}
