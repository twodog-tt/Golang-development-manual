package main

import "testing"

func TestIntegratedKnowledgeFixtures(t *testing.T) {
	r, e := run(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if r.HTTPAttempts != 2 || r.Costs.Attempts != 2 || r.Costs.Unknown != 0 || r.Costs.Failed != 1 {
		t.Fatal(r)
	}
}
