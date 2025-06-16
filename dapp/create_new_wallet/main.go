package main

import (
	"crypto/ecdsa"
	"fmt"
	"log"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"golang.org/x/crypto/sha3"
)

func main() {
	// 1. 生成私钥 - 生成一对新的椭圆曲线密钥对（secp256k1），用于以太坊账户。
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		log.Fatal(err)
	}
	// 2. 提取私钥字节
	privateKeyBytes := crypto.FromECDSA(privateKey)
	// 将私钥转为字节数组，再编码成十六进制字符串（去掉前缀 0x）。
	fmt.Println(hexutil.Encode(privateKeyBytes)[2:]) // 去掉'0x'

	// 3. 提取公钥
	publicKey := privateKey.Public()
	// 获取私钥对应的公钥（椭圆曲线点）。
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("cannot assert type: publicKey is not of type *ecdsa.PublicKey")
	}

	//  4. 提取公钥字节 & 打印
	publicKeyBytes := crypto.FromECDSAPub(publicKeyECDSA)
	fmt.Println("from pubKey:", hexutil.Encode(publicKeyBytes)[4:]) // 去掉'0x04'

	// 5. 从公钥推导以太坊地址
	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()
	fmt.Println(address)

	// 6. 手动计算地址
	hash := sha3.NewLegacyKeccak256()
	hash.Write(publicKeyBytes[1:])
	fmt.Println("full:", hexutil.Encode(hash.Sum(nil)[:]))
	fmt.Println(hexutil.Encode(hash.Sum(nil)[12:])) // 原长32位，截去12位，保留后20位
}
