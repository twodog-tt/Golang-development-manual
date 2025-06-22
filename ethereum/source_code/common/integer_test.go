package common

import (
	"testing"

	"github.com/ethereum/go-ethereum/common/math"
)

func TestNewInteger64(t *testing.T) {

	v, ok := math.ParseUint64("0x100") // v == 256, ok == true
	t.Logf(" ParseUint64 : %v, %v", v, ok)
	m, ok := math.ParseUint64("12345") // v == 12345, ok == true
	t.Logf(" ParseUint64 : %v, %v", m, ok)

	sum, overflow := math.SafeAdd(1<<63, 1<<63) // overflow == true
	t.Logf(" SafeAdd : %v, %v", sum, overflow)
	diff, underflow := math.SafeSub(1, 2) // underflow == true
	t.Logf(" SafeSub : %v, %v", diff, underflow)
	prod, overflow := math.SafeMul(1<<32, 1<<32) // overflow == true
	t.Logf(" SafeMul : %v, %v", prod, overflow)
}
