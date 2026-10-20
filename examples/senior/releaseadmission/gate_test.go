package releaseadmission

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func fixture(t *testing.T) *Fixture {
	t.Helper()
	f, err := NewFixture(t.TempDir(), time.Unix(1800000000, 0))
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func gate(t *testing.T, f *Fixture) *Gate {
	t.Helper()
	g, err := NewGate(f.Policy, f.Target, f.Root, func() time.Time { return f.Time })
	if err != nil {
		t.Fatal(err)
	}
	return g
}
func rewrite(t *testing.T, f *Fixture, e *Envelope, role string, change func(object)) {
	t.Helper()
	m, err := parse(e.Payload)
	if err != nil {
		t.Fatal(err)
	}
	change(m)
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	*e = Sign(raw, role, f.Keys[role])
}
func predicate(t *testing.T, f *Fixture, e *Envelope, role string, change func(object)) {
	t.Helper()
	rewrite(t, f, e, role, func(m object) { change(obj(m["predicate"])) })
}
func report(t *testing.T, raw *[]byte, change func(object)) {
	t.Helper()
	m, err := parse(*raw)
	if err != nil {
		t.Fatal(err)
	}
	change(m)
	*raw, err = json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
}
func denied(t *testing.T, g *Gate, b Bundle) {
	t.Helper()
	if _, err := g.Prepare(context.Background(), "release", b); err == nil || g.Effects() != 0 {
		t.Fatal("unsafe release accepted or effect recorded", err, g.Effects())
	}
}
func accepted(t *testing.T, g *Gate, b Bundle) Ticket {
	t.Helper()
	ticket, err := g.Prepare(context.Background(), "release", b)
	if err != nil {
		t.Fatal(err)
	}
	return ticket
}
func risky(t *testing.T, f *Fixture) {
	report(t, &f.Bundle.ScanReport, func(m object) { m["findings"] = []any{object{"id": "CVE-FIXTURE-1", "severity": "high"}} })
	f.LinkReports()
	f.Approve()
}

func TestHappyPathAndConcurrentIdempotency(t *testing.T) {
	f := fixture(t)
	g := gate(t, f)
	ticket := accepted(t, g, f.Bundle)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := g.Commit(context.Background(), ticket)
			if err != nil || r.Sequence != 1 {
				t.Error(r, err)
			}
		}()
	}
	wg.Wait()
	if g.Effects() != 1 {
		t.Fatal(g.Effects())
	}
	retry := accepted(t, g, f.Bundle)
	if _, err := g.Commit(context.Background(), retry); err != nil || g.Effects() != 1 {
		t.Fatal(err)
	}
}

func TestRequiredTestEvidence(t *testing.T) {
	cases := map[string]func(object){
		"failed-unit":     func(m object) { obj(m["results"])["unit"] = "failed" },
		"missing-race":    func(m object) { delete(obj(m["results"]), "race") },
		"skipped-vet":     func(m object) { obj(m["results"])["vet"] = "skipped" },
		"missing-results": func(m object) { delete(m, "results") },
		"wrong-run":       func(m object) { m["run"] = "another-build" },
		"wrong-commit":    func(m object) { m["commit"] = strings.Repeat("b", 40) },
		"wrong-inputs":    func(m object) { m["inputs"] = Digest([]byte("other")) },
		"wrong-suite":     func(m object) { m["suite"] = "unit-only" },
		"wrong-platform":  func(m object) { m["platform"] = "linux/arm64" },
		"stale":           func(m object) { m["completedAt"] = int64(1799999600) },
		"future":          func(m object) { m["completedAt"] = int64(1800000001) },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f := fixture(t)
			predicate(t, f, &f.Bundle.Tests, "test", change)
			f.Approve()
			denied(t, gate(t, f), f.Bundle)
		})
	}
	t.Run("absent", func(t *testing.T) {
		f := fixture(t)
		f.Bundle.Tests = Envelope{}
		f.Approve()
		denied(t, gate(t, f), f.Bundle)
	})
	t.Run("artifact-substitution", func(t *testing.T) {
		f := fixture(t)
		f.Bundle.Artifact = []byte("rebuilt after testing")
		f.Target.Image = f.Policy.Repository + "@sha256:" + Digest(f.Bundle.Artifact)
		f.Approve()
		denied(t, gate(t, f), f.Bundle)
	})
}

func TestSignatureAndAuthorization(t *testing.T) {
	cases := map[string]func(*Fixture){
		"tampered-payload":   func(f *Fixture) { f.Bundle.Provenance.Payload = append(f.Bundle.Provenance.Payload, ' ') },
		"wrong-key":          func(f *Fixture) { f.Bundle.Provenance = Sign(f.Bundle.Provenance.Payload, "build", f.Keys["approve"]) },
		"unknown-key":        func(f *Fixture) { f.Bundle.Provenance.Signatures[0].KeyID = "attacker" },
		"wrong-payload-type": func(f *Fixture) { f.Bundle.Provenance.PayloadType = "application/json" },
		"multiple-signatures": func(f *Fixture) {
			f.Bundle.Provenance.Signatures = append(f.Bundle.Provenance.Signatures, f.Bundle.Provenance.Signatures[0])
		},
		"wrong-role":        func(f *Fixture) { s := f.Policy.Trust["build"]; s.Role = "approve"; f.Policy.Trust["build"] = s },
		"wrong-environment": func(f *Fixture) { s := f.Policy.Trust["build"]; s.Environment = "staging"; f.Policy.Trust["build"] = s },
		"wrong-issuer":      func(f *Fixture) { s := f.Policy.Trust["build"]; s.Issuer = "untrusted"; f.Policy.Trust["build"] = s },
		"wrong-identity": func(f *Fixture) {
			s := f.Policy.Trust["build"]
			s.Identity = "pull-request/workflow"
			f.Policy.Trust["build"] = s
		},
		"wrong-builder-mapping": func(f *Fixture) {
			s := f.Policy.Trust["build"]
			s.Builder = "self-claimed"
			f.Policy.Trust["build"] = s
		},
		"builder-approves": func(f *Fixture) { f.Bundle.Approval = Sign(f.Bundle.Approval.Payload, "build", f.Keys["build"]) },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f := fixture(t)
			change(f)
			if name != "builder-approves" {
				f.Approve()
			}
			denied(t, gate(t, f), f.Bundle)
		})
	}
	t.Run("key-alias-rejected", func(t *testing.T) {
		f := fixture(t)
		s := f.Policy.Trust["approve"]
		s.PublicKey = f.Policy.Trust["build"].PublicKey
		f.Policy.Trust["approve"] = s
		if _, err := NewGate(f.Policy, f.Target, f.Root, nil); err == nil {
			t.Fatal("same key authorized for separate roles")
		}
	})
}

func TestProvenanceContract(t *testing.T) {
	cases := map[string]func(object){
		"wrong-buildType":    func(m object) { obj(m["buildDefinition"])["buildType"] = "unreviewed" },
		"wrong-builder":      func(m object) { obj(obj(m["runDetails"])["builder"])["id"] = "attacker" },
		"missing-parameters": func(m object) { delete(obj(m["buildDefinition"]), "externalParameters") },
		"null-parameters":    func(m object) { obj(m["buildDefinition"])["externalParameters"] = nil },
		"extra-parameter":    func(m object) { obj(obj(m["buildDefinition"])["externalParameters"])["skipTests"] = true },
		"wrong-repo":         func(m object) { obj(obj(m["buildDefinition"])["externalParameters"])["repository"] = "fork" },
		"wrong-ref":          func(m object) { obj(obj(m["buildDefinition"])["externalParameters"])["ref"] = "refs/pull/1/head" },
		"wrong-source": func(m object) {
			obj(obj(list(obj(m["buildDefinition"])["resolvedDependencies"])[0])["digest"])["gitCommit"] = "bad"
		},
		"wrong-input": func(m object) {
			obj(obj(list(obj(m["buildDefinition"])["resolvedDependencies"])[1])["digest"])["sha256"] = "bad"
		},
		"missing-source": func(m object) { obj(m["buildDefinition"])["resolvedDependencies"] = []any{} },
		"duplicate-source": func(m object) {
			b := obj(m["buildDefinition"])
			b["resolvedDependencies"] = append(list(b["resolvedDependencies"]), list(b["resolvedDependencies"])[0])
		},
		"stale": func(m object) { obj(obj(m["runDetails"])["metadata"])["finishedOn"] = "2020-01-01T00:00:00Z" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f := fixture(t)
			predicate(t, f, &f.Bundle.Provenance, "build", change)
			f.Approve()
			denied(t, gate(t, f), f.Bundle)
		})
	}
	for _, field := range []string{"_type", "predicateType", "subject"} {
		t.Run(field, func(t *testing.T) {
			f := fixture(t)
			rewrite(t, f, &f.Bundle.Provenance, "build", func(m object) {
				switch field {
				case "_type":
					m[field] = "wrong"
				case "predicateType":
					m[field] = "https://slsa.dev/provenance/v1.2"
				case "subject":
					obj(list(m[field])[0])["name"] = "other-repository"
				}
			})
			f.Approve()
			denied(t, gate(t, f), f.Bundle)
		})
	}
	t.Run("ignore-generic-extensions", func(t *testing.T) {
		f := fixture(t)
		predicate(t, f, &f.Bundle.Provenance, "build", func(m object) {
			m["futureExtension"] = object{"anything": true}
			obj(m["buildDefinition"])["futureExtension"] = nil
		})
		f.Approve()
		accepted(t, gate(t, f), f.Bundle)
	})
}

func TestSBOMActualBytesAndProfile(t *testing.T) {
	cases := map[string]func(object){
		"unsupported-version": func(m object) { m["spdxVersion"] = "SPDX-3.0" },
		"missing-license":     func(m object) { delete(obj(list(m["packages"])[0]), "licenseConcluded") },
		"wrong-checksum": func(m object) {
			obj(list(obj(list(m["packages"])[0])["checksums"])[0])["checksumValue"] = Digest([]byte("other-artifact"))
		},
		"missing-creator":    func(m object) { delete(obj(m["creationInfo"]), "creators") },
		"missing-packages":   func(m object) { delete(m, "packages") },
		"duplicate-package":  func(m object) { m["packages"] = append(list(m["packages"]), list(m["packages"])[0]) },
		"dangling-relation":  func(m object) { obj(list(m["relationships"])[0])["relatedSpdxElement"] = "SPDXRef-Unknown" },
		"missing-describes":  func(m object) { m["relationships"] = []any{} },
		"duplicate-relation": func(m object) { m["relationships"] = append(list(m["relationships"]), list(m["relationships"])[0]) },
		"stale":              func(m object) { obj(m["creationInfo"])["created"] = "2020-01-01T00:00:00Z" },
		"invalid-namespace":  func(m object) { m["documentNamespace"] = "https://knowledgebase.invalid/sbom/" },
		"unsupported-field":  func(m object) { m["files"] = []any{} },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f := fixture(t)
			report(t, &f.Bundle.SBOMReport, change)
			f.LinkReports()
			f.Approve()
			denied(t, gate(t, f), f.Bundle)
		})
	}
	t.Run("unsigned-byte-change", func(t *testing.T) {
		f := fixture(t)
		f.Bundle.SBOMReport = append(f.Bundle.SBOMReport, ' ')
		f.Approve()
		denied(t, gate(t, f), f.Bundle)
	})
	t.Run("dependency-package", func(t *testing.T) {
		f := fixture(t)
		report(t, &f.Bundle.SBOMReport, func(m object) {
			pkg := copyValue(obj(list(m["packages"])[0]))
			pkg["SPDXID"] = "SPDXRef-Dependency"
			pkg["name"] = "fixture dependency"
			pkg["checksums"] = []any{object{"algorithm": "SHA256", "checksumValue": Digest([]byte("dependency bytes"))}}
			m["packages"] = append(list(m["packages"]), pkg)
			m["relationships"] = append(list(m["relationships"]), object{"spdxElementId": "SPDXRef-Artifact", "relationshipType": "DEPENDS_ON", "relatedSpdxElement": "SPDXRef-Dependency"})
		})
		f.LinkReports()
		f.Approve()
		accepted(t, gate(t, f), f.Bundle)
	})
}

func TestScanFailuresAndFreshness(t *testing.T) {
	cases := map[string]func(object){
		"scanner-error":    func(m object) { m["status"] = "error" },
		"missing-findings": func(m object) { delete(m, "findings") },
		"null-findings":    func(m object) { m["findings"] = nil },
		"wrong-db":         func(m object) { m["dbVersion"] = "unapproved-db" },
		"stale-db":         func(m object) { m["dbUpdatedAt"] = int64(1799900000) },
		"future-db":        func(m object) { m["dbUpdatedAt"] = int64(1800000001) },
		"unknown-severity": func(m object) { m["findings"] = []any{object{"id": "X", "severity": "unknown"}} },
		"duplicate-id": func(m object) {
			m["findings"] = []any{object{"id": "X", "severity": "low"}, object{"id": "X", "severity": "low"}}
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f := fixture(t)
			report(t, &f.Bundle.ScanReport, change)
			f.LinkReports()
			f.Approve()
			denied(t, gate(t, f), f.Bundle)
		})
	}
	for _, role := range []string{"sbom", "scan"} {
		for _, field := range []string{"platform", "tool", "completedAt"} {
			t.Run(role+"-"+field, func(t *testing.T) {
				f := fixture(t)
				e := &f.Bundle.SBOM
				if role == "scan" {
					e = &f.Bundle.Scan
				}
				predicate(t, f, e, role, func(m object) {
					if field == "completedAt" {
						m[field] = int64(1799999600)
					} else {
						m[field] = "wrong"
					}
				})
				f.Approve()
				denied(t, gate(t, f), f.Bundle)
			})
		}
	}
	for _, raw := range [][]byte{nil, []byte("{broken"), []byte(`{"findings":[]}`)} {
		t.Run(fmt.Sprintf("bad-bytes-%d", len(raw)), func(t *testing.T) {
			f := fixture(t)
			f.Bundle.ScanReport = raw
			f.LinkReports()
			f.Approve()
			denied(t, gate(t, f), f.Bundle)
		})
	}
}

func TestExceptionScopeAndHardGates(t *testing.T) {
	t.Run("unused-exception", func(t *testing.T) { f := fixture(t); f.Waive("CVE-FIXTURE-1"); denied(t, gate(t, f), f.Bundle) })
	t.Run("absent", func(t *testing.T) { f := fixture(t); risky(t, f); denied(t, gate(t, f), f.Bundle) })
	t.Run("scoped-acceptance", func(t *testing.T) {
		f := fixture(t)
		risky(t, f)
		f.Waive("CVE-FIXTURE-1")
		g := gate(t, f)
		if _, err := g.Commit(context.Background(), accepted(t, g, f.Bundle)); err != nil {
			t.Fatal(err)
		}
	})
	cases := map[string]func(object){
		"wrong-target":    func(m object) { m["targetDigest"] = Digest([]byte("other")) },
		"wrong-evidence":  func(m object) { m["evidenceDigest"] = Digest([]byte("other")) },
		"wrong-policy":    func(m object) { m["policy"] = "old-policy" },
		"wrong-risk":      func(m object) { m["risks"] = []any{"another-CVE"} },
		"duplicate-risk":  func(m object) { m["risks"] = []any{"CVE-FIXTURE-1", "CVE-FIXTURE-1"} },
		"expired":         func(m object) { m["expiresAt"] = int64(1800000000) },
		"missing-owner":   func(m object) { delete(m, "owner") },
		"missing-control": func(m object) { delete(m, "control") },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f := fixture(t)
			risky(t, f)
			f.Waive("CVE-FIXTURE-1")
			predicate(t, f, f.Bundle.Exception, "exception", change)
			denied(t, gate(t, f), f.Bundle)
		})
	}
	t.Run("builder-waives", func(t *testing.T) {
		f := fixture(t)
		risky(t, f)
		f.Waive("CVE-FIXTURE-1")
		e := Sign(f.Bundle.Exception.Payload, "build", f.Keys["build"])
		f.Bundle.Exception = &e
		denied(t, gate(t, f), f.Bundle)
	})
	for _, fault := range []string{"failed-test", "bad-signature", "missing-sbom"} {
		t.Run(fault+"-cannot-be-waived", func(t *testing.T) {
			f := fixture(t)
			risky(t, f)
			switch fault {
			case "failed-test":
				predicate(t, f, &f.Bundle.Tests, "test", func(m object) { obj(m["results"])["unit"] = "failed" })
			case "bad-signature":
				f.Bundle.Provenance.Signatures[0].Sig[0] ^= 1
			case "missing-sbom":
				f.Bundle.SBOMReport = nil
			}
			f.Approve()
			f.Waive("CVE-FIXTURE-1")
			denied(t, gate(t, f), f.Bundle)
		})
	}
}

func TestApprovalBinding(t *testing.T) {
	for _, field := range []string{"policy", "targetDigest", "evidenceDigest", "id", "issuedAt", "expiresAt"} {
		t.Run(field, func(t *testing.T) {
			f := fixture(t)
			predicate(t, f, &f.Bundle.Approval, "approve", func(m object) {
				switch field {
				case "id":
					m[field] = ""
				case "expiresAt":
					m[field] = f.Time.Unix()
				case "issuedAt":
					m[field] = f.Time.Unix() + 1
				default:
					m[field] = "wrong"
				}
			})
			denied(t, gate(t, f), f.Bundle)
		})
	}
}

func TestTargetChangesAndABA(t *testing.T) {
	cases := map[string]func(*Target){
		"environment": func(t *Target) { t.Environment = "staging" }, "image": func(t *Target) { t.Image += "bad" }, "platform": func(t *Target) { t.Platform = "linux/arm64" },
		"config": func(t *Target) { t.Config = Digest([]byte("new")) }, "migration": func(t *Target) { t.Migration = Digest([]byte("new")) }, "flags": func(t *Target) { t.Flags = Digest([]byte("new")) },
		"chain": func(t *Target) { t.ChainID = "other-chain" }, "proxy": func(t *Target) { t.Proxy = "another-proxy" }, "implementation": func(t *Target) { t.Implementation = "implementation-v2" }, "codeHash": func(t *Target) { t.CodeHash = Digest([]byte("new")) }, "inputs": func(t *Target) { t.Inputs = Digest([]byte("new")) },
	}
	for name, change := range cases {
		for _, aba := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-aba-%v", name, aba), func(t *testing.T) {
				f := fixture(t)
				g := gate(t, f)
				ticket := accepted(t, g, f.Bundle)
				target := f.Target
				change(&target)
				g.SetTarget(target)
				if aba {
					g.SetTarget(f.Target)
				}
				if _, err := g.Commit(context.Background(), ticket); err == nil || g.Effects() != 0 {
					t.Fatal("stale ticket accepted")
				}
			})
		}
	}
}

func TestRevocationRollbackAndPolicy(t *testing.T) {
	for _, kind := range []string{"key", "approval", "artifact", "exception"} {
		t.Run(kind, func(t *testing.T) {
			f := fixture(t)
			if kind == "exception" {
				risky(t, f)
				f.Waive("CVE-FIXTURE-1")
			}
			g := gate(t, f)
			ticket := accepted(t, g, f.Bundle)
			switch kind {
			case "key":
				g.RevokeKey("build")
			case "approval":
				g.RevokeApproval("approval-1")
			case "artifact":
				g.RevokeArtifact(Digest(f.Bundle.Artifact))
			case "exception":
				g.RevokeException("exception-1")
			}
			if _, err := g.Commit(context.Background(), ticket); err == nil {
				t.Fatal("revoked ticket accepted")
			}
			denied(t, g, f.Bundle)
		})
	}
	t.Run("policy-version", func(t *testing.T) {
		f := fixture(t)
		g := gate(t, f)
		ticket := accepted(t, g, f.Bundle)
		p := copyValue(f.Policy)
		p.Version = "policy-v2"
		if err := g.SetPolicy(p); err != nil {
			t.Fatal(err)
		}
		if _, err := g.Commit(context.Background(), ticket); err == nil {
			t.Fatal("old policy ticket")
		}
		denied(t, g, f.Bundle)
		if err := g.SetPolicy(f.Policy); err == nil {
			t.Fatal("old version reused")
		}
		f.Policy = p
		f.Approve()
		if _, err := g.Prepare(context.Background(), "new-policy-release", f.Bundle); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("rollback-revoked-after-success", func(t *testing.T) {
		f := fixture(t)
		g := gate(t, f)
		ticket := accepted(t, g, f.Bundle)
		if _, err := g.Commit(context.Background(), ticket); err != nil {
			t.Fatal(err)
		}
		g.RevokeArtifact(Digest(f.Bundle.Artifact))
		if _, err := g.Prepare(context.Background(), "rollback", f.Bundle); err == nil || g.Effects() != 1 {
			t.Fatal("rollback bypassed revocation")
		}
	})
}

func TestCommitRevalidatesEvidenceAndInput(t *testing.T) {
	for _, fault := range []string{"expiry", "exception-expiry", "file-change", "missing-root", "cancel", "restart"} {
		t.Run(fault, func(t *testing.T) {
			f := fixture(t)
			if fault == "exception-expiry" {
				risky(t, f)
				f.Waive("CVE-FIXTURE-1")
			}
			var seconds atomic.Int64
			seconds.Store(f.Time.Unix())
			g, err := NewGate(f.Policy, f.Target, f.Root, func() time.Time { return time.Unix(seconds.Load(), 0) })
			if err != nil {
				t.Fatal(err)
			}
			ticket := accepted(t, g, f.Bundle)
			ctx := context.Background()
			switch fault {
			case "expiry":
				seconds.Add(241)
			case "exception-expiry":
				seconds.Add(121)
			case "file-change":
				if err := os.WriteFile(filepath.Join(f.Root, "main.go"), []byte("changed"), 0600); err != nil {
					t.Fatal(err)
				}
			case "missing-root":
				if err := os.RemoveAll(f.Root); err != nil {
					t.Fatal(err)
				}
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "restart":
				g = gate(t, f)
			}
			if _, err := g.Commit(ctx, ticket); err == nil || g.Effects() != 0 {
				t.Fatal("commit skipped current checks")
			}
		})
	}
}

func TestPrivateCopiesAndIntentConflicts(t *testing.T) {
	f := fixture(t)
	g := gate(t, f)
	ticket := accepted(t, g, f.Bundle)
	// Caller mutations must not rewrite a prepared approval or trusted key.
	f.Bundle.Artifact[0] ^= 1
	f.Bundle.Provenance.Signatures[0].Sig[0] ^= 1
	f.Policy.Trust["build"].PublicKey[0] ^= 1
	if _, err := g.Commit(context.Background(), ticket); err != nil {
		t.Fatal(err)
	}
	f2 := fixture(t)
	g2 := gate(t, f2)
	accepted(t, g2, f2.Bundle)
	predicate(t, f2, &f2.Bundle.Approval, "approve", func(m object) { m["id"] = "approval-2" })
	if _, err := g2.Prepare(context.Background(), "release", f2.Bundle); err == nil {
		t.Fatal("intent rebound")
	}
}

func TestCommitAndRevocationSerialize(t *testing.T) {
	f := fixture(t)
	g := gate(t, f)
	ticket := accepted(t, g, f.Bundle)
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); <-start; _, _ = g.Commit(context.Background(), ticket) }()
	go func() { defer wg.Done(); <-start; g.RevokeApproval("approval-1") }()
	close(start)
	wg.Wait()
	if g.Effects() > 1 {
		t.Fatal(g.Effects())
	}
	if _, err := g.Commit(context.Background(), ticket); err == nil {
		t.Fatal("revocation did not fence later attempts")
	}
}

func TestStrictJSONAndBounds(t *testing.T) {
	for _, raw := range []string{`{"x":1,"x":2}`, `{"x":1,"\u0078":2}`, `{} {}`, `[]`, `{"x":"\ud800"}`, `{"x":"\udc00"}`, `{"x":` + strings.Repeat("[", 18) + `0` + strings.Repeat("]", 18) + `}`, `{"x":"` + strings.Repeat("x", 128<<10) + `"}`} {
		if _, err := parse([]byte(raw)); err == nil {
			t.Fatalf("accepted malformed/oversized JSON: %.80s", raw)
		}
	}
	f := fixture(t)
	payload := strings.Replace(string(f.Bundle.Provenance.Payload), `"predicateType":`, `"PredicateType":`, 1)
	f.Bundle.Provenance = Sign([]byte(payload), "build", f.Keys["build"])
	f.Approve()
	denied(t, gate(t, f), f.Bundle)
	if _, err := parse([]byte(`{"x":"\ud83d\ude00"}`)); err != nil {
		t.Fatal("valid surrogate pair rejected", err)
	}
	t.Run("signed-duplicate", func(t *testing.T) {
		f := fixture(t)
		raw := append([]byte(`{"_type":"attacker",`), f.Bundle.Provenance.Payload[1:]...)
		f.Bundle.Provenance = Sign(raw, "build", f.Keys["build"])
		f.Approve()
		denied(t, gate(t, f), f.Bundle)
	})
	t.Run("oversized-artifact", func(t *testing.T) {
		f := fixture(t)
		f.Bundle.Artifact = make([]byte, (1<<20)+1)
		denied(t, gate(t, f), f.Bundle)
	})
	t.Run("bad-public-key", func(t *testing.T) {
		f := fixture(t)
		s := f.Policy.Trust["build"]
		s.PublicKey = ed25519.PublicKey{1}
		f.Policy.Trust["build"] = s
		if f.Policy.valid() {
			t.Fatal("invalid key")
		}
	})
}
