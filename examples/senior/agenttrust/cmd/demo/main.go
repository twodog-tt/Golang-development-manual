package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	t "td-homework/examples/senior/agenttrust"
)

type eoa struct{}

func (eoa) IsContract(context.Context, common.Address) (bool, error) { return false, nil }
func (eoa) Check1271(context.Context, common.Address, common.Hash, []byte) ([4]byte, error) {
	return [4]byte{}, fmt.Errorf("EOA fixture has no contract")
}
func run() error {
	ref := t.Ref{Chain: 84532, Registry: common.HexToAddress("0x100"), AgentID: "1"}
	owner := common.HexToAddress("0x200")
	reviewer := common.HexToAddress("0x300")
	rep := common.HexToAddress("0x400")
	val := common.HexToAddress("0x500")
	validator := common.HexToAddress("0x600")
	identity, err := t.NewIdentity(ref, owner)
	if err != nil {
		return err
	}
	initial := identity.Snapshot().Wallet == owner
	seed := sha256.Sum256([]byte("public teaching wallet; NEVER fund"))
	key, err := crypto.ToECDSA(seed[:])
	if err != nil {
		return err
	}
	wallet := crypto.PubkeyToAddress(key.PublicKey)
	now := time.Unix(1000, 0)
	digest, err := t.BindingDigest(identity.Snapshot(), wallet, 1100)
	if err != nil {
		return err
	}
	sig, err := crypto.Sign(digest[:], key)
	if err != nil {
		return err
	}
	if err = identity.Bind(context.Background(), owner, wallet, 1100, sig, now, eoa{}); err != nil {
		return err
	}
	bound := identity.Snapshot().Wallet == wallet
	if err = identity.Transfer(owner, common.HexToAddress("0x700")); err != nil {
		return err
	}
	cleared := identity.Snapshot().Wallet == (common.Address{})
	_, _, ssrf := t.NewResolver().Fetch(context.Background(), "https://169.254.169.254/latest/meta-data/", [32]byte{})
	p, err := t.NewProjection(ref, identity.Snapshot().Owner, rep, val)
	if err != nil {
		return err
	}
	h1 := crypto.Keccak256Hash([]byte("block1"))
	h2 := crypto.Keccak256Hash([]byte("block2"))
	payload := `{"task":"report-1","code":"v1"}`
	rh := crypto.Keccak256Hash([]byte(payload))
	first := t.Block{Number: 1, Hash: h1, Events: []t.Event{
		{Kind: "feedback", Registry: rep, Ref: ref, Actor: reviewer, Index: 1, Value: "100", Decimals: 2, Tag: "quality"},
		{Kind: "request", Registry: val, Ref: ref, Actor: identity.Snapshot().Owner, Validator: validator, RequestHash: rh, Payload: payload},
		{Kind: "response", Registry: val, Ref: ref, Actor: validator, RequestHash: rh, Score: 100, Tag: "soft"},
	}}
	if err = p.Apply(first); err != nil {
		return err
	}
	mean, n, err := p.Mean([]common.Address{reviewer}, "quality")
	if err != nil {
		return err
	}
	if err = p.Apply(t.Block{Number: 2, Hash: h2, Parent: h1, Events: []t.Event{{Kind: "revoke", Registry: rep, Ref: ref, Actor: reviewer, Index: 1}, {Kind: "response", Registry: val, Ref: ref, Actor: validator, RequestHash: rh, Score: 0, Tag: "invalidated"}}}); err != nil {
		return err
	}
	_, _, revoked := p.Mean([]common.Address{reviewer}, "quality")
	latest, err := p.Validation(rh)
	if err != nil {
		return err
	}
	if err = p.Rewind(h1); err != nil {
		return err
	}
	restored, err := p.Validation(rh)
	if err != nil {
		return err
	}
	pass := initial && bound && cleared && ssrf != nil && mean == "1" && n == 1 && revoked != nil && latest.Score == 0 && restored.Score == 100
	if !pass {
		return fmt.Errorf("fixture invariant failed")
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"passed": pass, "initial_owner_wallet": initial, "eoa_bound": bound, "transfer_cleared": cleared, "ssrf_blocked": ssrf != nil, "normalized_mean": mean, "revoked_excluded": revoked != nil, "latest_score": latest.Score, "reorg_restored_score": restored.Score})
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
