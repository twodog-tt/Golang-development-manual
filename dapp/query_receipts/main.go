package main

import (
	"context"
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

func main() {
	client, err := ethclient.Dial("https://eth-sepolia.g.alchemy.com/v2/Cb4j5devGl6ggzj3iEW8M67btfjB9zOa")
	if err != nil {
		log.Fatal(err)
	}

	blockNumber := big.NewInt(5671744)
	blockHash := common.HexToHash("0xae713dea1419ac72b928ebe6ba9915cd4fc1ef125a606f90f5e783c47cb1a4b5")

	// 通过区块哈希获取回执（receipts）
	// BlockReceipts 返回的是区块中所有交易的回执列表（[]*types.Receipt）。
	// 参数是区块哈希。
	receiptByHash, err := client.BlockReceipts(context.Background(), rpc.BlockNumberOrHashWithHash(blockHash, false))
	if err != nil {
		log.Fatal(err)
	}

	// 通过区块高度获取回执 （另一种方式：通过区块号查询所有交易的回执。）
	receiptsByNum, err := client.BlockReceipts(context.Background(), rpc.BlockNumberOrHashWithNumber(rpc.BlockNumber(blockNumber.Int64())))
	if err != nil {
		log.Fatal(err)
	}

	// 对于相同的区块，无论通过 hash 还是 number，结果应一致。
	fmt.Println(receiptByHash[0] == receiptsByNum[0]) // true

	for _, receipt := range receiptByHash {
		fmt.Println(receipt.Status)                // 交易是否成功 - 1
		fmt.Println(receipt.Logs)                  // 合约事件日志 - []
		fmt.Println(receipt.TxHash.Hex())          // 交易哈希 - 0x20294a03e8766e9aeab58327fc4112756017c6c28f6f99c7722f4a29075601c5
		fmt.Println(receipt.TransactionIndex)      // 交易在区块中的索引 - 0
		fmt.Println(receipt.ContractAddress.Hex()) // 新创建的合约地址（如果是部署交易） - 0x0000000000000000000000000000000000000000
		break
	}

	txHash := common.HexToHash("0x20294a03e8766e9aeab58327fc4112756017c6c28f6f99c7722f4a29075601c5")

	// 通过交易哈希单独获取回执
	receipt, err := client.TransactionReceipt(context.Background(), txHash)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(receipt.Status)                // 1
	fmt.Println(receipt.Logs)                  // []
	fmt.Println(receipt.TxHash.Hex())          // 0x20294a03e8766e9aeab58327fc4112756017c6c28f6f99c7722f4a29075601c5
	fmt.Println(receipt.TransactionIndex)      // 0
	fmt.Println(receipt.ContractAddress.Hex()) // 0x0000000000000000000000000000000000000000
}
