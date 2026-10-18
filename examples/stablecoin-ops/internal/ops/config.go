package ops

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

func Env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
func Seconds(name string, fallback int) int {
	n, err := strconv.Atoi(Env(name, strconv.Itoa(fallback)))
	if err != nil || n < 1 {
		log.Fatal("invalid duration " + name)
	}
	return n
}
func Open() *Store {
	p, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	if err = p.Ping(context.Background()); err != nil {
		log.Fatal(err)
	}
	return &Store{p}
}
func Serve(handler http.Handler) {
	addr := Env("LISTEN_ADDR", "127.0.0.1:8081")
	server := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second}
	log.Printf("local lab listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}
