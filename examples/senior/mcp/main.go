// Package main runs the local MCP authorization fixture over stdio or loopback HTTP.
package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"td-homework/examples/senior/mcp/internal/server"
	"time"
)

func main() {
	transport := flag.String("transport", "stdio", "stdio or http")
	addr := flag.String("addr", "127.0.0.1:8088", "loopback HTTP address")
	flag.Parse()
	token := os.Getenv("MCP_DEMO_TOKEN")
	if token == "" {
		log.Fatal("set MCP_DEMO_TOKEN to a local demo secret")
	}
	resource := "http://" + *addr + "/mcp"
	a := server.NewAuthority(resource)
	a.Put(token, server.Grant{Principal: server.Principal{Subject: "demo-user", Tenant: "tenant-a", Scopes: []string{"greet", "orders:read"}}, Audience: resource, Expires: time.Now().Add(time.Hour)})
	c := server.Config{Authority: a}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	switch *transport {
	case "stdio":
		c.LocalToken = token
		if err := server.Run(ctx, server.New(c)); err != nil {
			log.Fatal(err)
		}
	case "http":
		host, _, err := net.SplitHostPort(*addr)
		if err != nil || !net.ParseIP(host).IsLoopback() {
			log.Fatal("HTTP fixture requires a loopback IP address")
		}
		srv := &http.Server{Addr: *addr, Handler: server.HTTP(c, []string{"http://" + *addr}, time.Minute), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
		go func() {
			<-ctx.Done()
			shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(shutdown)
		}()
		log.Printf("local MCP fixture: %s (no OAuth login flow)", resource)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	default:
		log.Fatal("unknown transport")
	}
}
