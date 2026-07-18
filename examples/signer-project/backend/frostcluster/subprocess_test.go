package frostcluster

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/taurusgroup/multi-party-sig/pkg/taproot"
)

// TestClusterProcessHelper is entered only by TestIndependentParticipantProcesses.
// One helper OS process creates either a share-free coordinator or exactly one
// participant with exactly one share path.
func TestClusterProcessHelper(t *testing.T) {
	role := os.Getenv("FROST_CLUSTER_HELPER_ROLE")
	if role == "" {
		return
	}
	if err := runClusterProcessHelper(role); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Exit(0)
}

func runClusterProcessHelper(role string) error {
	var handler http.Handler
	switch role {
	case "coordinator":
		var tokens map[string]string
		if err := json.Unmarshal([]byte(os.Getenv("FROST_CLUSTER_TOKENS")), &tokens); err != nil {
			return err
		}
		coordinator, err := NewCoordinator(CoordinatorConfig{
			Authenticator:   Authenticator{LoopbackTokens: tokens},
			AdminIdentities: map[string]bool{testAdminIdentity: true},
			SessionTTL:      time.Minute,
		})
		if err != nil {
			return err
		}
		handler = coordinator.Handler()
	case "participant":
		id := os.Getenv("FROST_CLUSTER_PARTY_ID")
		store, err := NewShareStore(os.Getenv("FROST_CLUSTER_SHARE_PATH"), nil)
		if err != nil {
			return err
		}
		ledger, err := OpenSessionLedger(os.Getenv("FROST_CLUSTER_LEDGER_PATH"))
		if err != nil {
			return err
		}
		defer ledger.Close()
		relay, err := NewRelayClient(RelayClientConfig{
			BaseURL: os.Getenv("FROST_CLUSTER_COORDINATOR"),
			PartyID: id,
			Token:   os.Getenv("FROST_CLUSTER_PARTY_TOKEN"),
		})
		if err != nil {
			return err
		}
		participant, err := NewParticipant(ParticipantConfig{
			PartyID:         id,
			Store:           store,
			Relay:           relay,
			Ledger:          ledger,
			Authenticator:   Authenticator{LoopbackTokens: map[string]string{testAdminIdentity: testAdminToken}},
			AdminIdentities: map[string]bool{testAdminIdentity: true},
			ProtocolTimeout: 5 * time.Second,
		})
		if err != nil {
			return err
		}
		handler = participant.Handler()
	default:
		return fmt.Errorf("unknown helper role %q", role)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	ready := os.NewFile(3, "frost-cluster-ready")
	if ready == nil {
		listener.Close()
		return errors.New("ready file descriptor is unavailable")
	}
	if _, err := fmt.Fprintln(ready, listener.Addr().String()); err != nil {
		ready.Close()
		listener.Close()
		return err
	}
	ready.Close()
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
	}
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func TestIndependentParticipantProcesses(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess FROST integration in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	partyTokens := map[string]string{
		"alice": "alice-token-00000000000000000000000000",
		"bob":   "bob-token-0000000000000000000000000000",
		"carol": "carol-token-00000000000000000000000000",
	}
	coordinatorTokens := map[string]string{testAdminIdentity: testAdminToken}
	for id, token := range partyTokens {
		coordinatorTokens[id] = token
	}
	encodedTokens, err := json.Marshal(coordinatorTokens)
	if err != nil {
		t.Fatal(err)
	}
	coordinator := startClusterHelper(t, ctx, map[string]string{
		"FROST_CLUSTER_HELPER_ROLE": "coordinator",
		"FROST_CLUSTER_TOKENS":      string(encodedTokens),
	})
	coordinatorURL := "http://" + coordinator.address
	waitForHealth(t, ctx, coordinatorURL+"/healthz")
	coordinatorClient, err := NewCoordinatorClient(CoordinatorClientConfig{
		BaseURL: coordinatorURL,
		Token:   testAdminToken,
	})
	if err != nil {
		t.Fatal(err)
	}

	participants := make(map[string]*helperProcess)
	sharePaths := make(map[string]string)
	for _, id := range []string{"alice", "bob", "carol"} {
		shareDir := filepath.Join(t.TempDir(), id)
		if err := os.Mkdir(shareDir, 0o700); err != nil {
			t.Fatal(err)
		}
		sharePath := filepath.Join(shareDir, "share.json")
		sharePaths[id] = sharePath
		process := startClusterHelper(t, ctx, map[string]string{
			"FROST_CLUSTER_HELPER_ROLE": "participant",
			"FROST_CLUSTER_PARTY_ID":    id,
			"FROST_CLUSTER_SHARE_PATH":  sharePath,
			"FROST_CLUSTER_LEDGER_PATH": filepath.Join(shareDir, "sessions.db"),
			"FROST_CLUSTER_COORDINATOR": coordinatorURL,
			"FROST_CLUSTER_PARTY_TOKEN": partyTokens[id],
		})
		participants[id] = process
		waitForHealth(t, ctx, "http://"+process.address+"/healthz")
	}

	dkgSessionID, err := NewSessionID()
	if err != nil {
		t.Fatal(err)
	}
	parties := []string{"alice", "bob", "carol"}
	if _, err := coordinatorClient.CreateSession(ctx, SessionSpec{
		ID:        dkgSessionID,
		Kind:      SessionKindDKG,
		KeyID:     "process-key",
		Parties:   parties,
		Threshold: 1,
	}); err != nil {
		t.Fatal(err)
	}
	dkgRequest := DKGRequest{
		SessionID: dkgSessionID,
		KeyID:     "process-key",
		Parties:   parties,
		Threshold: 1,
	}
	dkgResponses := make(map[string]DKGResponse)
	var dkgResponsesMu sync.Mutex
	runConcurrent(t, parties, func(id string) error {
		var response DKGResponse
		if err := postJSON(
			ctx,
			"http://"+participants[id].address+"/v1/dkg",
			testAdminToken,
			dkgRequest,
			&response,
		); err != nil {
			return err
		}
		dkgResponsesMu.Lock()
		dkgResponses[id] = response
		dkgResponsesMu.Unlock()
		return nil
	})
	publicKey := dkgResponses["alice"].PublicKey
	for _, id := range parties {
		if dkgResponses[id].PublicKey != publicKey {
			t.Fatal("independent processes disagree on DKG public key")
		}
		info, err := os.Stat(sharePaths[id])
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s subprocess share mode=%#o", id, info.Mode().Perm())
		}
	}

	digest := sha256.Sum256([]byte("independent FROST participant processes"))
	signSessionID, err := NewSessionID()
	if err != nil {
		t.Fatal(err)
	}
	signers := []string{"alice", "bob"}
	if _, err := coordinatorClient.CreateSession(ctx, SessionSpec{
		ID:        signSessionID,
		Kind:      SessionKindSign,
		KeyID:     "process-key",
		Parties:   signers,
		Threshold: 1,
		DigestHex: hex.EncodeToString(digest[:]),
	}); err != nil {
		t.Fatal(err)
	}
	signRequest := SignRequest{
		SessionID: signSessionID,
		KeyID:     "process-key",
		Signers:   signers,
		DigestHex: hex.EncodeToString(digest[:]),
	}
	signResponses := make(map[string]SignResponse)
	var signResponsesMu sync.Mutex
	runConcurrent(t, signers, func(id string) error {
		var response SignResponse
		if err := postJSON(
			ctx,
			"http://"+participants[id].address+"/v1/sign",
			testAdminToken,
			signRequest,
			&response,
		); err != nil {
			return err
		}
		signResponsesMu.Lock()
		signResponses[id] = response
		signResponsesMu.Unlock()
		return nil
	})
	if signResponses["alice"].Signature != signResponses["bob"].Signature {
		t.Fatal("independent signer processes disagree on signature")
	}
	publicBytes, err := hex.DecodeString(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	signatureBytes, err := hex.DecodeString(signResponses["alice"].Signature)
	if err != nil {
		t.Fatal(err)
	}
	if !taproot.PublicKey(publicBytes).Verify(taproot.Signature(signatureBytes), digest[:]) {
		t.Fatal("subprocess BIP-340 signature did not verify")
	}
}

type helperProcess struct {
	command *exec.Cmd
	address string
	stderr  *bytes.Buffer
	done    chan error
}

func startClusterHelper(t *testing.T, ctx context.Context, values map[string]string) *helperProcess {
	t.Helper()
	readyReader, readyWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr := new(bytes.Buffer)
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestClusterProcessHelper$")
	command.Env = append(os.Environ(), "FROST_CLUSTER_HELPER=1")
	for key, value := range values {
		command.Env = append(command.Env, key+"="+value)
	}
	command.ExtraFiles = []*os.File{readyWriter}
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		readyReader.Close()
		readyWriter.Close()
		t.Fatal(err)
	}
	readyWriter.Close()
	addressResult := make(chan struct {
		address string
		err     error
	}, 1)
	go func() {
		scanner := bufio.NewScanner(readyReader)
		if scanner.Scan() {
			addressResult <- struct {
				address string
				err     error
			}{address: strings.TrimSpace(scanner.Text())}
			return
		}
		addressResult <- struct {
			address string
			err     error
		}{err: scanner.Err()}
	}()
	var address string
	select {
	case result := <-addressResult:
		readyReader.Close()
		if result.err != nil || result.address == "" {
			_ = command.Process.Kill()
			_ = command.Wait()
			t.Fatalf("helper did not report address: %v: %s", result.err, stderr.String())
		}
		address = result.address
	case <-time.After(5 * time.Second):
		readyReader.Close()
		_ = command.Process.Kill()
		_ = command.Wait()
		t.Fatalf("helper startup timed out: %s", stderr.String())
	}
	process := &helperProcess{
		command: command,
		address: address,
		stderr:  stderr,
		done:    make(chan error, 1),
	}
	go func() {
		process.done <- command.Wait()
	}()
	t.Cleanup(func() {
		if command.Process == nil {
			return
		}
		_ = command.Process.Signal(os.Interrupt)
		select {
		case <-process.done:
		case <-time.After(2 * time.Second):
			_ = command.Process.Kill()
			<-process.done
		}
	})
	return process
}

func waitForHealth(t *testing.T, ctx context.Context, endpoint string) {
	t.Helper()
	for {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			t.Fatal(err)
		}
		response, err := http.DefaultClient.Do(request)
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("health check %s: %v", endpoint, ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
}
