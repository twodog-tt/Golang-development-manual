package main

import (
	"context"
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

func main() {
	// 连接以太坊 Sepolia 测试网
	client, err := ethclient.Dial("https://eth-sepolia.g.alchemy.com/v2/Cb4j5devGl6ggzj3iEW8M67btfjB9zOa")
	if err != nil {
		log.Fatal(err)
	}

	// 获取链的 ChainID
	chainID, err := client.ChainID(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// 获取指定区块（编号：5671744）
	blockNumber := big.NewInt(5671744)
	block, err := client.BlockByNumber(context.Background(), blockNumber)
	if err != nil {
		log.Fatal(err)
	}

	// 遍历交易并获取关键信息（这里只处理了第一个交易）
	for _, tx := range block.Transactions() {
		fmt.Println(tx.Hash().Hex())        // 交易哈希 - 0x20294a03e8766e9aeab58327fc4112756017c6c28f6f99c7722f4a29075601c5
		fmt.Println(tx.Value().String())    // 交易金额（单位：wei） - 100000000000000000
		fmt.Println(tx.Gas())               // Gas 限额 - 21000
		fmt.Println(tx.GasPrice().Uint64()) // Gas 单价 - 100000000000
		fmt.Println(tx.Nonce())             // nonce - 245132
		fmt.Println(tx.Data())              // 调用的数据（常用于合约调用）- []
		fmt.Println(tx.To().Hex())          // 接收方地址 - 0x8F9aFd209339088Ced7Bc0f57Fe08566ADda3587

		if sender, err := types.Sender(types.NewEIP155Signer(chainID), tx); err == nil { // EIP-155 是以太坊的一种防止交易重放的机制。
			fmt.Println("sender", sender.Hex()) // 获取发送者（From 地址） - 0x2CdA41645F2dBffB852a605E92B185501801FC28
		} else {
			log.Fatal(err)
		}

		//  获取交易回执（Receipt）
		receipt, err := client.TransactionReceipt(context.Background(), tx.Hash())
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(receipt.Status) // 1
		fmt.Println(receipt.Logs)   // []
		break
	}

	// 通过区块哈希读取交易数量和交易
	blockHash := common.HexToHash("0xae713dea1419ac72b928ebe6ba9915cd4fc1ef125a606f90f5e783c47cb1a4b5")
	count, err := client.TransactionCount(context.Background(), blockHash) // 获取某个区块中的交易总数；
	if err != nil {
		log.Fatal(err)
	}

	for idx := uint(0); idx < count; idx++ {
		tx, err := client.TransactionInBlock(context.Background(), blockHash, idx)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(tx.Hash().Hex()) // 这就是该交易的 哈希值 - 0x20294a03e8766e9aeab58327fc4112756017c6c28f6f99c7722f4a29075601c5
		break
	}

	// 还可以使用 TransactionByHash 在给定具体事务哈希值的情况下直接查询单个事务。
	txHash := common.HexToHash("0x20294a03e8766e9aeab58327fc4112756017c6c28f6f99c7722f4a29075601c5")
	tx, isPending, err := client.TransactionByHash(context.Background(), txHash)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(isPending)       // true → 仍需等待；false → 可进一步使用 TransactionReceipt 查询区块编号、状态等信息。
	fmt.Println(tx.Hash().Hex()) // 0x20294a03e8766e9aeab58327fc4112756017c6c28f6f99c7722f4a29075601c5.Println(isPending)       // false
}
