package agentplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Canonical implements ONLY local fixed-string intent-v1, not general JCS.
// Six exact ASCII keys, all values strings; uint256 amount is decimal text.
func Canonical(raw []byte) (string, string, error) {
	if len(raw) > 8192 || !utf8.Valid(raw) || !validEscapes(raw) {
		return "", "", ErrDenied
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	t, e := d.Token()
	if e != nil || t != json.Delim('{') {
		return "", "", ErrDenied
	}
	m := map[string]string{}
	for d.More() {
		k, e := d.Token()
		if e != nil {
			return "", "", ErrDenied
		}
		key, ok := k.(string)
		if !ok {
			return "", "", ErrDenied
		}
		if _, seen := m[key]; seen {
			return "", "", ErrDenied
		}
		v, e := d.Token()
		if e != nil {
			return "", "", ErrDenied
		}
		s, ok := v.(string)
		if !ok {
			return "", "", ErrDenied
		}
		m[key] = s
	}
	if t, e = d.Token(); e != nil || t != json.Delim('}') {
		return "", "", ErrDenied
	}
	if _, e = d.Token(); e != io.EOF {
		return "", "", ErrDenied
	}
	keys := []string{"amount", "domain", "memo", "operation", "tenant", "version"}
	if len(m) != len(keys) || m["version"] != "1" || m["domain"] != "agent-platform/intent" || m["operation"] != "pay" || m["tenant"] == "" || len([]rune(m["tenant"])) > 128 || len([]rune(m["memo"])) > 1024 {
		return "", "", ErrDenied
	}
	a := m["amount"]
	n, ok := new(big.Int).SetString(a, 10)
	if !ok || n.Sign() <= 0 || n.BitLen() > 256 || n.String() != a {
		return "", "", ErrDenied
	}
	var out strings.Builder
	out.WriteByte('{')
	for i, k := range keys {
		v, ok := m[k]
		if !ok {
			return "", "", ErrDenied
		}
		if i > 0 {
			out.WriteByte(',')
		}
		out.WriteString(jsonString(k))
		out.WriteByte(':')
		out.WriteString(jsonString(v))
	}
	out.WriteByte('}')
	s := out.String()
	return s, digest([]byte("agent-platform:wire:v1\x00" + s)), nil
}
func jsonString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 32 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// encoding/json replaces unpaired surrogate escapes with U+FFFD; reject these
// before decoding rather than silently changing the signed input.
func validEscapes(b []byte) bool {
	for i := 0; i < len(b); i++ {
		if b[i] != '\\' {
			continue
		}
		i++
		if i >= len(b) {
			return false
		}
		if b[i] != 'u' {
			continue
		}
		if i+4 >= len(b) {
			return false
		}
		n, e := strconv.ParseUint(string(b[i+1:i+5]), 16, 16)
		if e != nil {
			return false
		}
		i += 4
		if n >= 0xdc00 && n <= 0xdfff {
			return false
		}
		if n >= 0xd800 && n <= 0xdbff {
			if i+6 >= len(b) || b[i+1] != '\\' || b[i+2] != 'u' {
				return false
			}
			low, e := strconv.ParseUint(string(b[i+3:i+7]), 16, 16)
			if e != nil || low < 0xdc00 || low > 0xdfff {
				return false
			}
			i += 6
		}
	}
	return true
}
