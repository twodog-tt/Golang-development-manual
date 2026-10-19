// Package mediaio contains local media protocol fixtures, not a general provider SDK.
package mediaio

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"time"
)

type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail"`
}
type InputAudio struct {
	Data   string `json:"data"`
	Format string `json:"format"`
}
type Part struct {
	Type  string      `json:"type"`
	Text  string      `json:"text,omitempty"`
	Image *ImageURL   `json:"image_url,omitempty"`
	Audio *InputAudio `json:"input_audio,omitempty"`
}

func (p Part) MarshalJSON() ([]byte, error) {
	count := 0
	if p.Text != "" {
		count++
	}
	if p.Image != nil {
		count++
	}
	if p.Audio != nil {
		count++
	}
	if count != 1 {
		return nil, errors.New("exactly one content variant required")
	}
	switch p.Type {
	case "text":
		if p.Text == "" || len(p.Text) > 4096 {
			return nil, errors.New("invalid text")
		}
	case "image_url":
		if p.Image == nil {
			return nil, errors.New("image variant required")
		}
		u, e := url.Parse(p.Image.URL)
		if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || (p.Image.Detail != "low" && p.Image.Detail != "high" && p.Image.Detail != "auto") {
			return nil, errors.New("invalid fixture image URL/detail")
		}
	case "input_audio":
		if p.Audio == nil || p.Audio.Format != "wav" {
			return nil, errors.New("fixture accepts WAV input only")
		}
		if len(p.Audio.Data) > 128<<10 {
			return nil, errors.New("audio base64 too large")
		}
		b, e := base64.StdEncoding.Strict().DecodeString(p.Audio.Data)
		if e != nil {
			return nil, e
		}
		m, e := Inspect(nil, b, Limits{})
		if e != nil || m.MIME != "audio/wav" {
			return nil, errors.New("invalid WAV input data")
		}
	default:
		return nil, errors.New("unsupported content type")
	}
	type wire Part
	return json.Marshal(wire(p))
}

// MarshalMessage serializes a bounded Chat-Completions-style user message only.
// It does not mutate llmclient.Message or transmit media to any provider.
func MarshalMessage(parts []Part) ([]byte, error) {
	if len(parts) == 0 || len(parts) > 8 {
		return nil, errors.New("invalid part count")
	}
	b, e := json.Marshal(struct {
		Role    string `json:"role"`
		Content []Part `json:"content"`
	}{"user", parts})
	if e == nil && len(b) > 192<<10 {
		return nil, errors.New("message too large")
	}
	return b, e
}

type LocalClient struct {
	base string
	http *http.Client
}

func NewLocalClient(endpoint string) (*LocalClient, error) {
	u, e := url.Parse(endpoint)
	if e != nil {
		return nil, e
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "http" || ip == nil || !ip.IsLoopback() || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return nil, errors.New("literal loopback HTTP origin required")
	}
	tr := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: time.Second}).DialContext, ResponseHeaderTimeout: time.Second, IdleConnTimeout: time.Minute}
	return &LocalClient{u.String(), &http.Client{Transport: tr, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *LocalClient) Close() { c.http.CloseIdleConnections() }
