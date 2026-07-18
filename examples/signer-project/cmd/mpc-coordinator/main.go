package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/twodog-tt/Golang-development-manual/examples/signer-project/backend/frostcluster"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	var (
		listen             = flag.String("listen", "127.0.0.1:9443", "HTTP listen address")
		tlsCert            = flag.String("tls-cert", "", "server certificate PEM")
		tlsKey             = flag.String("tls-key", "", "server private key PEM")
		tlsClientCA        = flag.String("tls-client-ca", "", "CA PEM used to verify participant/operator client certificates")
		mtlsIdentityMap    = flag.String("mtls-identity-map-file", "", "optional 0600 JSON certificate-identity -> application-identity map")
		loopbackTokenFile  = flag.String("loopback-token-file", "", "0600 JSON identity->token file; loopback testing only")
		adminIdentities    = flag.String("admin-identities", "operator", "comma-separated identities allowed to register sessions")
		sessionTTL         = flag.Duration("session-ttl", frostcluster.DefaultSessionTTL, "default session lifetime")
		maxBodyBytes       = flag.Int64("max-body-bytes", frostcluster.DefaultMaxBodyBytes, "maximum request/message body")
		maxQueueMessages   = flag.Int("max-queue-messages", frostcluster.DefaultMaxQueueMessages, "maximum queued messages per participant/session")
		maxSessionMessages = flag.Int("max-session-messages", frostcluster.DefaultMaxSessionMessages, "maximum unique messages retained per session")
	)
	flag.Parse()

	tlsEnabled, err := completeTLSFlags(*tlsCert, *tlsKey, *tlsClientCA)
	if err != nil {
		return err
	}
	var tokens map[string]string
	if *loopbackTokenFile != "" {
		tokens, err = frostcluster.LoadIdentityTokens(*loopbackTokenFile)
		if err != nil {
			return fmt.Errorf("load loopback token file: %w", err)
		}
	}
	var certificateIdentities map[string]string
	if *mtlsIdentityMap != "" {
		certificateIdentities, err = frostcluster.LoadIdentityMap(*mtlsIdentityMap)
		if err != nil {
			return fmt.Errorf("load mTLS identity map: %w", err)
		}
	}
	if !tlsEnabled {
		if !isLoopbackListen(*listen) {
			return errors.New("plaintext coordinator must listen on loopback")
		}
		if len(tokens) == 0 {
			return errors.New("plaintext coordinator requires -loopback-token-file")
		}
		log.Print("WARNING: loopback bearer-token mode is for controlled local testing, not production")
	}

	coordinator, err := frostcluster.NewCoordinator(frostcluster.CoordinatorConfig{
		Authenticator: frostcluster.Authenticator{
			MTLSIdentities: certificateIdentities,
			LoopbackTokens: tokens,
		},
		AdminIdentities:    parseIdentitySet(*adminIdentities),
		MaxBodyBytes:       *maxBodyBytes,
		MaxQueueMessages:   *maxQueueMessages,
		MaxSessionMessages: *maxSessionMessages,
		SessionTTL:         *sessionTTL,
	})
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              *listen,
		Handler:           coordinator.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       35 * time.Second,
		WriteTimeout:      35 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	if tlsEnabled {
		server.TLSConfig, err = frostcluster.ServerTLSConfig(*tlsClientCA)
		if err != nil {
			return err
		}
	}

	shutdownDone := make(chan struct{})
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() {
		defer close(shutdownDone)
		<-signals
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("coordinator shutdown: %v", err)
		}
	}()

	scheme := "http"
	if tlsEnabled {
		scheme = "https"
	}
	log.Printf("FROST coordinator listening on %s://%s", scheme, *listen)
	if tlsEnabled {
		err = server.ListenAndServeTLS(*tlsCert, *tlsKey)
	} else {
		err = server.ListenAndServe()
	}
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	select {
	case <-shutdownDone:
	default:
	}
	return nil
}

func completeTLSFlags(cert, key, clientCA string) (bool, error) {
	count := 0
	for _, value := range []string{cert, key, clientCA} {
		if value != "" {
			count++
		}
	}
	if count != 0 && count != 3 {
		return false, errors.New("-tls-cert, -tls-key, and -tls-client-ca must be provided together")
	}
	return count == 3, nil
}

func parseIdentitySet(raw string) map[string]bool {
	result := make(map[string]bool)
	for _, value := range strings.Split(raw, ",") {
		value = strings.TrimSpace(value)
		if value != "" {
			result[value] = true
		}
	}
	return result
}

func isLoopbackListen(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}
