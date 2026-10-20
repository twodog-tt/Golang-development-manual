// Package releaseadmission is a bounded, offline release-policy teaching fixture.
// Its keys, reports and deployment effects are local; it claims no SLSA level.
package releaseadmission

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"unicode/utf8"
)

const (
	StatementType  = "https://in-toto.io/Statement/v1"
	ProvenanceType = "https://slsa.dev/provenance/v1"
	TestType       = "https://knowledgebase.invalid/test/v1"
	SBOMType       = "https://knowledgebase.invalid/sbom-link/v1"
	ScanType       = "https://knowledgebase.invalid/scan-link/v1"
	ApprovalType   = "https://knowledgebase.invalid/release-approval/v1"
	ExceptionType  = "https://knowledgebase.invalid/vulnerability-exception/v1"
	PayloadType    = "application/vnd.in-toto+json"
)

var ErrDenied = errors.New("release admission denied")

type object = map[string]any
type Signature struct {
	KeyID string `json:"keyid"`
	Sig   []byte `json:"sig"`
}
type Envelope struct {
	PayloadType string      `json:"payloadType"`
	Payload     []byte      `json:"payload"`
	Signatures  []Signature `json:"signatures"`
}

func Digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func binding(v any) string {
	b, _ := json.Marshal(v)
	return Digest(append([]byte("local-release-binding:v1\x00"), b...))
}
func isDigest(s string) bool {
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == 32 && hex.EncodeToString(b) == s
}
func pae(typ string, payload []byte) []byte {
	return append([]byte(fmt.Sprintf("DSSEv1 %d %s %d ", len(typ), typ, len(payload))), payload...)
}

// Sign uses actual DSSE PAE + Ed25519. The demo keys are PUBLIC teaching seeds.
// Exactly one signature is supported. Other envelopes/algorithms require adapters.
func Sign(payload []byte, keyID string, key ed25519.PrivateKey) Envelope {
	return Envelope{PayloadType, payload, []Signature{{keyID, ed25519.Sign(key, pae(PayloadType, payload))}}}
}
func Statement(repo, digest, typ string, predicate any) []byte {
	b, _ := json.Marshal(object{"_type": StatementType, "subject": []any{object{"name": repo, "digest": object{"sha256": digest}}}, "predicateType": typ, "predicate": predicate})
	return b
}
func obj(v any) object { x, _ := v.(map[string]any); return x }
func str(v any) string { s, _ := v.(string); return s }
func list(v any) []any { a, _ := v.([]any); return a }
func integer(v any) (int64, bool) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, false
	}
	x, e := strconv.ParseInt(string(n), 10, 64)
	return x, e == nil
}
func exactKeys(m object, keys ...string) bool {
	if len(m) != len(keys) {
		return false
	}
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			return false
		}
	}
	return true
}

// JSON is bounded and duplicate-free. Semantic lookups are CASE-SENSITIVE.
// Generic SLSA extensions are ignored, but externalParameters use an allowlist.
func parse(raw []byte) (object, error) {
	if len(raw) == 0 || len(raw) > 128<<10 || !utf8.Valid(raw) || !validEscapes(raw) {
		return nil, ErrDenied
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	budget := 2048
	var read func(int) (any, error)
	read = func(depth int) (any, error) {
		budget--
		if depth > 16 || budget < 0 {
			return nil, ErrDenied
		}
		t, e := d.Token()
		if e != nil {
			return nil, ErrDenied
		}
		if t == json.Delim('{') {
			m := object{}
			for d.More() {
				key, e := d.Token()
				if e != nil {
					return nil, ErrDenied
				}
				k, ok := key.(string)
				if !ok {
					return nil, ErrDenied
				}
				if _, ok = m[k]; ok {
					return nil, ErrDenied
				}
				v, e := read(depth + 1)
				if e != nil {
					return nil, e
				}
				m[k] = v
			}
			end, e := d.Token()
			if e != nil || end != json.Delim('}') {
				return nil, ErrDenied
			}
			return m, nil
		}
		if t == json.Delim('[') {
			a := []any{}
			for d.More() {
				v, e := read(depth + 1)
				if e != nil {
					return nil, e
				}
				a = append(a, v)
			}
			end, e := d.Token()
			if e != nil || end != json.Delim(']') {
				return nil, ErrDenied
			}
			return a, nil
		}
		if _, ok := t.(json.Delim); ok {
			return nil, ErrDenied
		}
		return t, nil
	}
	v, e := read(0)
	if e != nil {
		return nil, e
	}
	if _, e = d.Token(); e != io.EOF {
		return nil, ErrDenied
	}
	m := obj(v)
	if m == nil {
		return nil, ErrDenied
	}
	return m, nil
}
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
