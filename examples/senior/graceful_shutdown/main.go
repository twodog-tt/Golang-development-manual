// 优雅关闭 HTTP 服务（编码练习 S-CODE-03）。
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		select {
		case <-r.Context().Done():
			http.Error(w, "request canceled", http.StatusRequestTimeout)
			return
		case <-timer.C:
			fmt.Fprintln(w, "ok")
		}
	})

	srv := &http.Server{
		Addr:    ":18080",
		Handler: mux,
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Println("listening on :18080")
		serveErr <- srv.ListenAndServe()
	}()

	signalCtx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("serve: %v", err)
		}
		return
	case <-signalCtx.Done():
		// 恢复默认 signal 行为，使第二次信号可强制终止。
		stop()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Println("shutting down...")
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
		if closeErr := srv.Close(); closeErr != nil {
			log.Printf("forced close: %v", closeErr)
		}
	}
	log.Println("done")
}
