package solanatx

import (
	"bytes"
	"testing"

	solana "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func TestBuildSignedTransferIsDeterministicAndVerifiable(t *testing.T) {
	var seed [32]byte
	copy(seed[:], bytes.Repeat([]byte{0x11}, len(seed)))
	recipientKey, err := solana.NewRandomPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	blockhash := solana.HashFromBytes(bytes.Repeat([]byte{0x22}, 32))

	first, err := BuildSignedTransfer(seed, recipientKey.PublicKey(), 12345, blockhash)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildSignedTransfer(seed, recipientKey.PublicKey(), 12345, blockhash)
	if err != nil {
		t.Fatal(err)
	}
	if first.Base64 != second.Base64 || first.Signature != second.Signature {
		t.Fatal("same intent and blockhash must produce the same signed bytes")
	}
	if err = first.Transaction.VerifySignatures(); err != nil {
		t.Fatal(err)
	}
	if first.Transaction.Message.RecentBlockhash != blockhash {
		t.Fatalf("blockhash = %s, want %s", first.Transaction.Message.RecentBlockhash, blockhash)
	}
	if len(first.Transaction.Message.Instructions) != 1 {
		t.Fatalf("instructions = %d, want 1", len(first.Transaction.Message.Instructions))
	}
}

func TestEvaluateStatus(t *testing.T) {
	if got := EvaluateStatus(nil, rpc.ConfirmationStatusConfirmed); got != OutcomeUnknown {
		t.Fatalf("nil status = %s", got)
	}
	status := &rpc.SignatureStatusesResult{ConfirmationStatus: rpc.ConfirmationStatusProcessed}
	if got := EvaluateStatus(status, rpc.ConfirmationStatusConfirmed); got != OutcomePending {
		t.Fatalf("processed = %s", got)
	}
	status.Err = map[string]any{"InstructionError": []any{0, "Custom"}}
	if got := EvaluateStatus(status, rpc.ConfirmationStatusConfirmed); got != OutcomePending {
		t.Fatalf("processed error = %s, want pending", got)
	}
	status.ConfirmationStatus = rpc.ConfirmationStatusConfirmed
	if got := EvaluateStatus(status, rpc.ConfirmationStatusConfirmed); got != OutcomeFailed {
		t.Fatalf("confirmed error = %s, want failed", got)
	}
	status.Err = nil
	status.ConfirmationStatus = rpc.ConfirmationStatusFinalized
	if got := EvaluateStatus(status, rpc.ConfirmationStatusConfirmed); got != OutcomeSucceeded {
		t.Fatalf("finalized = %s", got)
	}
}
