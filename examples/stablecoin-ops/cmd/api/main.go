package main

import (
	"example.com/stablecoin-ops/internal/ops"
	"go.temporal.io/sdk/client"
	"log"
	"net/url"
)

func main() {
	store := ops.Open()
	defer store.DB.Close()
	c, err := client.Dial(client.Options{HostPort: ops.Env("TEMPORAL_ADDRESS", "127.0.0.1:7233")})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()
	origin := ops.Env("APP_ORIGIN", "http://localhost:8081")
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		log.Fatal("invalid APP_ORIGIN")
	}
	secret := ops.Env("LAB_CHANNEL_SECRET", "")
	if len(secret) < 24 {
		log.Fatal("set a local LAB_CHANNEL_SECRET of at least 24 bytes")
	}
	auth := ops.Auth{Store: store, Domain: u.Host, URI: origin + "/ops", RPC: ops.Env("ETH_RPC", "")}
	ops.Serve(ops.API(auth, c, secret, ops.Seconds("APPROVAL_SECONDS", 300), ops.Seconds("OBSERVE_SECONDS", 300)))
}
