package main

import (
	"example.com/stablecoin-ops/internal/ops"
	"log"
)

func main() {
	store := ops.Open()
	defer store.DB.Close()
	secret := ops.Env("LAB_CHANNEL_SECRET", "")
	if len(secret) < 24 {
		log.Fatal("set LAB_CHANNEL_SECRET")
	}
	ops.Serve(ops.Provider(store, secret))
}
