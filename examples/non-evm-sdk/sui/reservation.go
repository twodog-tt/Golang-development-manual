// Package suiadapter demonstrates a capability-aware reservation boundary for
// Sui's object inputs and newer address-balance funding modes.
//
// It deliberately does not pretend to be a transaction SDK. Transaction bytes,
// protocol versions, and RPC/gRPC schemas belong behind a version-pinned adapter.
package suiadapter

import (
	"errors"
	"fmt"
	"sort"
)

type FundingMode string

const (
	FundingCoinObjects    FundingMode = "COIN_OBJECTS"
	FundingAddressBalance FundingMode = "ADDRESS_BALANCE"
	FundingHybrid         FundingMode = "HYBRID"
)

type GasMode string

const (
	GasCoinObjects    GasMode = "COIN_OBJECTS"
	GasAddressBalance GasMode = "ADDRESS_BALANCE"
	Gasless           GasMode = "GASLESS"
)

type ObjectRef struct {
	ID      string
	Version uint64
	Digest  string
}

type Capabilities struct {
	ProtocolVersion       uint64
	AddressBalances       bool
	GaslessTransferAssets map[string]bool
}

type TransferIntent struct {
	Sender     string
	Recipient  string
	AssetType  string
	Amount     uint64
	Funding    FundingMode
	GasFunding GasMode
	CoinInputs []ObjectRef
	GasInputs  []ObjectRef
}

func (intent TransferIntent) Validate(capabilities Capabilities) error {
	if capabilities.ProtocolVersion == 0 {
		return errors.New("protocol version is required")
	}
	if intent.Sender == "" || intent.Recipient == "" || intent.AssetType == "" || intent.Amount == 0 {
		return errors.New("sender, recipient, asset, and amount are required")
	}
	switch intent.Funding {
	case FundingCoinObjects:
		if len(intent.CoinInputs) == 0 {
			return errors.New("coin-object funding requires coin input refs")
		}
	case FundingAddressBalance:
		if !capabilities.AddressBalances {
			return errors.New("provider/protocol does not support address balances")
		}
		if len(intent.CoinInputs) != 0 {
			return errors.New("address-balance mode must not carry coin object inputs")
		}
	case FundingHybrid:
		if !capabilities.AddressBalances || len(intent.CoinInputs) == 0 {
			return errors.New("hybrid mode requires address-balance support and object inputs")
		}
	default:
		return fmt.Errorf("unsupported funding mode %q", intent.Funding)
	}

	switch intent.GasFunding {
	case GasCoinObjects:
		if len(intent.GasInputs) == 0 {
			return errors.New("coin-object gas mode requires gas input refs")
		}
	case GasAddressBalance:
		if !capabilities.AddressBalances {
			return errors.New("address-balance gas mode is not supported")
		}
		if len(intent.GasInputs) != 0 {
			return errors.New("address-balance gas mode must not carry gas object inputs")
		}
	case Gasless:
		if !capabilities.GaslessTransferAssets[intent.AssetType] {
			return errors.New("asset is not enabled for gasless transfers")
		}
		// The protocol feature demonstrated here is powered by Address
		// Balances. A future protocol/provider may expose other combinations,
		// but an adapter must not infer that support from the asset alone.
		if intent.Funding != FundingAddressBalance {
			return errors.New("gasless transfer requires address-balance funding")
		}
		if len(intent.GasInputs) != 0 {
			return errors.New("gasless mode must not carry gas object inputs")
		}
	default:
		return fmt.Errorf("unsupported gas mode %q", intent.GasFunding)
	}
	return validateUniqueRefs(append(append([]ObjectRef(nil), intent.CoinInputs...), intent.GasInputs...))
}

// ReservationKeys are concurrency-control keys, not Sui transaction inputs.
// The adapter must re-read current refs immediately before building transaction
// bytes and must never silently replace a reserved version/digest.
func (intent TransferIntent) ReservationKeys() []string {
	keys := make([]string, 0, len(intent.CoinInputs)+len(intent.GasInputs)+1)
	if intent.Funding == FundingAddressBalance || intent.Funding == FundingHybrid {
		keys = append(keys, "balance:"+intent.Sender+":"+intent.AssetType)
	}
	if intent.GasFunding == GasAddressBalance {
		keys = append(keys, "gas-balance:"+intent.Sender+":SUI")
	}
	for _, ref := range append(append([]ObjectRef(nil), intent.CoinInputs...), intent.GasInputs...) {
		keys = append(keys, fmt.Sprintf("object:%s:%d:%s", ref.ID, ref.Version, ref.Digest))
	}
	sort.Strings(keys)
	return keys
}

func validateUniqueRefs(refs []ObjectRef) error {
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		if ref.ID == "" || ref.Version == 0 || ref.Digest == "" {
			return errors.New("every object ref requires ID, version, and digest")
		}
		if _, exists := seen[ref.ID]; exists {
			return fmt.Errorf("object %s is used more than once", ref.ID)
		}
		seen[ref.ID] = struct{}{}
	}
	return nil
}
