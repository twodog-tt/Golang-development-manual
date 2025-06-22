package main

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"log"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ERC20 ABI 的 balanceOf 函数部分
const erc20ABI = `[{"constant":true,"inputs":[{"name":"account","type":"address"}],"name":"balanceOf","outputs":[{"name":"","type":"uint256"}],"type":"function"}]`

func main() {
	client, err := ethclient.Dial("https://eth-sepolia.g.alchemy.com/v2/Cb4j5devGl6ggzj3iEW8M67btfjB9zOa")
	if err != nil {
		log.Fatal(err)
	}
	// Golem (GNT) Address
	tokenAddress := common.HexToAddress("0x2C378f07B2e29A787D4484E4B25233d26525E847")

	accountAddress := common.HexToAddress("0x7Cd44D221243911c127716200A1787ddE007E711")

	// 解析合约 ABI
	parsedABI, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		log.Fatal(err)
	}

	// 构造调用的 input data
	data, err := parsedABI.Pack("balanceOf", accountAddress)
	if err != nil {
		log.Fatal(err)
	}

	// 构造调用消息
	msg := ethereum.CallMsg{
		To:   &tokenAddress,
		Data: data,
	}

	// 执行 eth_call
	result, err := client.CallContract(context.Background(), msg, nil)
	if err != nil {
		log.Fatal(err)
	}

	// 解析返回值
	var balance *big.Int
	err = parsedABI.UnpackIntoInterface(&balance, "balanceOf", result)
	if err != nil {
		log.Fatal(err)
	}

	// 打印余额（单位：最小单位，如 18 decimals）
	fmt.Printf("Token Balance: %s\n", balance.String())
}
