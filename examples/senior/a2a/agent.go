package a2a

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	Working       = "TASK_STATE_WORKING"
	InputRequired = "TASK_STATE_INPUT_REQUIRED"
	AuthRequired  = "TASK_STATE_AUTH_REQUIRED"
	Completed     = "TASK_STATE_COMPLETED"
	Canceled      = "TASK_STATE_CANCELED"
	Failed        = "TASK_STATE_FAILED"
	Rejected      = "TASK_STATE_REJECTED"
)

var ErrDenied = errors.New("resource unavailable or permission denied")
var ErrNotCancelable = errors.New("TaskNotCancelable: task is already terminal")
var ErrUnknown = errors.New("UNKNOWN: reconcile required; automatic submit denied")
var ErrConflict = errors.New("intent or message payload conflict")

func terminal(s string) bool { return s == Completed || s == Canceled || s == Failed || s == Rejected }
func id() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func digest(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func key(v ...string) string { b, _ := json.Marshal(v); return string(b) }

type Identity struct{ Tenant, Subject string }
type Grant struct {
	Identity   Identity
	Operations []string
	Expires    time.Time
}
type Input struct {
	Action  string `json:"action"`
	Payload string `json:"payload"`
}
type Part struct {
	Data *Input `json:"data,omitempty"`
	Text string `json:"text,omitempty"`
}
type Message struct {
	MessageID string `json:"messageId"`
	TaskID    string `json:"taskId,omitempty"`
	ContextID string `json:"contextId,omitempty"`
	Role      string `json:"role"`
	Parts     []Part `json:"parts"`
}
type Status struct {
	State   string   `json:"state"`
	Message *Message `json:"message,omitempty"`
}
type Task struct {
	ID        string `json:"id"`
	ContextID string `json:"contextId"`
	Status    Status `json:"status"`
	// Ownership and approval belong to the local agent, never the wire contract.
	Owner    Identity    `json:"-"`
	Input    Input       `json:"-"`
	Revision int         `json:"-"`
	Approval *Approval   `json:"-"`
	Push     *PushConfig `json:"-"`
}

// Separate persistent representation preserves private fields without exposing them on wire.
func (t Task) MarshalJSON() ([]byte, error) {
	type wire Task
	return json.Marshal(struct {
		wire
		Owner    Identity
		Input    Input
		Revision int
		Approval *Approval
		Push     *PushConfig
	}{wire(t), t.Owner, t.Input, t.Revision, t.Approval, t.Push})
}
func (t *Task) UnmarshalJSON(b []byte) error {
	type wire Task
	var v struct {
		wire
		Owner    Identity
		Input    Input
		Revision int
		Approval *Approval
		Push     *PushConfig
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*t = Task(v.wire)
	t.Owner = v.Owner
	t.Input = v.Input
	t.Revision = v.Revision
	t.Approval = v.Approval
	t.Push = v.Push
	return nil
}

type Approval struct {
	Owner    Identity
	Approver Identity
	Digest   string
	Revision int
	Expires  time.Time
}
type Reply struct {
	Task    *Task    `json:"task,omitempty"`
	Message *Message `json:"message,omitempty"`
}
type savedReply struct {
	Digest string
	Reply  Reply
}

type Peer struct {
	Dir      string
	Endpoint string
	// Deduplicate is an explicit local application contract, not standard A2A behavior.
	Deduplicate    bool
	mu             sync.RWMutex
	grants         map[string]Grant
	callbackLookup lookupFunc // nil uses the system resolver; tests supply deterministic DNS
	// AfterSend enables a real process-exit/lost-reply fault after durable remote commit.
	AfterSend func()
}

func NewPeer(dir string, deduplicate bool) *Peer {
	return &Peer{Dir: dir, Deduplicate: deduplicate, grants: map[string]Grant{}}
}
func (p *Peer) Put(token string, g Grant) {
	p.mu.Lock()
	defer p.mu.Unlock()
	g.Operations = append([]string(nil), g.Operations...)
	p.grants[token] = g
}
func (p *Peer) Revoke(token string) { p.mu.Lock(); defer p.mu.Unlock(); delete(p.grants, token) }
func (p *Peer) authorize(token, op string) (Identity, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	g, ok := p.grants[token]
	if !ok || !g.Expires.After(time.Now()) || g.Identity.Tenant == "" || g.Identity.Subject == "" {
		return Identity{}, ErrDenied
	}
	for _, x := range g.Operations {
		if x == op {
			return g.Identity, nil
		}
	}
	return Identity{}, ErrDenied
}
func owned(d *database, who Identity, taskID string) (Task, error) {
	t, ok := d.Tasks[taskID]
	if !ok || t.Owner != who {
		return Task{}, ErrDenied
	}
	return t, nil
}
func (p *Peer) Send(token string, m Message) (Reply, error) {
	who, err := p.authorize(token, "send")
	if err != nil {
		return Reply{}, err
	}
	if m.MessageID == "" || m.Role != "ROLE_USER" || len(m.Parts) != 1 || m.Parts[0].Data == nil || m.Parts[0].Text != "" {
		return Reply{}, errors.New("invalid message")
	}
	in := *m.Parts[0].Data
	if (in.Action != "publish" && in.Action != "echo" && in.Action != "clarify") || in.Payload == "" || len(in.Payload) > 4096 {
		return Reply{}, errors.New("invalid input")
	}
	var out Reply
	err = (store{p.Dir}).tx(func(d *database) error {
		if d.Deduplicate == nil {
			b := p.Deduplicate
			d.Deduplicate = &b
		} else if *d.Deduplicate != p.Deduplicate {
			return errors.New("deduplication contract changed; migration required")
		}
		k := key(who.Tenant, who.Subject, m.MessageID)
		hash := digest(m)
		if prev, ok := d.Messages[k]; ok && p.Deduplicate {
			if prev.Digest != hash {
				return ErrConflict
			}
			out = prev.Reply
			return nil
		}
		if m.TaskID == "" && in.Action == "echo" {
			out.Message = &Message{MessageID: id(), ContextID: id(), Role: "ROLE_AGENT", Parts: []Part{{Text: in.Payload}}}
		} else {
			var t Task
			if m.TaskID == "" {
				if m.ContextID != "" {
					return errors.New("client context IDs unsupported in this subset")
				}
				state := AuthRequired
				if in.Action == "clarify" {
					state = InputRequired
				}
				t = Task{ID: id(), ContextID: id(), Owner: who, Input: in, Revision: 1, Status: Status{State: state}}
			} else {
				var err error
				t, err = owned(d, who, m.TaskID)
				if err != nil {
					return err
				}
				if m.ContextID != "" && m.ContextID != t.ContextID {
					return ErrDenied
				}
				if terminal(t.Status.State) {
					return errors.New("terminal task cannot accept messages")
				}
				if t.Input != in {
					t.Input = in
					t.Revision++
					t.Approval = nil
					t.Status.State = AuthRequired
				} else {
					a := t.Approval
					if a == nil || !p.allowed(a.Approver, "approve") || a.Owner != who || a.Revision != t.Revision || a.Digest != digest(in) || !a.Expires.After(time.Now()) {
						// A valid caller without operation approval remains interrupted.
						// Persist this response under messageId, so a lost denial reply
						// can be recovered without later executing that old message.
						if a != nil {
							t.Revision++
						}
						t.Approval = nil
						t.Status.State = AuthRequired
					} else {
						t.Approval = nil
						t.Revision++
						t.Status.State = Completed
						d.Effects++
					}
				}
			}
			d.Tasks[t.ID] = t
			out.Task = &t
		}
		if p.Deduplicate {
			d.Messages[k] = savedReply{Digest: hash, Reply: out}
		}
		return nil
	})
	if err == nil && p.AfterSend != nil {
		p.AfterSend()
	}
	return out, err
}

// Approve is a trusted operator API (not an unauthenticated A2A endpoint).
// The operator must pass the digest and revision actually reviewed.
func (p *Peer) Approve(token, taskID, reviewedDigest string, revision int, expires time.Time) error {
	who, err := p.authorize(token, "approve")
	if err != nil {
		return err
	}
	return (store{p.Dir}).tx(func(d *database) error {
		t, ok := d.Tasks[taskID]
		if !ok || t.Owner.Tenant != who.Tenant || t.Owner.Subject == who.Subject {
			return ErrDenied
		}
		if terminal(t.Status.State) || t.Status.State != AuthRequired || digest(t.Input) != reviewedDigest || t.Revision != revision || !expires.After(time.Now()) {
			return ErrConflict
		}
		t.Approval = &Approval{Owner: t.Owner, Approver: who, Digest: reviewedDigest, Revision: revision, Expires: expires}
		d.Tasks[t.ID] = t
		return nil
	})
}
func (p *Peer) RevokeApproval(token, taskID string) error {
	who, err := p.authorize(token, "approve")
	if err != nil {
		return err
	}
	return (store{p.Dir}).tx(func(d *database) error {
		t, ok := d.Tasks[taskID]
		if !ok || t.Owner.Tenant != who.Tenant {
			return ErrDenied
		}
		t.Approval = nil
		t.Revision++
		d.Tasks[t.ID] = t
		return nil
	})
}
func (p *Peer) Get(token, taskID string) (Task, error) {
	who, err := p.authorize(token, "get")
	if err != nil {
		return Task{}, err
	}
	var t Task
	err = (store{p.Dir}).tx(func(d *database) error { var err error; t, err = owned(d, who, taskID); return err })
	return t, err
}
func (p *Peer) Cancel(token, taskID string) (Task, error) {
	who, err := p.authorize(token, "cancel")
	if err != nil {
		return Task{}, err
	}
	var t Task
	err = (store{p.Dir}).tx(func(d *database) error {
		var err error
		t, err = owned(d, who, taskID)
		if err != nil {
			return err
		}
		if terminal(t.Status.State) && t.Status.State != Canceled {
			return ErrNotCancelable
		}
		if !terminal(t.Status.State) {
			t.Status.State = Canceled
			t.Approval = nil
			t.Revision++
			d.Tasks[t.ID] = t
		}
		return nil
	})
	return t, err
}
func (p *Peer) Effects() (int, error) {
	var n int
	err := (store{p.Dir}).tx(func(d *database) error { n = d.Effects; return nil })
	return n, err
}

// wireTask strips all private state; Task.MarshalJSON is persistence-only.
func wireTask(t Task) any {
	if t.Status.State == AuthRequired || t.Status.State == InputRequired {
		text := "Additional input required"
		if t.Status.State == AuthRequired {
			text = "Local operator approval required; use the trusted out-of-band review channel"
		}
		t.Status.Message = &Message{MessageID: digest([]any{t.ID, t.Revision, "status"}), TaskID: t.ID, ContextID: t.ContextID, Role: "ROLE_AGENT", Parts: []Part{{Text: text}}}
	}
	return struct {
		ID        string `json:"id"`
		ContextID string `json:"contextId"`
		Status    Status `json:"status"`
	}{t.ID, t.ContextID, t.Status}
}
func wireReply(r Reply) any {
	if r.Task != nil {
		return map[string]any{"task": wireTask(*r.Task)}
	}
	return map[string]any{"message": r.Message}
}
func decode(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return errors.New("trailing or oversized body")
	}
	return nil
}
func replyHTTP(w http.ResponseWriter, v any, err error) {
	w.Header().Set("Content-Type", "application/a2a+json")
	if err != nil {
		code := 400
		if errors.Is(err, ErrDenied) {
			code = 404
		}
		if errors.Is(err, ErrConflict) {
			code = 409
		}
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": code, "message": err.Error()}})
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}
func (p *Peer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/.well-known/agent-card.json" && r.Method == "GET" {
		if p.Endpoint == "" {
			replyHTTP(w, nil, errors.New("pinned endpoint not configured"))
			return
		}
		replyHTTP(w, map[string]any{"name": "local recovery fixture", "version": "1.0.0", "description": "bounded local A2A subset", "supportedInterfaces": []any{map[string]any{"url": p.Endpoint, "protocolBinding": "HTTP+JSON", "protocolVersion": "1.0"}}, "capabilities": map[string]any{"streaming": false, "pushNotifications": false, "extensions": []any{map[string]any{"uri": "urn:manual:callback-fixture:v1", "description": "local callback rehearsal at /fixture/tasks/...; no automatic push delivery", "required": false}}}, "defaultInputModes": []string{"application/json"}, "defaultOutputModes": []string{"application/json"}, "skills": []any{map[string]any{"id": "publish", "name": "publish", "description": "local approval fixture", "tags": []string{"demo"}}}, "securitySchemes": map[string]any{"local": map[string]any{"httpAuthSecurityScheme": map[string]string{"scheme": "Bearer"}}}, "security": []any{map[string]any{"local": []string{}}}}, nil)
		return
	}
	if r.Header.Get("A2A-Version") != "1.0" {
		replyHTTP(w, nil, errors.New("VersionNotSupported: select 1.0 explicitly"))
		return
	}
	token := bearer(r.Header.Get("Authorization"))
	if _, err := p.authenticate(token); err != nil {
		w.Header().Set("WWW-Authenticate", "Bearer")
		w.WriteHeader(401)
		return
	}
	if r.Method == "POST" && r.URL.Path == "/message:send" {
		var req struct {
			Message Message `json:"message"`
		}
		if err := decode(r, &req); err != nil {
			replyHTTP(w, nil, err)
			return
		}
		out, err := p.Send(token, req.Message)
		replyHTTP(w, wireReply(out), err)
		return
	}
	if r.Method == "GET" && r.URL.Path == "/tasks" {
		if r.URL.RawQuery != "" {
			replyHTTP(w, nil, errors.New("list filters and pagination unsupported by this subset"))
			return
		}
		who, err := p.authorize(token, "list")
		if err != nil {
			replyHTTP(w, nil, err)
			return
		}
		out := []any{}
		err = (store{p.Dir}).tx(func(d *database) error {
			for _, t := range d.Tasks {
				if t.Owner == who {
					out = append(out, wireTask(t))
				}
			}
			return nil
		})
		replyHTTP(w, map[string]any{"tasks": out}, err)
		return
	}
	path := r.URL.Path
	fixtureCallback := strings.HasPrefix(path, "/fixture/tasks/")
	if fixtureCallback {
		path = strings.TrimPrefix(path, "/fixture")
	}
	if !strings.HasPrefix(path, "/tasks/") {
		http.NotFound(w, r)
		return
	}
	tail := strings.TrimPrefix(path, "/tasks/")
	taskID := strings.Split(strings.Split(tail, ":")[0], "/")[0]
	if fixtureCallback && !strings.HasPrefix(tail, taskID+"/pushNotificationConfigs") {
		http.NotFound(w, r)
		return
	}
	switch {
	case r.Method == "GET" && tail == taskID:
		t, err := p.Get(token, taskID)
		replyHTTP(w, wireTask(t), err)
	case r.Method == "POST" && tail == taskID+":cancel":
		t, err := p.Cancel(token, taskID)
		replyHTTP(w, wireTask(t), err)
	case r.Method == "POST" && tail == taskID+":subscribe":
		who, err := p.authorize(token, "subscribe")
		if err == nil {
			err = (store{p.Dir}).tx(func(d *database) error { _, e := owned(d, who, taskID); return e })
		}
		if err == nil {
			err = errors.New("UnsupportedOperation: polling only; no replay cursor")
		}
		replyHTTP(w, nil, err)
	case strings.HasPrefix(tail, taskID+"/pushNotificationConfigs"):
		if !fixtureCallback {
			replyHTTP(w, nil, errors.New("PushNotificationNotSupported: use the explicit local callback fixture"))
			return
		}
		p.pushHTTP(w, r, token, taskID, tail)
	default:
		http.NotFound(w, r)
	}
}
func (p *Peer) authenticate(token string) (Identity, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	g, ok := p.grants[token]
	if !ok || !g.Expires.After(time.Now()) {
		return Identity{}, ErrDenied
	}
	return g.Identity, nil
}

func (p *Peer) allowed(who Identity, op string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, g := range p.grants {
		if g.Identity == who && g.Expires.After(time.Now()) {
			for _, x := range g.Operations {
				if x == op {
					return true
				}
			}
		}
	}
	return false
}

// InputDigest is the operator review digest of the application's complete input.
func InputDigest(in Input) string { return digest(in) }

// Part is a oneof even when the text value happens to be empty.
func (p *Part) UnmarshalJSON(b []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		return err
	}
	if len(fields) != 1 {
		return errors.New("Part must contain exactly one supported value")
	}
	*p = Part{}
	if raw, ok := fields["data"]; ok {
		if string(raw) == "null" {
			return errors.New("null data")
		}
		dec := json.NewDecoder(strings.NewReader(string(raw)))
		dec.DisallowUnknownFields()
		var in Input
		if err := dec.Decode(&in); err != nil {
			return err
		}
		p.Data = &in
		return nil
	}
	if raw, ok := fields["text"]; ok {
		return json.Unmarshal(raw, &p.Text)
	}
	return errors.New("unsupported Part")
}

func bearer(header string) string {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}
