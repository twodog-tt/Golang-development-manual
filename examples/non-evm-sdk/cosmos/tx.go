// Package cosmostx demonstrates an offline SIGN_MODE_DIRECT bank transfer
// built with the official Cosmos SDK TxBuilder and signing APIs.
package cosmostx

import (
	"context"
	"errors"
	"fmt"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

type BuildInput struct {
	Secret        []byte
	AddressPrefix string
	Recipient     string
	Denom         string
	Amount        uint64
	FeeAmount     uint64
	GasLimit      uint64
	ChainID       string
	AccountNumber uint64
	Sequence      uint64
	Memo          string
}

type SignedTx struct {
	From      string
	TxBytes   []byte
	SignBytes []byte
	Signature []byte
	Tx        sdk.Tx
}

func BuildSignedBankSend(ctx context.Context, input BuildInput) (SignedTx, error) {
	if len(input.Secret) == 0 || input.AddressPrefix == "" || input.Recipient == "" ||
		input.Denom == "" || input.ChainID == "" {
		return SignedTx{}, errors.New("secret, prefix, recipient, denom, and chain ID are required")
	}
	if input.Amount == 0 || input.FeeAmount == 0 || input.GasLimit == 0 {
		return SignedTx{}, errors.New("amount, fee, and gas limit must be positive")
	}
	if _, err := sdk.GetFromBech32(input.Recipient, input.AddressPrefix); err != nil {
		return SignedTx{}, fmt.Errorf("recipient: %w", err)
	}

	privateKey := secp256k1.GenPrivKeyFromSecret(input.Secret)
	from, err := sdk.Bech32ifyAddressBytes(input.AddressPrefix, privateKey.PubKey().Address())
	if err != nil {
		return SignedTx{}, err
	}

	registry := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(registry)
	banktypes.RegisterInterfaces(registry)
	protoCodec := codec.NewProtoCodec(registry)
	txConfig := authtx.NewTxConfig(protoCodec, authtx.DefaultSignModes)
	builder := txConfig.NewTxBuilder()

	message := &banktypes.MsgSend{
		FromAddress: from,
		ToAddress:   input.Recipient,
		Amount: sdk.NewCoins(sdk.NewCoin(
			input.Denom,
			sdkmath.NewIntFromUint64(input.Amount),
		)),
	}
	if err = builder.SetMsgs(message); err != nil {
		return SignedTx{}, err
	}
	builder.SetMemo(input.Memo)
	builder.SetGasLimit(input.GasLimit)
	builder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(
		input.Denom,
		sdkmath.NewIntFromUint64(input.FeeAmount),
	)))

	signMode, err := authsigning.APISignModeToInternal(txConfig.SignModeHandler().DefaultMode())
	if err != nil {
		return SignedTx{}, err
	}
	if signMode != signing.SignMode_SIGN_MODE_DIRECT {
		return SignedTx{}, fmt.Errorf("unexpected default sign mode %s; example requires SIGN_MODE_DIRECT", signMode)
	}
	emptySignature := signing.SignatureV2{
		PubKey: privateKey.PubKey(),
		Data: &signing.SingleSignatureData{
			SignMode:  signMode,
			Signature: nil,
		},
		Sequence: input.Sequence,
	}
	if err = builder.SetSignatures(emptySignature); err != nil {
		return SignedTx{}, err
	}
	signerData := authsigning.SignerData{
		Address:       from,
		ChainID:       input.ChainID,
		AccountNumber: input.AccountNumber,
		Sequence:      input.Sequence,
		PubKey:        privateKey.PubKey(),
	}
	signBytes, err := authsigning.GetSignBytesAdapter(
		ctx,
		txConfig.SignModeHandler(),
		signMode,
		signerData,
		builder.GetTx(),
	)
	if err != nil {
		return SignedTx{}, err
	}
	signature, err := tx.SignWithPrivKey(
		ctx,
		signMode,
		signerData,
		builder,
		privateKey,
		txConfig,
		input.Sequence,
	)
	if err != nil {
		return SignedTx{}, err
	}
	single, ok := signature.Data.(*signing.SingleSignatureData)
	if !ok {
		return SignedTx{}, errors.New("expected a single signature")
	}
	if !privateKey.PubKey().VerifySignature(signBytes, single.Signature) {
		return SignedTx{}, errors.New("generated signature does not verify")
	}
	if err = builder.SetSignatures(signature); err != nil {
		return SignedTx{}, err
	}
	txBytes, err := txConfig.TxEncoder()(builder.GetTx())
	if err != nil {
		return SignedTx{}, err
	}
	return SignedTx{
		From:      from,
		TxBytes:   txBytes,
		SignBytes: signBytes,
		Signature: append([]byte(nil), single.Signature...),
		Tx:        builder.GetTx(),
	}, nil
}
