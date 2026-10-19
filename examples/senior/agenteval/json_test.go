package agenteval

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestRequiredFieldsFailClosed(t *testing.T) {
	s, b, c := fixtures(t)
	for _, obj := range []struct {
		name  string
		value any
		dest  func() any
	}{
		{"suite", s, func() any { return new(Suite) }},
		{"case", s.Cases[4], func() any { return new(Case) }},
		{"artifact", b, func() any { return new(Artifact) }},
		{"run", c.Runs[0], func() any { return new(Run) }},
		{"call", c.Runs[0].Calls[0], func() any { return new(Call) }},
	} {
		data, err := json.Marshal(obj.value)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err != nil {
			t.Fatal(err)
		}
		for field := range fields {
			for _, kind := range []string{"missing", "null"} {
				t.Run(obj.name+"/"+field+"/"+kind, func(t *testing.T) {
					var changed map[string]json.RawMessage
					if err := json.Unmarshal(data, &changed); err != nil {
						t.Fatal(err)
					}
					if kind == "missing" {
						delete(changed, field)
					} else {
						changed[field] = json.RawMessage("null")
					}
					raw, err := json.Marshal(changed)
					if err != nil {
						t.Fatal(err)
					}
					if err := DecodeJSON(raw, obj.dest()); err == nil {
						t.Fatal("incomplete input accepted")
					}
				})
			}
		}
	}
}

func TestAmbiguousAndMalformedJSONRejected(t *testing.T) {
	data, err := os.ReadFile("testdata/candidate-good.json")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for name, raw := range map[string]string{
		"duplicate-root":         strings.Replace(source, `"version":`, `"version":"overwritten", "version":`, 1),
		"duplicate-run-cost":     strings.Replace(source, `"cost_microusd": 20`, `"cost_microusd":99999,"cost_microusd":20`, 1),
		"duplicate-tenant":       strings.Replace(source, `"tenant": "tenant-a"`, `"tenant":"tenant-b","tenant":"tenant-a"`, 1),
		"escaped-duplicate":      strings.Replace(source, `"tenant": "tenant-a"`, `"tenant":"tenant-b","\u0074enant":"tenant-a"`, 1),
		"nested-array-duplicate": strings.Replace(source, `"tenant": "tenant-a"`, `"tenant":[{"id":1,"id":2}]`, 1),
		"case-alias":             strings.Replace(source, `"cost_microusd": 20`, `"cost_microusd":20,"COST_MICROUSD":0`, 1),
		"unknown-root":           strings.Replace(source, `"version":`, `"typo":true,"version":`, 1),
		"unknown-run":            strings.Replace(source, `"status":`, `"typo":true,"status":`, 1),
		"unknown-call":           strings.Replace(source, `"name":`, `"typo":true,"name":`, 1),
		"trailing-value":         source + ` {}`,
		"trailing-garbage":       source + ` broken`,
		"truncated":              source[:len(source)/2],
		"null":                   "null",
	} {
		t.Run(name, func(t *testing.T) {
			var a Artifact
			if err := DecodeJSON([]byte(raw), &a); err == nil {
				t.Fatal("ambiguous/malformed input accepted")
			}
		})
	}
}
