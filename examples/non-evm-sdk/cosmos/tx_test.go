package cosmostx

import (
	"bytes"
	"context"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

func TestBuildSignedBankSend(t *testing.T) {
	recipientKey := secp256k1.GenPrivKeyFromSecret([]byte("recipient"))
	recipient, err := sdk.Bech32ifyAddressBytes("cosmos", recipientKey.PubKey().Address())
	if err != nil {
		t.Fatal(err)
	}
	input := BuildInput{
		Secret:        []byte("sender"),
		AddressPrefix: "cosmos",
		Recipient:     recipient,
		Denom:         "uatom",
		Amount:        1234,
		FeeAmount:     25,
		GasLimit:      200_000,
		ChainID:       "cosmoshub-4",
		AccountNumber: 42,
		Sequence:      7,
		Memo:          "invoice-123",
	}
	first, err := BuildSignedBankSend(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildSignedBankSend(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.TxBytes, second.TxBytes) || !bytes.Equal(first.Signature, second.Signature) {
		t.Fatal("same SignDoc inputs must produce deterministic signed bytes")
	}
	if len(first.TxBytes) == 0 || len(first.SignBytes) == 0 || len(first.Signature) != 64 {
		t.Fatalf("unexpected encoded sizes: tx=%d sign=%d signature=%d", len(first.TxBytes), len(first.SignBytes), len(first.Signature))
	}

	messages := first.Tx.GetMsgs()
	if len(messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(messages))
	}
	send, ok := messages[0].(*banktypes.MsgSend)
	if !ok {
		t.Fatalf("message type = %T", messages[0])
	}
	if send.FromAddress != first.From || send.ToAddress != recipient || send.Amount[0].Amount.Uint64() != input.Amount {
		t.Fatalf("MsgSend mismatch: %+v", send)
	}
	feeTx, ok := first.Tx.(sdk.FeeTx)
	if !ok {
		t.Fatalf("tx type %T does not implement sdk.FeeTx", first.Tx)
	}
	if feeTx.GetGas() != input.GasLimit || feeTx.GetFee()[0].Amount.Uint64() != input.FeeAmount {
		t.Fatalf("fee mismatch: tx=%T fee=%v gas=%d", first.Tx, feeTx.GetFee(), feeTx.GetGas())
	}
}

func TestChainIDAndSequenceChangeSignBytes(t *testing.T) {
	recipientKey := secp256k1.GenPrivKeyFromSecret([]byte("recipient"))
	recipient, err := sdk.Bech32ifyAddressBytes("cosmos", recipientKey.PubKey().Address())
	if err != nil {
		t.Fatal(err)
	}
	base := BuildInput{
		Secret:        []byte("sender"),
		AddressPrefix: "cosmos",
		Recipient:     recipient,
		Denom:         "uatom",
		Amount:        1,
		FeeAmount:     1,
		GasLimit:      100_000,
		ChainID:       "chain-a",
		AccountNumber: 1,
		Sequence:      1,
	}
	first, err := BuildSignedBankSend(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	base.ChainID = "chain-b"
	second, err := BuildSignedBankSend(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first.SignBytes, second.SignBytes) {
		t.Fatal("chain ID must be bound into SignDoc")
	}
	base.ChainID = "chain-a"
	base.Sequence = 2
	third, err := BuildSignedBankSend(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first.SignBytes, third.SignBytes) {
		t.Fatal("sequence must be bound into AuthInfo/SignDoc")
	}
}
