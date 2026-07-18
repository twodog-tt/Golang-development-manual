package aptostx

import (
	"bytes"
	"testing"

	aptos "github.com/aptos-labs/aptos-go-sdk"
)

func TestBuildSignedTransfer(t *testing.T) {
	var seed [32]byte
	copy(seed[:], bytes.Repeat([]byte{0x31}, len(seed)))
	input := TransferInput{
		PrivateKeySeed:    seed,
		Recipient:         aptos.AccountTwo,
		Amount:            1000,
		Sequence:          7,
		MaxGasAmount:      2000,
		GasUnitPrice:      100,
		ExpirationSeconds: 2_000_000_000,
		ChainID:           1,
	}
	first, err := BuildSignedTransfer(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildSignedTransfer(input)
	if err != nil {
		t.Fatal(err)
	}
	if first.Hash != second.Hash || !bytes.Equal(first.BCS, second.BCS) {
		t.Fatal("same raw transaction must produce deterministic BCS and signature")
	}
	if err = first.Transaction.Verify(); err != nil {
		t.Fatal(err)
	}
	raw := first.Transaction.Transaction
	if raw.SequenceNumber != input.Sequence || raw.ChainId != input.ChainID || raw.Sender != first.Sender {
		t.Fatalf("raw transaction mismatch: %+v", raw)
	}
	if len(first.BCS) == 0 || first.Hash == "" {
		t.Fatal("missing BCS or transaction hash")
	}
}
