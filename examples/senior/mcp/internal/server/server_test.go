package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func fixture(t *testing.T) (*Authority, Config) {
	t.Helper()
	a := NewAuthority("fixture")
	a.Put("a", Grant{Principal: Principal{"alice", "tenant-a", []string{"greet", "orders:read"}}, Audience: "fixture", Expires: time.Now().Add(time.Hour)})
	return a, Config{Authority: a, LocalToken: "a"}
}
func connect(t *testing.T, c Config) *mcp.ClientSession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	ct, st := mcp.NewInMemoryTransports()
	ss, err := New(c).Connect(ctx, st, nil)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		_ = ss.Close()
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close(); _ = ss.Close(); cancel() })
	return cs
}
func TestToolContracts(t *testing.T) {
	_, c := fixture(t)
	cs := connect(t, c)
	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil || len(tools.Tools) != 2 {
		t.Fatalf("tools: %+v %v", tools, err)
	}
	for _, tool := range tools.Tools {
		if tool.InputSchema == nil || tool.OutputSchema == nil {
			t.Fatal("missing tool schemas")
		}
	}
	r, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_order", Arguments: map[string]any{"order_id": "10001"}})
	if err != nil || r.IsError {
		t.Fatalf("%+v %v", r, err)
	}
	b, err := json.Marshal(r.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var got Order
	if err = json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got != (Order{"10001", "paid", 9900}) {
		t.Fatalf("%+v", got)
	}
	if len(r.Content) != 1 {
		t.Fatal("missing legacy content")
	}
	text, ok := r.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("not text")
	}
	var legacy Order
	if err = json.Unmarshal([]byte(text.Text), &legacy); err != nil || legacy != got {
		t.Fatal("structured/legacy output diverged")
	}
	for _, name := range []string{"Go", ""} {
		r, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "greet", Arguments: map[string]any{"name": name}})
		if err != nil || r.IsError {
			t.Fatalf("%+v %v", r, err)
		}
		want := name
		if want == "" {
			want = "friend"
		}
		if r.Content[0].(*mcp.TextContent).Text != "{\"greeting\":\"Hello, "+want+" (from Go MCP)\"}" {
			t.Fatal("greet contract drift")
		}
	}
}
func TestSchemaAndExecutionErrors(t *testing.T) {
	_, c := fixture(t)
	cs := connect(t, c)
	for _, args := range []map[string]any{{}, {"order_id": 1}, {"order_id": "10001", "tenant_id": "tenant-b"}} {
		_, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_order", Arguments: args})
		if err == nil {
			t.Fatalf("invalid input accepted: %v", args)
		}
	}
	if _, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "no_such_tool", Arguments: map[string]any{}}); err == nil {
		t.Fatal("unknown tool accepted")
	}
	for _, id := range []string{"10002", "missing"} {
		r, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_order", Arguments: map[string]any{"order_id": id}})
		if err != nil || !r.IsError {
			t.Fatalf("execution error was not IsError: %+v %v", r, err)
		}
		if r.Content[0].(*mcp.TextContent).Text != "order unavailable" {
			t.Fatal("existence leaked")
		}
	}
}
func TestIdentityScopeAndRevocation(t *testing.T) {
	a, c := fixture(t)
	cs := connect(t, c)
	for _, grant := range []Grant{{Principal: Principal{"alice", "tenant-a", []string{"greet"}}, Audience: "fixture", Expires: time.Now().Add(time.Hour)}, {Principal: Principal{"alice", "tenant-a", []string{"orders:read"}}, Audience: "other", Expires: time.Now().Add(time.Hour)}, {Principal: Principal{"alice", "tenant-a", []string{"orders:read"}}, Audience: "fixture", Expires: time.Now().Add(-time.Hour)}} {
		a.Put("a", grant)
		r, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_order", Arguments: map[string]any{"order_id": "10001"}})
		if err != nil || !r.IsError {
			t.Fatal("invalid grant accepted")
		}
	}
	a.Revoke("a")
	r, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_order", Arguments: map[string]any{"order_id": "10001"}})
	if err != nil || !r.IsError {
		t.Fatal("revoked grant accepted")
	}
}
func TestDeadlineAndCancellationReachBackend(t *testing.T) {
	for _, cancelCaller := range []bool{false, true} {
		t.Run(map[bool]string{false: "deadline", true: "cancel"}[cancelCaller], func(t *testing.T) {
			_, c := fixture(t)
			c.Timeout = 20 * time.Millisecond
			if cancelCaller {
				c.Timeout = time.Second
			}
			entered := make(chan struct{})
			stopped := make(chan error, 1)
			c.Query = func(ctx context.Context, _, _ string) (Order, error) {
				close(entered)
				<-ctx.Done()
				stopped <- ctx.Err()
				return Order{}, ctx.Err()
			}
			cs := connect(t, c)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan error, 1)
			go func() {
				r, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_order", Arguments: map[string]any{"order_id": "10001"}})
				if err == nil && !r.IsError {
					err = io.ErrUnexpectedEOF
				}
				result <- err
			}()
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("not dispatched")
			}
			if cancelCaller {
				cancel()
			}
			select {
			case err := <-stopped:
				if err == nil {
					t.Fatal("backend not canceled")
				}
			case <-time.After(time.Second):
				t.Fatal("cancellation not propagated")
			}
			select {
			case <-result:
			case <-time.After(time.Second):
				t.Fatal("call stuck")
			}
		})
	}
}
func request(t *testing.T, url, token, origin, sid, method, body string) (int, http.Header, string) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if sid != "" {
		req.Header.Set("MCP-Session-Id", sid)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, res.Header, string(b)
}

const initBody = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`
const callBody = `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_order","arguments":{"order_id":"10001"}}}`

func httpFixture(t *testing.T) (*httptest.Server, *Authority) {
	t.Helper()
	a, c := fixture(t)
	c.LocalToken = ""
	ts := httptest.NewServer(HTTP(c, []string{"https://host.example"}, 50*time.Millisecond))
	t.Cleanup(ts.Close)
	return ts, a
}
func TestHTTPAuthenticationOriginAndSession(t *testing.T) {
	ts, a := httpFixture(t)
	for _, tc := range []struct {
		token, origin string
		want          int
	}{{"", "", 401}, {"bad", "", 401}, {"a", "https://evil.example", 403}, {"a", "null", 403}} {
		code, _, _ := request(t, ts.URL+"/mcp", tc.token, tc.origin, "", "POST", initBody)
		if code != tc.want {
			t.Fatalf("%+v: %d", tc, code)
		}
	}
	for _, bad := range []Grant{{Principal: Principal{"x", "tenant-a", []string{"orders:read"}}, Audience: "wrong", Expires: time.Now().Add(time.Hour)}, {Principal: Principal{"x", "tenant-a", nil}, Audience: "fixture", Expires: time.Now().Add(-time.Hour)}} {
		a.Put("bad", bad)
		code, _, _ := request(t, ts.URL+"/mcp", "bad", "", "", "POST", initBody)
		if code != 401 {
			t.Fatal("invalid audience/expiry accepted")
		}
	}
	code, h, b := request(t, ts.URL+"/mcp", "a", "https://host.example", "", "POST", initBody)
	if code != 200 || !strings.Contains(b, "2025-11-25") {
		t.Fatalf("init %d %s", code, b)
	}
	sid := h.Get("MCP-Session-Id")
	request(t, ts.URL+"/mcp", "a", "", sid, "POST", `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if sid == "" {
		t.Fatal("no session")
	}
	a.Put("b", Grant{Principal: Principal{"bob", "tenant-b", []string{"orders:read"}}, Audience: "fixture", Expires: time.Now().Add(time.Hour)})
	code, _, _ = request(t, ts.URL+"/mcp", "b", "", sid, "POST", callBody)
	if code != 403 {
		t.Fatal("session hijack accepted")
	}
	code, _, _ = request(t, ts.URL+"/mcp", "", "", sid, "POST", callBody)
	if code != 401 {
		t.Fatal("session used as credential")
	}
	code, _, b = request(t, ts.URL+"/mcp", "a", "", sid, "POST", callBody)
	if code != 200 || !strings.Contains(b, "9900") {
		t.Fatalf("call %d %s", code, b)
	}
	a.Revoke("a")
	code, _, _ = request(t, ts.URL+"/mcp", "a", "", sid, "POST", callBody)
	if code != 401 {
		t.Fatal("revocation not checked on existing session")
	}
}
func TestHTTPSessionExpiryAndReinitialize(t *testing.T) {
	ts, _ := httpFixture(t)
	_, h, _ := request(t, ts.URL+"/mcp", "a", "", "", "POST", initBody)
	sid := h.Get("MCP-Session-Id")
	// Poll for the idle timeout; no mutation or side-effect request is retried.
	deadline := time.Now().Add(time.Second)
	for {
		code, _, _ := request(t, ts.URL+"/mcp", "a", "", sid, "GET", "")
		if code == 404 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("session did not expire")
		}
		time.Sleep(10 * time.Millisecond)
	}
	code, h, _ := request(t, ts.URL+"/mcp", "a", "", "", "POST", initBody)
	if code != 200 || h.Get("MCP-Session-Id") == "" || h.Get("MCP-Session-Id") == sid {
		t.Fatal("reinitialization failed")
	}
	code, _, _ = request(t, ts.URL+"/mcp", "a", "", h.Get("MCP-Session-Id"), "DELETE", "")
	if code != 204 {
		t.Fatal("session deletion failed")
	}
}
