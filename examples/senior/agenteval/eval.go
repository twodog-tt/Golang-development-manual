// Package agenteval evaluates recorded agent runs without calling a model or tools.
package agenteval

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

type Call struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type Case struct {
	ID           string   `json:"id"`
	Slice        string   `json:"slice"`
	Critical     bool     `json:"critical"`
	AllowedTools []string `json:"allowed_tools"`
	WantStatus   string   `json:"want_status"`
	WantResult   string   `json:"want_result"`
	WantCalls    []Call   `json:"want_calls"`
	MaxCost      int64    `json:"max_cost_microusd"`
	MaxLatency   int64    `json:"max_latency_ms"`
}

type Suite struct {
	Revision string `json:"revision"`
	Cases    []Case `json:"cases"`
}

type Run struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Result  string `json:"result"`
	Calls   []Call `json:"calls"`
	Cost    int64  `json:"cost_microusd"`
	Latency int64  `json:"latency_ms"`
}

// UnmarshalJSON requires explicit observations. Missing/null telemetry is not a
// free run, and a missing tool trace is not evidence that no tools executed.
func (r *Run) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, name := range []string{"id", "status", "result", "calls", "cost_microusd", "latency_ms"} {
		value, exists := fields[name]
		if !exists || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("run requires non-null %q", name)
		}
	}
	type wireRun Run
	var wire wireRun
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return err
	}
	*r = Run(wire)
	return nil
}

type Artifact struct {
	Version         string `json:"version"`
	DatasetRevision string `json:"dataset_revision"`
	Runs            []Run  `json:"runs"`
}

type Policy struct {
	MinSuccess    float64
	MaxRegression float64
}

type Result struct {
	ID      string   `json:"id"`
	Slice   string   `json:"slice"`
	Passed  bool     `json:"passed"`
	Reasons []string `json:"reasons,omitempty"`
}

type Report struct {
	Version          string   `json:"version"`
	DatasetSHA256    string   `json:"dataset_sha256"`
	SuccessRate      float64  `json:"success_rate"`
	CriticalFailures int      `json:"critical_failures"`
	BudgetFailures   int      `json:"budget_failures"`
	TotalCost        int64    `json:"total_cost_microusd"`
	P95Latency       int64    `json:"p95_latency_ms"`
	Results          []Result `json:"results"`
	GatePassed       bool     `json:"gate_passed"`
	GateReasons      []string `json:"gate_reasons,omitempty"`
}

func canonical(raw json.RawMessage) ([]byte, error) {
	var value any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := d.Decode(&value); err != nil {
		return nil, err
	}
	// Unmarshal rejects a trailing JSON value as well.
	if !json.Valid(raw) {
		return nil, fmt.Errorf("invalid JSON arguments")
	}
	return json.Marshal(value)
}

func score(s Suite, a Artifact) (Report, error) {
	r := Report{Version: a.Version}
	if s.Revision == "" || len(s.Cases) == 0 || a.Version == "" || a.DatasetRevision != s.Revision {
		return r, fmt.Errorf("nonempty suite/version and matching dataset revision required")
	}
	encoded, _ := json.Marshal(s)
	r.DatasetSHA256 = fmt.Sprintf("%x", sha256.Sum256(encoded))
	runs := make(map[string]Run)
	for _, run := range a.Runs {
		if _, exists := runs[run.ID]; exists {
			return r, fmt.Errorf("duplicate run %q", run.ID)
		}
		if run.Cost < 0 || run.Latency < 0 {
			return r, fmt.Errorf("negative telemetry: %s", run.ID)
		}
		runs[run.ID] = run
	}
	if len(runs) != len(s.Cases) {
		return r, fmt.Errorf("coverage mismatch: want %d runs, got %d", len(s.Cases), len(runs))
	}
	seen := make(map[string]bool)
	passed := 0
	var latencies []int64
	for _, c := range s.Cases {
		if c.ID == "" || seen[c.ID] || c.Slice == "" || c.WantStatus == "" || c.MaxCost < 0 || c.MaxLatency <= 0 {
			return r, fmt.Errorf("invalid or duplicate case %q", c.ID)
		}
		seen[c.ID] = true
		run, exists := runs[c.ID]
		if !exists {
			return r, fmt.Errorf("missing case %s", c.ID)
		}
		result := Result{ID: c.ID, Slice: c.Slice, Passed: true}
		fail := func(reason string) { result.Passed = false; result.Reasons = append(result.Reasons, reason) }
		allowed := make(map[string]bool)
		for _, name := range c.AllowedTools {
			allowed[name] = true
		}
		unsafe := false
		for _, call := range run.Calls {
			if !allowed[call.Name] {
				unsafe = true
				fail("unauthorized tool: " + call.Name)
			}
			if _, err := canonical(call.Args); err != nil {
				return r, fmt.Errorf("case %s: %w", c.ID, err)
			}
		}
		for _, call := range c.WantCalls {
			if !allowed[call.Name] {
				return r, fmt.Errorf("invalid golden tool: %s", c.ID)
			}
			if _, err := canonical(call.Args); err != nil {
				return r, err
			}
		}
		if run.Status != c.WantStatus || run.Result != c.WantResult {
			fail("status/result mismatch")
		}
		if len(run.Calls) != len(c.WantCalls) {
			fail("tool count mismatch")
			if len(run.Calls) > len(c.WantCalls) {
				unsafe = true
			}
		}
		// Check executed calls even when the trace is shorter than the golden
		// trace; a count mismatch must not hide a wrong tenant or recipient.
		for i, call := range run.Calls {
			if i >= len(c.WantCalls) {
				break // Extra calls already make the run unsafe above.
			}
			actual, _ := canonical(call.Args)
			want, _ := canonical(c.WantCalls[i].Args)
			if call.Name != c.WantCalls[i].Name || !bytes.Equal(actual, want) {
				unsafe = true
				fail("tool order/arguments mismatch")
			}
		}
		if run.Cost > c.MaxCost || run.Latency > c.MaxLatency {
			r.BudgetFailures++
			fail("cost/latency budget exceeded")
		}
		if !result.Passed && (c.Critical || unsafe) {
			r.CriticalFailures++
		}
		if result.Passed {
			passed++
		}
		if run.Cost > math.MaxInt64-r.TotalCost {
			return r, fmt.Errorf("cost overflow")
		}
		r.TotalCost += run.Cost
		latencies = append(latencies, run.Latency)
		r.Results = append(r.Results, result)
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	r.P95Latency = latencies[int(math.Ceil(.95*float64(len(latencies))))-1]
	r.SuccessRate = float64(passed) / float64(len(s.Cases))
	return r, nil
}

// Compare fails closed on missing/duplicate cases, safety failures and budget violations.
// Both artifacts must cover exactly the same frozen suite. It compares recorded samples,
// not statistical significance or live model quality.
func Compare(s Suite, baseline, candidate Artifact, p Policy) (Report, error) {
	if math.IsNaN(p.MinSuccess) || math.IsNaN(p.MaxRegression) || p.MinSuccess < 0 || p.MinSuccess > 1 || p.MaxRegression < 0 || p.MaxRegression > 1 {
		return Report{}, fmt.Errorf("policy rates must be in [0,1]")
	}
	b, err := score(s, baseline)
	if err != nil {
		return Report{}, fmt.Errorf("baseline: %w", err)
	}
	r, err := score(s, candidate)
	if err != nil {
		return r, fmt.Errorf("candidate: %w", err)
	}
	if r.CriticalFailures > 0 {
		r.GateReasons = append(r.GateReasons, "critical/safety failure")
	}
	if r.BudgetFailures > 0 {
		r.GateReasons = append(r.GateReasons, "budget exceeded")
	}
	if r.SuccessRate < p.MinSuccess {
		r.GateReasons = append(r.GateReasons, "below minimum success rate")
	}
	if b.SuccessRate-r.SuccessRate > p.MaxRegression+1e-12 {
		r.GateReasons = append(r.GateReasons, "regression against baseline")
	}
	r.GatePassed = len(r.GateReasons) == 0
	return r, nil
}
