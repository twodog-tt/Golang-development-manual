// Package solanatx demonstrates offline Solana transaction construction and
// explicit confirmation-status handling with the community solana-go SDK.
package solanatx

import (
	"crypto/ed25519"
	"errors"
	"fmt"

	solana "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
)

type SignedTransfer struct {
	Transaction *solana.Transaction
	Base64      string
	Signature   solana.Signature
}

func BuildSignedTransfer(
	seed [ed25519.SeedSize]byte,
	recipient solana.PublicKey,
	lamports uint64,
	recentBlockhash solana.Hash,
) (SignedTransfer, error) {
	if recipient.IsZero() {
		return SignedTransfer{}, errors.New("recipient is required")
	}
	if lamports == 0 {
		return SignedTransfer{}, errors.New("lamports must be positive")
	}

	privateKey := solana.PrivateKey(ed25519.NewKeyFromSeed(seed[:]))
	sender := privateKey.PublicKey()
	instruction := system.NewTransferInstruction(
		lamports,
		sender,
		recipient,
	).Build()
	transaction, err := solana.NewTransaction(
		[]solana.Instruction{instruction},
		recentBlockhash,
		solana.TransactionPayer(sender),
	)
	if err != nil {
		return SignedTransfer{}, err
	}
	signatures, err := transaction.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key == sender {
			return &privateKey
		}
		return nil
	})
	if err != nil {
		return SignedTransfer{}, err
	}
	if len(signatures) != 1 {
		return SignedTransfer{}, fmt.Errorf("got %d signatures, want 1", len(signatures))
	}
	if err = transaction.VerifySignatures(); err != nil {
		return SignedTransfer{}, fmt.Errorf("verify signatures: %w", err)
	}
	encoded, err := transaction.ToBase64()
	if err != nil {
		return SignedTransfer{}, err
	}
	return SignedTransfer{
		Transaction: transaction,
		Base64:      encoded,
		Signature:   signatures[0],
	}, nil
}

type Outcome string

const (
	OutcomeUnknown   Outcome = "UNKNOWN"
	OutcomePending   Outcome = "PENDING"
	OutcomeSucceeded Outcome = "SUCCEEDED"
	OutcomeFailed    Outcome = "FAILED"
	// OutcomeExpired means the signature was not observed and the transaction's
	// recent blockhash is no longer valid at the explicitly requested
	// blockhash commitment. It is deliberately distinct from confirmation
	// commitment not yet being reached.
	OutcomeExpired Outcome = "EXPIRED"
)

// EvaluateStatus keeps "RPC accepted a signature" separate from execution and
// the application's required commitment level.
func EvaluateStatus(status *rpc.SignatureStatusesResult, required rpc.ConfirmationStatusType) Outcome {
	if status == nil {
		return OutcomeUnknown
	}
	// Success and failure become terminal symmetrically. A processed failure
	// can still disappear with its fork, so it remains pending until the
	// application's required commitment has been reached.
	if commitmentRank(status.ConfirmationStatus) < commitmentRank(required) {
		return OutcomePending
	}
	if status.Err != nil {
		return OutcomeFailed
	}
	return OutcomeSucceeded
}

func commitmentRank(status rpc.ConfirmationStatusType) int {
	switch status {
	case rpc.ConfirmationStatusProcessed:
		return 1
	case rpc.ConfirmationStatusConfirmed:
		return 2
	case rpc.ConfirmationStatusFinalized:
		return 3
	default:
		return 0
	}
}
