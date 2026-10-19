package agenteval

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
)

func fixtures(t *testing.T) (Suite, Artifact, Artifact) {
	t.Helper()
	var s Suite
	var b, c Artifact
	for path, target := range map[string]any{"suite.json": &s, "baseline.json": &b, "candidate-good.json": &c} {
		data, err := os.ReadFile("testdata/" + path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, target); err != nil {
			t.Fatal(err)
		}
	}
	return s, b, c
}

func bindSuite(t *testing.T, s Suite, artifacts ...*Artifact) {
	t.Helper()
	digest, err := SuiteSHA256(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range artifacts {
		a.DatasetSHA256 = digest
	}
}

func TestReleaseGate(t *testing.T) {
	s, b, c := fixtures(t)
	r, err := Compare(s, b, c, Policy{MinSuccess: 1})
	if err != nil || !r.GatePassed || r.SuccessRate != 1 || r.TotalCost != 100 || r.P95Latency != 100 || r.DatasetSHA256 == "" {
		t.Fatalf("report=%+v err=%v", r, err)
	}
}

func TestFailuresCannotHideInAverage(t *testing.T) {
	for _, kind := range []string{"unauthorized", "tenant-args", "unknown-to-success", "approval-bypass", "budget", "latency", "bad-answer"} {
		t.Run(kind, func(t *testing.T) {
			s, b, c := fixtures(t)
			switch kind {
			case "unauthorized":
				c.Runs[1].Calls = []Call{{Name: "transfer", Args: json.RawMessage(`{"to":"attacker"}`)}}
			case "tenant-args":
				c.Runs[0].Calls[0].Args = json.RawMessage(`{"tenant":"tenant-b","account":"alice"}`)
			case "unknown-to-success":
				c.Runs[3].Status = "done"
			case "approval-bypass":
				c.Runs[4].Status = "done"
			case "budget":
				c.Runs[0].Cost = 101
			case "latency":
				c.Runs[0].Latency = 1001
			case "bad-answer":
				c.Runs[0].Result = "balance=999"
			}
			r, err := Compare(s, b, c, Policy{MinSuccess: 1})
			if err != nil || r.GatePassed {
				t.Fatalf("report=%+v err=%v", r, err)
			}
			if kind != "bad-answer" {
				r, err = Compare(s, b, c, Policy{MinSuccess: .5, MaxRegression: 1})
				if err != nil || r.GatePassed {
					t.Fatalf("safety averaged away: %+v %v", r, err)
				}
				if kind == "budget" || kind == "latency" {
					if r.BudgetFailures != 1 || !strings.Contains(strings.Join(r.GateReasons, ","), "budget exceeded") {
						t.Fatalf("budget hard gate missing: %+v", r)
					}
				} else if r.CriticalFailures != 1 || !strings.Contains(strings.Join(r.GateReasons, ","), "critical/safety failure") {
					t.Fatalf("critical hard gate missing: %+v", r)
				}
			}
		})
	}
}

func TestMalformedArtifactsFailClosed(t *testing.T) {
	for _, kind := range []string{"missing", "duplicate", "extra", "revision", "negative", "invalid-json", "duplicate-case"} {
		t.Run(kind, func(t *testing.T) {
			s, b, c := fixtures(t)
			switch kind {
			case "missing":
				c.Runs = c.Runs[1:]
			case "duplicate":
				c.Runs[1] = c.Runs[0]
			case "extra":
				c.Runs = append(c.Runs, Run{ID: "unscored"})
			case "revision":
				c.DatasetRevision = "another-suite"
			case "negative":
				c.Runs[0].Cost = -1
			case "invalid-json":
				c.Runs[0].Calls[0].Args = json.RawMessage(`{"x":`)
			case "duplicate-case":
				s.Cases[1] = s.Cases[0]
			}
			if _, err := Compare(s, b, c, Policy{MinSuccess: 1}); err == nil {
				t.Fatal("expected artifact validation error")
			}
		})
	}
}

func TestRegressionAndCanonicalArguments(t *testing.T) {
	s, b, c := fixtures(t)
	c.Runs[0].Calls[0].Args = json.RawMessage(`{"account":"alice", "tenant":"tenant-a"}`)
	r, err := Compare(s, b, c, Policy{MinSuccess: 1})
	if err != nil || !r.GatePassed {
		t.Fatalf("key order must not change result: %+v %v", r, err)
	}
	c.Runs[0].Result = "wrong"
	r, err = Compare(s, b, c, Policy{MinSuccess: .5, MaxRegression: .1})
	if err != nil || r.GatePassed {
		t.Fatalf("regression accepted: %+v %v", r, err)
	}
}

func TestMissingTelemetryIsNotFree(t *testing.T) {
	for _, field := range []string{"cost_microusd", "latency_ms", "calls"} {
		for _, null := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/null=%v", field, null), func(t *testing.T) {
				_, _, candidate := fixtures(t)
				data, err := json.Marshal(candidate.Runs[0])
				if err != nil {
					t.Fatal(err)
				}
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(data, &fields); err != nil {
					t.Fatal(err)
				}
				if null {
					fields[field] = json.RawMessage("null")
				} else {
					delete(fields, field)
				}
				data, err = json.Marshal(fields)
				if err != nil {
					t.Fatal(err)
				}
				var run Run
				if err := json.Unmarshal(data, &run); err == nil {
					t.Fatal("incomplete trace must not silently become zero cost/latency or no tools")
				}
			})
		}
	}
}

func TestShortTraceStillChecksExecutedArguments(t *testing.T) {
	suite, base, candidate := fixtures(t)
	// The expected task has two tool calls; the candidate omits one and changes
	// the tenant of the executed call. Relaxed quality thresholds cannot hide it.
	suite.Cases[0].WantCalls = append(suite.Cases[0].WantCalls, suite.Cases[0].WantCalls[0])
	base.Runs[0].Calls = append(base.Runs[0].Calls, base.Runs[0].Calls[0])
	bindSuite(t, suite, &base, &candidate)
	candidate.Runs[0].Calls[0].Args = json.RawMessage(`{"tenant":"tenant-b","account":"alice"}`)
	report, err := Compare(suite, base, candidate, Policy{MinSuccess: .5, MaxRegression: 1})
	if err != nil {
		t.Fatal(err)
	}
	if report.GatePassed || report.CriticalFailures != 1 {
		t.Fatalf("shortened trace hid unsafe arguments: %+v", report)
	}
}

func TestSuiteContentBinding(t *testing.T) {
	for _, side := range []string{"baseline", "candidate"} {
		for _, kind := range []string{"missing-digest", "wrong-digest", "same-revision-changed-content"} {
			t.Run(side+"/"+kind, func(t *testing.T) {
				s, b, c := fixtures(t)
				a := &c
				if side == "baseline" {
					a = &b
				}
				switch kind {
				case "missing-digest":
					a.DatasetSHA256 = ""
				case "wrong-digest":
					a.DatasetSHA256 = strings.Repeat("0", 64)
				case "same-revision-changed-content":
					s.Cases[0].MaxLatency--
					// Bind the other artifact to the new suite to ensure each
					// side is independently checked, not masked by baseline failure.
					other := &b
					if side == "baseline" {
						other = &c
					}
					bindSuite(t, s, other)
				}
				if _, err := Compare(s, b, c, Policy{MinSuccess: 1}); err == nil || !strings.Contains(err.Error(), side+":") {
					t.Fatalf("binding failure not rejected for %s: %v", side, err)
				}
			})
		}
	}
}

func TestDigestIgnoresObjectKeyOrderOnly(t *testing.T) {
	s, b, c := fixtures(t)
	want, err := SuiteSHA256(s)
	if err != nil {
		t.Fatal(err)
	}
	s.Cases[0].WantCalls[0].Args = json.RawMessage(`{"account":"alice", "tenant":"tenant-a"}`)
	got, err := SuiteSHA256(s)
	if err != nil || got != want {
		t.Fatalf("key order changed suite digest: %s %v", got, err)
	}
	c.Runs[0].Calls[0].Args = json.RawMessage(`{"account":"alice", "tenant":"tenant-a"}`)
	r, err := Compare(s, b, c, Policy{MinSuccess: 1})
	if err != nil || !r.GatePassed {
		t.Fatalf("equivalent JSON rejected: %+v %v", r, err)
	}
	s.Cases[0], s.Cases[1] = s.Cases[1], s.Cases[0]
	got, err = SuiteSHA256(s)
	if err != nil || got == want {
		t.Fatalf("array order must change digest: %s %v", got, err)
	}
}

func TestReportRecordsComparisonInputs(t *testing.T) {
	s, b, c := fixtures(t)
	b.Version = "baseline-review"
	b.Runs[0].Result = "wrong"
	p := Policy{MinSuccess: .5, MaxRegression: 1}
	r, err := Compare(s, b, c, p)
	if err != nil || !r.GatePassed || r.Policy != p || r.BaselineVersion != b.Version || r.BaselineSuccess != .8 || r.DatasetRevision != s.Revision || len(r.BaselineSHA256) != 64 || len(r.CandidateSHA256) != 64 {
		t.Fatalf("incomplete comparison evidence: %+v %v", r, err)
	}
	baselineDigest, candidateDigest := r.BaselineSHA256, r.CandidateSHA256
	stricter, err := Compare(s, b, c, Policy{MinSuccess: 1})
	if err != nil || stricter.Policy == r.Policy || stricter.BaselineSHA256 != baselineDigest || stricter.CandidateSHA256 != candidateDigest {
		t.Fatalf("policy change not recorded: %+v %v", stricter, err)
	}
	b.Runs[0].Result = "another wrong answer"
	c.Runs[0].Cost++
	changed, err := Compare(s, b, c, p)
	if err != nil || changed.BaselineSHA256 == baselineDigest || changed.CandidateSHA256 == candidateDigest {
		t.Fatalf("artifact changes not recorded: %+v %v", changed, err)
	}
}

func TestDuplicateArgumentsRejectedForDirectCallers(t *testing.T) {
	for _, side := range []string{"suite", "baseline", "candidate"} {
		t.Run(side, func(t *testing.T) {
			s, b, c := fixtures(t)
			bad := json.RawMessage(`{"tenant":"tenant-b","tenant":"tenant-a","account":"alice"}`)
			switch side {
			case "suite":
				s.Cases[0].WantCalls[0].Args = bad
			case "baseline":
				b.Runs[0].Calls[0].Args = bad
			case "candidate":
				c.Runs[0].Calls[0].Args = bad
			}
			if _, err := Compare(s, b, c, Policy{MinSuccess: 1}); err == nil || !strings.Contains(err.Error(), "duplicate JSON key") {
				t.Fatalf("duplicate arguments accepted: %v", err)
			}
		})
	}
}

func TestInvalidPolicyFailsClosed(t *testing.T) {
	s, b, c := fixtures(t)
	for _, value := range []float64{-.01, 1.01, math.NaN(), math.Inf(1), math.Inf(-1)} {
		for _, p := range []Policy{{MinSuccess: value}, {MinSuccess: 1, MaxRegression: value}} {
			if _, err := Compare(s, b, c, p); err == nil {
				t.Fatalf("invalid policy accepted: %+v", p)
			}
		}
	}
}
