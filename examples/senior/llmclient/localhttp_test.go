package llmclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const terminal = `data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":10,"output_tokens":2,"input_tokens_details":{"cached_tokens":4}}}}` + "\n\n"
const delta = `data: {"type":"response.output_text.delta","delta":"hello"}` + "\n\n"

func local(t *testing.T, h http.HandlerFunc, n int, observer func(Attempt)) *LocalHTTPClient {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	c, e := NewLocalHTTP(s.URL, n, observer)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(c.Close)
	return c
}
func TestSSEFramingAndUsage(t *testing.T) {
	for _, sep := range []string{"\n", "\r\n", "\r"} {
		t.Run(fmt.Sprintf("%q", sep), func(t *testing.T) {
			c := local(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "" {
					t.Error("unexpected key")
				}
				w.Header().Set("Content-Type", "text/event-stream")
				body := "\ufeff: comment\n\nevent: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\ndata: \"delta\":\"hello\"}\n\n" + terminal
				fmt.Fprint(w, strings.ReplaceAll(body, "\n", sep))
			}, 1, nil)
			text, u, e := c.Complete(context.Background(), []Message{{"user", "hi"}})
			if e != nil || text != "hello" || !u.Known || u.PromptTokens != 10 || u.CachedInputTokens != 4 {
				t.Fatalf("%q %+v %v", text, u, e)
			}
		})
	}
}
func TestExplicitCompletionAndFailuresNeverRetry(t *testing.T) {
	cases := map[string]string{"drop": delta, "failed": delta + "data: {\"type\":\"response.failed\"}\n\n", "incomplete": "data: {\"type\":\"response.incomplete\"}\n\n", "error": "data: {\"type\":\"error\"}\n\n", "done": "data: [DONE]\n\n", "bad-json": "data: {\n\n", "status": "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"failed\"}}\n\n", "bad-usage": strings.ReplaceAll(terminal, `"cached_tokens":4`, `"cached_tokens":99`), "missing-count": strings.ReplaceAll(terminal, `"input_tokens":10,`, ""), "tool": "data: {\"type\":\"response.function_call_arguments.delta\"}\n\n", "oversize": "data: " + strings.Repeat("x", 65537) + "\n\n", "mismatch": "event: error\n" + terminal}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int32
			observed := []Attempt{}
			c := local(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("X-Request-ID", "req-fault")
				fmt.Fprint(w, body)
			}, 3, func(a Attempt) { observed = append(observed, a) })
			text, _, e := c.Complete(context.Background(), []Message{{"user", "hi"}})
			var se *StreamError
			if !errors.As(e, &se) || text != "" || calls.Load() != 1 || len(observed) != 1 || se.RequestID != "req-fault" {
				t.Fatalf("text=%q calls=%d err=%v", text, calls.Load(), e)
			}
			if name == "drop" && (!se.Partial || observed[0].Usage.Known) {
				t.Fatal("lost partial/unknown state")
			}
		})
	}
}
func TestSafeRejectionRetriesAndAmbiguousErrorsDoNot(t *testing.T) {
	for _, mode := range []string{"safe", "429-unknown", "500", "redirect"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			observed := []Attempt{}
			c := local(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if calls.Load() == 1 {
					switch mode {
					case "safe":
						w.Header().Set("X-Fixture-Rejected", "before-generation")
						w.Header().Set("Retry-After", "0")
						w.WriteHeader(429)
					case "429-unknown":
						w.WriteHeader(429)
					case "500":
						w.WriteHeader(500)
					case "redirect":
						w.Header().Set("Location", "https://example.invalid")
						w.WriteHeader(302)
					}
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, delta+terminal)
			}, 3, func(a Attempt) { observed = append(observed, a) })
			text, _, e := c.Complete(context.Background(), []Message{{"user", "hi"}})
			if mode == "safe" {
				if e != nil || text != "hello" || calls.Load() != 2 || len(observed) != 2 || !observed[0].Usage.Known {
					t.Fatal(calls.Load(), e)
				}
			} else if e == nil || calls.Load() != 1 {
				t.Fatal(calls.Load(), e)
			}
		})
	}
}
func TestDeadlineCallbackAndCancel(t *testing.T) {
	t.Run("retry-deadline", func(t *testing.T) {
		var calls atomic.Int32
		c := local(t, func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.Header().Set("X-Fixture-Rejected", "before-generation")
			w.Header().Set("Retry-After", "10")
			w.WriteHeader(429)
		}, 3, nil)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()
		_, _, e := c.Complete(ctx, []Message{{"user", "hi"}})
		if !errors.Is(e, context.DeadlineExceeded) || calls.Load() != 1 {
			t.Fatal(calls.Load(), e)
		}
	})
	for _, mode := range []string{"callback", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			done := make(chan struct{})
			var calls atomic.Int32
			c := local(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, delta)
				w.(http.Flusher).Flush()
				<-r.Context().Done()
				close(done)
			}, 3, nil)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			_, e := c.StreamChat(ctx, []Message{{"user", "hi"}}, func(string) error {
				if mode == "cancel" {
					cancel()
					return nil
				}
				return errors.New("downstream closed")
			})
			if e == nil || calls.Load() != 1 {
				t.Fatal(e)
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("server not canceled")
			}
		})
	}
}
func TestUnknownUsageAndEndpointLimits(t *testing.T) {
	c := local(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n")
	}, 1, nil)
	_, u, e := c.Complete(context.Background(), []Message{{"user", "hi"}})
	if e != nil || u.Known {
		t.Fatal(u, e)
	}
	for _, endpoint := range []string{"https://api.openai.com/v1/responses", "http://localhost:80", "http://123.57.48.214", "http://user:pass@127.0.0.1"} {
		if _, e := NewLocalHTTP(endpoint, 1, nil); e == nil {
			t.Fatal("nonlocal endpoint accepted")
		}
	}
	for _, v := range []string{"-1", "invalid"} {
		if _, e := retryDelay(v); e == nil {
			t.Fatal(v)
		}
	}
	if _, e := retryDelay(time.Now().Add(time.Minute).UTC().Format(http.TimeFormat)); e != nil {
		t.Fatal(e)
	}
}

func TestInputAndStreamBudgets(t *testing.T) {
	var calls atomic.Int32
	c := local(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		for i := 0; i < 600; i++ {
			fmt.Fprintf(w, ": %s\n", strings.Repeat("x", 2048))
		}
		fmt.Fprint(w, terminal)
	}, 1, nil)
	for _, m := range [][]Message{nil, make([]Message, 65), {{Role: "user", Content: strings.Repeat("x", 64<<10)}}} {
		if _, _, e := c.Complete(context.Background(), m); e == nil {
			t.Fatal("input accepted")
		}
	}
	if _, e := c.StreamChat(context.Background(), []Message{{Role: "user", Content: "hi"}}, nil); e == nil {
		t.Fatal("nil callback accepted")
	}
	if calls.Load() != 0 {
		t.Fatal("invalid request reached server")
	}
	if _, _, e := c.Complete(context.Background(), []Message{{Role: "user", Content: "hi"}}); e == nil {
		t.Fatal("oversize stream accepted")
	}
}
func TestUnacceptedConnectionAndInvalidResponsesDoNotRetry(t *testing.T) {
	for _, mode := range []string{"connection-close", "wrong-type", "no-blank-terminal", "negative-usage", "missing-cached", "invalid-retry-after"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			c := local(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				switch mode {
				case "connection-close":
					conn, _, e := w.(http.Hijacker).Hijack()
					if e != nil {
						t.Error(e)
						return
					}
					_ = conn.Close()
				case "wrong-type":
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, terminal)
				case "no-blank-terminal":
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(w, strings.TrimRight(terminal, "\n"))
				case "negative-usage":
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(w, strings.Replace(terminal, `"output_tokens":2`, `"output_tokens":-1`, 1))
				case "missing-cached":
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(w, strings.Replace(terminal, `"cached_tokens":4`, "", 1))
				case "invalid-retry-after":
					w.Header().Set("X-Fixture-Rejected", "before-generation")
					w.Header().Set("Retry-After", "invalid")
					w.WriteHeader(429)
				}
			}, 3, nil)
			text, _, e := c.Complete(context.Background(), []Message{{Role: "user", Content: "hi"}})
			if e == nil || text != "" || calls.Load() != 1 {
				t.Fatal(calls.Load(), e)
			}
		})
	}
}
