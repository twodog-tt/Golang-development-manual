// Package agenttrust contains bounded local ERC-8004 integration fixtures, not registry contracts.
package agenttrust

import (
	"context"
	"errors"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

var ErrDenied = errors.New("trust: denied or stale evidence")

type Ref struct {
	Chain    uint64
	Registry common.Address
	AgentID  string
}

func (r Ref) Valid() bool {
	if len(r.AgentID) == 0 || len(r.AgentID) > 78 {
		return false
	}
	n, ok := new(big.Int).SetString(r.AgentID, 10)
	return r.Chain > 0 && r.Registry != (common.Address{}) && ok && n.Sign() >= 0 && n.BitLen() <= 256 && n.String() == r.AgentID
}

type Snapshot struct {
	Ref           Ref
	Owner, Wallet common.Address
	Revision      uint64
}
type Identity struct {
	mu    sync.Mutex
	state Snapshot
}

func NewIdentity(ref Ref, owner common.Address) (*Identity, error) {
	if !ref.Valid() || owner == (common.Address{}) {
		return nil, ErrDenied
	}
	return &Identity{state: Snapshot{ref, owner, owner, 1}}, nil
}
func (i *Identity) Snapshot() Snapshot { i.mu.Lock(); defer i.mu.Unlock(); return i.state }
func (i *Identity) Transfer(caller, newOwner common.Address) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if caller != i.state.Owner || newOwner == (common.Address{}) {
		return ErrDenied
	}
	i.state.Owner = newOwner
	i.state.Wallet = common.Address{}
	i.state.Revision++
	return nil
}
func (i *Identity) Unset(caller common.Address) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if caller != i.state.Owner {
		return ErrDenied
	}
	i.state.Wallet = common.Address{}
	i.state.Revision++
	return nil
}

// BindingDigest is LOCAL typed-data v1, deliberately not an asserted ERC-8004 ABI.
// Target deployments must supply their actual type/domain/replay rules. Revision
// binds the owner epoch, including transfer-away-and-back; EIP-712 alone cannot.
func BindingDigest(s Snapshot, wallet common.Address, deadline int64) (common.Hash, error) {
	if !s.Ref.Valid() || s.Owner == (common.Address{}) || wallet == (common.Address{}) || deadline <= 0 {
		return common.Hash{}, ErrDenied
	}
	chain := math.HexOrDecimal256(*new(big.Int).SetUint64(s.Ref.Chain))
	typed := apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain":       {{Name: "name", Type: "string"}, {Name: "version", Type: "string"}, {Name: "chainId", Type: "uint256"}, {Name: "verifyingContract", Type: "address"}},
			"LocalWalletBinding": {{Name: "agentId", Type: "uint256"}, {Name: "owner", Type: "address"}, {Name: "wallet", Type: "address"}, {Name: "revision", Type: "uint256"}, {Name: "deadline", Type: "uint256"}},
		}, PrimaryType: "LocalWalletBinding", Domain: apitypes.TypedDataDomain{Name: "LocalAgentBinding", Version: "1", ChainId: &chain, VerifyingContract: s.Ref.Registry.Hex()},
		Message: apitypes.TypedDataMessage{"agentId": s.Ref.AgentID, "owner": s.Owner.Hex(), "wallet": wallet.Hex(), "revision": new(big.Int).SetUint64(s.Revision).String(), "deadline": big.NewInt(deadline).String()},
	}
	digest, _, err := apitypes.TypedDataAndHash(typed)
	return common.BytesToHash(digest), err
}

// WalletVerifier is trusted chain-adapter configuration, never a model assertion.
// Both methods MUST honor ctx. No EOA fallback is permitted after a lookup error.
type WalletVerifier interface {
	IsContract(context.Context, common.Address) (bool, error)
	Check1271(context.Context, common.Address, common.Hash, []byte) ([4]byte, error)
}

func (i *Identity) Bind(ctx context.Context, caller, wallet common.Address, deadline int64, signature []byte, now time.Time, v WalletVerifier) error {
	s := i.Snapshot()
	if caller != s.Owner || deadline <= now.Unix() || deadline > now.Add(10*time.Minute).Unix() || v == nil || len(signature) == 0 || len(signature) > 4096 {
		return ErrDenied
	}
	digest, err := BindingDigest(s, wallet, deadline)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, min(time.Second, time.Unix(deadline, 0).Sub(now)))
	defer cancel()
	contract, err := v.IsContract(ctx, wallet)
	if err != nil {
		return err
	}
	if contract {
		magic, e := v.Check1271(ctx, wallet, digest, signature)
		if e != nil {
			return e
		}
		if magic != [4]byte{0x16, 0x26, 0xba, 0x7e} {
			return ErrDenied
		}
	} else {
		if len(signature) != 65 || signature[64] > 1 {
			return ErrDenied
		}
		pub, e := crypto.SigToPub(digest[:], signature)
		if e != nil || crypto.PubkeyToAddress(*pub) != wallet {
			return ErrDenied
		}
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.state != s {
		return ErrDenied
	}
	i.state.Wallet = wallet
	i.state.Revision++
	return nil
}
