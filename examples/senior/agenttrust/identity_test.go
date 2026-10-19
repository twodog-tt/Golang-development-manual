package agenttrust

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

var ref = Ref{84532, common.HexToAddress("0x100"), "1"}
var owner = common.HexToAddress("0x200")

type verifier struct {
	contract bool
	checkErr error
	magic    [4]byte
	err      error
	wait     bool
	hook     func()
}

func (v verifier) IsContract(ctx context.Context, _ common.Address) (bool, error) {
	if v.hook != nil {
		v.hook()
	}
	if v.wait {
		<-ctx.Done()
		return false, ctx.Err()
	}
	return v.contract, v.err
}
func (v verifier) Check1271(context.Context, common.Address, common.Hash, []byte) ([4]byte, error) {
	return v.magic, v.checkErr
}
func TestWalletInitialDefaultEOAProofAndTransferClear(t *testing.T) {
	now := time.Unix(1000, 0)
	i, e := NewIdentity(ref, owner)
	if e != nil {
		t.Fatal(e)
	}
	if i.Snapshot().Wallet != owner {
		t.Fatal("initial wallet must equal owner")
	}
	key, e := crypto.GenerateKey()
	if e != nil {
		t.Fatal(e)
	}
	wallet := crypto.PubkeyToAddress(key.PublicKey)
	digest, e := BindingDigest(i.Snapshot(), wallet, 1100)
	if e != nil {
		t.Fatal(e)
	}
	sig, e := crypto.Sign(digest[:], key)
	if e != nil {
		t.Fatal(e)
	}
	if e = i.Bind(context.Background(), owner, wallet, 1100, sig, now, verifier{}); e != nil {
		t.Fatal(e)
	}
	if e = i.Bind(context.Background(), owner, wallet, 1100, sig, now, verifier{}); e == nil {
		t.Fatal("replayed old revision")
	}
	if e = i.Transfer(owner, common.HexToAddress("0x300")); e != nil || i.Snapshot().Wallet != (common.Address{}) {
		t.Fatal("transfer failed to clear wallet", e)
	}
}
func TestWalletDomainOwnerEpochExpiryAnd1271Failures(t *testing.T) {
	key, _ := crypto.GenerateKey()
	wallet := crypto.PubkeyToAddress(key.PublicKey)
	now := time.Unix(1000, 0)
	for _, mode := range []string{"chain", "registry", "agent", "owner", "epoch", "expired", "rpc-error", "wrong-magic", "revert", "timeout", "changed-during-check", "owner-transfer-return", "expires-during-check"} {
		t.Run(mode, func(t *testing.T) {
			i, _ := NewIdentity(ref, owner)
			s := i.Snapshot()
			deadline := int64(1100)
			checkTime := now
			v := verifier{}
			switch mode {
			case "chain":
				s.Ref.Chain++
			case "registry":
				s.Ref.Registry = common.HexToAddress("0x999")
			case "agent":
				s.Ref.AgentID = "2"
			case "owner":
				s.Owner = wallet
			case "epoch":
				s.Revision++
			case "expired":
				deadline = 1000
			case "expires-during-check":
				deadline = 1001
				checkTime = time.Unix(1000, 990000000)
				v.wait = true
			case "rpc-error":
				v.err = errors.New("RPC unavailable")
			case "revert":
				v.contract = true
				v.checkErr = errors.New("contract reverted")
			case "owner-transfer-return":
				next := common.HexToAddress("0x999")
				_ = i.Transfer(owner, next)
				_ = i.Transfer(next, owner)
			case "wrong-magic":
				v.contract = true
			case "timeout":
				v.wait = true
			case "changed-during-check":
				v.hook = func() { _ = i.Unset(owner) }
			}
			digest, e := BindingDigest(s, wallet, deadline)
			if e != nil {
				t.Fatal(e)
			}
			sig, _ := crypto.Sign(digest[:], key)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
			defer cancel()
			if e = i.Bind(ctx, owner, wallet, deadline, sig, checkTime, v); e == nil || i.Snapshot().Wallet == wallet {
				t.Fatal("invalid proof bound wallet", e)
			}
		})
	}
}
func Test1271MagicAndStateDependentRevalidation(t *testing.T) {
	i, _ := NewIdentity(ref, owner)
	wallet := common.HexToAddress("0x400")
	v := verifier{contract: true, magic: [4]byte{0x16, 0x26, 0xba, 0x7e}}
	if e := i.Bind(context.Background(), owner, wallet, 1100, []byte("local contract signature"), time.Unix(1000, 0), v); e != nil {
		t.Fatal(e)
	}
	v.magic = [4]byte{}
	if e := i.Bind(context.Background(), owner, wallet, 1100, []byte("same signature"), time.Unix(1000, 0), v); e == nil {
		t.Fatal("cached contract validity")
	}
	if e := i.Unset(common.HexToAddress("0xbad")); e == nil {
		t.Fatal("unauthorized clear")
	}
}
