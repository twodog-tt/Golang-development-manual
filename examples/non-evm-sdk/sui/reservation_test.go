package suiadapter

import (
	"reflect"
	"testing"
)

func TestCapabilityAwareValidation(t *testing.T) {
	capabilities := Capabilities{
		ProtocolVersion:       125,
		AddressBalances:       true,
		GaslessTransferAssets: map[string]bool{"USDC": true},
	}
	addressBalance := TransferIntent{
		Sender:     "0xsender",
		Recipient:  "0xrecipient",
		AssetType:  "USDC",
		Amount:     1_000_000,
		Funding:    FundingAddressBalance,
		GasFunding: Gasless,
	}
	if err := addressBalance.Validate(capabilities); err != nil {
		t.Fatal(err)
	}
	if got, want := addressBalance.ReservationKeys(), []string{"balance:0xsender:USDC"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("keys=%v want=%v", got, want)
	}

	legacy := TransferIntent{
		Sender:     "0xsender",
		Recipient:  "0xrecipient",
		AssetType:  "0x2::sui::SUI",
		Amount:     10,
		Funding:    FundingCoinObjects,
		GasFunding: GasCoinObjects,
		CoinInputs: []ObjectRef{
			{ID: "coin", Version: 7, Digest: "coin-digest"},
		},
		GasInputs: []ObjectRef{
			{ID: "gas", Version: 9, Digest: "gas-digest"},
		},
	}
	if err := legacy.Validate(capabilities); err != nil {
		t.Fatal(err)
	}
}

func TestRejectsStaleOrConflictingObjectRefs(t *testing.T) {
	capabilities := Capabilities{ProtocolVersion: 124}
	intent := TransferIntent{
		Sender:     "0xsender",
		Recipient:  "0xrecipient",
		AssetType:  "0x2::sui::SUI",
		Amount:     10,
		Funding:    FundingCoinObjects,
		GasFunding: GasCoinObjects,
		CoinInputs: []ObjectRef{
			{ID: "same", Version: 7, Digest: "old"},
		},
		GasInputs: []ObjectRef{
			{ID: "same", Version: 8, Digest: "new"},
		},
	}
	if err := intent.Validate(capabilities); err == nil {
		t.Fatal("expected duplicate object reservation error")
	}

	intent.GasInputs = []ObjectRef{{ID: "gas", Version: 0, Digest: "missing-version"}}
	if err := intent.Validate(capabilities); err == nil {
		t.Fatal("expected incomplete object ref error")
	}
}

func TestAddressBalanceRequiresCapability(t *testing.T) {
	intent := TransferIntent{
		Sender:     "0xsender",
		Recipient:  "0xrecipient",
		AssetType:  "USDC",
		Amount:     1,
		Funding:    FundingAddressBalance,
		GasFunding: Gasless,
	}
	if err := intent.Validate(Capabilities{ProtocolVersion: 123}); err == nil {
		t.Fatal("expected capability error")
	}
}

func TestGaslessDoesNotImplyCoinObjectSupport(t *testing.T) {
	intent := TransferIntent{
		Sender:     "0xsender",
		Recipient:  "0xrecipient",
		AssetType:  "USDC",
		Amount:     1,
		Funding:    FundingCoinObjects,
		GasFunding: Gasless,
		CoinInputs: []ObjectRef{
			{ID: "coin", Version: 1, Digest: "digest"},
		},
	}
	capabilities := Capabilities{
		ProtocolVersion:       125,
		AddressBalances:       true,
		GaslessTransferAssets: map[string]bool{"USDC": true},
	}
	if err := intent.Validate(capabilities); err == nil {
		t.Fatal("gasless stablecoin transfer must not be inferred for coin-object funding")
	}
}
