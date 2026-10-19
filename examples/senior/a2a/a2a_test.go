package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var alice = Identity{"tenant-a", "alice"}
var input = Input{"publish", "approved article"}

func grants(p *Peer) {
	p.Put("alice", Grant{alice, []string{"send", "get", "list", "cancel", "subscribe", "push"}, time.Now().Add(time.Hour)})
	p.Put("bob", Grant{Identity{"tenant-b", "bob"}, []string{"send", "get", "list", "cancel", "subscribe", "push", "approve"}, time.Now().Add(time.Hour)})
	p.Put("reviewer", Grant{Identity{"tenant-a", "reviewer"}, []string{"approve"}, time.Now().Add(time.Hour)})
	p.Put("same-tenant", Grant{Identity{"tenant-a", "other"}, []string{"get", "list", "cancel", "subscribe", "push", "send"}, time.Now().Add(time.Hour)})
}
func setup(t *testing.T, dedup bool) (*Peer, *Client, *httptest.Server) {
	t.Helper()
	dir := t.TempDir()
	p := NewPeer(filepath.Join(dir, "remote"), dedup)
	grants(p)
	ts := httptest.NewServer(p)
	p.Endpoint = ts.URL
	t.Cleanup(ts.Close)
	c := &Client{PeerID: "local-peer", Dir: filepath.Join(dir, "local"), BaseURL: ts.URL, Token: "alice", Owner: alice, RetryByMessageID: dedup}
	return p, c, ts
}
func submit(t *testing.T, c *Client, in Input) Intent {
	t.Helper()
	rec, err := c.Submit(context.Background(), id(), in)
	if err != nil {
		t.Fatal(err)
	}
	return rec
}
func approve(t *testing.T, p *Peer, rec Intent) {
	t.Helper()
	task, err := p.Get("alice", rec.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if err = p.Approve("reviewer", task.ID, digest(task.Input), task.Revision, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
}
func TestDirectMessageAndTaskVariants(t *testing.T) {
	_, c, _ := setup(t, true)
	r := submit(t, c, Input{"echo", "hello"})
	if r.TaskID != "" || r.State != Completed || r.Result == nil || r.Result.Parts[0].Text != "hello" {
		t.Fatalf("direct Message lost: %+v", r)
	}
	if _, err := c.Continue(context.Background(), r.IntentID, input); err == nil {
		t.Fatal("invented task for direct Message")
	}
	for _, in := range []Input{input, {"clarify", "missing details"}} {
		r = submit(t, c, in)
		want := AuthRequired
		if in.Action == "clarify" {
			want = InputRequired
		}
		if r.State != want || r.TaskID == "" {
			t.Fatalf("%+v", r)
		}
	}
}
func TestUnknownFirstSubmitAndDurableDedup(t *testing.T) {
	for _, dedup := range []bool{false, true} {
		t.Run(map[bool]string{false: "no-contract", true: "pinned-contract"}[dedup], func(t *testing.T) {
			p, c, ts := setup(t, dedup)
			// Drop the response after the remote journal commit, not before dispatch.
			ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/message:send" {
					rr := httptest.NewRecorder()
					p.ServeHTTP(rr, r)
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						panic(err)
					}
					_ = conn.Close()
					return
				}
				p.ServeHTTP(w, r)
			})
			rec, err := c.Submit(context.Background(), "intent", input)
			if !errors.Is(err, ErrUnknown) || rec.TaskID != "" {
				t.Fatalf("%+v %v", rec, err)
			}
			for i := 0; i < 3; i++ {
				if _, err = c.Submit(context.Background(), "intent", input); !errors.Is(err, ErrUnknown) {
					t.Fatal("blind resubmit allowed")
				}
			}
			if _, err = c.Submit(context.Background(), "intent", Input{"publish", "changed"}); !errors.Is(err, ErrConflict) {
				t.Fatal("payload drift allowed")
			}
			ts.Close()
			restarted := NewPeer(p.Dir, dedup)
			grants(restarted)
			next := httptest.NewServer(restarted)
			defer next.Close()
			c = &Client{PeerID: "local-peer", Dir: c.Dir, BaseURL: next.URL, Token: "alice", Owner: alice, RetryByMessageID: dedup}
			rec, err = c.Reconcile(context.Background(), "intent")
			if dedup {
				if err != nil || rec.TaskID == "" {
					t.Fatalf("recovery %v", err)
				}
			} else if !errors.Is(err, ErrUnknown) {
				t.Fatal("uncontracted retry allowed")
			}
			_ = (store{p.Dir}).tx(func(d *database) error {
				if len(d.Tasks) != 1 {
					t.Errorf("duplicate remote tasks: %d", len(d.Tasks))
				}
				return nil
			})
		})
	}
}
func TestApprovalBindingAndSafeRecovery(t *testing.T) {
	for _, fault := range []string{"no-approval", "self-approval", "foreign-reviewer", "wrong-digest", "wrong-revision", "expired", "revoked", "reviewer-revoked", "changed-input", "permission-revoked", "good"} {
		t.Run(fault, func(t *testing.T) {
			p, c, _ := setup(t, true)
			rec := submit(t, c, input)
			task, _ := p.Get("alice", rec.TaskID)
			switch fault {
			case "no-approval":
			case "self-approval":
				p.Put("self", Grant{alice, []string{"approve"}, time.Now().Add(time.Hour)})
				if err := p.Approve("self", task.ID, digest(input), 1, time.Now().Add(time.Hour)); !errors.Is(err, ErrDenied) {
					t.Fatal("self review accepted")
				}
			case "foreign-reviewer":
				if err := p.Approve("bob", task.ID, digest(input), 1, time.Now().Add(time.Hour)); !errors.Is(err, ErrDenied) {
					t.Fatal("foreign reviewer accepted")
				}
			case "wrong-digest", "wrong-revision":
				h := digest(input)
				v := 1
				if fault == "wrong-digest" {
					h = digest(Input{"publish", "other"})
				} else {
					v = 2
				}
				if err := p.Approve("reviewer", task.ID, h, v, time.Now().Add(time.Hour)); !errors.Is(err, ErrConflict) {
					t.Fatal("stale review accepted")
				}
			default:
				approve(t, p, rec)
			}
			if fault == "expired" {
				_ = (store{p.Dir}).tx(func(d *database) error {
					x := d.Tasks[task.ID]
					x.Approval.Expires = time.Now().Add(-time.Minute)
					d.Tasks[x.ID] = x
					return nil
				})
			}
			if fault == "revoked" {
				if err := p.RevokeApproval("reviewer", task.ID); err != nil {
					t.Fatal(err)
				}
			}
			if fault == "reviewer-revoked" {
				p.Revoke("reviewer")
			}
			if fault == "permission-revoked" {
				p.Revoke("alice")
			}
			next := input
			if fault == "changed-input" {
				next.Payload = "unreviewed edit"
			}
			out, err := c.Continue(context.Background(), rec.IntentID, next)
			if fault == "good" {
				if err != nil || out.State != Completed {
					t.Fatalf("%+v %v", out, err)
				}
			} else {
				if fault == "permission-revoked" {
					if err == nil {
						t.Fatal("revoked caller accepted")
					}
				} else if err != nil || out.State != AuthRequired {
					t.Fatalf("missing approval did not interrupt: %+v %v", out, err)
				}
			}
			n, _ := p.Effects()
			want := 0
			if fault == "good" {
				want = 1
			}
			if n != want {
				t.Fatalf("effects %d", n)
			}
			if fault == "good" {
				if _, err = c.Continue(context.Background(), rec.IntentID, input); err == nil {
					t.Fatal("terminal resumed")
				}
			}
		})
	}
}
func wire(t *testing.T, base, token, method, path string, body any) (int, string) {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := http.NewRequest(method, base+path, bytes.NewReader(b))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("A2A-Version", "1.0")
	res, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(data)
}
func TestEveryOperationEnforcesOwnershipAndRevocation(t *testing.T) {
	p, c, ts := setup(t, true)
	rec := submit(t, c, input)
	ops := []struct {
		method, path string
		body         any
	}{{"GET", "/tasks/" + rec.TaskID, nil}, {"POST", "/tasks/" + rec.TaskID + ":cancel", nil}, {"POST", "/tasks/" + rec.TaskID + ":subscribe", nil}, {"POST", "/fixture/tasks/" + rec.TaskID + "/pushNotificationConfigs", callback("http://127.0.0.1", "callback")}, {"GET", "/fixture/tasks/" + rec.TaskID + "/pushNotificationConfigs", nil}, {"DELETE", "/fixture/tasks/" + rec.TaskID + "/pushNotificationConfigs/fake", nil}, {"POST", "/message:send", map[string]any{"message": Message{MessageID: id(), TaskID: rec.TaskID, Role: "ROLE_USER", Parts: []Part{{Data: &input}}}}}}
	for _, token := range []string{"bob", "same-tenant"} {
		for _, op := range ops {
			code, b := wire(t, ts.URL, token, op.method, op.path, op.body)
			if code != 404 || strings.Contains(b, rec.TaskID) || strings.Contains(b, "approved article") {
				t.Fatalf("ownership leak %s %s: %d %s", token, op.path, code, b)
			}
		}
		code, b := wire(t, ts.URL, token, "GET", "/tasks", nil)
		if code != 200 || strings.Contains(b, rec.TaskID) {
			t.Fatal("list leaked another owner's task")
		}
	}
	p.Put("alice", Grant{alice, []string{"send"}, time.Now().Add(time.Hour)})
	for _, op := range ops[:6] {
		code, _ := wire(t, ts.URL, "alice", op.method, op.path, op.body)
		if code != 404 {
			t.Fatal("operation scope ignored")
		}
	}
	p.Revoke("alice")
	for _, op := range ops {
		code, _ := wire(t, ts.URL, "alice", op.method, op.path, op.body)
		if code != 401 {
			t.Fatal("revoked token accepted")
		}
	}
}
func TestPollRecoveryCursorAndTerminalGuard(t *testing.T) {
	p, c, _ := setup(t, true)
	rec := submit(t, c, input)
	if _, err := c.RecoverStream(context.Background(), rec.IntentID, "invented-cursor"); err == nil {
		t.Fatal("unsupported replay accepted")
	}
	approve(t, p, rec)
	out, err := c.Continue(context.Background(), rec.IntentID, input)
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.RecoverStream(context.Background(), rec.IntentID, "")
	if err != nil || got.State != Completed {
		t.Fatal("poll recovery failed")
	}
	canceled, err := p.Cancel("alice", out.TaskID)
	if !errors.Is(err, ErrNotCancelable) || canceled.Status.State != Completed {
		t.Fatal("terminal task regressed")
	}
	n, _ := p.Effects()
	if n != 1 {
		t.Fatal("effect count")
	}
}
func TestConcurrentSubmitAndPayloadBoundDedup(t *testing.T) {
	p, c, _ := setup(t, true)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := c.Submit(context.Background(), "one", input)
			if err != nil && !errors.Is(err, ErrUnknown) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	_ = (store{p.Dir}).tx(func(d *database) error {
		if len(d.Tasks) != 1 {
			t.Errorf("tasks %d", len(d.Tasks))
		}
		return nil
	})
	rec, err := c.read("one")
	if err != nil {
		t.Fatal(err)
	}
	m := rec.Message
	m.Parts = []Part{{Data: &Input{"publish", "changed"}}}
	if _, err = p.Send("alice", m); !errors.Is(err, ErrConflict) {
		t.Fatal("same message ID accepted different payload")
	}
}
func TestNoPrivateTaskStateOnWireAndVersionGate(t *testing.T) {
	_, c, ts := setup(t, true)
	code, card := wire(t, ts.URL, "alice", "GET", "/.well-known/agent-card.json", nil)
	if code != 200 || !strings.Contains(card, `"pushNotifications":false`) || !strings.Contains(card, `"streaming":false`) || !strings.Contains(card, ts.URL) {
		t.Fatal("misleading capability card")
	}
	rec := submit(t, c, input)
	code, b := wire(t, ts.URL, "alice", "GET", "/tasks/"+rec.TaskID, nil)
	if code != 200 || strings.Contains(b, "Owner") || strings.Contains(b, "Approval") || strings.Contains(b, "approved article") {
		t.Fatal("private state leaked")
	}
	code, _ = wire(t, ts.URL, "alice", "POST", "/tasks/"+rec.TaskID+"/pushNotificationConfigs", callback("https://example.com", "secret"))
	if code != 400 {
		t.Fatal("standard Push advertised/accepted without automatic delivery")
	}
	req, _ := http.NewRequest("GET", ts.URL+"/tasks/"+rec.TaskID, nil)
	req.Header.Set("Authorization", "Bearer alice")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode == 200 {
		t.Fatal("version default silently assumed 1.0")
	}
}
func TestRealCrashRecovery(t *testing.T) {
	for _, stage := range []string{"before-send", "after-create", "after-effect"} {
		t.Run(stage, func(t *testing.T) {
			root := t.TempDir()
			p := NewPeer(filepath.Join(root, "remote"), true)
			grants(p)
			if stage == "after-effect" {
				ts := httptest.NewServer(p)
				c := &Client{PeerID: "local-peer", Dir: filepath.Join(root, "local"), BaseURL: ts.URL, Token: "alice", Owner: alice, RetryByMessageID: true}
				rec, err := c.Submit(context.Background(), "crash", input)
				if err != nil {
					t.Fatal(err)
				}
				approve(t, p, rec)
				ts.Close()
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestCrashHelper$")
			cmd.Env = append(os.Environ(), "A2A_CRASH_ROOT="+root, "A2A_CRASH_STAGE="+stage)
			b, err := cmd.CombinedOutput()
			var e *exec.ExitError
			if !errors.As(err, &e) || e.ExitCode() != 77 {
				t.Fatalf("crash helper %v %s", err, b)
			}
			p = NewPeer(p.Dir, true)
			grants(p)
			ts := httptest.NewServer(p)
			defer ts.Close()
			c := &Client{PeerID: "local-peer", Dir: filepath.Join(root, "local"), BaseURL: ts.URL, Token: "alice", Owner: alice, RetryByMessageID: true}
			rec, err := c.read("crash")
			if err != nil || rec.State != "UNKNOWN" {
				t.Fatalf("reservation lost: %+v %v", rec, err)
			}
			if _, err = c.Submit(context.Background(), "crash", input); !errors.Is(err, ErrUnknown) {
				t.Fatal("crash retried automatically")
			}
			rec, err = c.Reconcile(context.Background(), "crash")
			if err != nil {
				t.Fatal(err)
			}
			n, _ := p.Effects()
			want := 0
			state := AuthRequired
			if stage == "after-effect" {
				want = 1
				state = Completed
			}
			if n != want || rec.State != state {
				t.Fatalf("%+v effects=%d", rec, n)
			}
			_ = (store{p.Dir}).tx(func(d *database) error {
				if len(d.Tasks) != 1 {
					t.Errorf("tasks after restart %d", len(d.Tasks))
				}
				return nil
			})
		})
	}
}
func TestCrashHelper(t *testing.T) {
	root := os.Getenv("A2A_CRASH_ROOT")
	if root == "" {
		t.Skip("subprocess only")
	}
	stage := os.Getenv("A2A_CRASH_STAGE")
	p := NewPeer(filepath.Join(root, "remote"), true)
	grants(p)
	if stage != "before-send" {
		p.AfterSend = func() { os.Exit(77) }
	}
	ts := httptest.NewServer(p)
	defer ts.Close()
	c := &Client{PeerID: "local-peer", Dir: filepath.Join(root, "local"), BaseURL: ts.URL, Token: "alice", Owner: alice, RetryByMessageID: true}
	if stage == "before-send" {
		c.BeforeSend = func() { os.Exit(77) }
	}
	var err error
	if stage == "after-effect" {
		_, err = c.Continue(context.Background(), "crash", input)
	} else {
		_, err = c.Submit(context.Background(), "crash", input)
	}
	t.Fatalf("expected process exit: %v", err)
}

func TestMalformedMessagesAndReplyUnionFailClosed(t *testing.T) {
	p, c, ts := setup(t, true)
	for _, body := range []string{`{"message":{"messageId":"x","role":"ROLE_USER","parts":[{"data":{"action":"publish","payload":"x"},"text":""}]}}`, `{"message":{"messageId":"x","role":"ROLE_USER","parts":[{"data":null}]}}`, `{"message":{"messageId":"x","role":"ROLE_USER","parts":[{"data":{"action":"publish","payload":"x","tenant":"foreign"}}]}}`, `{} {}`, strings.Repeat(" ", 70<<10)} {
		req, _ := http.NewRequest("POST", ts.URL+"/message:send", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer alice")
		req.Header.Set("A2A-Version", "1.0")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		if res.StatusCode == 200 {
			t.Fatal("malformed input accepted")
		}
	}
	_ = (store{p.Dir}).tx(func(d *database) error {
		if len(d.Tasks) != 0 {
			t.Error("malformed message created a task")
		}
		return nil
	})
	// Conflicting top-level result alternatives must leave the durable reservation UNKNOWN.
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"task":{"id":"x","contextId":"y","status":{"state":"TASK_STATE_COMPLETED"}},"message":{"messageId":"m","contextId":"y","role":"ROLE_AGENT","parts":[{"text":"x"}]}}`)
	}))
	defer bad.Close()
	c.BaseURL = bad.URL
	rec, err := c.Submit(context.Background(), "bad-union", input)
	if !errors.Is(err, ErrUnknown) || rec.State != "UNKNOWN" {
		t.Fatal("ambiguous response advanced journal")
	}
}
func TestUnknownFollowupNonterminalSnapshotDoesNotClearQuarantine(t *testing.T) {
	p, c, _ := setup(t, false)
	rec := submit(t, c, input)
	c.BeforeSend = func() { panic("crash-before-followup") }
	func() {
		defer func() {
			if recover() == nil {
				t.Error("fault not reached")
			}
		}()
		_, _ = c.Continue(context.Background(), rec.IntentID, input)
	}()
	c.BeforeSend = nil
	got, err := c.Reconcile(context.Background(), rec.IntentID)
	if !errors.Is(err, ErrUnknown) || got.State != "UNKNOWN" {
		t.Fatal("nonterminal snapshot incorrectly proved non-execution")
	}
	if n, _ := p.Effects(); n != 0 {
		t.Fatal("unapproved effect")
	}
}
func TestCancellationAndJournalCorruption(t *testing.T) {
	p, c, _ := setup(t, true)
	rec := submit(t, c, input)
	approve(t, p, rec)
	for i := 0; i < 2; i++ {
		task, err := p.Cancel("alice", rec.TaskID)
		if err != nil || task.Status.State != Canceled {
			t.Fatal("cancel not idempotent")
		}
	}
	got, err := c.Reconcile(context.Background(), rec.IntentID)
	if err != nil || got.State != Canceled {
		t.Fatal("cancel reconciliation")
	}
	if _, err = c.Continue(context.Background(), rec.IntentID, input); err == nil {
		t.Fatal("canceled task resumed")
	}
	if err = os.WriteFile(filepath.Join(c.Dir, "state.json"), []byte(`{"Tasks":`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Submit(context.Background(), "corrupt", input); err == nil {
		t.Fatal("corrupt journal silently reset")
	}
	if n, _ := p.Effects(); n != 0 {
		t.Fatal("effect after cancellation")
	}
}

func TestDeniedOperationReceiptDoesNotInheritLaterApproval(t *testing.T) {
	p, c, _ := setup(t, true)
	rec := submit(t, c, input)
	waiting, err := c.Continue(context.Background(), rec.IntentID, input)
	if err != nil || waiting.State != AuthRequired {
		t.Fatal("operation must remain interrupted")
	}
	old := waiting.Message
	approve(t, p, waiting)
	response, err := p.Send("alice", old)
	if err != nil || response.Task.Status.State != AuthRequired {
		t.Fatal("old message inherited later approval")
	}
	if n, _ := p.Effects(); n != 0 {
		t.Fatal("replayed denial executed an effect")
	}
	done, err := c.Continue(context.Background(), rec.IntentID, input)
	if err != nil || done.State != Completed {
		t.Fatalf("explicit approved continuation failed: %+v %v", done, err)
	}
}

func TestRecoveryCannotChangePeerOrRetryContract(t *testing.T) {
	_, c, _ := setup(t, true)
	rec := submit(t, c, input)
	for _, change := range []string{"peer", "contract"} {
		copy := *c
		if change == "peer" {
			copy.PeerID = "other-peer"
		} else {
			copy.RetryByMessageID = false
		}
		if _, err := copy.Submit(context.Background(), rec.IntentID, input); !errors.Is(err, ErrConflict) {
			t.Fatal("submit changed target/contract")
		}
		if _, err := copy.Reconcile(context.Background(), rec.IntentID); !errors.Is(err, ErrConflict) {
			t.Fatal("recovery changed target/contract")
		}
		if _, err := copy.Continue(context.Background(), rec.IntentID, input); !errors.Is(err, ErrConflict) {
			t.Fatal("continue changed target/contract")
		}
	}
}

func TestRecoveredCreationReceiptRefreshesCurrentSnapshot(t *testing.T) {
	p, c, ts := setup(t, true)
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rr := httptest.NewRecorder()
		p.ServeHTTP(rr, r)
		http.Error(w, "lost reply", 503)
	})
	_, err := c.Submit(context.Background(), "late", input)
	if !errors.Is(err, ErrUnknown) {
		t.Fatal("expected lost creation reply")
	}
	var taskID string
	_ = (store{p.Dir}).tx(func(d *database) error {
		for id := range d.Tasks {
			taskID = id
		}
		return nil
	})
	if _, err = p.Cancel("alice", taskID); err != nil {
		t.Fatal(err)
	}
	restarted := httptest.NewServer(p)
	defer restarted.Close()
	c.BaseURL = restarted.URL
	rec, err := c.Reconcile(context.Background(), "late")
	if err != nil || rec.State != Canceled {
		t.Fatalf("stale dedup snapshot used: %+v %v", rec, err)
	}
}
