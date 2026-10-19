package lab

import (
	"bytes"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestDecoderRejectsPoison(t *testing.T) {
	for _, l := range []types.Log{
		{Topics: []common.Hash{transfer}, Data: make([]byte, 32)},
		{Topics: []common.Hash{transfer, {}, {}}, Data: make([]byte, 31)},
		{Topics: []common.Hash{swap, {}, {}}, Data: make([]byte, 32)},
		{Topics: []common.Hash{transfer, common.BytesToHash(bytes.Repeat([]byte{1}, 32)), {}}, Data: make([]byte, 32)},
		{Topics: []common.Hash{transfer, {}, {}}, Data: make([]byte, 32), Removed: true},
	} {
		if _, err := Decode(l); err == nil {
			t.Fatal("accepted malformed ABI log")
		}
	}
}

func TestPaidReferenceIsBytes32NotAddress(t *testing.T) {
	l := types.Log{Topics: []common.Hash{paid, common.BytesToHash(bytes.Repeat([]byte{255}, 32)), {}}, Data: make([]byte, 32)}
	if kind, err := Decode(l); err != nil || kind != "paid" {
		t.Fatalf("valid reference rejected: %s %v", kind, err)
	}
	l.Topics[2] = common.BytesToHash(bytes.Repeat([]byte{255}, 32))
	if _, err := Decode(l); err == nil {
		t.Fatal("accepted poison payer padding")
	}
}
func TestAmountBoundaries(t *testing.T) {
	for _, s := range []string{"0", "01", "-1", "1.1", "1e6", " 1", "1000000000000000000000000000000"} {
		if _, err := Amount(s); err == nil {
			t.Fatal("accepted", s)
		}
	}
	n, err := Amount("9007199254740993")
	if err != nil || n.String() != "9007199254740993" {
		t.Fatal("precision loss")
	}
}
