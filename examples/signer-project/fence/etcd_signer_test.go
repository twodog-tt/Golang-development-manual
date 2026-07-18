package fence

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

func TestEtcdKeyspaceEscapesUserComponents(t *testing.T) {
	prefix := "/test/fence"
	first := etcdRequestKey(prefix, "a/b", "request")
	second := etcdRequestKey(prefix, "a", "b/request")
	if first == second {
		t.Fatalf("escaped etcd keys collided: %q", first)
	}
	if bytes.Contains([]byte(first), []byte("a/b")) {
		t.Fatalf("key ID was not escaped: %q", first)
	}
	if got := etcdLockPrefix(prefix, "a/b"); got == prefix+"/locks/a/b" {
		t.Fatalf("lock component was not escaped: %q", got)
	}
}

func TestNormalizeEtcdPrefix(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "default", want: DefaultEtcdFencePrefix},
		{name: "trim trailing", input: "/tenant/signer///", want: "/tenant/signer"},
		{name: "relative", input: "tenant/signer", wantErr: true},
		{name: "root", input: "/", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeEtcdPrefix(test.input)
			if test.wantErr {
				if !errors.Is(err, ErrInvalidRequest) {
					t.Fatalf("normalizeEtcdPrefix error=%v, want ErrInvalidRequest", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("normalizeEtcdPrefix=%q, want %q", got, test.want)
			}
		})
	}
}

func TestDecodeEtcdRequestRejectsCrossKeyReceipt(t *testing.T) {
	digest := DigestPayload([]byte("payload"))
	record := etcdPersistedRequest{
		Version:       storeVersion,
		KeyID:         "key-a",
		RequestID:     "request-a",
		Status:        StatusCompleted,
		Owner:         "owner-a",
		Epoch:         7,
		PayloadDigest: digest,
		Receipt: &Receipt{
			Version:       storeVersion,
			KeyID:         "key-b",
			RequestID:     "request-a",
			Owner:         "owner-a",
			Epoch:         7,
			PayloadDigest: digest,
			Algorithm:     "test",
			PublicKey:     []byte("public-key"),
			Signature:     []byte("signature"),
		},
	}
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeEtcdRequest(
		raw,
		"key-a",
		"request-a",
	); !errors.Is(err, ErrCorruptStore) {
		t.Fatalf("decodeEtcdRequest error=%v, want ErrCorruptStore", err)
	}
}
