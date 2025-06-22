package common

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common/math"
)

func TestParseBig256(t *testing.T) {
	var n math.HexOrDecimal256
	//  [false：正数或零,true：负数]
	err := json.Unmarshal([]byte(`"0x100"`), &n)
	if err != nil {
		return
	} // n == 256
	t.Log("n:", n)
	err1 := json.Unmarshal([]byte(`"12345"`), &n)
	if err1 != nil {
		return
	} // n == 12345
	t.Log("n:", n)
	data, _ := json.Marshal(n) // 输出: "0x3039"
	t.Log(string(data))

	v, ok := math.ParseBig256("0x10000000000000000000000000000000000000000000000000000000000000000") // ok == false（超过 256 位）
	t.Log("v:", v, "ok:", ok)                                                                        // <nil> ok: false
	m, ok := math.ParseBig256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")  // ok == true
	t.Log("v:", m, "ok:", ok)                                                                        // v: 115792089237316195423570985008687907853269984665640564039457584007913129639935 ok: true

	j := big.NewInt(-1)
	j = math.U256(j)           // n == 2^256 - 1
	bytes := math.U256Bytes(j) // 输出: 32 字节的 0xff
	t.Log("bytes:", bytes)     // bytes: [255 255 255 255 255 255 255 255 255 255 255 255 255 255 255 255 255 255 255 255 255 255 255 255 255 255 255 255 255 255 255 255]
}
