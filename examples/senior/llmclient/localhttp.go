package llmclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Attempt is emitted exactly once per HTTP attempt, including unknown usage on failure.
type Attempt struct {
	RequestID string
	Usage     Usage
	Err       error
}
type StreamError struct {
	RequestID string
	Partial   bool
	Cause     error
}

func (e *StreamError) Error() string {
	return fmt.Sprintf("local stream request=%q partial=%t: %v", e.RequestID, e.Partial, e.Cause)
}
func (e *StreamError) Unwrap() error { return e.Cause }

// LocalHTTPClient implements only the repository's text SSE fixture contract.
// It accepts literal loopback IPs only, disables proxies/redirects, and never sends keys.
// It is NOT a complete Responses API provider adapter.
type LocalHTTPClient struct {
	endpoint string
	client   *http.Client
	attempts int
	observe  func(Attempt)
}

func NewLocalHTTP(endpoint string, attempts int, observe func(Attempt)) (*LocalHTTPClient, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, errors.New("invalid local endpoint")
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "http" || ip == nil || !ip.IsLoopback() || u.User != nil || u.Fragment != "" || u.RawQuery != "" {
		return nil, errors.New("endpoint must be literal loopback HTTP without credentials or query")
	}
	if attempts < 1 || attempts > 5 {
		return nil, errors.New("attempt limit must be 1..5")
	}
	tr := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: time.Second}).DialContext, MaxIdleConns: 8, IdleConnTimeout: 30 * time.Second, ResponseHeaderTimeout: 2 * time.Second}
	return &LocalHTTPClient{u.String(), &http.Client{Transport: tr, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, attempts, observe}, nil
}
func (c *LocalHTTPClient) Close() { c.client.CloseIdleConnections() }
func (c *LocalHTTPClient) Complete(ctx context.Context, m []Message) (string, Usage, error) {
	var b strings.Builder
	u, err := c.StreamChat(ctx, m, func(s string) error { b.WriteString(s); return nil })
	if err != nil {
		return "", u, err
	}
	return b.String(), u, nil
}
func (c *LocalHTTPClient) StreamChat(ctx context.Context, m []Message, onChunk StreamHandler) (Usage, error) {
	if onChunk == nil {
		return Usage{}, errors.New("nil stream handler")
	}
	if len(m) == 0 || len(m) > 64 {
		return Usage{}, errors.New("invalid message count")
	}
	body, err := json.Marshal(struct {
		Contract string    `json:"contract"`
		Messages []Message `json:"messages"`
		Stream   bool      `json:"stream"`
	}{"local-text-sse-v1", m, true})
	if err != nil {
		return Usage{}, err
	}
	if len(body) > 64<<10 {
		return Usage{}, errors.New("input budget exceeded")
	}
	// A total deadline is required even when the caller supplies none.
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for n := 0; n < c.attempts; n++ {
		u, id, partial, retry, delay, err := c.once(ctx, body, onChunk)
		if err != nil {
			err = &StreamError{id, partial, err}
		}
		if c.observe != nil {
			c.observe(Attempt{id, u, err})
		}
		if err == nil || !retry || n+1 == c.attempts {
			return u, err
		}
		if delay == 0 {
			delay = time.Duration(10*(1<<n))*time.Millisecond + time.Duration(rand.IntN(10))*time.Millisecond
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return Usage{}, &StreamError{id, false, ctx.Err()}
		case <-timer.C:
		}
	}
	return Usage{}, errors.New("unreachable attempt state")
}
func retryDelay(v string) (time.Duration, error) {
	if v == "" {
		return 0, nil
	}
	if n, err := strconv.ParseInt(v, 10, 32); err == nil {
		if n < 0 {
			return 0, errors.New("negative Retry-After")
		}
		return time.Duration(n) * time.Second, nil
	}
	t, err := http.ParseTime(v)
	if err != nil {
		return 0, errors.New("invalid Retry-After")
	}
	return max(0, time.Until(t)), nil
}
func (c *LocalHTTPClient) once(ctx context.Context, body []byte, onChunk StreamHandler) (u Usage, id string, partial, retry bool, delay time.Duration, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return u, "", false, false, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return u, "", false, false, 0, fmt.Errorf("transport failure; result unknown: %w", err)
	}
	defer resp.Body.Close()
	id = resp.Header.Get("X-Request-ID")
	if resp.StatusCode != http.StatusOK {
		// Only this explicit local rejection contract proves no generation was dispatched.
		retry = resp.StatusCode == 429 && resp.Header.Get("X-Fixture-Rejected") == "before-generation"
		if retry {
			u = Usage{Known: true, Source: "local-rejected"}
			delay, err = retryDelay(resp.Header.Get("Retry-After"))
			if err != nil {
				retry = false
				return u, id, false, false, 0, err
			}
		}
		return u, id, false, retry, delay, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	mt, _, e := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if e != nil || mt != "text/event-stream" {
		return u, id, false, false, 0, errors.New("expected SSE content type")
	}
	reader := bufio.NewReader(io.LimitReader(resp.Body, (1<<20)+1))
	total := 0
	frame := []string{}
	frameSize := 0
	eventName := ""
	first := true
	for {
		line, e := sseLine(reader, 64<<10)
		total += len(line) + 1
		if total > 1<<20 {
			return u, id, partial, false, 0, errors.New("stream budget exceeded")
		}
		if first {
			line = strings.TrimPrefix(line, "\ufeff")
			first = false
		}
		if e != nil {
			return u, id, partial, false, 0, errors.New("stream interrupted or oversized before completion")
		}
		if line != "" {
			if strings.HasPrefix(line, ":") {
				continue
			}
			field, value, ok := strings.Cut(line, ":")
			if !ok {
				value = ""
			}
			value = strings.TrimPrefix(value, " ")
			switch field {
			case "data":
				frameSize += len(value) + 1
				if frameSize > 64<<10 {
					return u, id, partial, false, 0, errors.New("SSE frame too large")
				}
				frame = append(frame, value)
			case "event":
				eventName = value
			}
			continue
		}
		if len(frame) == 0 {
			eventName = ""
			continue
		}
		var event struct {
			Type     string `json:"type"`
			Delta    string `json:"delta"`
			Response struct {
				Status string `json:"status"`
				Usage  *struct {
					Input   *int `json:"input_tokens"`
					Output  *int `json:"output_tokens"`
					Details *struct {
						Cached *int `json:"cached_tokens"`
					} `json:"input_tokens_details"`
				} `json:"usage"`
			} `json:"response"`
		}
		if err = json.Unmarshal([]byte(strings.Join(frame, "\n")), &event); err != nil {
			return u, id, partial, false, 0, errors.New("invalid event JSON")
		}
		frame = nil
		frameSize = 0
		if eventName != "" && eventName != event.Type {
			return u, id, partial, false, 0, errors.New("event type mismatch")
		}
		eventName = ""
		switch event.Type {
		case "response.created":
		case "response.output_text.delta":
			if event.Delta != "" {
				partial = true
				if err = onChunk(event.Delta); err != nil {
					return u, id, partial, false, 0, err
				}
			}
		case "response.completed":
			if event.Response.Status != "completed" {
				return u, id, partial, false, 0, errors.New("invalid terminal status")
			}
			reported := event.Response.Usage
			if reported == nil {
				return u, id, partial, false, 0, nil
			}
			if reported.Input == nil || reported.Output == nil {
				return u, id, partial, false, 0, errors.New("incomplete usage")
			}
			cached := 0
			if reported.Details != nil {
				if reported.Details.Cached == nil {
					return u, id, partial, false, 0, errors.New("missing cached usage")
				}
				cached = *reported.Details.Cached
			}
			if *reported.Input < 0 || *reported.Output < 0 || cached < 0 || cached > *reported.Input {
				return u, id, partial, false, 0, errors.New("invalid usage counts")
			}
			u = Usage{Known: true, Source: "local-reported", PromptTokens: *reported.Input, CompletionTokens: *reported.Output, CachedInputTokens: cached}
			return u, id, partial, false, 0, nil
		case "response.failed", "response.incomplete", "error":
			return u, id, partial, false, 0, errors.New("provider did not complete")
		default:
			return u, id, partial, false, 0, errors.New("unsupported event (text fixture only)")
		}
	}
}

// SSE permits LF, CRLF and bare CR. A frame is dispatched only at a blank line.
func sseLine(r *bufio.Reader, limit int) (string, error) {
	var b strings.Builder
	for {
		ch, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		if ch == '\n' {
			return b.String(), nil
		}
		if ch == '\r' {
			next, e := r.Peek(1)
			if e == nil && next[0] == '\n' {
				_, _ = r.ReadByte()
			}
			return b.String(), nil
		}
		if b.Len() >= limit {
			return "", errors.New("SSE line too large")
		}
		b.WriteByte(ch)
	}
}
