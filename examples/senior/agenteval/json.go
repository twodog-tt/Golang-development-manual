package agenteval

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// checkJSON rejects ambiguous objects before decoding into maps or structs can
// discard duplicate keys. Keys are compared after JSON escape decoding.
func checkJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	var value func() error
	value = func() error {
		token, err := d.Token()
		if err != nil {
			return err
		}
		switch token {
		case json.Delim('{'):
			seen := make(map[string]bool)
			for d.More() {
				keyToken, err := d.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok {
					return fmt.Errorf("expected JSON object key")
				}
				if seen[key] {
					return fmt.Errorf("duplicate JSON key %q", key)
				}
				seen[key] = true
				if err := value(); err != nil {
					return err
				}
			}
			_, err = d.Token() // Decoder validates the closing delimiter.
			return err
		case json.Delim('['):
			for d.More() {
				if err := value(); err != nil {
					return err
				}
			}
			_, err = d.Token()
			return err
		}
		return nil
	}
	if err := value(); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("expected one JSON value")
	}
	return nil
}

// DecodeJSON is the shared strict entry point for CLI inputs and integrations.
func DecodeJSON(data []byte, dest any) error {
	if err := checkJSON(data); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	return d.Decode(dest)
}

// All fields of the fixture wire types are explicit and non-null, including
// critical=false and empty call lists. Exact keys also avoid Go's default
// case-insensitive field matching overriding a previously decoded value.
func decodeRequired(data []byte, dest any, names ...string) error {
	if err := checkJSON(data); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, name := range names {
		v, ok := fields[name]
		if !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return fmt.Errorf("requires non-null %q", name)
		}
		delete(fields, name)
	}
	for name := range fields {
		return fmt.Errorf("unknown field %q", name)
	}
	return json.Unmarshal(data, dest)
}

func (c *Call) UnmarshalJSON(data []byte) error {
	type wire Call
	return decodeRequired(data, (*wire)(c), "name", "args")
}

func (c *Case) UnmarshalJSON(data []byte) error {
	type wire Case
	return decodeRequired(data, (*wire)(c), "id", "slice", "critical", "allowed_tools", "want_status", "want_result", "want_calls", "max_cost_microusd", "max_latency_ms")
}

func (s *Suite) UnmarshalJSON(data []byte) error {
	type wire Suite
	return decodeRequired(data, (*wire)(s), "revision", "cases")
}

func (r *Run) UnmarshalJSON(data []byte) error {
	type wire Run
	return decodeRequired(data, (*wire)(r), "id", "status", "result", "calls", "cost_microusd", "latency_ms")
}

func (a *Artifact) UnmarshalJSON(data []byte) error {
	type wire Artifact
	return decodeRequired(data, (*wire)(a), "version", "dataset_revision", "dataset_sha256", "runs")
}
