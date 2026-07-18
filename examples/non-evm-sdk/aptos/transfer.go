// Package aptostx demonstrates fully offline Aptos BCS transaction
// construction, domain-separated signing, verification, and hashing.
package aptostx

import (
	"errors"

	aptos "github.com/aptos-labs/aptos-go-sdk"
	"github.com/aptos-labs/aptos-go-sdk/bcs"
	aptoscrypto "github.com/aptos-labs/aptos-go-sdk/crypto"
)

type TransferInput struct {
	PrivateKeySeed    [32]byte
	Recipient         aptos.AccountAddress
	Amount            uint64
	Sequence          uint64
	MaxGasAmount      uint64
	GasUnitPrice      uint64
	ExpirationSeconds uint64
	ChainID           uint8
}

type SignedTransfer struct {
	Sender      aptos.AccountAddress
	Transaction *aptos.SignedTransaction
	BCS         []byte
	Hash        string
}

func BuildSignedTransfer(input TransferInput) (SignedTransfer, error) {
	if input.Amount == 0 || input.MaxGasAmount == 0 || input.ExpirationSeconds == 0 || input.ChainID == 0 {
		return SignedTransfer{}, errors.New("amount, max gas, expiration, and chain ID must be positive")
	}
	privateKey := &aptoscrypto.Ed25519PrivateKey{}
	if err := privateKey.FromBytes(input.PrivateKeySeed[:]); err != nil {
		return SignedTransfer{}, err
	}
	sender, err := aptos.NewAccountFromSigner(privateKey)
	if err != nil {
		return SignedTransfer{}, err
	}

	recipientBytes, err := bcs.Serialize(&input.Recipient)
	if err != nil {
		return SignedTransfer{}, err
	}
	amountBytes, err := bcs.SerializeU64(input.Amount)
	if err != nil {
		return SignedTransfer{}, err
	}
	raw := &aptos.RawTransaction{
		Sender:         sender.AccountAddress(),
		SequenceNumber: input.Sequence,
		Payload: aptos.TransactionPayload{Payload: &aptos.EntryFunction{
			Module: aptos.ModuleId{
				Address: aptos.AccountOne,
				Name:    "aptos_account",
			},
			Function: "transfer",
			ArgTypes: []aptos.TypeTag{},
			Args:     [][]byte{recipientBytes, amountBytes},
		}},
		MaxGasAmount:               input.MaxGasAmount,
		GasUnitPrice:               input.GasUnitPrice,
		ExpirationTimestampSeconds: input.ExpirationSeconds,
		ChainId:                    input.ChainID,
	}
	signed, err := raw.SignedTransaction(sender)
	if err != nil {
		return SignedTransfer{}, err
	}
	if err = signed.Verify(); err != nil {
		return SignedTransfer{}, err
	}
	encoded, err := bcs.Serialize(signed)
	if err != nil {
		return SignedTransfer{}, err
	}
	hash, err := signed.Hash()
	if err != nil {
		return SignedTransfer{}, err
	}
	return SignedTransfer{
		Sender:      sender.AccountAddress(),
		Transaction: signed,
		BCS:         encoded,
		Hash:        hash,
	}, nil
}
