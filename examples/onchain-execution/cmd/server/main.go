package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"example.com/onchain-execution/internal/lab"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx := context.Background()
	addr := os.Getenv("LISTEN_ADDR")
	host, _, err := net.SplitHostPort(addr)
	if err != nil || (host != "127.0.0.1" && host != "::1") {
		log.Fatal("LISTEN_ADDR must be a loopback IP and port")
	}
	db, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(ctx, lab.Schema); err != nil {
		log.Fatal(err)
	}
	rpc, err := ethclient.Dial(os.Getenv("RPC_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer rpc.Close()
	key, err := crypto.HexToECDSA(os.Getenv("LAB_SIGNER_KEY"))
	if err != nil {
		log.Fatal("invalid fixture key")
	}
	b, err := os.ReadFile(os.Getenv("LAB_CONFIG"))
	if err != nil {
		log.Fatal(err)
	}
	var config lab.Config
	if err = json.Unmarshal(b, &config); err != nil {
		log.Fatal(err)
	}
	a := &lab.App{DB: db, RPC: rpc, Key: key, Sender: crypto.PubkeyToAddress(key.PublicKey), Config: config}
	if err = a.CheckChain(ctx); err != nil {
		log.Fatal(err)
	}
	log.Print("local gateway ready on ", addr)
	log.Fatal((&http.Server{Addr: addr, Handler: a.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 25 * time.Second, WriteTimeout: 25 * time.Second, MaxHeaderBytes: 8192}).ListenAndServe())
}
