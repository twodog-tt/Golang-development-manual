package main

import (
	"context"
	"encoding/json"
	"os"
	r "td-homework/examples/senior/releaseadmission"
	"time"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func main() {
	dir, err := os.MkdirTemp("", "release-admission-demo-")
	must(err)
	defer os.RemoveAll(dir)
	now := time.Unix(1800000000, 0)
	f, err := r.NewFixture(dir, now)
	must(err)
	clock := func() time.Time { return now }
	g, err := r.NewGate(f.Policy, f.Target, f.Root, clock)
	must(err)
	ticket, err := g.Prepare(context.Background(), "release-1", f.Bundle)
	must(err)
	_, err = g.Commit(context.Background(), ticket)
	must(err)
	_, err = g.Commit(context.Background(), ticket)
	must(err)
	blocked, err := r.NewGate(f.Policy, f.Target, f.Root, clock)
	must(err)
	old, err := blocked.Prepare(context.Background(), "release-2", f.Bundle)
	must(err)
	changed := f.Target
	changed.Implementation = "implementation-v2"
	blocked.SetTarget(changed)
	_, err = blocked.Commit(context.Background(), old)
	staleBlocked := err != nil
	// The exception permits only this one authenticated vulnerability. It cannot
	// replace failed tests or authorize an altered target.
	f.Bundle.ScanReport = []byte(`{"schema":"local-scan-v1","status":"complete","dbVersion":"fixture-db-v1","dbUpdatedAt":1800000000,"findings":[{"id":"CVE-FIXTURE-1","severity":"high"}]}`)
	f.LinkReports()
	f.Approve()
	riskGate, err := r.NewGate(f.Policy, f.Target, f.Root, clock)
	must(err)
	_, err = riskGate.Prepare(context.Background(), "risk", f.Bundle)
	riskBlocked := err != nil
	f.Waive("CVE-FIXTURE-1")
	waiver, err := riskGate.Prepare(context.Background(), "risk", f.Bundle)
	must(err)
	riskGate.RevokeException("exception-1")
	_, err = riskGate.Commit(context.Background(), waiver)
	revokedBlocked := err != nil
	passed := g.Effects() == 1 && staleBlocked && blocked.Effects() == 0 && riskBlocked && revokedBlocked && riskGate.Effects() == 0
	must(json.NewEncoder(os.Stdout).Encode(map[string]any{"passed": passed, "dry_run_effects": g.Effects(), "stale_target_blocked": staleBlocked, "unwaived_risk_blocked": riskBlocked, "revoked_exception_blocked": revokedBlocked, "real_deployments": 0, "slsa_level_claimed": false}))
	if !passed {
		os.Exit(1)
	}
}
