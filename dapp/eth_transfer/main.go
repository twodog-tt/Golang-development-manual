package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func main() {
	// 连接到 Infura 提供的 Rinkeby 测试网节点。
	client, err := ethclient.Dial("https://eth-sepolia.g.alchemy.com/v2/Cb4j5devGl6ggzj3iEW8M67btfjB9zOa")
	if err != nil {
		log.Fatal(err)
	}

	// 导入私钥 （私钥是硬编码在代码中的，这样非常危险，仅限测试用途！）
	privateKey, err := crypto.HexToECDSA("private key address...")
	if err != nil {
		log.Fatal(err)
	}

	// 从私钥生成对应的公钥 → 从公钥计算出以太坊地址。
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("cannot assert type: publicKey is not of type *ecdsa.PublicKey")
	}

	// 得到的 fromAddress 就是这笔交易的发送者。
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	// 获取 nonce （交易计数器：表示这个地址之前已经发送了多少笔交易。） 这一步是为保证交易不被链拒绝（防止重放攻击）。
	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		log.Fatal(err)
	}

	// 设置交易参数
	//value := big.NewInt(1000000000000000000) // in wei (1 eth)
	value := big.NewInt(1000000000000000) // in wei (0.001 eth)
	gasLimit := uint64(21000)             // in units 燃气上限
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// 构造交易对象，创建一个普通转账交易（无数据）。
	// - toAddress 是目标账户。
	toAddress := common.HexToAddress("0xBF8D286340Ac990b4c92533118F2f1Cb08994421")
	var data []byte
	tx := types.NewTransaction(nonce, toAddress, value, gasLimit, gasPrice, data)

	// 签名交易
	// 获取链的 ID（Rinkeby 是 4），用于防止跨链重放攻击。
	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// 使用 EIP-155 签名机制（支持链 ID）。
	// 使用私钥对交易进行签名，返回的是 signedTx。
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		log.Fatal(err)
	}

	// 发送交易到链上
	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("tx sent: %s", signedTx.Hash().Hex()) // 可以用这个哈希到 Rinkeby 区块浏览器（如：https://rinkeby.etherscan.io/）中查看交易状态。
}
