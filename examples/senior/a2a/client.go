package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Intent struct {
	PeerID        string
	RetryContract bool
	Owner         Identity
	IntentID      string
	Input         Input
	Message       Message
	TaskID        string
	ContextID     string
	State         string
	Result        *Message
}
type Client struct {
	PeerID              string // stable target identity from trusted configuration; loopback ports may change on restart
	Dir, BaseURL, Token string
	Owner               Identity // from the trusted host identity; never model arguments
	HTTP                *http.Client
	// RetryByMessageID must be explicitly configured from a pinned application contract.
	RetryByMessageID bool
	// BeforeSend faults after the UNKNOWN reservation, before dispatch.
	BeforeSend func()
}

func (c *Client) exchange(ctx context.Context, method, path string, in, out any) error {
	base, err := url.Parse(c.BaseURL)
	if err != nil || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return errors.New("invalid pinned endpoint")
	}
	if base.Scheme != "https" && !(base.Scheme == "http" && (base.Hostname() == "127.0.0.1" || base.Hostname() == "::1")) {
		return errors.New("use HTTPS or explicit loopback fixture")
	}
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.BaseURL, "/")+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("A2A-Version", "1.0")
	req.Header.Set("Content-Type", "application/a2a+json")
	hc := http.Client{Timeout: 3 * time.Second}
	if c.HTTP != nil {
		hc = *c.HTTP
	}
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, (64<<10)+1))
	if err != nil {
		return err
	}
	if len(data) > 64<<10 {
		return errors.New("oversized response")
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("remote status %d", res.StatusCode)
	}
	return json.Unmarshal(data, out)
}
func (c *Client) index(intentID string) string { return key(c.Owner.Tenant, c.Owner.Subject, intentID) }
func (c *Client) read(intentID string) (Intent, error) {
	var rec Intent
	err := (store{c.Dir}).tx(func(d *database) error {
		var ok bool
		rec, ok = d.Intents[c.index(intentID)]
		if ok && (rec.PeerID != c.PeerID || rec.RetryContract != c.RetryByMessageID) {
			return ErrConflict
		}
		if !ok {
			return errors.New("intent not found")
		}
		return nil
	})
	return rec, err
}
func (c *Client) Submit(ctx context.Context, intentID string, in Input) (Intent, error) {
	if c.PeerID == "" || intentID == "" || c.Owner.Tenant == "" || c.Owner.Subject == "" || in.Action == "" {
		return Intent{}, errors.New("missing trusted identity or input")
	}
	var rec Intent
	created := false
	err := (store{c.Dir}).tx(func(d *database) error {
		if prev, ok := d.Intents[c.index(intentID)]; ok {
			if prev.PeerID != c.PeerID || prev.RetryContract != c.RetryByMessageID || prev.Input != in {
				return ErrConflict
			}
			rec = prev
			return nil
		}
		rec = Intent{PeerID: c.PeerID, RetryContract: c.RetryByMessageID, Owner: c.Owner, IntentID: intentID, Input: in, State: "UNKNOWN", Message: Message{MessageID: id(), Role: "ROLE_USER", Parts: []Part{{Data: &in}}}}
		d.Intents[c.index(intentID)] = rec
		created = true
		return nil
	})
	if err != nil {
		return rec, err
	}
	if !created {
		if rec.State == "UNKNOWN" {
			return rec, ErrUnknown
		}
		return rec, nil
	}
	if c.BeforeSend != nil {
		c.BeforeSend()
	}
	return c.send(ctx, rec)
}
func (c *Client) send(ctx context.Context, rec Intent) (Intent, error) {
	var response Reply
	if err := c.exchange(ctx, "POST", "/message:send", map[string]any{"message": rec.Message}, &response); err != nil {
		return rec, fmt.Errorf("%w: %v", ErrUnknown, err)
	}
	if (response.Task == nil) == (response.Message == nil) {
		return rec, ErrUnknown
	}
	err := (store{c.Dir}).tx(func(d *database) error {
		current := d.Intents[c.index(rec.IntentID)]
		if current.Message.MessageID != rec.Message.MessageID || current.Input != rec.Input {
			return ErrConflict
		}
		if response.Task != nil {
			if response.Task.ID == "" || response.Task.ContextID == "" || !validState(response.Task.Status.State) {
				return ErrUnknown
			}
			if current.TaskID != "" && (current.TaskID != response.Task.ID || current.ContextID != response.Task.ContextID) {
				return ErrConflict
			}
			if terminal(current.State) && current.State != response.Task.Status.State {
				rec = current
				return nil
			}
			current.ContextID = response.Task.ContextID
			current.TaskID = response.Task.ID
			current.State = response.Task.Status.State
		} else {
			m := response.Message
			if current.TaskID != "" || m.MessageID == "" || m.ContextID == "" || m.TaskID != "" || m.Role != "ROLE_AGENT" || len(m.Parts) == 0 {
				return ErrUnknown
			}
			current.Result = m
			current.State = Completed
		}
		d.Intents[c.index(rec.IntentID)] = current
		rec = current
		return nil
	})
	return rec, err
}
func validState(s string) bool {
	return s == Working || s == InputRequired || s == AuthRequired || s == "TASK_STATE_SUBMITTED" || terminal(s)
}
func (c *Client) Reconcile(ctx context.Context, intentID string) (Intent, error) {
	rec, err := c.read(intentID)
	if err != nil {
		return rec, err
	}
	if rec.TaskID != "" {
		var task Task
		if err = c.exchange(ctx, "GET", "/tasks/"+url.PathEscape(rec.TaskID), nil, &task); err != nil {
			return rec, err
		}
		if task.ID != rec.TaskID || task.ContextID != rec.ContextID || !validState(task.Status.State) {
			return rec, errors.New("invalid task snapshot")
		}
		// A nonterminal snapshot cannot prove an ambiguous follow-up was not accepted.
		if rec.State == "UNKNOWN" && !terminal(task.Status.State) {
			if c.RetryByMessageID {
				return c.recoverMessage(ctx, rec)
			}
			return rec, ErrUnknown
		}
		err = (store{c.Dir}).tx(func(d *database) error {
			current := d.Intents[c.index(intentID)]
			if terminal(current.State) && current.State != task.Status.State {
				return ErrConflict
			}
			current.State = task.Status.State
			d.Intents[c.index(intentID)] = current
			rec = current
			return nil
		})
		return rec, err
	}
	if rec.State != "UNKNOWN" {
		return rec, nil
	}
	if !c.RetryByMessageID {
		return rec, ErrUnknown
	}
	return c.recoverMessage(ctx, rec) // exact persisted message; remote must deduplicate durably
}

// RecoverStream deliberately supports snapshot polling only. A2A does not promise
// a portable resume cursor; no event history is synthesized from a snapshot.
func (c *Client) RecoverStream(ctx context.Context, intentID, cursor string) (Intent, error) {
	if cursor != "" {
		return Intent{}, errors.New("UnsupportedOperation: replay cursor not negotiated")
	}
	return c.Reconcile(ctx, intentID)
}

// Continue creates one new message only after an explicit caller decision at an
// interrupted task. UNKNOWN must be reconciled before another message is planned.
func (c *Client) Continue(ctx context.Context, intentID string, in Input) (Intent, error) {
	var rec Intent
	err := (store{c.Dir}).tx(func(d *database) error {
		var ok bool
		rec, ok = d.Intents[c.index(intentID)]
		if !ok {
			return errors.New("intent not found")
		}
		if rec.PeerID != c.PeerID || rec.RetryContract != c.RetryByMessageID {
			return ErrConflict
		}
		if rec.State == "UNKNOWN" {
			return ErrUnknown
		}
		if rec.TaskID == "" || (rec.State != AuthRequired && rec.State != InputRequired) {
			return errors.New("task is not awaiting input or authorization")
		}
		rec.Input = in
		rec.State = "UNKNOWN"
		rec.Message = Message{MessageID: id(), TaskID: rec.TaskID, ContextID: rec.ContextID, Role: "ROLE_USER", Parts: []Part{{Data: &in}}}
		d.Intents[c.index(intentID)] = rec
		return nil
	})
	if err != nil {
		return rec, err
	}
	if c.BeforeSend != nil {
		c.BeforeSend()
	}
	return c.send(ctx, rec)
}

func (c *Client) recoverMessage(ctx context.Context, rec Intent) (Intent, error) {
	out, err := c.send(ctx, rec)
	if err != nil {
		return out, err
	}
	// A dedup receipt can be an old snapshot. After learning the task ID, fetch
	// its current authenticated state before inviting any further user action.
	if out.TaskID != "" {
		return c.Reconcile(ctx, rec.IntentID)
	}
	return out, nil
}
