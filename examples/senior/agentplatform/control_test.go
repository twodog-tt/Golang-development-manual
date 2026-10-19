package agentplatform

import (
	"encoding/json"
	"os"
	"sync"
	"sync/atomic"
	"testing"
)

func binding() Binding {
	return Binding{"tenant-a", "v1", digest([]byte("artifact")), digest([]byte("manifest")), digest([]byte("runtime")), digest([]byte("config")), digest([]byte("read")), digest([]byte("policy")), "prod"}
}
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func denied(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected rejection")
	}
}
func published(t *testing.T) (*Control, Binding) {
	t.Helper()
	c := NewControl()
	b := binding()
	must(t, c.Register(b))
	must(t, c.Approve(b))
	must(t, c.Deploy(b))
	return c, b
}

func TestReleaseApprovalPinsAllInputsAndImmutableVersion(t *testing.T) {
	_, err := NewControl().Claim("empty", Binding{})
	denied(t, err)
	c, b := published(t)
	for _, field := range []string{"artifact", "manifest", "runtime", "config", "permissions", "policy", "environment", "tenant", "version"} {
		x := b
		switch field {
		case "artifact":
			x.Artifact = digest([]byte("changed"))
		case "manifest":
			x.Manifest = digest([]byte("changed"))
		case "runtime":
			x.Runtime = digest([]byte("changed"))
		case "config":
			x.Config = digest([]byte("changed"))
		case "permissions":
			x.Permissions = digest([]byte("changed"))
		case "policy":
			x.Policy = digest([]byte("changed"))
		case "environment":
			x.Environment = "staging"
		case "tenant":
			x.Tenant = "other"
		case "version":
			x.Version = "v2"
		}
		t.Run(field, func(t *testing.T) {
			denied(t, c.Deploy(x))
			if field == "artifact" || field == "manifest" || field == "runtime" || field == "permissions" {
				denied(t, c.Register(x))
			}
		})
	}
	config := b
	config.Config = digest([]byte("new-config"))
	must(t, c.Register(config))
	denied(t, c.Deploy(config))
	must(t, c.Approve(config))
	must(t, c.Deploy(config))
	must(t, c.Stop(b, false))
	denied(t, c.Deploy(b))
	must(t, c.Approve(b))
	must(t, c.Deploy(b)) // same digest re-review
	fixed := b
	fixed.Version = "v2"
	fixed.Artifact = digest([]byte("fixed"))
	must(t, c.Register(fixed))
	denied(t, c.Deploy(fixed))
	must(t, c.Approve(fixed))
	must(t, c.Deploy(fixed))
	must(t, c.Stop(b, true))
	denied(t, c.Approve(b))
	denied(t, c.Deploy(b)) // rollback cannot resurrect revoked version
}

func TestStopCancelAndABAFenceOldWorkerAtSignerAdmission(t *testing.T) {
	c, b := published(t)
	epoch, err := c.Claim("claimed", b)
	must(t, err)
	must(t, c.Stop(b, false))
	denied(t, c.Submit("claimed", epoch))
	must(t, c.Approve(b))
	must(t, c.Deploy(b))
	denied(t, c.Submit("claimed", epoch)) // same binding, new generation
	x, err := c.Claim("late-broadcast", b)
	must(t, err)
	must(t, c.Submit("late-broadcast", x))
	must(t, c.Stop(b, true))
	must(t, c.Cancel("late-broadcast"))
	if c.JobState("late-broadcast") != "unknown" {
		t.Fatal("stop/cancel incorrectly erased an admitted effect")
	}
	denied(t, c.Submit("late-broadcast", x))
	e := EffectEvidence{"late-broadcast", b, x, "tx", true, true, true}
	bad := e
	bad.Generation++
	denied(t, c.Reconcile(bad))
	bad = e
	bad.Canonical = false
	denied(t, c.Reconcile(bad))
	must(t, c.Reconcile(e))
	must(t, c.Reconcile(e))
	conflict := e
	conflict.Transaction = "another-tx"
	denied(t, c.Reconcile(conflict))
	if c.JobState("late-broadcast") != "confirmed" {
		t.Fatal("revocation blocked read-only recovery")
	}
	denied(t, c.Deploy(b))
	c, b = published(t)
	x, err = c.Claim("cancel-before-submit", b)
	must(t, err)
	must(t, c.Cancel("cancel-before-submit"))
	denied(t, c.Submit("cancel-before-submit", x))
}

func TestConcurrentStopAndSubmitHasOneAdmissionBoundary(t *testing.T) {
	for i := 0; i < 30; i++ {
		c, b := published(t)
		epoch, err := c.Claim("job", b)
		must(t, err)
		var wg sync.WaitGroup
		var admitted atomic.Bool
		wg.Add(2)
		go func() { defer wg.Done(); admitted.Store(c.Submit("job", epoch) == nil) }()
		go func() { defer wg.Done(); _ = c.Stop(b, true) }()
		wg.Wait()
		if admitted.Load() && c.JobState("job") != "unknown" {
			t.Fatal("admitted effect lost during revocation")
		}
		denied(t, c.Submit("job", epoch))
	}
}

func TestNativePluginRejectionAndBrokerCapabilityBoundary(t *testing.T) {
	p := Plugin{"declarative-v1", "report", "local-report"}
	_, err := Broker(p, "read:local-report")
	must(t, err)
	for _, q := range []Plugin{{"native", "report", "local-report"}, {"declarative-v1", "exec", "/bin/sh"}, {"declarative-v1", "read", "/run/secrets/key"}, {"declarative-v1", "fetch", "http://169.254.169.254"}, {"declarative-v1", "sign", "signer.sock"}} {
		denied(t, Admit(q))
	}
	for _, cap := range []string{"read:secret", "egress:any", "sign:wallet", "write:filesystem"} {
		_, err = Broker(p, cap)
		denied(t, err)
	}
}

func TestCommittedWireVectors(t *testing.T) {
	b, err := os.ReadFile("testdata/wire.json")
	must(t, err)
	var rows []struct {
		Name, Raw, Hex string
		Reject         bool
		Expected       struct{ Canonical, Digest string }
	}
	must(t, json.Unmarshal(b, &rows))
	for _, v := range rows {
		t.Run(v.Name, func(t *testing.T) {
			raw := []byte(v.Raw)
			if v.Hex != "" {
				raw = []byte{0xff}
			}
			c, h, err := Canonical(raw)
			if v.Reject {
				denied(t, err)
			} else {
				must(t, err)
				if c != v.Expected.Canonical || h != v.Expected.Digest {
					t.Fatal("canonical mismatch")
				}
			}
		})
	}
}
