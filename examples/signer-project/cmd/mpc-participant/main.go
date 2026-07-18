package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
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
		partyID                   = flag.String("id", "", "participant party ID")
		listen                    = flag.String("listen", "127.0.0.1:9543", "participant control API listen address")
		coordinatorURL            = flag.String("coordinator", "", "coordinator base URL")
		shareFile                 = flag.String("share-file", "", "path for this participant's share")
		sessionLedgerFile         = flag.String("session-ledger", "", "bbolt path for durable one-use DKG/signing session IDs (default: <share-file>.sessions.db)")
		shareKeyFile              = flag.String("share-key-file", "", "optional 0600 file containing a static 32-byte AES key")
		adminIdentities           = flag.String("admin-identities", "operator", "comma-separated control-plane admin identities")
		protocolTimeout           = flag.Duration("protocol-timeout", frostcluster.DefaultProtocolTimeout, "DKG/signing deadline")
		maxBodyBytes              = flag.Int64("max-body-bytes", frostcluster.DefaultMaxBodyBytes, "maximum control/message body")
		tlsCert                   = flag.String("tls-cert", "", "participant control-server certificate PEM")
		tlsKey                    = flag.String("tls-key", "", "participant control-server private key PEM")
		tlsClientCA               = flag.String("tls-client-ca", "", "CA PEM used to verify participant control clients")
		mtlsControlIdentityMap    = flag.String("mtls-control-identity-map-file", "", "optional 0600 JSON certificate-identity -> control identity map")
		coordinatorClientCert     = flag.String("coordinator-client-cert", "", "participant client certificate PEM for coordinator mTLS")
		coordinatorClientKey      = flag.String("coordinator-client-key", "", "participant client private key PEM for coordinator mTLS")
		coordinatorCA             = flag.String("coordinator-ca", "", "CA PEM used to verify the coordinator")
		coordinatorServerName     = flag.String("coordinator-server-name", "", "coordinator TLS server name")
		loopbackControlTokenFile  = flag.String("loopback-control-token-file", "", "0600 JSON identity->token file; loopback testing only")
		loopbackCoordinatorSecret = flag.String("loopback-coordinator-token-file", "", "0600 bearer-token file; loopback testing only")
	)
	flag.Parse()

	if *partyID == "" || *coordinatorURL == "" || *shareFile == "" {
		return errors.New("-id, -coordinator, and -share-file are required")
	}
	serverTLSEnabled, err := completeServerTLSFlags(*tlsCert, *tlsKey, *tlsClientCA)
	if err != nil {
		return err
	}
	parsedCoordinator, err := url.Parse(*coordinatorURL)
	if err != nil {
		return fmt.Errorf("parse coordinator URL: %w", err)
	}

	var controlTokens map[string]string
	if *loopbackControlTokenFile != "" {
		controlTokens, err = frostcluster.LoadIdentityTokens(*loopbackControlTokenFile)
		if err != nil {
			return fmt.Errorf("load control token file: %w", err)
		}
	}
	var controlCertificateIdentities map[string]string
	if *mtlsControlIdentityMap != "" {
		controlCertificateIdentities, err = frostcluster.LoadIdentityMap(*mtlsControlIdentityMap)
		if err != nil {
			return fmt.Errorf("load control mTLS identity map: %w", err)
		}
	}
	if !serverTLSEnabled {
		if !isLoopbackListen(*listen) {
			return errors.New("plaintext participant control API must listen on loopback")
		}
		if len(controlTokens) == 0 {
			return errors.New("plaintext participant requires -loopback-control-token-file")
		}
		log.Print("WARNING: participant control API uses loopback bearer tokens; this is not a production mTLS substitute")
	}

	httpClient := &http.Client{Timeout: 35 * time.Second}
	var coordinatorToken string
	switch parsedCoordinator.Scheme {
	case "https":
		if *coordinatorClientCert == "" || *coordinatorClientKey == "" || *coordinatorCA == "" {
			return errors.New("HTTPS coordinator requires -coordinator-client-cert, -coordinator-client-key, and -coordinator-ca for mTLS")
		}
		clientTLS, err := frostcluster.ClientTLSConfig(
			*coordinatorClientCert,
			*coordinatorClientKey,
			*coordinatorCA,
			*coordinatorServerName,
		)
		if err != nil {
			return err
		}
		httpClient.Transport = &http.Transport{TLSClientConfig: clientTLS}
	case "http":
		if *loopbackCoordinatorSecret == "" {
			return errors.New("HTTP coordinator requires -loopback-coordinator-token-file")
		}
		coordinatorToken, err = frostcluster.LoadSecret(*loopbackCoordinatorSecret)
		if err != nil {
			return fmt.Errorf("load coordinator token: %w", err)
		}
		log.Print("WARNING: coordinator connection uses a loopback bearer token; this is not a production mTLS substitute")
	default:
		return errors.New("coordinator URL must use http or https")
	}

	var staticKey []byte
	if *shareKeyFile != "" {
		staticKey, err = frostcluster.LoadShareEncryptionKey(*shareKeyFile)
		if err != nil {
			return fmt.Errorf("load static share key: %w", err)
		}
	} else {
		log.Print("WARNING: share persistence is plaintext; use -share-key-file for static AES-GCM protection and a KMS/HSM envelope in production")
	}
	store, err := frostcluster.NewShareStore(*shareFile, staticKey)
	if err != nil {
		return err
	}
	ledgerPath := *sessionLedgerFile
	if ledgerPath == "" {
		ledgerPath = *shareFile + ".sessions.db"
	}
	ledger, err := frostcluster.OpenSessionLedger(ledgerPath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := ledger.Close(); closeErr != nil {
			log.Printf("close participant session ledger: %v", closeErr)
		}
	}()
	relay, err := frostcluster.NewRelayClient(frostcluster.RelayClientConfig{
		BaseURL:         *coordinatorURL,
		PartyID:         *partyID,
		Token:           coordinatorToken,
		HTTPClient:      httpClient,
		MaxMessageBytes: *maxBodyBytes,
	})
	if err != nil {
		return err
	}
	participant, err := frostcluster.NewParticipant(frostcluster.ParticipantConfig{
		PartyID: *partyID,
		Store:   store,
		Relay:   relay,
		Ledger:  ledger,
		Authenticator: frostcluster.Authenticator{
			MTLSIdentities: controlCertificateIdentities,
			LoopbackTokens: controlTokens,
		},
		AdminIdentities: parseIdentitySet(*adminIdentities),
		MaxBodyBytes:    *maxBodyBytes,
		ProtocolTimeout: *protocolTimeout,
	})
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              *listen,
		Handler:           participant.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       *protocolTimeout + 5*time.Second,
		WriteTimeout:      *protocolTimeout + 5*time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	if serverTLSEnabled {
		server.TLSConfig, err = frostcluster.ServerTLSConfig(*tlsClientCA)
		if err != nil {
			return err
		}
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() {
		<-signals
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("participant shutdown: %v", err)
		}
	}()

	scheme := "http"
	if serverTLSEnabled {
		scheme = "https"
	}
	log.Printf("FROST participant %q listening on %s://%s with share protection %q", *partyID, scheme, *listen, store.Protection())
	if serverTLSEnabled {
		err = server.ListenAndServeTLS(*tlsCert, *tlsKey)
	} else {
		err = server.ListenAndServe()
	}
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func completeServerTLSFlags(cert, key, clientCA string) (bool, error) {
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
