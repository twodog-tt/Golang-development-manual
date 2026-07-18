package frostcluster

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

type RelayClientConfig struct {
	BaseURL         string
	PartyID         string
	Token           string
	HTTPClient      *http.Client
	MaxMessageBytes int64
	SendAttempts    int
}

// RelayClient is owned by one participant and can only read that party's
// inbox when the coordinator enforces its authenticated identity.
type RelayClient struct {
	baseURL         string
	partyID         string
	token           string
	httpClient      *http.Client
	maxMessageBytes int64
	sendAttempts    int
}

func NewRelayClient(config RelayClientConfig) (*RelayClient, error) {
	baseURL, err := validateBaseURL(config.BaseURL)
	if err != nil {
		return nil, err
	}
	if !validIdentifier(config.PartyID) {
		return nil, fmt.Errorf("invalid relay party ID %q", config.PartyID)
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: 35 * time.Second}
	}
	if config.MaxMessageBytes == 0 {
		config.MaxMessageBytes = DefaultMaxBodyBytes
	}
	if config.MaxMessageBytes < 1024 {
		return nil, errors.New("relay max message size is too small")
	}
	if config.SendAttempts == 0 {
		config.SendAttempts = 3
	}
	if config.SendAttempts < 1 || config.SendAttempts > 10 {
		return nil, errors.New("relay send attempts must be between 1 and 10")
	}
	return &RelayClient{
		baseURL:         baseURL,
		partyID:         config.PartyID,
		token:           config.Token,
		httpClient:      config.HTTPClient,
		maxMessageBytes: config.MaxMessageBytes,
		sendAttempts:    config.SendAttempts,
	}, nil
}

func (c *RelayClient) PartyID() string {
	return c.partyID
}

func (c *RelayClient) GetSession(ctx context.Context, sessionID string) (SessionSpec, error) {
	endpoint := c.baseURL + "/v1/sessions/" + url.PathEscape(sessionID)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return SessionSpec{}, err
	}
	c.authorize(request)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return SessionSpec{}, fmt.Errorf("get coordinator session: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return SessionSpec{}, decodeHTTPError(response, c.maxMessageBytes)
	}
	var spec SessionSpec
	if err := decodeResponseJSON(response, c.maxMessageBytes, &spec); err != nil {
		return SessionSpec{}, err
	}
	if err := spec.Validate(time.Now().UTC()); err != nil {
		return SessionSpec{}, fmt.Errorf("coordinator returned invalid session: %w", err)
	}
	if !spec.contains(c.partyID) {
		return SessionSpec{}, errors.New("coordinator session does not contain local party")
	}
	return spec, nil
}

// Send retries ambiguous network/5xx failures. Coordinator-side replay
// deduplication makes an accepted message idempotent across those retries.
func (c *RelayClient) Send(ctx context.Context, sessionID string, raw []byte) (string, error) {
	if int64(len(raw)) > c.maxMessageBytes {
		return "", fmt.Errorf("protocol message exceeds %d bytes", c.maxMessageBytes)
	}
	endpoint := c.baseURL + "/v1/sessions/" + url.PathEscape(sessionID) + "/messages"
	var lastErr error
	for attempt := 1; attempt <= c.sendAttempts; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
		if err != nil {
			return "", err
		}
		request.Header.Set("Content-Type", "application/octet-stream")
		c.authorize(request)
		response, err := c.httpClient.Do(request)
		if err == nil {
			if response.StatusCode == http.StatusOK {
				var result deliveryResult
				decodeErr := decodeResponseJSON(response, c.maxMessageBytes, &result)
				response.Body.Close()
				if decodeErr != nil {
					return "", decodeErr
				}
				if result.Status != "accepted" && result.Status != "duplicate" {
					return "", fmt.Errorf("coordinator returned unknown delivery status %q", result.Status)
				}
				return result.Status, nil
			}
			httpErr := decodeHTTPError(response, c.maxMessageBytes)
			response.Body.Close()
			if response.StatusCode < 500 {
				return "", httpErr
			}
			lastErr = httpErr
		} else {
			lastErr = err
		}
		if attempt < c.sendAttempts {
			delay := time.Duration(attempt*25) * time.Millisecond
			timer := time.NewTimer(delay)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return "", ctx.Err()
			}
		}
	}
	return "", fmt.Errorf("send protocol message after %d attempts: %w", c.sendAttempts, lastErr)
}

func (c *RelayClient) Receive(ctx context.Context, sessionID string, wait time.Duration) ([]byte, error) {
	if wait < 0 || wait > 30*time.Second {
		return nil, errors.New("relay wait must be between zero and 30 seconds")
	}
	waitMillis := strconv.FormatInt(wait.Milliseconds(), 10)
	endpoint := c.baseURL + "/v1/sessions/" + url.PathEscape(sessionID) +
		"/inbox/" + url.PathEscape(c.partyID) + "?wait_ms=" + waitMillis
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	c.authorize(request)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("poll coordinator inbox: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if response.StatusCode != http.StatusOK {
		return nil, decodeHTTPError(response, c.maxMessageBytes)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, c.maxMessageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read coordinator message: %w", err)
	}
	if int64(len(raw)) > c.maxMessageBytes {
		return nil, errors.New("coordinator message exceeds configured limit")
	}
	return raw, nil
}

func (c *RelayClient) authorize(request *http.Request) {
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
}

type CoordinatorClientConfig struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
	MaxBytes   int64
}

type CoordinatorClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
	maxBytes   int64
}

func NewCoordinatorClient(config CoordinatorClientConfig) (*CoordinatorClient, error) {
	baseURL, err := validateBaseURL(config.BaseURL)
	if err != nil {
		return nil, err
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	if config.MaxBytes == 0 {
		config.MaxBytes = DefaultMaxBodyBytes
	}
	return &CoordinatorClient{
		baseURL:    baseURL,
		token:      config.Token,
		httpClient: config.HTTPClient,
		maxBytes:   config.MaxBytes,
	}, nil
}

func (c *CoordinatorClient) CreateSession(ctx context.Context, spec SessionSpec) (SessionSpec, error) {
	body, err := json.Marshal(spec)
	if err != nil {
		return SessionSpec{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/sessions", bytes.NewReader(body))
	if err != nil {
		return SessionSpec{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return SessionSpec{}, fmt.Errorf("create coordinator session: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		return SessionSpec{}, decodeHTTPError(response, c.maxBytes)
	}
	var created SessionSpec
	if err := decodeResponseJSON(response, c.maxBytes, &created); err != nil {
		return SessionSpec{}, err
	}
	return created, nil
}

func validateBaseURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(raw, "/"))
	if err != nil {
		return "", fmt.Errorf("parse base URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("base URL scheme must be http or https")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" {
		return "", errors.New("base URL must not contain credentials, path, query, or fragment")
	}
	if parsed.Host == "" {
		return "", errors.New("base URL host is required")
	}
	if parsed.Scheme == "http" {
		host := parsed.Hostname()
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return "", errors.New("plaintext HTTP is restricted to loopback endpoints")
		}
	}
	return parsed.String(), nil
}

func decodeResponseJSON(response *http.Response, maxBytes int64, target any) error {
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return fmt.Errorf("read HTTP response: %w", err)
	}
	if int64(len(raw)) > maxBytes {
		return errors.New("HTTP response exceeds configured limit")
	}
	if err := decodeJSONStrict(raw, target); err != nil {
		return fmt.Errorf("decode HTTP response: %w", err)
	}
	return nil
}

func decodeHTTPError(response *http.Response, maxBytes int64) error {
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return &HTTPError{StatusCode: response.StatusCode, Message: "unable to read error response"}
	}
	var payload errorResponse
	if int64(len(raw)) <= maxBytes && json.Unmarshal(raw, &payload) == nil && payload.Error != "" {
		return &HTTPError{StatusCode: response.StatusCode, Message: payload.Error}
	}
	message := strings.TrimSpace(string(raw))
	if message == "" {
		message = response.Status
	}
	return &HTTPError{StatusCode: response.StatusCode, Message: message}
}
