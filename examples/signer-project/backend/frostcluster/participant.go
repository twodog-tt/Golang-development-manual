package frostcluster

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/taurusgroup/multi-party-sig/pkg/party"
	"github.com/taurusgroup/multi-party-sig/pkg/protocol"
	"github.com/taurusgroup/multi-party-sig/pkg/taproot"
	upstreamfrost "github.com/taurusgroup/multi-party-sig/protocols/frost"
)

type ParticipantConfig struct {
	PartyID         string
	Store           *ShareStore
	Relay           *RelayClient
	Ledger          *SessionLedger
	Authenticator   Authenticator
	AdminIdentities map[string]bool
	MaxBodyBytes    int64
	ProtocolTimeout time.Duration
}

// Participant owns one ShareStore and constructs Taurus handlers only for its
// configured PartyID. There is deliberately no API for importing another
// participant's TaprootConfig.
type Participant struct {
	config  ParticipantConfig
	slot    chan struct{}
	handler http.Handler
}

var ErrSessionReplay = errors.New("participant session ID was already durably reserved")

func NewParticipant(config ParticipantConfig) (*Participant, error) {
	if !validIdentifier(config.PartyID) {
		return nil, fmt.Errorf("invalid participant ID %q", config.PartyID)
	}
	if config.Store == nil {
		return nil, errors.New("participant share store is required")
	}
	if config.Relay == nil {
		return nil, errors.New("participant relay client is required")
	}
	if config.Ledger == nil {
		return nil, errors.New("participant session ledger is required")
	}
	if config.Relay.PartyID() != config.PartyID {
		return nil, errors.New("participant and relay identities disagree")
	}
	if len(config.AdminIdentities) == 0 {
		return nil, errors.New("at least one participant admin identity is required")
	}
	if config.MaxBodyBytes == 0 {
		config.MaxBodyBytes = DefaultMaxBodyBytes
	}
	if config.MaxBodyBytes < 1024 {
		return nil, errors.New("participant max body size is too small")
	}
	if config.ProtocolTimeout == 0 {
		config.ProtocolTimeout = DefaultProtocolTimeout
	}
	if config.ProtocolTimeout <= 0 {
		return nil, errors.New("participant protocol timeout must be positive")
	}
	participant := &Participant{
		config: config,
		slot:   make(chan struct{}, 1),
	}
	participant.slot <- struct{}{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", participant.handleHealth)
	mux.HandleFunc("POST /v1/dkg", participant.handleDKG)
	mux.HandleFunc("POST /v1/sign", participant.handleSign)
	participant.handler = securityHeaders(mux)
	return participant, nil
}

func (p *Participant) Handler() http.Handler {
	return p.handler
}

func (p *Participant) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":   "ok",
		"party_id": p.config.PartyID,
	})
}

func (p *Participant) handleDKG(w http.ResponseWriter, r *http.Request) {
	if !p.requireAdmin(w, r) {
		return
	}
	var request DKGRequest
	if err := decodeRequestJSON(w, r, p.config.MaxBodyBytes, &request); err != nil {
		writeJSON(w, requestErrorStatus(err), errorResponse{Error: err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), p.config.ProtocolTimeout)
	defer cancel()
	response, err := p.RunDKG(ctx, request)
	if err != nil {
		writeParticipantError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (p *Participant) handleSign(w http.ResponseWriter, r *http.Request) {
	if !p.requireAdmin(w, r) {
		return
	}
	var request SignRequest
	if err := decodeRequestJSON(w, r, p.config.MaxBodyBytes, &request); err != nil {
		writeJSON(w, requestErrorStatus(err), errorResponse{Error: err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), p.config.ProtocolTimeout)
	defer cancel()
	response, err := p.RunSign(ctx, request)
	if err != nil {
		writeParticipantError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (p *Participant) RunDKG(ctx context.Context, request DKGRequest) (DKGResponse, error) {
	parties, err := canonicalPartyIDs(request.Parties)
	if err != nil {
		return DKGResponse{}, err
	}
	if _, err := (SessionSpec{ID: request.SessionID}).sessionBytes(); err != nil {
		return DKGResponse{}, err
	}
	if !validIdentifier(request.KeyID) {
		return DKGResponse{}, fmt.Errorf("invalid key ID %q", request.KeyID)
	}
	if request.Threshold < 0 || request.Threshold >= len(parties) {
		return DKGResponse{}, errors.New("DKG threshold is invalid for party set")
	}
	if !parties.Contains(party.ID(p.config.PartyID)) {
		return DKGResponse{}, errors.New("local participant is absent from DKG party set")
	}
	exists, err := p.config.Store.Exists()
	if err != nil {
		return DKGResponse{}, err
	}
	if exists {
		return DKGResponse{}, ErrShareExists
	}
	if err := p.reserveSession(request.SessionID, SessionKindDKG); err != nil {
		return DKGResponse{}, err
	}
	release, err := p.acquire(ctx)
	if err != nil {
		return DKGResponse{}, err
	}
	defer release()

	expected := SessionSpec{
		ID:        request.SessionID,
		Kind:      SessionKindDKG,
		KeyID:     request.KeyID,
		Parties:   partyStrings(parties),
		Threshold: request.Threshold,
	}
	spec, err := p.config.Relay.GetSession(ctx, request.SessionID)
	if err != nil {
		return DKGResponse{}, err
	}
	if err := matchSession(expected, spec); err != nil {
		return DKGResponse{}, err
	}
	sessionBytes, err := spec.sessionBytes()
	if err != nil {
		return DKGResponse{}, err
	}
	handler, err := protocol.NewMultiHandler(
		upstreamfrost.KeygenTaproot(party.ID(p.config.PartyID), parties, request.Threshold),
		sessionBytes,
	)
	if err != nil {
		return DKGResponse{}, fmt.Errorf("start Taproot DKG: %w", err)
	}
	result, err := p.runProtocol(ctx, spec, handler)
	if err != nil {
		return DKGResponse{}, fmt.Errorf("run Taproot DKG: %w", err)
	}
	config, ok := result.(*upstreamfrost.TaprootConfig)
	if !ok || config == nil {
		return DKGResponse{}, fmt.Errorf("Taproot DKG returned %T", result)
	}
	if err := validateDKGResult(config, parties, request.Threshold, p.config.PartyID); err != nil {
		return DKGResponse{}, err
	}
	if err := p.config.Store.SaveNew(request.KeyID, config); err != nil {
		return DKGResponse{}, err
	}
	return DKGResponse{
		SessionID: request.SessionID,
		KeyID:     request.KeyID,
		PartyID:   p.config.PartyID,
		PublicKey: hex.EncodeToString(config.PublicKey),
		Threshold: config.Threshold,
	}, nil
}

func (p *Participant) RunSign(ctx context.Context, request SignRequest) (SignResponse, error) {
	signers, err := canonicalPartyIDs(request.Signers)
	if err != nil {
		return SignResponse{}, err
	}
	if _, err := (SessionSpec{ID: request.SessionID}).sessionBytes(); err != nil {
		return SignResponse{}, err
	}
	if !validIdentifier(request.KeyID) {
		return SignResponse{}, fmt.Errorf("invalid key ID %q", request.KeyID)
	}
	if !signers.Contains(party.ID(p.config.PartyID)) {
		return SignResponse{}, errors.New("local participant is absent from signer set")
	}
	digest, err := hex.DecodeString(request.DigestHex)
	if err != nil || len(digest) != 32 || request.DigestHex != hex.EncodeToString(digest) {
		return SignResponse{}, errors.New("signing digest must be canonical 32-byte lowercase hexadecimal")
	}
	if err := p.reserveSession(request.SessionID, SessionKindSign); err != nil {
		return SignResponse{}, err
	}
	release, err := p.acquire(ctx)
	if err != nil {
		return SignResponse{}, err
	}
	defer release()

	config, err := p.config.Store.Load(request.KeyID)
	if err != nil {
		return SignResponse{}, err
	}
	if config.ID != party.ID(p.config.PartyID) {
		return SignResponse{}, errors.New("loaded share belongs to a different participant")
	}
	if len(signers) < config.Threshold+1 {
		return SignResponse{}, errors.New("signer count is below threshold+1")
	}
	for _, signer := range signers {
		if _, ok := config.VerificationShares[signer]; !ok {
			return SignResponse{}, fmt.Errorf("signer %q is not part of the DKG consortium", signer)
		}
	}
	expected := SessionSpec{
		ID:        request.SessionID,
		Kind:      SessionKindSign,
		KeyID:     request.KeyID,
		Parties:   partyStrings(signers),
		Threshold: config.Threshold,
		DigestHex: request.DigestHex,
	}
	spec, err := p.config.Relay.GetSession(ctx, request.SessionID)
	if err != nil {
		return SignResponse{}, err
	}
	if err := matchSession(expected, spec); err != nil {
		return SignResponse{}, err
	}
	sessionBytes, err := spec.sessionBytes()
	if err != nil {
		return SignResponse{}, err
	}
	handler, err := protocol.NewMultiHandler(
		upstreamfrost.SignTaproot(config, signers, digest),
		sessionBytes,
	)
	if err != nil {
		return SignResponse{}, fmt.Errorf("start Taproot signing: %w", err)
	}
	result, err := p.runProtocol(ctx, spec, handler)
	if err != nil {
		return SignResponse{}, fmt.Errorf("run Taproot signing: %w", err)
	}
	signature, ok := result.(taproot.Signature)
	if !ok {
		return SignResponse{}, fmt.Errorf("Taproot signing returned %T", result)
	}
	if len(signature) != taproot.SignatureLen || !config.PublicKey.Verify(signature, digest) {
		return SignResponse{}, errors.New("participant rejected invalid BIP-340 signature result")
	}
	return SignResponse{
		SessionID: request.SessionID,
		KeyID:     request.KeyID,
		PartyID:   p.config.PartyID,
		PublicKey: hex.EncodeToString(config.PublicKey),
		Signature: hex.EncodeToString(signature),
	}, nil
}

type protocolInput struct {
	raw []byte
	err error
}

func (p *Participant) runProtocol(ctx context.Context, spec SessionSpec, handler *protocol.MultiHandler) (any, error) {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	incoming := make(chan protocolInput, 1)
	go p.pollProtocol(runCtx, spec.ID, incoming)

	seen := make(map[[32]byte]struct{})
	var expectedSSID []byte
	for {
		select {
		case outgoing, open := <-handler.Listen():
			if !open {
				return handler.Result()
			}
			if err := validateOutboundMessage(outgoing, spec, p.config.PartyID, &expectedSSID); err != nil {
				return nil, err
			}
			raw, err := outgoing.MarshalBinary()
			if err != nil {
				return nil, fmt.Errorf("marshal Taurus protocol message: %w", err)
			}
			if _, err := p.config.Relay.Send(runCtx, spec.ID, raw); err != nil {
				return nil, err
			}
		case input := <-incoming:
			if input.err != nil {
				return nil, input.err
			}
			message, canonical, err := decodeProtocolMessage(input.raw)
			if err != nil {
				return nil, err
			}
			if !bytes.Equal(input.raw, canonical) {
				return nil, errors.New("received non-canonical Taurus protocol message")
			}
			digest := sha256.Sum256(input.raw)
			if _, duplicate := seen[digest]; duplicate {
				continue
			}
			if len(expectedSSID) == 0 {
				if !handler.CanAccept(message) {
					return nil, errors.New("Taurus handler rejected initial routed session/protocol/party metadata")
				}
				expectedSSID = append([]byte(nil), message.SSID...)
			}
			if err := validateInboundMessage(message, spec, p.config.PartyID, expectedSSID); err != nil {
				return nil, err
			}
			if !handler.CanAccept(message) {
				return nil, errors.New("Taurus handler rejected routed session/protocol/party metadata")
			}
			seen[digest] = struct{}{}
			handler.Accept(message)
		case <-runCtx.Done():
			return nil, runCtx.Err()
		}
	}
}

func (p *Participant) pollProtocol(ctx context.Context, sessionID string, incoming chan<- protocolInput) {
	for {
		raw, err := p.config.Relay.Receive(ctx, sessionID, 250*time.Millisecond)
		if err != nil {
			select {
			case incoming <- protocolInput{err: err}:
			case <-ctx.Done():
			}
			return
		}
		if raw == nil {
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}
		select {
		case incoming <- protocolInput{raw: raw}:
		case <-ctx.Done():
			return
		}
	}
}

func validateOutboundMessage(message *protocol.Message, spec SessionSpec, localParty string, expectedSSID *[]byte) error {
	if message == nil {
		return errors.New("Taurus emitted a nil protocol message")
	}
	if string(message.From) != localParty {
		return errors.New("Taurus emitted a message for the wrong local party")
	}
	if message.Protocol != spec.protocolID() {
		return errors.New("Taurus emitted a message for the wrong protocol")
	}
	if len(message.SSID) != 64 {
		return errors.New("Taurus emitted an invalid SSID")
	}
	if len(*expectedSSID) == 0 {
		*expectedSSID = append([]byte(nil), message.SSID...)
	} else if !bytes.Equal(*expectedSSID, message.SSID) {
		return errors.New("Taurus emitted inconsistent SSIDs")
	}
	if message.To != "" && !spec.contains(string(message.To)) {
		return errors.New("Taurus emitted a message for a party outside the session")
	}
	if message.To == party.ID(localParty) {
		return errors.New("Taurus emitted a message to itself")
	}
	return nil
}

func validateInboundMessage(message *protocol.Message, spec SessionSpec, localParty string, expectedSSID []byte) error {
	if message.Protocol != spec.protocolID() {
		return errors.New("routed message has wrong protocol")
	}
	if !spec.contains(string(message.From)) || message.From == party.ID(localParty) {
		return errors.New("routed message has invalid sender")
	}
	if message.To != "" && message.To != party.ID(localParty) {
		return errors.New("routed message has wrong recipient")
	}
	if len(expectedSSID) == 0 || !bytes.Equal(message.SSID, expectedSSID) {
		return errors.New("routed message has wrong session SSID")
	}
	if message.Data == nil {
		return errors.New("routed message has nil data")
	}
	return nil
}

func validateDKGResult(config *upstreamfrost.TaprootConfig, parties party.IDSlice, threshold int, localParty string) error {
	if config.ID != party.ID(localParty) {
		return errors.New("DKG returned another participant's share")
	}
	if config.Threshold != threshold {
		return errors.New("DKG returned an unexpected threshold")
	}
	if len(config.VerificationShares) != len(parties) {
		return errors.New("DKG returned an incomplete verification-share set")
	}
	for _, id := range parties {
		if _, ok := config.VerificationShares[id]; !ok {
			return fmt.Errorf("DKG result is missing verification share %q", id)
		}
	}
	_, err := encodeTaprootConfig(config)
	return err
}

func matchSession(expected, actual SessionSpec) error {
	expected.ExpiresAt = actual.ExpiresAt
	if !expected.equal(actual) {
		return errors.New("control request does not match coordinator session metadata")
	}
	return nil
}

func canonicalPartyIDs(values []string) (party.IDSlice, error) {
	if len(values) < 2 {
		return nil, errors.New("at least two parties are required")
	}
	rawIDs := make([]party.ID, len(values))
	for i, value := range values {
		if !validIdentifier(value) {
			return nil, fmt.Errorf("invalid party ID %q", value)
		}
		rawIDs[i] = party.ID(value)
	}
	ids := party.NewIDSlice(rawIDs)
	if !ids.Valid() || len(ids) != len(values) {
		return nil, errors.New("party IDs must be unique")
	}
	return ids, nil
}

func partyStrings(ids party.IDSlice) []string {
	result := make([]string, len(ids))
	for i, id := range ids {
		result[i] = string(id)
	}
	return result
}

func (p *Participant) acquire(ctx context.Context) (func(), error) {
	select {
	case <-p.slot:
		return func() { p.slot <- struct{}{} }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *Participant) reserveSession(sessionID string, kind SessionKind) error {
	return p.config.Ledger.Reserve(sessionID, kind)
}

func (p *Participant) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	identity, err := p.config.Authenticator.Authenticate(r)
	if err != nil {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: err.Error()})
		return false
	}
	if !p.config.AdminIdentities[identity.Name] {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "participant admin identity is required"})
		return false
	}
	return true
}

func writeParticipantError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		status = http.StatusGatewayTimeout
	case errors.Is(err, context.Canceled):
		status = http.StatusRequestTimeout
	case errors.Is(err, ErrShareExists):
		status = http.StatusConflict
	case errors.Is(err, ErrSessionReplay):
		status = http.StatusConflict
	default:
		var httpErr *HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode >= 500 {
			status = http.StatusBadGateway
		}
	}
	writeJSON(w, status, errorResponse{Error: err.Error()})
}
