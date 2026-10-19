package ops

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

// Only the fresh local runner enables this test; never connect to an existing DB.
func TestWalletSessionIdentityIntegration(t *testing.T) {
	if os.Getenv("LAB_AUTH_INTEGRATION") != "1" {
		t.Skip("requires isolated stablecoin-ops PostgreSQL")
	}
	s := Open()
	defer s.DB.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	key, _ := crypto.HexToECDSA(strings.Repeat("0", 63) + "3")
	address := crypto.PubkeyToAddress(key.PublicKey).Hex()
	if _, err := s.DB.Exec(ctx, "INSERT INTO ops.wallets VALUES($1,'session_fixture','eoa')", address); err != nil {
		t.Fatal(err)
	}
	defer s.DB.Exec(context.Background(), "DELETE FROM ops.wallets WHERE address=$1", address)
	a := Auth{Store: s, Domain: "localhost", URI: "http://localhost/ops"}
	challenge, err := a.Challenge(ctx, address)
	if err != nil {
		t.Fatal(err)
	}
	sig, _ := crypto.Sign(accounts.TextHash([]byte(challenge.Message)), key)
	token, err := a.Login(ctx, challenge.ID, hexutil.Encode(sig))
	if err != nil {
		t.Fatal(err)
	}
	var savedAddress, merchant string
	var chain int64
	if err = s.DB.QueryRow(ctx, "SELECT address,merchant,chain_id FROM ops.sessions WHERE digest=$1", digest(token)).Scan(&savedAddress, &merchant, &chain); err != nil || savedAddress != address || merchant != "session_fixture" || chain != 31337 {
		t.Fatalf("lost signing identity: address=%s merchant=%s chain=%d err=%v", savedAddress, merchant, chain, err)
	}
	if got, err := a.Merchant(ctx, "Bearer "+token); err != nil || got != merchant {
		t.Fatalf("valid session rejected: %s %v", got, err)
	}
	// A stale merchant mapping must not be accepted even while the session exists.
	if _, err = s.DB.Exec(ctx, "UPDATE ops.wallets SET merchant='different_merchant' WHERE address=$1", address); err != nil {
		t.Fatal(err)
	}
	if _, err = a.Merchant(ctx, "Bearer "+token); !errors.Is(err, ErrDenied) {
		t.Fatal("session accepted after its wallet binding changed", err)
	}
	if _, err = s.DB.Exec(ctx, "DELETE FROM ops.wallets WHERE address=$1", address); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = s.DB.QueryRow(ctx, "SELECT count(*) FROM ops.sessions WHERE digest=$1", digest(token)).Scan(&count); err != nil || count != 0 {
		t.Fatal("wallet deletion retained session", count, err)
	}
	if _, err = a.Merchant(ctx, "Bearer "+token); !errors.Is(err, ErrDenied) {
		t.Fatal("deleted wallet session accepted", err)
	}

	// Unbind while ERC-1271 RPC is in flight; a valid RPC reply must not revive it.
	if _, err = s.DB.Exec(ctx, "INSERT INTO ops.wallets VALUES($1,'session_fixture','erc1271')", address); err != nil {
		t.Fatal(err)
	}
	entered, resume := make(chan struct{}), make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     any    `json:"id"`
			Method string `json:"method"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			t.Error("invalid RPC request")
			return
		}
		result := "0x7a69"
		switch req.Method {
		case "eth_blockNumber":
			result = "0x10"
		case "eth_call":
			close(entered)
			select {
			case <-resume:
			case <-r.Context().Done():
				return
			}
			result = "0x1626ba7e" + strings.Repeat("0", 56)
		}
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer server.Close()
	defer close(resume)
	a.RPC = server.URL
	challenge, err = a.Challenge(ctx, address)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := a.Login(ctx, challenge.ID, "0x1234"); done <- err }()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("RPC did not start")
	}
	if _, err = s.DB.Exec(ctx, "DELETE FROM ops.wallets WHERE address=$1", address); err != nil {
		t.Fatal(err)
	}
	// Use a value rather than close so the deferred close also releases on failure.
	resume <- struct{}{}
	select {
	case err := <-done:
		if !errors.Is(err, ErrDenied) {
			t.Fatal("in-flight login revived unbound wallet", err)
		}
	case <-ctx.Done():
		t.Fatal("login did not finish")
	}
	if err = s.DB.QueryRow(ctx, "SELECT count(*) FROM ops.sessions WHERE address=$1", address).Scan(&count); err != nil || count != 0 {
		t.Fatal("in-flight login left session", count, err)
	}
}
