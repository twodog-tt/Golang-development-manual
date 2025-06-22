package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"

	"golang.org/x/crypto/sha3"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func main() {
	client, err := ethclient.Dial("https://eth-sepolia.g.alchemy.com/v2/Cb4j5devGl6ggzj3iEW8M67btfjB9zOa")
	if err != nil {
		log.Fatal(err)
	}

	// 导入账户私钥并获取地址 将私钥转为 ECDSA 类型。
	privateKey, err := crypto.HexToECDSA("private key address...")
	if err != nil {
		log.Fatal(err)
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("cannot assert type: publicKey is not of type *ecdsa.PublicKey")
	}

	// 提取出地址（fromAddress）作为转出账户。
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	// 获取发送者当前的交易序号，防止交易冲突。
	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		log.Fatal(err)
	}

	value := big.NewInt(0) // in wei (0 eth)
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	toAddress := common.HexToAddress("0xBF8D286340Ac990b4c92533118F2f1Cb08994421")
	tokenAddress := common.HexToAddress("0xC509606492A8f4eCF4176a98B24a62a2c64DcEd7")

	// 构造调用合约的交易数据
	transferFnSignature := []byte("transfer(address,uint256)")
	hash := sha3.NewLegacyKeccak256()
	hash.Write(transferFnSignature)
	methodID := hash.Sum(nil)[:4]
	fmt.Println(hexutil.Encode(methodID)) // 0xa9059cbb
	// 按照 ABI 编码规范，将地址和金额左补 0 对齐为 32 字节（256 bit）。
	paddedAddress := common.LeftPadBytes(toAddress.Bytes(), 32)
	fmt.Println(hexutil.Encode(paddedAddress)) // 0x0000000000000000000000004592d8f8d7b001e72cb26a73e4fa1806a51ac79d
	amount := new(big.Int)
	amount.SetString("10000000000000000000000", 10) // 10000 * 10^18 tokens
	paddedAmount := common.LeftPadBytes(amount.Bytes(), 32)
	fmt.Println(hexutil.Encode(paddedAmount)) // 0x00000000000000000000000000000000000000000000003635c9adc5dea00000
	//  构造最终 data
	var data []byte
	data = append(data, methodID...)
	data = append(data, paddedAddress...)
	data = append(data, paddedAmount...)

	balance, err := client.BalanceAt(context.Background(), fromAddress, nil)
	fmt.Println("ETH 余额（wei）：", balance)
	// 预估 Gas
	gasLimit, err := client.EstimateGas(context.Background(), ethereum.CallMsg{
		To:   &toAddress,
		Data: data,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("gasLimit:", gasLimit) // 23256
	gasLimit = uint64(100000)          // 不推荐长期硬编码，但可以用来验证是不是 gas 太小导致失败
	fmt.Println("gasLimit:", gasLimit) // 100000

	// 构造交易
	// to = tokenAddress：目标为 ERC20 合约地址。
	// value = 0：转 ERC20，不涉及 ETH 转账
	//data 是 ABI 编码后的调用参数。
	tx := types.NewTransaction(nonce, tokenAddress, value, gasLimit, gasPrice, data)

	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// 使用私钥和 EIP155 方式对交易签名。
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		log.Fatal(err)
	}

	// 最后广播到链上。
	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("tx sent: %s", signedTx.Hash().Hex()) // tx sent: 0xa56316b637a94c4cc0331c73ef26389d6c097506d581073f927275e7a6ece0bc
}
