package coinselect

import (
	"errors"
	"testing"
)

func TestLargestFirstCreatesChange(t *testing.T) {
	selection, err := LargestFirst(Request{
		UTXOs: []UTXO{
			{ID: "small", Value: 3_000, InputVBytes: 68},
			{ID: "large", Value: 10_000, InputVBytes: 68},
			{ID: "medium", Value: 7_000, InputVBytes: 68},
		},
		Target:        12_000,
		FeeRate:       2,
		BaseVBytes:    10,
		ChangeVBytes:  31,
		DustThreshold: 546,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(selection.UTXOs) != 2 {
		t.Fatalf("selected %d inputs, want 2", len(selection.UTXOs))
	}
	if selection.Total != 17_000 || selection.Fee != 354 || selection.Change != 4_646 {
		t.Fatalf("selection = %+v", selection)
	}
	if selection.Total != 12_000+selection.Fee+selection.Change {
		t.Fatal("value conservation failed")
	}
}

func TestLargestFirstAddsSmallRemainderToFee(t *testing.T) {
	selection, err := LargestFirst(Request{
		UTXOs:         []UTXO{{ID: "only", Value: 10_000, InputVBytes: 68}},
		Target:        9_800,
		FeeRate:       2,
		BaseVBytes:    10,
		ChangeVBytes:  31,
		DustThreshold: 546,
	})
	if err != nil {
		t.Fatal(err)
	}
	if selection.Change != 0 || selection.Fee != 200 {
		t.Fatalf("selection = %+v", selection)
	}
}

func TestLargestFirstInsufficientFunds(t *testing.T) {
	_, err := LargestFirst(Request{
		UTXOs:         []UTXO{{ID: "only", Value: 1_000, InputVBytes: 68}},
		Target:        1_000,
		FeeRate:       2,
		BaseVBytes:    10,
		ChangeVBytes:  31,
		DustThreshold: 546,
	})
	if !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("error = %v, want insufficient funds", err)
	}
}
