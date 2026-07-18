package frostcluster

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/taurusgroup/multi-party-sig/pkg/party"
	"github.com/taurusgroup/multi-party-sig/pkg/protocol"
	"github.com/taurusgroup/multi-party-sig/pkg/taproot"
	upstreamfrost "github.com/taurusgroup/multi-party-sig/protocols/frost"
)

const (
	testAdminIdentity = "operator"
	testAdminToken    = "operator-token-000000000000000000000000"
)

type clusterHarness struct {
	t                    *testing.T
	coordinator          *Coordinator
	coordinatorServer    *httptest.Server
	coordinatorClient    *CoordinatorClient
	participants         map[string]*Participant
	participantServers   map[string]*httptest.Server
	stores               map[string]*ShareStore
	partyTokens          map[string]string
	participantAdminAuth Authenticator
}

func newClusterHarness(t *testing.T, protocolTimeout time.Duration) *clusterHarness {
	t.Helper()
	partyTokens := map[string]string{
		"alice": "alice-token-00000000000000000000000000",
		"bob":   "bob-token-0000000000000000000000000000",
		"carol": "carol-token-00000000000000000000000000",
	}
	coordinatorTokens := map[string]string{testAdminIdentity: testAdminToken}
	for id, token := range partyTokens {
		coordinatorTokens[id] = token
	}
	coordinator, err := NewCoordinator(CoordinatorConfig{
		Authenticator:   Authenticator{LoopbackTokens: coordinatorTokens},
		AdminIdentities: map[string]bool{testAdminIdentity: true},
		SessionTTL:      time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	coordinatorServer := httptest.NewServer(coordinator.Handler())
	t.Cleanup(coordinatorServer.Close)
	coordinatorClient, err := NewCoordinatorClient(CoordinatorClientConfig{
		BaseURL: coordinatorServer.URL,
		Token:   testAdminToken,
	})
	if err != nil {
		t.Fatal(err)
	}
	harness := &clusterHarness{
		t:                    t,
		coordinator:          coordinator,
		coordinatorServer:    coordinatorServer,
		coordinatorClient:    coordinatorClient,
		participants:         make(map[string]*Participant),
		participantServers:   make(map[string]*httptest.Server),
		stores:               make(map[string]*ShareStore),
		partyTokens:          partyTokens,
		participantAdminAuth: Authenticator{LoopbackTokens: map[string]string{testAdminIdentity: testAdminToken}},
	}
	for _, id := range []string{"alice", "bob", "carol"} {
		participantDir := filepath.Join(t.TempDir(), id)
		store, err := NewShareStore(filepath.Join(participantDir, "taproot-share.json"), nil)
		if err != nil {
			t.Fatal(err)
		}
		ledger, err := OpenSessionLedger(filepath.Join(participantDir, "sessions.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := ledger.Close(); err != nil {
				t.Errorf("close session ledger: %v", err)
			}
		})
		relay, err := NewRelayClient(RelayClientConfig{
			BaseURL: coordinatorServer.URL,
			PartyID: id,
			Token:   partyTokens[id],
		})
		if err != nil {
			t.Fatal(err)
		}
		participant, err := NewParticipant(ParticipantConfig{
			PartyID:         id,
			Store:           store,
			Relay:           relay,
			Ledger:          ledger,
			Authenticator:   harness.participantAdminAuth,
			AdminIdentities: map[string]bool{testAdminIdentity: true},
			ProtocolTimeout: protocolTimeout,
		})
		if err != nil {
			t.Fatal(err)
		}
		server := httptest.NewServer(participant.Handler())
		t.Cleanup(server.Close)
		harness.stores[id] = store
		harness.participants[id] = participant
		harness.participantServers[id] = server
	}
	return harness
}

func TestEndToEndThreePartyDKGAndTwoOfThreeBIP340Signing(t *testing.T) {
	harness := newClusterHarness(t, 10*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	dkgSessionID, err := NewSessionID()
	if err != nil {
		t.Fatal(err)
	}
	parties := []string{"alice", "bob", "carol"}
	_, err = harness.coordinatorClient.CreateSession(ctx, SessionSpec{
		ID:        dkgSessionID,
		Kind:      SessionKindDKG,
		KeyID:     "cluster-key",
		Parties:   parties,
		Threshold: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	dkgRequest := DKGRequest{
		SessionID: dkgSessionID,
		KeyID:     "cluster-key",
		Parties:   parties,
		Threshold: 1,
	}
	dkgResponses := make(map[string]DKGResponse)
	var dkgResponsesMu sync.Mutex
	runConcurrent(t, parties, func(id string) error {
		var response DKGResponse
		if err := postJSON(ctx, harness.participantServers[id].URL+"/v1/dkg", testAdminToken, dkgRequest, &response); err != nil {
			return err
		}
		dkgResponsesMu.Lock()
		dkgResponses[id] = response
		dkgResponsesMu.Unlock()
		return nil
	})

	var publicKey string
	privateShares := make(map[string]string)
	for _, id := range parties {
		response := dkgResponses[id]
		if response.PartyID != id || response.Threshold != 1 {
			t.Fatalf("%s DKG response=%+v", id, response)
		}
		if publicKey == "" {
			publicKey = response.PublicKey
		} else if response.PublicKey != publicKey {
			t.Fatalf("participants disagree on public key: %s != %s", response.PublicKey, publicKey)
		}
		info, err := os.Stat(harness.stores[id].Path())
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("%s share mode=%#o, want 0600", id, got)
		}
		config, err := harness.stores[id].Load("cluster-key")
		if err != nil {
			t.Fatal(err)
		}
		if config.ID != party.ID(id) {
			t.Fatalf("%s loaded share belongs to %q", id, config.ID)
		}
		encoded, err := config.PrivateShare.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		privateShares[id] = hex.EncodeToString(encoded)
	}
	if privateShares["alice"] == privateShares["bob"] ||
		privateShares["alice"] == privateShares["carol"] ||
		privateShares["bob"] == privateShares["carol"] {
		t.Fatal("independent participants unexpectedly persisted an identical private share")
	}
	secondDKGSessionID, err := NewSessionID()
	if err != nil {
		t.Fatal(err)
	}
	secondDKG := dkgRequest
	secondDKG.SessionID = secondDKGSessionID
	if _, err := harness.participants["alice"].RunDKG(ctx, secondDKG); !errors.Is(err, ErrShareExists) {
		t.Fatalf("second DKG error=%v, want ErrShareExists before protocol start", err)
	}

	digest := sha256.Sum256([]byte("cross-process FROST BIP-340 signing"))
	signSessionID, err := NewSessionID()
	if err != nil {
		t.Fatal(err)
	}
	signers := []string{"alice", "bob"}
	_, err = harness.coordinatorClient.CreateSession(ctx, SessionSpec{
		ID:        signSessionID,
		Kind:      SessionKindSign,
		KeyID:     "cluster-key",
		Parties:   signers,
		Threshold: 1,
		DigestHex: hex.EncodeToString(digest[:]),
	})
	if err != nil {
		t.Fatal(err)
	}
	signRequest := SignRequest{
		SessionID: signSessionID,
		KeyID:     "cluster-key",
		Signers:   signers,
		DigestHex: hex.EncodeToString(digest[:]),
	}
	signResponses := make(map[string]SignResponse)
	var signResponsesMu sync.Mutex
	runConcurrent(t, signers, func(id string) error {
		var response SignResponse
		if err := postJSON(ctx, harness.participantServers[id].URL+"/v1/sign", testAdminToken, signRequest, &response); err != nil {
			return err
		}
		signResponsesMu.Lock()
		signResponses[id] = response
		signResponsesMu.Unlock()
		return nil
	})
	if signResponses["alice"].Signature != signResponses["bob"].Signature {
		t.Fatal("signers disagree on final signature")
	}
	if signResponses["alice"].PublicKey != publicKey || signResponses["bob"].PublicKey != publicKey {
		t.Fatal("signing public key differs from DKG public key")
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
		t.Fatal("two-of-three BIP-340 signature did not verify")
	}
	if _, err := harness.participants["alice"].RunSign(ctx, signRequest); !errors.Is(err, ErrSessionReplay) {
		t.Fatalf("replayed signing control request error=%v, want ErrSessionReplay", err)
	}
}

func TestParticipantOfflineCausesProtocolDeadline(t *testing.T) {
	harness := newClusterHarness(t, 5*time.Second)
	sessionID, err := NewSessionID()
	if err != nil {
		t.Fatal(err)
	}
	parties := []string{"alice", "bob", "carol"}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := harness.coordinatorClient.CreateSession(ctx, SessionSpec{
		ID:        sessionID,
		Kind:      SessionKindDKG,
		KeyID:     "offline-key",
		Parties:   parties,
		Threshold: 1,
	}); err != nil {
		t.Fatal(err)
	}
	request := DKGRequest{
		SessionID: sessionID,
		KeyID:     "offline-key",
		Parties:   parties,
		Threshold: 1,
	}
	started := time.Now()
	var wg sync.WaitGroup
	errorsByParty := make(chan error, 2)
	for _, id := range []string{"alice", "bob"} {
		id := id
		wg.Add(1)
		go func() {
			defer wg.Done()
			runCtx, runCancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
			defer runCancel()
			_, err := harness.participants[id].RunDKG(runCtx, request)
			errorsByParty <- err
		}()
	}
	wg.Wait()
	close(errorsByParty)
	for err := range errorsByParty {
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("offline participant error=%v, want deadline exceeded", err)
		}
	}
	if elapsed := time.Since(started); elapsed < 300*time.Millisecond || elapsed > 2*time.Second {
		t.Fatalf("offline protocol elapsed %v, expected bounded deadline", elapsed)
	}
	for _, id := range []string{"alice", "bob"} {
		if _, err := os.Stat(harness.stores[id].Path()); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s persisted a partial DKG share: %v", id, err)
		}
	}
}

func TestCoordinatorRejectsWrongRouteAndDeduplicatesReplay(t *testing.T) {
	harness := newClusterHarness(t, 5*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sessionID, err := NewSessionID()
	if err != nil {
		t.Fatal(err)
	}
	parties := party.IDSlice{"alice", "bob", "carol"}
	if _, err := harness.coordinatorClient.CreateSession(ctx, SessionSpec{
		ID:        sessionID,
		Kind:      SessionKindDKG,
		KeyID:     "routing-key",
		Parties:   []string{"alice", "bob", "carol"},
		Threshold: 1,
	}); err != nil {
		t.Fatal(err)
	}
	sessionBytes, err := hex.DecodeString(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := protocol.NewMultiHandler(
		upstreamfrost.KeygenTaproot("alice", parties, 1),
		sessionBytes,
	)
	if err != nil {
		t.Fatal(err)
	}
	var message *protocol.Message
	select {
	case message = <-handler.Listen():
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	raw, err := message.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	aliceRelay, err := NewRelayClient(RelayClientConfig{
		BaseURL: harness.coordinatorServer.URL,
		PartyID: "alice",
		Token:   harness.partyTokens["alice"],
	})
	if err != nil {
		t.Fatal(err)
	}
	if status, err := aliceRelay.Send(ctx, sessionID, raw); err != nil || status != "accepted" {
		t.Fatalf("first delivery status=%q err=%v", status, err)
	}
	if status, err := aliceRelay.Send(ctx, sessionID, raw); err != nil || status != "duplicate" {
		t.Fatalf("replay status=%q err=%v", status, err)
	}
	bobRelay, err := NewRelayClient(RelayClientConfig{
		BaseURL: harness.coordinatorServer.URL,
		PartyID: "bob",
		Token:   harness.partyTokens["bob"],
	})
	if err != nil {
		t.Fatal(err)
	}
	delivered, err := bobRelay.Receive(ctx, sessionID, 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(delivered, raw) {
		t.Fatal("coordinator changed Taurus binary message")
	}
	duplicate, err := bobRelay.Receive(ctx, sessionID, 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if duplicate != nil {
		t.Fatal("replayed message was enqueued twice")
	}

	if _, err := bobRelay.Send(ctx, sessionID, raw); err == nil || httpStatus(err) != http.StatusForbidden {
		t.Fatalf("sender impersonation error=%v, want HTTP 403", err)
	}
	wrongRoute := *message
	wrongRoute.Broadcast = false
	wrongRoute.To = "mallory"
	wrongRaw, err := wrongRoute.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := aliceRelay.Send(ctx, sessionID, wrongRaw); err == nil || httpStatus(err) != http.StatusBadRequest {
		t.Fatalf("wrong recipient error=%v, want HTTP 400", err)
	}
	wrongProtocol := *message
	wrongProtocol.Protocol = TaprootSignProtocol
	wrongRaw, err = wrongProtocol.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := aliceRelay.Send(ctx, sessionID, wrongRaw); err == nil || httpStatus(err) != http.StatusBadRequest {
		t.Fatalf("wrong protocol error=%v, want HTTP 400", err)
	}
	wrongSSID := *message
	wrongSSID.SSID = append([]byte(nil), message.SSID...)
	wrongSSID.SSID[0] ^= 1
	wrongRaw, err = wrongSSID.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := aliceRelay.Send(ctx, sessionID, wrongRaw); err == nil || httpStatus(err) != http.StatusBadRequest {
		t.Fatalf("wrong SSID error=%v, want HTTP 400", err)
	}
}

func TestAuthenticatorPrefersVerifiedMTLSAndRestrictsTokensToLoopback(t *testing.T) {
	authenticator := Authenticator{
		LoopbackTokens: map[string]string{"alice": strings.Repeat("a", 32)},
	}
	request := httptest.NewRequest(http.MethodGet, "https://cluster.test", nil)
	request.RemoteAddr = "203.0.113.9:1234"
	request.TLS = &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{{Subject: pkix.Name{CommonName: "alice"}}},
		VerifiedChains:   [][]*x509.Certificate{{{Subject: pkix.Name{CommonName: "alice"}}}},
	}
	identity, err := authenticator.Authenticate(request)
	if err != nil {
		t.Fatal(err)
	}
	if identity.Name != "alice" || identity.Method != AuthMethodMTLS {
		t.Fatalf("mTLS identity=%+v", identity)
	}

	request = httptest.NewRequest(http.MethodGet, "http://cluster.test", nil)
	request.RemoteAddr = "203.0.113.9:1234"
	request.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 32))
	if _, err := authenticator.Authenticate(request); err == nil {
		t.Fatal("non-loopback bearer token unexpectedly authenticated")
	}
	request.RemoteAddr = "127.0.0.1:1234"
	identity, err = authenticator.Authenticate(request)
	if err != nil {
		t.Fatal(err)
	}
	if identity.Method != AuthMethodLoopbackToken {
		t.Fatalf("token identity method=%q", identity.Method)
	}
}

func TestHTTPServicesEnforceRequestBodyLimit(t *testing.T) {
	tokens := map[string]string{testAdminIdentity: testAdminToken}
	coordinator, err := NewCoordinator(CoordinatorConfig{
		Authenticator:   Authenticator{LoopbackTokens: tokens},
		AdminIdentities: map[string]bool{testAdminIdentity: true},
		MaxBodyBytes:    1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	coordinatorServer := httptest.NewServer(coordinator.Handler())
	defer coordinatorServer.Close()
	assertOversizedRequest(t, coordinatorServer.URL+"/v1/sessions")

	store, err := NewShareStore(filepath.Join(t.TempDir(), "private", "share.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	relay, err := NewRelayClient(RelayClientConfig{
		BaseURL: coordinatorServer.URL,
		PartyID: "alice",
		Token:   testAdminToken,
	})
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := OpenSessionLedger(filepath.Join(t.TempDir(), "ledger", "sessions.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	participant, err := NewParticipant(ParticipantConfig{
		PartyID:         "alice",
		Store:           store,
		Relay:           relay,
		Ledger:          ledger,
		Authenticator:   Authenticator{LoopbackTokens: tokens},
		AdminIdentities: map[string]bool{testAdminIdentity: true},
		MaxBodyBytes:    1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	participantServer := httptest.NewServer(participant.Handler())
	defer participantServer.Close()
	assertOversizedRequest(t, participantServer.URL+"/v1/dkg")
}

func assertOversizedRequest(t *testing.T, endpoint string) {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(make([]byte, 2048)))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+testAdminToken)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusRequestEntityTooLarge {
		raw, _ := io.ReadAll(response.Body)
		t.Fatalf("%s status=%d body=%s", endpoint, response.StatusCode, raw)
	}
}

func runConcurrent(t *testing.T, ids []string, operation func(string) error) {
	t.Helper()
	var wg sync.WaitGroup
	errs := make(chan error, len(ids))
	for _, id := range ids {
		id := id
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := operation(id); err != nil {
				errs <- fmt.Errorf("%s: %w", id, err)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if t.Failed() {
		t.FailNow()
	}
}

func postJSON(ctx context.Context, endpoint, token string, requestBody, responseBody any) error {
	body, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, DefaultMaxBodyBytes+1))
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(raw)))
	}
	return decodeJSONStrict(raw, responseBody)
}

func httpStatus(err error) int {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode
	}
	return 0
}
