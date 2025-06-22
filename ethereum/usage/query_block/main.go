package main

import (
	"context"
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/ethclient"
)

// 查询区块
func main() {
	// 连接到 Alchemy 提供的 Sepolia 测试网节点
	client, err := ethclient.Dial("https://eth-sepolia.g.alchemy.com/v2/Cb4j5devGl6ggzj3iEW8M67btfjB9zOa")
	if err != nil {
		log.Fatal(err)
	}

	// 指定要查询的区块号，查询第 5671744 个区块；
	blockNumber := big.NewInt(5671744)

	// 只获取 区块头（Header），比完整区块更轻量。
	header, err := client.HeaderByNumber(context.Background(), blockNumber)
	fmt.Println(header.Number.Uint64())     // 区块号 - 5671744
	fmt.Println(header.Time)                // 出块时间戳（秒） - 1712798400
	fmt.Println(header.Difficulty.Uint64()) // 工作量证明难度（在合并后可能为 0）- 0
	fmt.Println(header.Hash().Hex())        // 区块哈希 - 0xae713dea1419ac72b928ebe6ba9915cd4fc1ef125a606f90f5e783c47cb1a4b5

	if err != nil {
		log.Fatal(err)
	}
	// 获取完整区块信息
	block, err := client.BlockByNumber(context.Background(), blockNumber)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(block.Number().Uint64())     // 区块号 - 5671744
	fmt.Println(block.Time())                // 出块时间戳 - 1712798400
	fmt.Println(block.Difficulty().Uint64()) // 难度（合并后为 0） - 0
	fmt.Println(block.Hash().Hex())          // 区块哈希 - 0xae713dea1419ac72b928ebe6ba9915cd4fc1ef125a606f90f5e783c47cb1a4b5
	fmt.Println(len(block.Transactions()))   // 区块内交易数量 - 70

	// 另一种获取某区块中交易数量的方法（比遍历 block.Transactions() 更高效）。
	count, err := client.TransactionCount(context.Background(), block.Hash())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(count) // 区块内交易数量 - 70
}
