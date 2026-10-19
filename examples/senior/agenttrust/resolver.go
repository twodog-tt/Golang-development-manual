package agenttrust

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

type lookupFunc func(context.Context, string) ([]netip.Addr, error)

func resolve(ctx context.Context, host string) ([]netip.Addr, error) {
	return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
}
func publicAddress(a netip.Addr) bool {
	a = a.Unmap()
	if a.Is6() && !netip.MustParsePrefix("2000::/3").Contains(a) {
		return false
	}
	if !a.IsGlobalUnicast() || a.IsPrivate() || a.IsLoopback() || a.IsLinkLocalUnicast() {
		return false
	}
	// Deny special-use/reserved ranges as well as private addresses.
	for _, block := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32", "2001::/23", "64:ff9b::/96", "64:ff9b:1::/48", "2002::/16", "3fff::/20", "192.88.99.0/24"} {
		if netip.MustParsePrefix(block).Contains(a) {
			return false
		}
	}
	return true
}
func validateURL(ctx context.Context, raw string, lookup lookupFunc) (*url.URL, []netip.Addr, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") {
		return nil, nil, errors.New("fetch target must be HTTPS on port 443, without userinfo or fragment")
	}
	if strings.EqualFold(strings.TrimSuffix(u.Hostname(), "."), "localhost") {
		return nil, nil, errors.New("private fetch target denied")
	}
	var addrs []netip.Addr
	if a, err := netip.ParseAddr(u.Hostname()); err == nil {
		addrs = []netip.Addr{a}
	} else {
		addrs, err = lookup(ctx, u.Hostname())
		if err != nil {
			return nil, nil, err
		}
	}
	if len(addrs) == 0 {
		return nil, nil, errors.New("no fetch target addresses")
	}
	for _, a := range addrs {
		if !publicAddress(a) {
			return nil, nil, errors.New("non-public fetch target address")
		}
	}
	return u, addrs, nil
}

// Resolver accepts HTTPS only. IPFS/data need separate bounded codecs; unsupported
// URI schemes fail closed. The private seams exist solely for deterministic TLS tests.
type Resolver struct {
	lookup  lookupFunc
	dial    func(context.Context, string, string) (net.Conn, error)
	tls     *tls.Config
	timeout time.Duration
}

func NewResolver() *Resolver {
	return &Resolver{lookup: resolve, dial: (&net.Dialer{Timeout: time.Second}).DialContext, timeout: 2 * time.Second}
}
func (r *Resolver) Fetch(ctx context.Context, raw string, expected [32]byte) ([]byte, [32]byte, error) {
	var zero [32]byte
	if r == nil || r.lookup == nil || r.dial == nil || r.timeout <= 0 {
		return nil, zero, errors.New("configured resolver required")
	}
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	if _, _, err := validateURL(ctx, raw, r.lookup); err != nil {
		return nil, zero, err
	}
	tr := &http.Transport{Proxy: nil, TLSClientConfig: r.tls, DisableCompression: true, MaxResponseHeaderBytes: 16 << 10, ResponseHeaderTimeout: r.timeout}
	tr.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		u, addrs, err := validateURL(ctx, raw, r.lookup)
		if err != nil {
			return nil, err
		}
		if address != net.JoinHostPort(u.Hostname(), "443") {
			return nil, errors.New("unexpected dial target")
		}
		return r.dial(ctx, network, net.JoinHostPort(addrs[0].String(), "443"))
	}
	defer tr.CloseIdleConnections()
	hc := &http.Client{Transport: tr, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, err := http.NewRequestWithContext(ctx, "GET", raw, nil)
	if err != nil {
		return nil, zero, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, zero, err
	}
	defer resp.Body.Close()
	mt, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || mt != "application/json" || resp.StatusCode != 200 || resp.Header.Get("Content-Encoding") != "" {
		return nil, zero, errors.New("unexpected response status/type/encoding")
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, (64<<10)+1))
	if err != nil {
		return nil, zero, err
	}
	if len(b) > 64<<10 {
		return nil, zero, errors.New("document too large")
	}
	digest := sha256.Sum256(b)
	if expected != zero && digest != expected {
		return nil, zero, errors.New("document changed; review required")
	}
	return b, digest, nil
}

// MatchRegistration checks the reciprocal identity entry, not runtime authority.
// Only identity fields used here are decoded; unknown extension fields remain in
// the hashed original bytes. Duplicate keys are rejected before interpretation.
func MatchRegistration(raw []byte, ref Ref) error {
	if !ref.Valid() || len(raw) > 64<<10 {
		return ErrDenied
	}
	if err := uniqueJSON(raw); err != nil {
		return err
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return err
	}
	for key := range doc {
		if strings.EqualFold(key, "registrations") && key != "registrations" {
			return ErrDenied
		}
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(doc["registrations"], &entries); err != nil {
		return err
	}
	registry := fmt.Sprintf("eip155:%d:%s", ref.Chain, ref.Registry.Hex())
	matched := false
	for _, entry := range entries {
		for key := range entry {
			if (strings.EqualFold(key, "agentId") && key != "agentId") || (strings.EqualFold(key, "agentRegistry") && key != "agentRegistry") {
				return ErrDenied
			}
		}
		var id json.Number
		var address string
		if len(entry["agentId"]) == 0 || entry["agentId"][0] < '0' || entry["agentId"][0] > '9' {
			return ErrDenied
		}
		if err := json.Unmarshal(entry["agentId"], &id); err != nil {
			return err
		}
		if err := json.Unmarshal(entry["agentRegistry"], &address); err != nil {
			return err
		}
		if id.String() == ref.AgentID && strings.EqualFold(address, registry) {
			matched = true
		}
	}
	if matched {
		return nil
	}
	return ErrDenied
}

func uniqueJSON(raw []byte) error {
	d := json.NewDecoder(strings.NewReader(string(raw)))
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 16 {
			return errors.New("JSON too deep")
		}
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch tok {
		case json.Delim('{'):
			seen := map[string]bool{}
			for d.More() {
				k, err := d.Token()
				if err != nil {
					return err
				}
				key, ok := k.(string)
				if !ok || seen[key] {
					return errors.New("duplicate JSON key")
				}
				seen[key] = true
				if err = walk(depth + 1); err != nil {
					return err
				}
			}
			_, err = d.Token()
			return err
		case json.Delim('['):
			for d.More() {
				if err = walk(depth + 1); err != nil {
					return err
				}
			}
			_, err = d.Token()
			return err
		case json.Delim('}'), json.Delim(']'):
			return errors.New("unexpected delimiter")
		}
		return nil
	}
	if err := walk(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}
