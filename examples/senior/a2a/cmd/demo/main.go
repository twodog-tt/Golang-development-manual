// Command demo uses real loopback HTTP and separate local/remote file journals.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"td-homework/examples/senior/a2a"
)

func main() {
	dir := flag.String("dir", "", "private fixture directory (required)")
	step := flag.String("step", "demo", "demo, submit, status, approve, continue, reconcile, revoke, cancel")
	reviewed := flag.String("digest", "", "exact reviewed input digest for approve")
	revision := flag.Int("revision", 0, "exact reviewed task revision for approve")
	payload := flag.String("payload", "approved article", "application input")
	dedup := flag.Bool("dedup", true, "pinned local messageId dedup contract (must remain unchanged)")
	lose := flag.Bool("lose-reply", false, "return 503 after the durable remote Send commit")
	crash := flag.String("crash", "", "before-send or after-send: exit 77 at a durable boundary")
	flag.Parse()
	if *dir == "" {
		log.Fatal("-dir required")
	}
	p := a2a.NewPeer(filepath.Join(*dir, "remote"), *dedup)
	owner := a2a.Identity{Tenant: "tenant-a", Subject: "alice"}
	p.Put("local-caller", a2a.Grant{Identity: owner, Operations: []string{"send", "get", "list", "cancel", "subscribe", "push"}, Expires: time.Now().Add(time.Hour)})
	p.Put("local-reviewer", a2a.Grant{Identity: a2a.Identity{Tenant: "tenant-a", Subject: "human-reviewer"}, Operations: []string{"approve"}, Expires: time.Now().Add(time.Hour)})
	if *crash == "after-send" {
		p.AfterSend = func() { os.Exit(77) }
	} else if *crash != "" && *crash != "before-send" {
		log.Fatal("unknown crash boundary")
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if *lose && r.URL.Path == "/message:send" {
			rr := httptest.NewRecorder()
			p.ServeHTTP(rr, r)
			if rr.Code == 200 {
				http.Error(w, "injected lost reply", 503)
				return
			}
			for k, v := range rr.Header() {
				w.Header()[k] = v
			}
			w.WriteHeader(rr.Code)
			_, _ = w.Write(rr.Body.Bytes())
			return
		}
		p.ServeHTTP(w, r)
	})
	ts := httptest.NewServer(handler)
	p.Endpoint = ts.URL
	defer ts.Close()
	c := &a2a.Client{PeerID: "local-peer", Dir: filepath.Join(*dir, "local"), BaseURL: ts.URL, Token: "local-caller", Owner: owner, RetryByMessageID: *dedup}
	if *crash == "before-send" {
		c.BeforeSend = func() { os.Exit(77) }
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	in := a2a.Input{Action: "publish", Payload: *payload}
	var rec a2a.Intent
	var err error
	switch *step {
	case "demo":
		rec, err = c.Submit(ctx, "demo", in)
		if err == nil && rec.State == a2a.AuthRequired {
			task, e := p.Get("local-caller", rec.TaskID)
			err = e
			if err == nil {
				err = p.Approve("local-reviewer", task.ID, a2a.InputDigest(task.Input), task.Revision, time.Now().Add(time.Minute))
			}
			if err == nil {
				rec, err = c.Continue(ctx, "demo", in)
			}
		}
	case "submit":
		rec, err = c.Submit(ctx, "demo", in)
	case "status", "reconcile":
		rec, err = c.Reconcile(ctx, "demo")
	case "continue":
		rec, err = c.Continue(ctx, "demo", in)
	case "approve", "revoke", "cancel":
		rec, err = c.Reconcile(ctx, "demo")
		if err == nil {
			switch *step {
			case "approve":
				if *reviewed == "" || *revision < 1 {
					err = errors.New("pass the digest and revision from status after reviewing them")
				} else {
					err = p.Approve("local-reviewer", rec.TaskID, *reviewed, *revision, time.Now().Add(time.Minute))
				}
			case "revoke":
				err = p.RevokeApproval("local-reviewer", rec.TaskID)
			case "cancel":
				_, err = p.Cancel("local-caller", rec.TaskID)
			}
		}
	default:
		log.Fatal("unknown step")
	}
	result := map[string]any{"state": rec.State, "task_id": rec.TaskID, "message_id": rec.Message.MessageID}
	if rec.TaskID != "" {
		task, e := p.Get("local-caller", rec.TaskID)
		if e == nil {
			result["review_input"] = task.Input
			result["review_digest"] = a2a.InputDigest(task.Input)
			result["review_revision"] = task.Revision
		}
	}
	n, e := p.Effects()
	if e != nil {
		log.Fatal(e)
	}
	result["effects"] = n
	if err != nil {
		result["error"] = err.Error()
	}
	if err = json.NewEncoder(os.Stdout).Encode(result); err != nil {
		log.Fatal(err)
	}
	if _, ok := result["error"]; ok {
		fmt.Fprintln(os.Stderr, "fixture stopped; inspect JSON and reconcile before retrying")
		os.Exit(2)
	}
}
