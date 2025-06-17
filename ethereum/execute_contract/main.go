package main

import (
	"context"
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	store "td-homework/ethereum/deploy_contract"
)

const (
	contractAddr = "<deployed contract address>"
)

func main() {
	// 创建 eth client 实例：
	client, err := ethclient.Dial("<execution-layer-endpoint-url>")
	if err != nil {
		log.Fatal(err)
	}

	// 创建合约实例:
	storeContract, err := store.NewStore(common.HexToAddress(contractAddr), client)
	if err != nil {
		log.Fatal(err)
	}

	// 根据 hex 创建私钥实例：
	privateKey, err := crypto.HexToECDSA("<your private key>")
	if err != nil {
		log.Fatal(err)
	}

	// 调用合约方法：
	var key [32]byte
	var value [32]byte

	copy(key[:], []byte("demo_save_key"))
	copy(value[:], []byte("demo_save_value11111"))

	// // 初始化交易opt实例
	opt, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(11155111))
	if err != nil {
		log.Fatal(err)
	}
	// // 调用合约方法
	tx, err := storeContract.SetItem(opt, key, value)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("tx hash:", tx.Hash().Hex())

	// 查询合约中的数据并验证：
	callOpt := &bind.CallOpts{Context: context.Background()}
	valueInContract, err := storeContract.Items(callOpt, key)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("is value saving in contract equals to origin value:", valueInContract == value)
}
