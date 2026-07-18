package frostcluster

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/taurusgroup/multi-party-sig/pkg/protocol"
)

type CoordinatorConfig struct {
	Authenticator      Authenticator
	AdminIdentities    map[string]bool
	MaxBodyBytes       int64
	MaxQueueMessages   int
	MaxSessionMessages int
	SessionTTL         time.Duration
	Now                func() time.Time
}

type queuedMessage struct {
	id   string
	data []byte
}

type coordinatorSession struct {
	spec   SessionSpec
	ssid   []byte
	seen   map[[32]byte]struct{}
	inbox  map[string][]queuedMessage
	notify map[string]chan struct{}
}

// Coordinator is an authenticated, bounded, replay-deduplicating router. Its
// state contains only public SessionSpec values and marshaled protocol.Message
// bytes; it has no API that accepts or returns a TaprootConfig/private share.
//
// Queue state is intentionally in-memory. A coordinator restart aborts active
// ceremonies; participants must start a fresh session ID rather than trying to
// resume a nonce-bearing signing protocol.
type Coordinator struct {
	config   CoordinatorConfig
	mu       sync.Mutex
	sessions map[string]*coordinatorSession
	handler  http.Handler
}

func NewCoordinator(config CoordinatorConfig) (*Coordinator, error) {
	if len(config.AdminIdentities) == 0 {
		return nil, errors.New("at least one coordinator admin identity is required")
	}
	if config.MaxBodyBytes == 0 {
		config.MaxBodyBytes = DefaultMaxBodyBytes
	}
	if config.MaxBodyBytes < 1024 {
		return nil, errors.New("coordinator max body size is too small")
	}
	if config.MaxQueueMessages == 0 {
		config.MaxQueueMessages = DefaultMaxQueueMessages
	}
	if config.MaxQueueMessages < 1 {
		return nil, errors.New("coordinator max queue size must be positive")
	}
	if config.MaxSessionMessages == 0 {
		config.MaxSessionMessages = DefaultMaxSessionMessages
	}
	if config.MaxSessionMessages < 1 {
		return nil, errors.New("coordinator max session message count must be positive")
	}
	if config.SessionTTL == 0 {
		config.SessionTTL = DefaultSessionTTL
	}
	if config.SessionTTL <= 0 {
		return nil, errors.New("coordinator session TTL must be positive")
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	coordinator := &Coordinator{
		config:   config,
		sessions: make(map[string]*coordinatorSession),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", coordinator.handleHealth)
	mux.HandleFunc("POST /v1/sessions", coordinator.handleCreateSession)
	mux.HandleFunc("GET /v1/sessions/{sessionID}", coordinator.handleGetSession)
	mux.HandleFunc("POST /v1/sessions/{sessionID}/messages", coordinator.handleMessage)
	mux.HandleFunc("GET /v1/sessions/{sessionID}/inbox/{partyID}", coordinator.handleInbox)
	coordinator.handler = securityHeaders(mux)
	return coordinator, nil
}

func (c *Coordinator) Handler() http.Handler {
	return c.handler
}

func (c *Coordinator) CreateSession(spec SessionSpec) (SessionSpec, error) {
	now := c.config.Now().UTC()
	if spec.ExpiresAt.IsZero() {
		spec.ExpiresAt = now.Add(c.config.SessionTTL)
	} else {
		spec.ExpiresAt = spec.ExpiresAt.UTC()
	}
	if err := spec.Validate(now); err != nil {
		return SessionSpec{}, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeExpiredLocked(now)
	if existing, ok := c.sessions[spec.ID]; ok {
		if existing.spec.equal(spec) {
			return existing.spec, nil
		}
		return SessionSpec{}, errors.New("session ID is already registered with different metadata")
	}
	inbox := make(map[string][]queuedMessage, len(spec.Parties))
	notify := make(map[string]chan struct{}, len(spec.Parties))
	for _, id := range spec.Parties {
		inbox[id] = nil
		notify[id] = make(chan struct{}, 1)
	}
	c.sessions[spec.ID] = &coordinatorSession{
		spec:   spec,
		seen:   make(map[[32]byte]struct{}),
		inbox:  inbox,
		notify: notify,
	}
	return spec, nil
}

func (c *Coordinator) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (c *Coordinator) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	identity, ok := c.requireIdentity(w, r)
	if !ok {
		return
	}
	if !c.config.AdminIdentities[identity.Name] {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "coordinator admin identity is required"})
		return
	}
	var spec SessionSpec
	if err := decodeRequestJSON(w, r, c.config.MaxBodyBytes, &spec); err != nil {
		writeJSON(w, requestErrorStatus(err), errorResponse{Error: err.Error()})
		return
	}
	created, err := c.CreateSession(spec)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (c *Coordinator) handleGetSession(w http.ResponseWriter, r *http.Request) {
	identity, ok := c.requireIdentity(w, r)
	if !ok {
		return
	}
	spec, status, err := c.lookupSession(r.PathValue("sessionID"))
	if err != nil {
		writeJSON(w, status, errorResponse{Error: err.Error()})
		return
	}
	if !spec.contains(identity.Name) && !c.config.AdminIdentities[identity.Name] {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "identity is not a session member"})
		return
	}
	writeJSON(w, http.StatusOK, spec)
}

func (c *Coordinator) handleMessage(w http.ResponseWriter, r *http.Request) {
	identity, ok := c.requireIdentity(w, r)
	if !ok {
		return
	}
	raw, err := readRequestBody(w, r, c.config.MaxBodyBytes)
	if err != nil {
		writeJSON(w, requestErrorStatus(err), errorResponse{Error: err.Error()})
		return
	}
	status, err := c.routeMessage(r.PathValue("sessionID"), identity.Name, raw)
	if err != nil {
		var routeErr *routeError
		if errors.As(err, &routeErr) {
			writeJSON(w, routeErr.status, errorResponse{Error: routeErr.Error()})
			return
		}
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, deliveryResult{Status: status})
}

func (c *Coordinator) handleInbox(w http.ResponseWriter, r *http.Request) {
	identity, ok := c.requireIdentity(w, r)
	if !ok {
		return
	}
	partyID := r.PathValue("partyID")
	if identity.Name != partyID {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "identity may only read its own inbox"})
		return
	}
	wait, err := parseWait(r.URL.Query().Get("wait_ms"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	message, status, err := c.receive(r.Context(), r.PathValue("sessionID"), partyID, wait)
	if err != nil {
		writeJSON(w, status, errorResponse{Error: err.Error()})
		return
	}
	if message == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Frost-Message-ID", message.id)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(message.data)
}

func (c *Coordinator) routeMessage(sessionID, authenticatedParty string, raw []byte) (string, error) {
	message, canonical, err := decodeProtocolMessage(raw)
	if err != nil {
		return "", newRouteError(http.StatusBadRequest, err)
	}
	if !bytes.Equal(raw, canonical) {
		return "", newRouteError(http.StatusBadRequest, errors.New("protocol message is not in canonical Taurus binary form"))
	}
	if string(message.From) != authenticatedParty {
		return "", newRouteError(http.StatusForbidden, errors.New("authenticated identity does not match message sender"))
	}

	now := c.config.Now().UTC()
	c.mu.Lock()
	defer c.mu.Unlock()
	session, ok := c.sessions[sessionID]
	if !ok {
		return "", newRouteError(http.StatusNotFound, errors.New("session not found"))
	}
	if !session.spec.ExpiresAt.After(now) {
		delete(c.sessions, sessionID)
		return "", newRouteError(http.StatusGone, errors.New("session expired"))
	}
	if !session.spec.contains(authenticatedParty) {
		return "", newRouteError(http.StatusForbidden, errors.New("sender is not a session member"))
	}
	if message.Protocol != session.spec.protocolID() {
		return "", newRouteError(http.StatusBadRequest, errors.New("message protocol does not match session"))
	}
	if len(message.SSID) != 64 {
		return "", newRouteError(http.StatusBadRequest, errors.New("message SSID must be 64 bytes"))
	}
	if len(session.ssid) == 0 {
		session.ssid = append([]byte(nil), message.SSID...)
	} else if !bytes.Equal(session.ssid, message.SSID) {
		return "", newRouteError(http.StatusBadRequest, errors.New("message SSID does not match session"))
	}
	if message.Data == nil {
		return "", newRouteError(http.StatusBadRequest, errors.New("message data is nil"))
	}
	if message.RoundNumber > 3 {
		return "", newRouteError(http.StatusBadRequest, errors.New("message round is outside the FROST protocol"))
	}
	if message.Broadcast && message.To != "" {
		return "", newRouteError(http.StatusBadRequest, errors.New("broadcast message must not name one recipient"))
	}

	var recipients []string
	if message.To == "" {
		for _, id := range session.spec.Parties {
			if id != authenticatedParty {
				recipients = append(recipients, id)
			}
		}
	} else {
		recipient := string(message.To)
		if recipient == authenticatedParty || !session.spec.contains(recipient) {
			return "", newRouteError(http.StatusBadRequest, errors.New("message recipient is invalid for session"))
		}
		recipients = []string{recipient}
	}
	if len(recipients) == 0 {
		return "", newRouteError(http.StatusBadRequest, errors.New("message has no valid recipient"))
	}

	digest := sha256.Sum256(raw)
	if _, duplicate := session.seen[digest]; duplicate {
		return "duplicate", nil
	}
	if len(session.seen) >= c.config.MaxSessionMessages {
		return "", newRouteError(http.StatusTooManyRequests, errors.New("session message limit reached"))
	}
	for _, recipient := range recipients {
		if len(session.inbox[recipient]) >= c.config.MaxQueueMessages {
			return "", newRouteError(http.StatusServiceUnavailable, fmt.Errorf("recipient %q queue is full", recipient))
		}
	}
	id := hex.EncodeToString(digest[:])
	for _, recipient := range recipients {
		session.inbox[recipient] = append(session.inbox[recipient], queuedMessage{
			id:   id,
			data: append([]byte(nil), raw...),
		})
		select {
		case session.notify[recipient] <- struct{}{}:
		default:
		}
	}
	session.seen[digest] = struct{}{}
	return "accepted", nil
}

func (c *Coordinator) receive(ctx context.Context, sessionID, partyID string, wait time.Duration) (*queuedMessage, int, error) {
	deadline := time.NewTimer(wait)
	defer deadline.Stop()
	for {
		now := c.config.Now().UTC()
		c.mu.Lock()
		session, ok := c.sessions[sessionID]
		if !ok {
			c.mu.Unlock()
			return nil, http.StatusNotFound, errors.New("session not found")
		}
		if !session.spec.ExpiresAt.After(now) {
			delete(c.sessions, sessionID)
			c.mu.Unlock()
			return nil, http.StatusGone, errors.New("session expired")
		}
		queue, member := session.inbox[partyID]
		if !member {
			c.mu.Unlock()
			return nil, http.StatusForbidden, errors.New("party is not a session member")
		}
		if len(queue) > 0 {
			message := queue[0]
			session.inbox[partyID] = queue[1:]
			c.mu.Unlock()
			return &message, http.StatusOK, nil
		}
		notify := session.notify[partyID]
		c.mu.Unlock()

		select {
		case <-notify:
		case <-deadline.C:
			return nil, http.StatusNoContent, nil
		case <-ctx.Done():
			return nil, http.StatusRequestTimeout, errors.New("request canceled")
		}
	}
}

func (c *Coordinator) lookupSession(sessionID string) (SessionSpec, int, error) {
	now := c.config.Now().UTC()
	c.mu.Lock()
	defer c.mu.Unlock()
	session, ok := c.sessions[sessionID]
	if !ok {
		return SessionSpec{}, http.StatusNotFound, errors.New("session not found")
	}
	if !session.spec.ExpiresAt.After(now) {
		delete(c.sessions, sessionID)
		return SessionSpec{}, http.StatusGone, errors.New("session expired")
	}
	return session.spec, http.StatusOK, nil
}

func (c *Coordinator) removeExpiredLocked(now time.Time) {
	for id, session := range c.sessions {
		if !session.spec.ExpiresAt.After(now) {
			delete(c.sessions, id)
		}
	}
}

func (c *Coordinator) requireIdentity(w http.ResponseWriter, r *http.Request) (Identity, bool) {
	identity, err := c.config.Authenticator.Authenticate(r)
	if err != nil {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: err.Error()})
		return Identity{}, false
	}
	return identity, true
}

func decodeProtocolMessage(raw []byte) (*protocol.Message, []byte, error) {
	if len(raw) == 0 {
		return nil, nil, errors.New("protocol message body is empty")
	}
	message := new(protocol.Message)
	// The pinned Taurus release's Message.UnmarshalBinary mistakenly returns
	// nil when CBOR decoding fails. Decode with a strict fxamacker mode instead
	// of relying on that method, then require the upstream marshal form below.
	if err := protocolMessageDecMode.Unmarshal(raw, message); err != nil {
		return nil, nil, fmt.Errorf("decode Taurus protocol message CBOR: %w", err)
	}
	canonical, err := message.MarshalBinary()
	if err != nil {
		return nil, nil, fmt.Errorf("remarshal Taurus protocol message: %w", err)
	}
	return message, canonical, nil
}

var protocolMessageDecMode = mustProtocolMessageDecMode()

func mustProtocolMessageDecMode() cbor.DecMode {
	mode, err := (cbor.DecOptions{
		DupMapKey:         cbor.DupMapKeyEnforcedAPF,
		MaxNestedLevels:   8,
		MaxArrayElements:  16,
		MaxMapPairs:       16,
		IndefLength:       cbor.IndefLengthForbidden,
		TagsMd:            cbor.TagsForbidden,
		ExtraReturnErrors: cbor.ExtraDecErrorUnknownField,
	}).DecMode()
	if err != nil {
		panic(fmt.Sprintf("create strict Taurus CBOR decoder: %v", err))
	}
	return mode
}

func parseWait(raw string) (time.Duration, error) {
	if raw == "" {
		return 2 * time.Second, nil
	}
	milliseconds, err := strconv.Atoi(raw)
	if err != nil || milliseconds < 0 || milliseconds > 30_000 {
		return 0, errors.New("wait_ms must be an integer from 0 through 30000")
	}
	return time.Duration(milliseconds) * time.Millisecond, nil
}

type routeError struct {
	status int
	err    error
}

func newRouteError(status int, err error) *routeError {
	return &routeError{status: status, err: err}
}

func (e *routeError) Error() string {
	return e.err.Error()
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

func decodeRequestJSON(w http.ResponseWriter, r *http.Request, maxBytes int64, target any) error {
	raw, err := readRequestBody(w, r, maxBytes)
	if err != nil {
		return err
	}
	if err := decodeJSONStrict(raw, target); err != nil {
		return fmt.Errorf("decode JSON request: %w", err)
	}
	return nil
}

func readRequestBody(w http.ResponseWriter, r *http.Request, maxBytes int64) ([]byte, error) {
	body := http.MaxBytesReader(w, r.Body, maxBytes)
	defer body.Close()
	raw, err := io.ReadAll(body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return nil, fmt.Errorf("request body exceeds %d bytes", maxBytes)
		}
		return nil, fmt.Errorf("read request body: %w", err)
	}
	return raw, nil
}

func requestErrorStatus(err error) int {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) || bytes.Contains([]byte(err.Error()), []byte("exceeds")) {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusBadRequest
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
