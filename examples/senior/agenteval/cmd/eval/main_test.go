package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"td-homework/examples/senior/agenteval"
)

func TestCLIExitCodesAndEvidence(t *testing.T) {
	for _, kind := range []string{
		"good", "unsafe", "unknown-to-success", "approval-bypass", "budget", "latency",
		"missing-critical", "null-critical", "duplicate-tenant", "duplicate-cost", "escaped-duplicate",
		"missing-telemetry", "missing-hash", "wrong-baseline-hash", "changed-suite", "trailing-json", "unknown-field",
		"invalid-policy", "unknown-flag", "positional-argument", "suite-hash",
	} {
		t.Run(kind, func(t *testing.T) {
			files := make(map[string][]byte)
			for _, name := range []string{"suite", "baseline", "candidate-good", "candidate-unsafe"} {
				data, err := os.ReadFile(filepath.Join("..", "..", "testdata", name+".json"))
				if err != nil {
					t.Fatal(err)
				}
				files[name] = data
			}
			suite, base, candidate := string(files["suite"]), string(files["baseline"]), string(files["candidate-good"])
			flags := []string{"-min-success", ".5", "-max-regression", "1"}
			want := 2
			switch kind {
			case "good":
				want = 0
			case "unsafe":
				candidate = string(files["candidate-unsafe"])
				want = 1
			case "unknown-to-success":
				candidate = strings.Replace(candidate, `"status": "unknown"`, `"status": "done"`, 1)
				want = 1
			case "approval-bypass":
				candidate = strings.Replace(candidate, `"status": "waiting_approval"`, `"status": "done"`, 1)
				want = 1
			case "budget":
				candidate = strings.Replace(candidate, `"cost_microusd": 20`, `"cost_microusd": 101`, 1)
				want = 1
			case "latency":
				candidate = strings.Replace(candidate, `"latency_ms": 100`, `"latency_ms": 1001`, 1)
				want = 1
			case "missing-critical":
				suite = strings.Replace(suite, `"critical": true,`, "", 1)
			case "null-critical":
				suite = strings.Replace(suite, `"critical": true`, `"critical": null`, 1)
			case "duplicate-tenant":
				candidate = strings.Replace(candidate, `"tenant": "tenant-a"`, `"tenant":"tenant-b","tenant":"tenant-a"`, 1)
			case "duplicate-cost":
				candidate = strings.Replace(candidate, `"cost_microusd": 20`, `"cost_microusd":99999,"cost_microusd":20`, 1)
			case "escaped-duplicate":
				candidate = strings.Replace(candidate, `"tenant": "tenant-a"`, `"tenant":"tenant-b","\u0074enant":"tenant-a"`, 1)
			case "missing-telemetry":
				candidate = strings.Replace(candidate, `"cost_microusd": 20,`, "", 1)
			case "missing-hash":
				var a map[string]json.RawMessage
				if err := json.Unmarshal([]byte(candidate), &a); err != nil {
					t.Fatal(err)
				}
				delete(a, "dataset_sha256")
				data, err := json.Marshal(a)
				if err != nil {
					t.Fatal(err)
				}
				candidate = string(data)
			case "wrong-baseline-hash":
				base = strings.Replace(base, `"dataset_sha256": "`, `"dataset_sha256": "wrong-`, 1)
			case "changed-suite":
				suite = strings.Replace(suite, `"max_latency_ms": 1000`, `"max_latency_ms": 999`, 1)
			case "trailing-json":
				candidate += " {}"
			case "unknown-field":
				candidate = strings.Replace(candidate, `"version":`, `"typo":true,"version":`, 1)
			case "invalid-policy":
				flags = []string{"-min-success", "NaN"}
			case "unknown-flag":
				flags = []string{"-unknown"}
			case "positional-argument":
				flags = []string{"ignored.json"}
			case "suite-hash":
				flags = []string{"-suite-hash"}
				want = 0
			}
			dir := t.TempDir()
			for name, data := range map[string]string{"suite": suite, "baseline": base, "candidate": candidate} {
				if err := os.WriteFile(filepath.Join(dir, name+".json"), []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"-suite", filepath.Join(dir, "suite.json"), "-baseline", filepath.Join(dir, "baseline.json"), "-candidate", filepath.Join(dir, "candidate.json")}
			var out, stderr bytes.Buffer
			status := run(append(args, flags...), &out, &stderr)
			if status != want {
				t.Fatalf("exit=%d want=%d out=%s err=%s", status, want, out.String(), stderr.String())
			}
			if kind == "suite-hash" {
				var s agenteval.Suite
				if err := agenteval.DecodeJSON([]byte(suite), &s); err != nil {
					t.Fatal(err)
				}
				digest, err := agenteval.SuiteSHA256(s)
				if err != nil || strings.TrimSpace(out.String()) != digest {
					t.Fatalf("wrong digest: %s %v", out.String(), err)
				}
				return
			}
			if want == 2 {
				if out.Len() != 0 || stderr.Len() == 0 {
					t.Fatalf("input failure must have no success report: out=%s err=%s", out.String(), stderr.String())
				}
				return
			}
			var report agenteval.Report
			if err := json.Unmarshal(out.Bytes(), &report); err != nil {
				t.Fatal(err)
			}
			if report.GatePassed != (want == 0) || report.Policy.MinSuccess != .5 || report.Policy.MaxRegression != 1 || report.BaselineVersion != "baseline-v1" || report.BaselineSuccess != 1 || len(report.BaselineSHA256) != 64 || len(report.CandidateSHA256) != 64 {
				t.Fatalf("incomplete/wrong report: %+v", report)
			}
			if kind == "unknown-to-success" || kind == "approval-bypass" {
				if report.CriticalFailures != 1 || len(report.GateReasons) != 1 || report.GateReasons[0] != "critical/safety failure" {
					t.Fatalf("critical failure masked by average: %+v", report)
				}
			}
			if kind == "budget" || kind == "latency" {
				if report.BudgetFailures != 1 || len(report.GateReasons) != 1 || report.GateReasons[0] != "budget exceeded" {
					t.Fatalf("budget failure masked by average: %+v", report)
				}
			}
		})
	}
}
