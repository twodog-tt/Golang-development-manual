package erc20bind_test

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient/simulated"

	"td-homework/examples/senior/erc20bind"
)

func TestSimpleToken_DeployTransferBalance(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(1337))
	if err != nil {
		t.Fatal(err)
	}

	backend := simulated.NewBackend(types.GenesisAlloc{
		auth.From: {Balance: big.NewInt(1_000_000_000_000_000_000)}, // 1 ETH
	})
	defer backend.Close()

	client := backend.Client()
	ctx := context.Background()

	supply := big.NewInt(1_000_000)
	_, _, token, err := erc20bind.DeploySimpleToken(auth, client, supply)
	if err != nil {
		t.Fatal(err)
	}
	backend.Commit()

	recipient := common.HexToAddress("0x1234567890123456789012345678901234567890")
	recipientAuth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(1337))
	if err != nil {
		t.Fatal(err)
	}

	tx, err := token.Transfer(recipientAuth, recipient, big.NewInt(100))
	if err != nil {
		t.Fatal(err)
	}
	backend.Commit()

	receipt, err := client.TransactionReceipt(ctx, tx.Hash())
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("transfer failed status=%d", receipt.Status)
	}

	bal, err := token.BalanceOf(&bind.CallOpts{Context: ctx}, recipient)
	if err != nil {
		t.Fatal(err)
	}
	if bal.Cmp(big.NewInt(100)) != 0 {
		t.Fatalf("recipient balance=%s", bal)
	}

	ownerBal, err := token.BalanceOf(&bind.CallOpts{Context: ctx}, auth.From)
	if err != nil {
		t.Fatal(err)
	}
	want := new(big.Int).Sub(supply, big.NewInt(100))
	if ownerBal.Cmp(want) != 0 {
		t.Fatalf("owner balance=%s want=%s", ownerBal, want)
	}
}

func TestSimpleToken_FilterTransfer(t *testing.T) {
	key, _ := crypto.GenerateKey()
	auth, _ := bind.NewKeyedTransactorWithChainID(key, big.NewInt(1337))
	backend := simulated.NewBackend(types.GenesisAlloc{
		auth.From: {Balance: big.NewInt(1_000_000_000_000_000_000)},
	})
	defer backend.Close()

	client := backend.Client()
	_, _, token, err := erc20bind.DeploySimpleToken(auth, client, big.NewInt(1_000))
	if err != nil {
		t.Fatal(err)
	}
	backend.Commit()

	recipient := common.HexToAddress("0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	if _, err := token.Transfer(auth, recipient, big.NewInt(50)); err != nil {
		t.Fatal(err)
	}
	backend.Commit()

	iter, err := token.FilterTransfer(&bind.FilterOpts{}, []common.Address{auth.From}, []common.Address{recipient})
	if err != nil {
		t.Fatal(err)
	}
	defer iter.Close()

	if !iter.Next() {
		t.Fatal("expected Transfer event")
	}
	ev := iter.Event
	if ev.Value.Cmp(big.NewInt(50)) != 0 {
		t.Fatalf("event value=%s", ev.Value)
	}
}
