package a2a

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

type PushConfig struct {
	ID             string       `json:"id"`
	URL            string       `json:"url"`
	Authentication CallbackAuth `json:"authentication"`
}
type CallbackAuth struct {
	Scheme      string `json:"scheme"`
	Credentials string `json:"credentials,omitempty"`
}

func callback(raw, secret string) PushConfig {
	return PushConfig{URL: raw, Authentication: CallbackAuth{Scheme: "Bearer", Credentials: secret}}
}

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
func validateCallback(ctx context.Context, raw string, lookup lookupFunc) (*url.URL, []netip.Addr, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") {
		return nil, nil, errors.New("callback must be HTTPS on port 443, without userinfo or fragment")
	}
	if strings.EqualFold(strings.TrimSuffix(u.Hostname(), "."), "localhost") {
		return nil, nil, errors.New("private callback denied")
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
		return nil, nil, errors.New("no callback addresses")
	}
	for _, a := range addrs {
		if !publicAddress(a) {
			return nil, nil, errors.New("non-public callback address")
		}
	}
	return u, addrs, nil
}

// DeliverPush revalidates DNS at dial time and pins the validated IP. No proxy or
// redirect is followed, so neither DNS changes nor redirects can forward secrets.
func DeliverPush(ctx context.Context, cfg PushConfig, event PushEvent) error {
	if !strings.EqualFold(cfg.Authentication.Scheme, "Bearer") || cfg.Authentication.Credentials == "" {
		return errors.New("missing dedicated callback secret")
	}
	if _, _, err := validateCallback(ctx, cfg.URL, resolve); err != nil {
		return err
	}
	tr := pushTransport(cfg.URL, resolve, (&net.Dialer{Timeout: 3 * time.Second}).DialContext)
	defer tr.CloseIdleConnections()
	hc := &http.Client{Transport: tr, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return deliver(ctx, hc, cfg, event)
}
func deliver(ctx context.Context, hc *http.Client, cfg PushConfig, event PushEvent) error {
	b, err := json.Marshal(event)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", cfg.URL, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Authentication.Credentials)
	req.Header.Set("Content-Type", "application/a2a+json")
	res, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("push status %d", res.StatusCode)
	}
	return nil
}
func (p *Peer) pushHTTP(w http.ResponseWriter, r *http.Request, token, taskID, tail string) {
	who, err := p.authorize(token, "push")
	if err != nil {
		replyHTTP(w, nil, err)
		return
	}
	// Check ownership before parsing the callback URL or attempting any DNS lookup.
	err = (store{p.Dir}).tx(func(d *database) error { _, err := owned(d, who, taskID); return err })
	if err != nil {
		replyHTTP(w, nil, err)
		return
	}
	base := taskID + "/pushNotificationConfigs"
	switch {
	case r.Method == "POST" && tail == base:
		var cfg PushConfig
		if err = decode(r, &cfg); err != nil {
			replyHTTP(w, nil, err)
			return
		}
		// Only the Bearer authentication scheme is supported by this subset.
		if !strings.EqualFold(cfg.Authentication.Scheme, "Bearer") || cfg.Authentication.Credentials == "" {
			replyHTTP(w, nil, errors.New("missing dedicated secret"))
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		lookup := p.callbackLookup
		if lookup == nil {
			lookup = resolve
		}
		if _, _, err = validateCallback(ctx, cfg.URL, lookup); err != nil {
			replyHTTP(w, nil, err)
			return
		}
		cfg.ID = id()
		err = (store{p.Dir}).tx(func(d *database) error {
			t, err := owned(d, who, taskID)
			if err != nil {
				return err
			}
			if t.Push != nil {
				return ErrConflict
			} // one config in this bounded fixture; never silently replace it
			t.Push = &cfg
			d.Tasks[t.ID] = t
			return nil
		})
		cfg.Authentication.Credentials = ""
		replyHTTP(w, cfg, err)
	case r.Method == "GET" && tail == base:
		out := []PushConfig{}
		err = (store{p.Dir}).tx(func(d *database) error {
			t, err := owned(d, who, taskID)
			if err != nil {
				return err
			}
			if t.Push != nil {
				cfg := *t.Push
				cfg.Authentication.Credentials = ""
				out = append(out, cfg)
			}
			return nil
		})
		replyHTTP(w, map[string]any{"configs": out}, err)
	case r.Method == "GET" && strings.HasPrefix(tail, base+"/"):
		var cfg PushConfig
		configID := strings.TrimPrefix(tail, base+"/")
		err = (store{p.Dir}).tx(func(d *database) error {
			t, err := owned(d, who, taskID)
			if err != nil {
				return err
			}
			if t.Push == nil || t.Push.ID != configID {
				return ErrDenied
			}
			cfg = *t.Push
			cfg.Authentication.Credentials = ""
			return nil
		})
		replyHTTP(w, cfg, err)
	case r.Method == "DELETE" && strings.HasPrefix(tail, base+"/"):
		configID := strings.TrimPrefix(tail, base+"/")
		err = (store{p.Dir}).tx(func(d *database) error {
			t, err := owned(d, who, taskID)
			if err != nil {
				return err
			}
			if t.Push == nil {
				return nil
			} // deletion has no effect when already absent
			if t.Push.ID != configID {
				return ErrDenied
			}
			t.Push = nil
			d.Tasks[t.ID] = t
			return nil
		})
		replyHTTP(w, map[string]any{}, err)
	default:
		http.NotFound(w, r)
	}
}

// DeliveryID and Revision are explicit local extension fields, not standard A2A
// guarantees. A receiver without this contract must poll an authenticated snapshot.
type PushEvent struct {
	ContextID  string
	TaskID     string `json:"taskId"`
	Tenant     string `json:"tenant"`
	DeliveryID string `json:"deliveryId"`
	Revision   int    `json:"revision"`
	State      string `json:"state"`
}
type Receiver struct {
	Dir, Token string
	Owner      Identity
	IntentID   string
}

func (x Receiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	if x.Token == "" || subtle.ConstantTimeCompare([]byte(bearer(r.Header.Get("Authorization"))), []byte(x.Token)) != 1 {
		w.WriteHeader(401)
		return
	}
	var event PushEvent
	if err := decode(r, &event); err != nil {
		w.WriteHeader(400)
		return
	}
	if event.DeliveryID == "" || event.Revision < 1 || !validState(event.State) {
		w.WriteHeader(400)
		return
	}
	err := (store{x.Dir}).tx(func(d *database) error {
		k := key(x.Owner.Tenant, x.Owner.Subject, x.IntentID)
		rec, ok := d.Intents[k]
		if !ok || rec.TaskID == "" || rec.TaskID != event.TaskID || rec.ContextID != event.ContextID || x.Owner.Tenant != event.Tenant {
			return ErrDenied
		}
		dk := key(x.Owner.Tenant, event.TaskID, event.DeliveryID)
		hash := digest(event)
		if prev, ok := d.Deliveries[dk]; ok {
			if prev != hash {
				return ErrConflict
			}
			return nil
		}
		// Push is a hint. It never resolves UNKNOWN or authorizes side effects.
		// Authenticated GET remains the authority for task snapshots/recovery.
		d.Deliveries[dk] = hash
		return nil
	})
	if err != nil {
		code := 409
		if errors.Is(err, ErrDenied) {
			code = 404
		}
		w.WriteHeader(code)
		return
	}
	w.WriteHeader(200) // only after durable receipt; lost ACK can be retried
}

func pushTransport(raw string, lookup lookupFunc, dial func(context.Context, string, string) (net.Conn, error)) *http.Transport {
	return &http.Transport{Proxy: nil, TLSHandshakeTimeout: 3 * time.Second, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		u, addrs, err := validateCallback(ctx, raw, lookup)
		if err != nil {
			return nil, err
		}
		host, _, err := net.SplitHostPort(address)
		if err != nil || !strings.EqualFold(host, u.Hostname()) {
			return nil, errors.New("unexpected dial target")
		}
		return dial(ctx, network, net.JoinHostPort(addrs[0].String(), "443"))
	}}
}

// Application delivery metadata stays in the standard statusUpdate envelope.
type deliveryMeta struct {
	Tenant     string `json:"tenant"`
	DeliveryID string `json:"delivery_id"`
	Revision   int    `json:"revision"`
}
type statusEvent struct {
	TaskID    string       `json:"taskId"`
	ContextID string       `json:"contextId"`
	Status    Status       `json:"status"`
	Metadata  deliveryMeta `json:"metadata"`
}
type pushWire struct {
	StatusUpdate *statusEvent `json:"statusUpdate"`
}

func (e PushEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(pushWire{&statusEvent{e.TaskID, e.ContextID, Status{State: e.State}, deliveryMeta{e.Tenant, e.DeliveryID, e.Revision}}})
}
func (e *PushEvent) UnmarshalJSON(b []byte) error {
	var v pushWire
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return err
	}
	if v.StatusUpdate == nil {
		return errors.New("statusUpdate required")
	}
	x := v.StatusUpdate
	*e = PushEvent{TaskID: x.TaskID, ContextID: x.ContextID, Tenant: x.Metadata.Tenant, DeliveryID: x.Metadata.DeliveryID, Revision: x.Metadata.Revision, State: x.Status.State}
	return nil
}
