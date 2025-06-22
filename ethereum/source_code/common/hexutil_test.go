package common

import (
	"testing"
)

func TestDecode(t *testing.T) {
	decodes := Decode("0x7ac41f9d32b8e670")
	t.Log(decodes)
}

func TestEncode(t *testing.T) {
	t.Log(Encode([]byte{0x3f, 0x9a, 0x7c, 0x1e, 0x8b, 0x42, 0xd5, 0xa0, 0xe6, 0xf3, 0xc1, 0xbd, 0x4a, 0x79, 0xd2, 0x14}))
}

func TestMustDecode(t *testing.T) {
	t.Log(MustDecode("0x3f9a7c1e8b42d5a0e6f3c1bd4a79d214"))
}

func TestDecodeUint64(t *testing.T) {
	t.Log(DecodeUint64("0x7ac41f9d32b8e670")) // 8846230328083801712
}
func TestEncodeUint64(t *testing.T) {
	t.Log(EncodeUint64(8846230328083801712)) // 0x7ac41f9d32b8e670
}
func TestMustDecodeUint64(t *testing.T) {
	t.Log(MustDecodeUint64("0x7ac41f9d32b8e670")) // 8846230328083801712
}
func TestDecodeBig(t *testing.T) {
	t.Log(DecodeBig("0x7ac41f9d32b8e670")) // 8846230328083801712
}

func TestEncodeBig(t *testing.T) {
	t.Log(EncodeBig(DecodeBig("0x7ac41f9d32b8e670")))
}
