package a2a

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func TestCallbackSSRFValidation(t *testing.T) {
	public := func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	}
	for _, raw := range []string{"http://example.com", "https://127.0.0.1", "https://localhost", "https://[::1]", "https://[::ffff:127.0.0.1]", "https://10.0.0.1", "https://169.254.169.254/latest/meta-data", "https://100.64.0.1", "https://192.0.2.1", "https://[fd00::1]", "https://[fec0::1]", "https://[::127.0.0.1]", "https://[3fff::1]", "https://[64:ff9b::7f00:1]", "https://user:secret@example.com", "https://example.com:8443", "https://example.com/#fragment"} {
		if _, _, err := validateCallback(context.Background(), raw, public); err == nil {
			t.Errorf("unsafe callback accepted: %s", raw)
		}
	}
	mixed := func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("10.0.0.1")}, nil
	}
	if _, _, err := validateCallback(context.Background(), "https://example.com", mixed); err == nil {
		t.Fatal("mixed public/private DNS accepted")
	}
	if _, _, err := validateCallback(context.Background(), "https://example.com/callback", public); err != nil {
		t.Fatal(err)
	}
}
func TestCallbackDNSRebindingAtDial(t *testing.T) {
	good := func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	}
	raw := "https://example.com/callback"
	if _, _, err := validateCallback(context.Background(), raw, good); err != nil {
		t.Fatal(err)
	}
	private := func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
	}
	dialed := false
	tr := pushTransport(raw, private, func(context.Context, string, string) (net.Conn, error) { dialed = true; return nil, io.EOF })
	if tr.Proxy != nil {
		t.Fatal("environment proxy may bypass address validation")
	}
	if _, err := tr.DialContext(context.Background(), "tcp", "example.com:443"); err == nil || dialed {
		t.Fatal("DNS rebinding reached dial")
	}
}
func TestCallbackTLSDeliveryAndNoRedirect(t *testing.T) {
	requests := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") != "Bearer callback-secret" {
			t.Error("callback authentication missing")
		}
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "https://127.0.0.1/private", http.StatusFound)
			return
		}
		w.WriteHeader(200)
	}))
	defer server.Close()
	lookup := func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	}
	makeClient := func(raw string) *http.Client {
		tr := pushTransport(raw, lookup, func(ctx context.Context, network, address string) (net.Conn, error) {
			if address != "8.8.8.8:443" {
				t.Errorf("unvalidated dial %s", address)
			}
			return (&net.Dialer{}).DialContext(ctx, network, strings.TrimPrefix(server.URL, "https://"))
		})
		tr.TLSClientConfig = server.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
		t.Cleanup(tr.CloseIdleConnections)
		return &http.Client{Transport: tr, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	event := PushEvent{TaskID: "task", Tenant: "tenant-a", DeliveryID: "one", Revision: 1, State: AuthRequired}
	cfg := callback("https://example.com/callback", "callback-secret")
	if err := deliver(context.Background(), makeClient(cfg.URL), cfg, event); err != nil {
		t.Fatal(err)
	}
	cfg.URL = "https://example.com/redirect"
	if err := deliver(context.Background(), makeClient(cfg.URL), cfg, event); err == nil {
		t.Fatal("redirect accepted")
	}
	if requests != 2 {
		t.Fatalf("redirect forwarded callback credentials, requests=%d", requests)
	}
	if err := DeliverPush(context.Background(), callback("https://127.0.0.1", "secret"), event); err == nil {
		t.Fatal("production sender bypassed private address check")
	}
}
func TestPushAuthenticationOwnershipDuplicatesAndLostAck(t *testing.T) {
	p, c, _ := setup(t, true)
	rec := submit(t, c, input)
	receiver := Receiver{Dir: c.Dir, Token: "callback", Owner: alice, IntentID: rec.IntentID}
	first := true
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rr := httptest.NewRecorder()
		receiver.ServeHTTP(rr, r)
		if first {
			first = false
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				panic(err)
			}
			_ = conn.Close()
			return
		}
		w.WriteHeader(rr.Code)
	}))
	defer ts.Close()
	cfg := callback(ts.URL, "callback")
	ev := PushEvent{TaskID: rec.TaskID, ContextID: rec.ContextID, Tenant: alice.Tenant, DeliveryID: "one", Revision: 1, State: Completed}
	if err := deliver(context.Background(), http.DefaultClient, cfg, ev); err == nil {
		t.Fatal("lost ACK not injected")
	}
	// A new Receiver instance reads the durable Inbox, so process-local dedup is insufficient.
	restarted := httptest.NewServer(Receiver{Dir: c.Dir, Token: "callback", Owner: alice, IntentID: rec.IntentID})
	defer restarted.Close()
	cfg.URL = restarted.URL
	if err := deliver(context.Background(), http.DefaultClient, cfg, ev); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"bad-secret", "wrong-tenant", "wrong-task", "wrong-context", "same-id-different-payload"} {
		e := ev
		bad := cfg
		switch fault {
		case "bad-secret":
			bad.Authentication.Credentials = "forged"
		case "wrong-tenant":
			e.Tenant = "tenant-b"
		case "wrong-task":
			e.TaskID = "foreign"
		case "wrong-context":
			e.ContextID = "foreign-context"
		case "same-id-different-payload":
			e.State = Working
		}
		if err := deliver(context.Background(), http.DefaultClient, bad, e); err == nil {
			t.Fatalf("%s accepted", fault)
		}
	}
	// Duplicate, out-of-order and forged completed hints cannot authorize execution.
	for _, revision := range []int{3, 2} {
		e := ev
		e.DeliveryID = id()
		e.Revision = revision
		if err := deliver(context.Background(), http.DefaultClient, cfg, e); err != nil {
			t.Fatal(err)
		}
	}
	got, err := c.Reconcile(context.Background(), rec.IntentID)
	if err != nil || got.State != AuthRequired {
		t.Fatal("push overwrote authoritative task state")
	}
	n, _ := p.Effects()
	if n != 0 {
		t.Fatal("push executed an effect")
	}
	_ = (store{c.Dir}).tx(func(d *database) error {
		if len(d.Deliveries) != 3 {
			t.Errorf("durable Inbox entries=%d", len(d.Deliveries))
		}
		return nil
	})
}
func TestPushConfigSSRFAndAuthorizedReadDelete(t *testing.T) {
	p, c, ts := setup(t, true)
	rec := submit(t, c, input)
	path := "/fixture/tasks/" + rec.TaskID + "/pushNotificationConfigs"
	code, _ := wire(t, ts.URL, "alice", "POST", path, callback("https://127.0.0.1", "secret"))
	if code != 400 {
		t.Fatal("private callback registered")
	}
	// Exercise the registration HTTP path with deterministic DNS, without any public traffic.
	p.callbackLookup = func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	}
	code, b := wire(t, ts.URL, "alice", "POST", path, callback("https://example.com", "hidden-secret"))
	if code != 200 || strings.Contains(b, "hidden-secret") {
		t.Fatalf("registration %d %s", code, b)
	}
	var cfg PushConfig
	if err := json.Unmarshal([]byte(b), &cfg); err != nil || cfg.ID == "" {
		t.Fatal("no config ID")
	}
	code, _ = wire(t, ts.URL, "alice", "POST", path, callback("https://example.com", "replacement"))
	if code != 409 {
		t.Fatal("existing config silently replaced")
	}
	code, b = wire(t, ts.URL, "alice", "GET", path+"/"+cfg.ID, nil)
	if code != 200 || strings.Contains(b, "hidden-secret") {
		t.Fatal("get config leaked secret")
	}
	code, b = wire(t, ts.URL, "alice", "GET", path, nil)
	if code != 200 || strings.Contains(b, "hidden-secret") {
		t.Fatal("callback config credentials exposed")
	}
	code, _ = wire(t, ts.URL, "bob", "DELETE", path+"/"+cfg.ID, nil)
	if code != 404 {
		t.Fatal("foreign deletion accepted")
	}
	code, _ = wire(t, ts.URL, "alice", "DELETE", path+"/"+cfg.ID, nil)
	if code != 200 {
		t.Fatal("owner deletion failed")
	}
	code, _ = wire(t, ts.URL, "alice", "DELETE", path+"/"+cfg.ID, nil)
	if code != 200 {
		t.Fatal("delete not idempotent")
	}
}
