// Package promptcontext implements a local, versioned context and JSON contract fixture.
package promptcontext

import (
	"context"
	"errors"
	"td-homework/examples/senior/agentmemory"
)

var ErrBudget = errors.New("context budget exceeded")

// Group is a host-created atomic unit, never a model-created authority label.
// A tool interaction groups opaque reasoning, calls and their results together.
type Item struct{ Kind, Text, CallID string }
type Group struct {
	Scope    agentmemory.Scope
	ID       string
	Required bool
	Items    []Item
}
type Counter interface {
	Count(context.Context, []Item) (int, error)
	Version() string
}

// FixtureCounter is deliberately byte-based, NOT a real model tokenizer.
// It includes fixture message framing. Production must replace the whole counter.
type FixtureCounter struct{}

func (FixtureCounter) Version() string { return "fixture-bytes-v1" }
func (FixtureCounter) Count(ctx context.Context, items []Item) (int, error) {
	n := 3
	for _, i := range items {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		n += len(i.Text) + len(i.CallID) + len(i.Kind) + 4
	}
	return n, nil
}

type Budget struct{ Window, Output, Reasoning, MaxOutput, Margin int }
type Assembly struct {
	Items          []Item
	Kept, Dropped  []string
	Input          int
	CounterVersion string
	scope          agentmemory.Scope
	identity       agentmemory.Identity
	counter        Counter
	allowance      int
}

func validate(groups []Group) error {
	if len(groups) == 0 || len(groups) > 128 {
		return errors.New("invalid group count")
	}
	seenGroups, seenCalls := map[string]bool{}, map[string]bool{}
	system, current := 0, 0
	size := 0
	for _, g := range groups {
		if g.ID == "" || seenGroups[g.ID] || len(g.Items) == 0 || len(g.Items) > 256 {
			return errors.New("invalid group")
		}
		seenGroups[g.ID] = true
		pending := map[string]bool{}
		for _, i := range g.Items {
			size += len(i.Text) + len(i.CallID)
			if size > 128<<10 {
				return errors.New("context input too large")
			}
			switch i.Kind {
			case "system":
				system++
				if !g.Required {
					return errors.New("system must be required")
				}
			case "current_user":
				current++
				if !g.Required {
					return errors.New("current task must be required")
				}
			case "user", "assistant", "data", "reasoning":
			case "call":
				if i.CallID == "" || seenCalls[i.CallID] {
					return errors.New("duplicate or missing call ID")
				}
				seenCalls[i.CallID] = true
				pending[i.CallID] = true
			case "result":
				if !pending[i.CallID] {
					return errors.New("orphan tool result")
				}
				delete(pending, i.CallID)
			default:
				return errors.New("unsupported item")
			}
		}
		if len(pending) != 0 {
			return errors.New("incomplete tool group; persist/resolve pending work outside the prompt")
		}
	}
	if system != 1 || current != 1 {
		return errors.New("one system and current task required")
	}
	return nil
}

// Assemble accepts only history/data already authorized by the trusted host.
// Scope permission is checked here and again before returning the assembly.
func Assemble(ctx context.Context, scope agentmemory.Scope, groups []Group, b Budget, c Counter) (Assembly, error) {
	who, err := agentmemory.Check(ctx, scope, "read")
	if err != nil {
		return Assembly{}, err
	}
	for _, g := range groups {
		if g.Scope != scope {
			return Assembly{}, agentmemory.ErrDenied
		}
	}
	if err := validate(groups); err != nil {
		return Assembly{}, err
	}
	if c == nil {
		return Assembly{}, errors.New("missing tokenizer contract")
	}
	version := c.Version()
	if version == "" {
		return Assembly{}, errors.New("missing tokenizer contract")
	}
	// Output is visible output; Reasoning is additional reserve. On APIs where
	// max_output_tokens includes reasoning, send Output+Reasoning, not both twice.
	if b.Window < 1 || b.Window > 1<<30 || b.Output < 1 || b.Reasoning < 0 || b.Margin < 0 || b.MaxOutput < 1 || b.MaxOutput > b.Window || b.Output > b.MaxOutput || b.Reasoning > b.MaxOutput-b.Output || b.Margin > b.Window-(b.Output+b.Reasoning) {
		return Assembly{}, ErrBudget
	}
	allowance := b.Window - b.Output - b.Reasoning - b.Margin
	kept := make([]bool, len(groups))
	for i := range kept {
		kept[i] = true
	}
	count := func() (int, []Item, error) {
		items := []Item{}
		for i, g := range groups {
			if kept[i] {
				items = append(items, g.Items...)
			}
		}
		n, err := c.Count(ctx, items)
		if err != nil {
			return 0, nil, err
		}
		if c.Version() != version {
			return 0, nil, errors.New("counter version changed during assembly")
		}
		if n < 0 {
			return 0, nil, errors.New("negative token count")
		}
		return n, items, nil
	}
	n, items, err := count()
	if err != nil {
		return Assembly{}, err
	}
	for i, g := range groups {
		if n <= allowance {
			break
		}
		if !g.Required {
			kept[i] = false
			n, items, err = count()
			if err != nil {
				return Assembly{}, err
			}
		}
	}
	if n > allowance {
		return Assembly{}, ErrBudget
	}
	if _, err = agentmemory.Check(ctx, scope, "read"); err != nil {
		return Assembly{}, err
	}
	out := Assembly{Items: items, Input: n, CounterVersion: version, scope: scope, identity: who, counter: c, allowance: allowance}
	for i, g := range groups {
		if kept[i] {
			out.Kept = append(out.Kept, g.ID)
		} else {
			out.Dropped = append(out.Dropped, g.ID)
		}
	}
	return out, nil
}
