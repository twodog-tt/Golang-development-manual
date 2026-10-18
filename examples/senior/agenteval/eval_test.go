package agenteval

import (
	"encoding/json"
	"fmt"
	"os"
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
			if kind == "unauthorized" || kind == "tenant-args" {
				r, err = Compare(s, b, c, Policy{MinSuccess: .5, MaxRegression: 1})
				if err != nil || r.GatePassed || r.CriticalFailures != 1 {
					t.Fatalf("safety averaged away: %+v %v", r, err)
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
	candidate.Runs[0].Calls[0].Args = json.RawMessage(`{"tenant":"tenant-b","account":"alice"}`)
	report, err := Compare(suite, base, candidate, Policy{MinSuccess: .5, MaxRegression: 1})
	if err != nil {
		t.Fatal(err)
	}
	if report.GatePassed || report.CriticalFailures != 1 {
		t.Fatalf("shortened trace hid unsafe arguments: %+v", report)
	}
}
