package ethrpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_BlockNumber(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x10"}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	hex, err := c.BlockNumber(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if hex != "0x10" {
		t.Fatalf("got %s", hex)
	}
}

func TestClient_RPCError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"method not found"}}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.BlockNumber(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_GetTransactionReceipt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"status":"0x1","blockNumber":"0x10"}}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	raw, err := c.GetTransactionReceipt(context.Background(), "0xabc")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m["status"] != "0x1" {
		t.Fatalf("got %+v", m)
	}
}
