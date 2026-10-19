package agenttrust

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func publicLookup(context.Context, string) ([]netip.Addr, error) {
	return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
}
func tlsResolver(t *testing.T, h http.HandlerFunc) (*Resolver, *atomic.Int32) {
	t.Helper()
	var requests atomic.Int32
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); h(w, r) }))
	t.Cleanup(ts.Close)
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ts.Certificate().Raw}))
	r := NewResolver()
	r.lookup = publicLookup
	r.tls = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	// httptest certificate covers example.com; DNS/dial checks stay in the real transport.
	r.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "8.8.8.8:443" {
			t.Errorf("unvalidated dial %s", address)
		}
		return (&net.Dialer{}).DialContext(ctx, network, strings.TrimPrefix(ts.URL, "https://"))
	}
	return r, &requests
}
func TestResolverTLSHashAndReciprocalRegistration(t *testing.T) {
	raw := fmt.Sprintf(`{"type":"https://eips.ethereum.org/EIPS/eip-8004#registration-v1","registrations":[{"agentId":1,"agentRegistry":"eip155:84532:%s"}],"services":[{"name":"future-extension","other":true}]}`, ref.Registry.Hex())
	r, _ := tlsResolver(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, raw)
	})
	want := sha256.Sum256([]byte(raw))
	b, hash, e := r.Fetch(context.Background(), "https://example.com/agent.json", want)
	if e != nil || hash != want {
		t.Fatal(hash, e)
	}
	if e = MatchRegistration(b, ref); e != nil {
		t.Fatal(e)
	}
	bad := ref
	bad.AgentID = "2"
	if e = MatchRegistration(b, bad); e == nil {
		t.Fatal("identity mismatch accepted")
	}
	if _, _, e = r.Fetch(context.Background(), "https://example.com/agent.json", sha256.Sum256([]byte("changed"))); e == nil {
		t.Fatal("silent document change")
	}
}
func TestResolverDeniesSpecialTargetsBeforeDial(t *testing.T) {
	for _, raw := range []string{"http://example.com", "https://127.0.0.1", "https://169.254.169.254/latest/meta-data/", "https://10.0.0.1", "https://[::ffff:127.0.0.1]", "https://100.64.0.1", "https://198.18.0.1", "https://[2001:db8::1]", "https://u:p@example.com", "https://example.com:8443", "file:///etc/passwd", "data:application/json;base64,e30=", "ipfs://cid"} {
		r := NewResolver()
		r.lookup = publicLookup
		r.dial = func(context.Context, string, string) (net.Conn, error) {
			t.Fatal("unsafe target dialed")
			return nil, io.EOF
		}
		if _, _, e := r.Fetch(context.Background(), raw, [32]byte{}); e == nil {
			t.Fatal(raw)
		}
	}
}
func TestResolverRebindingMixedDNSAndNoProxy(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		r := NewResolver()
		calls := 0
		r.lookup = func(context.Context, string) ([]netip.Addr, error) {
			calls++
			if calls == 1 && !mixed {
				return publicLookup(context.Background(), "")
			}
			return []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("127.0.0.1")}, nil
		}
		r.dial = func(context.Context, string, string) (net.Conn, error) {
			t.Fatal("rebound target dialed")
			return nil, io.EOF
		}
		t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
		if _, _, e := r.Fetch(context.Background(), "https://example.com/agent", [32]byte{}); e == nil {
			t.Fatal("unsafe DNS accepted")
		}
	}
}
func TestResolverRedirectTypeSizeAndSlowBody(t *testing.T) {
	for _, mode := range []string{"redirect", "type", "size", "slow", "encoding"} {
		t.Run(mode, func(t *testing.T) {
			r, requests := tlsResolver(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch mode {
				case "redirect":
					http.Redirect(w, r, "https://127.0.0.1/private", 302)
				case "type":
					w.Header().Set("Content-Type", "text/html")
				case "size":
					_, _ = io.WriteString(w, strings.Repeat("x", 65537))
				case "slow":
					w.WriteHeader(200)
					w.(http.Flusher).Flush()
					<-r.Context().Done()
				case "encoding":
					w.Header().Set("Content-Encoding", "gzip")
				}
			})
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if _, _, e := r.Fetch(ctx, "https://example.com/agent", [32]byte{}); e == nil {
				t.Fatal("unsafe response accepted")
			}
			if requests.Load() != 1 {
				t.Fatal("redirect followed or request not exercised", requests.Load())
			}
		})
	}
}
func TestRegistrationMalformedAndDuplicateKeys(t *testing.T) {
	for _, raw := range []string{`{}`, `{"registrations":null}`, `{"registrations":[],"registrations":[]}`, `{"registrations":[]} {}`, strings.Repeat("[", 20) + strings.Repeat("]", 20)} {
		if e := MatchRegistration([]byte(raw), ref); e == nil {
			t.Fatal("malformed registration accepted")
		}
	}
}

func TestRegistrationCaseAliasesAndWrongTypesFailClosed(t *testing.T) {
	validEntry := fmt.Sprintf(`{"agentId":1,"agentRegistry":"eip155:84532:%s"}`, ref.Registry.Hex())
	for _, raw := range []string{
		`{"Registrations":[` + validEntry + `]}`,
		`{"registrations":[` + validEntry + `],"Registrations":[]}`,
		fmt.Sprintf(`{"registrations":[{"AgentId":1,"agentRegistry":"eip155:84532:%s"}]}`, ref.Registry.Hex()),
		fmt.Sprintf(`{"registrations":[{"agentId":1,"agentRegistry":"eip155:84532:%s","AgentRegistry":"other"}]}`, ref.Registry.Hex()),
		`{"registrations":{}}`, `{"registrations":[null]}`,
	} {
		if err := MatchRegistration([]byte(raw), ref); err == nil {
			t.Fatal("ambiguous registration accepted", raw)
		}
	}
}
