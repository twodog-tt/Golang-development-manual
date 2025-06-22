package common

import (
	hexutil "github.com/ethereum/go-ethereum/common/hexutil"
	"math/big"
)

// Decode hex string to bytes
func Decode(input string) []byte {
	decode, err := hexutil.Decode(input)
	if err != nil {
		panic(err)
	}
	return decode
}

// Encode bytes to hex string
func Encode(b []byte) string {
	encode := hexutil.Encode(b)
	return encode
}

// MustDecode bytes to hex string
func MustDecode(input string) []byte {
	return hexutil.MustDecode(input)
}

// DecodeUint64 to uint64
func DecodeUint64(input string) uint64 {
	decodeUint64, err := hexutil.DecodeUint64(input)
	if err != nil {
		panic(err)
	}
	return decodeUint64
}

// EncodeUint64 to hex string
func EncodeUint64(i uint64) string {
	return hexutil.EncodeUint64(i)
}

// MustDecodeUint64 to uint64
func MustDecodeUint64(input string) uint64 {
	m := hexutil.MustDecodeUint64(input)
	return m
}

// DecodeBig to big.Int
//
//	type Int struct {
//		neg bool // sign [false：正数或零,true：负数]
//		abs nat  // absolute value of the integer
//	}
func DecodeBig(input string) *big.Int {
	decodeBig, err := hexutil.DecodeBig(input)
	if err != nil {
		panic(err)
	}
	return decodeBig
}

// EncodeBig to hex string
func EncodeBig(bigint *big.Int) string {
	return hexutil.EncodeBig(bigint)
}
