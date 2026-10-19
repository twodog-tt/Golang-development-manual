package agentworkflow

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"
)

// Item is a small offline protocol fixture, not a vendor SDK wire type. A real
// adapter must preserve all response items/IDs (including opaque reasoning), or
// explicitly use correctly scoped server-side response continuation.
type ToolCall struct {
	ID, Name string
	Proposal Proposal
}
type Item struct {
	Kind, Text, CallID string
	Call               *ToolCall
}
type Turn struct {
	Items        []Item
	Final        bool
	Tokens, Cost int64
}
type Reservation struct{ Tokens, Cost int64 }
type Model interface {
	Next(context.Context, []Item, Reservation) (Turn, error)
}
type Limits struct {
	MaxSteps, MaxCalls              int
	MaxTokens, MaxCost, ToolReserve int64
	Deadline                        time.Time
}
type Run struct {
	ID, Tenant, Actor, Input, Status string
	Limits                           Limits
	Steps, Calls                     int
	Tokens, Cost                     int64 // conservative reservations, not measured model billing
	History                          []Item
	Pending                          []ToolCall
	Next                             int
	ModelPending                     bool
}
type Runner struct {
	Engine  Engine
	Model   Model
	Reserve Reservation
	Tool    func(context.Context, ToolCall) (string, error)
}

func runKey(p Principal, id string) string {
	b, _ := json.Marshal([]string{p.Tenant, id})
	return string(b)
}
func (r Runner) Start(ctx context.Context, id, input string, l Limits) error {
	a, err := identity(ctx)
	if err != nil {
		return err
	}
	return r.Engine.Store.transaction(func(s *state) error {
		if id == "" || input == "" || l.MaxSteps <= 0 || l.MaxCalls <= 0 || l.MaxTokens <= 0 || l.MaxCost <= 0 || l.ToolReserve <= 0 || !l.Deadline.After(r.Engine.now()) {
			return ErrBudget
		}
		key := runKey(a, id)
		if old := s.Runs[key]; old != nil {
			if old.Actor == a.ID && old.Input == input && old.Limits == l {
				return nil
			}
			return ErrConflict
		}
		s.Runs[key] = &Run{ID: id, Tenant: a.Tenant, Actor: a.ID, Input: input, Limits: l, Status: "running", History: []Item{{Kind: "user", Text: input}}}
		return nil
	})
}
func (r Runner) Snapshot(ctx context.Context, id string) (Run, error) {
	a, err := identity(ctx)
	if err != nil {
		return Run{}, err
	}
	var out Run
	err = r.Engine.Store.transaction(func(s *state) error {
		x := s.Runs[runKey(a, id)]
		if x == nil || x.Actor != a.ID {
			return ErrDenied
		}
		out = *x
		return nil
	})
	return out, err
}
func validateTurn(history []Item, t Turn) ([]ToolCall, error) {
	seen := map[string]bool{}
	for _, v := range history {
		if v.Call != nil {
			seen[v.Call.ID] = true
		}
	}
	var calls []ToolCall
	message := false
	for _, item := range t.Items {
		switch item.Kind {
		case "reasoning", "message":
			if item.Call != nil || item.CallID != "" {
				return nil, ErrConflict
			}
			if item.Kind == "message" {
				message = true
			}
		case "function_call":
			if item.Call == nil || item.Call.ID == "" || item.Call.Name == "" || seen[item.Call.ID] || item.CallID != "" {
				return nil, ErrConflict
			}
			seen[item.Call.ID] = true
			calls = append(calls, *item.Call)
		default:
			return nil, ErrConflict // a model cannot forge host tool results
		}
	}
	if t.Final {
		if len(calls) > 0 || !message {
			return nil, ErrConflict
		}
	} else if len(calls) == 0 {
		return nil, ErrConflict
	}
	return calls, nil
}
func finalText(x Run) string {
	var text []string
	for _, v := range x.History {
		if v.Kind == "message" {
			text = append(text, v.Text)
		}
	}
	return strings.Join(text, "\n")
}

func (r Runner) Continue(ctx context.Context, id string) (string, error) {
	a, err := identity(ctx)
	if err != nil {
		return "", err
	}
	x, err := r.Snapshot(ctx, id)
	if err != nil {
		return "", err
	}
	if x.Status != "succeeded" && (r.Model == nil || r.Tool == nil || r.Reserve.Tokens <= 0 || r.Reserve.Cost <= 0) {
		return "", ErrConflict
	}
	// Nonblocking per-run OS ownership spans network calls, unlike the short
	// state transaction lock. A process crash releases it automatically.
	key := runKey(a, id)
	lock, err := os.OpenFile(fmt.Sprintf("%s.run-%x.lock", r.Engine.Store.Path, sha256.Sum256([]byte(key))), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return "", err
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return "", ErrConflict
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	ctx, cancel := context.WithDeadline(ctx, x.Limits.Deadline)
	defer cancel()
	// The OS lock owns this run; the durable marker preserves an uncertain
	// model request after a crash instead of silently spending another request.
	err = r.Engine.Store.transaction(func(s *state) error {
		x := s.Runs[key]
		if x.ModelPending {
			return ErrUnknown
		}
		if x.Status == "budget_blocked" {
			return ErrBudget
		}
		if x.Status == "succeeded" {
			return nil
		}
		x.Status = "busy"
		return nil
	})
	if err != nil {
		return "", err
	}
	defer r.Engine.Store.transaction(func(s *state) error {
		x := s.Runs[key]
		if x.Status == "busy" {
			x.Status = "paused"
		}
		return nil
	})
	for {
		if !x.Limits.Deadline.After(r.Engine.now()) {
			return "", context.DeadlineExceeded
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		x, err = r.Snapshot(ctx, id)
		if err != nil {
			return "", err
		}
		if x.Status == "succeeded" {
			return finalText(x), nil
		}
		if x.Next < len(x.Pending) {
			call := x.Pending[x.Next]
			err = r.Engine.Store.transaction(func(s *state) error {
				v := s.Runs[key]
				if v.Calls >= v.Limits.MaxCalls || v.Limits.ToolReserve > v.Limits.MaxCost-v.Cost {
					return ErrBudget
				}
				v.Calls++
				v.Cost += v.Limits.ToolReserve
				return nil
			})
			if err != nil {
				return "", err
			}
			out, callErr := r.Tool(ctx, call)
			if callErr != nil {
				// No model round is allowed after a side-effect/approval error.
				// Pending retains its call ID; resume consults the durable job.
				return "", callErr
			}
			err = r.Engine.Store.transaction(func(s *state) error {
				v := s.Runs[key]
				v.History = append(v.History, Item{Kind: "function_call_output", CallID: call.ID, Text: out})
				v.Next++
				return nil
			})
			if err != nil {
				return "", err
			}
			continue
		}
		if r.Reserve.Tokens <= 0 || r.Reserve.Cost <= 0 || r.Model == nil || r.Tool == nil {
			return "", ErrConflict
		}
		err = r.Engine.Store.transaction(func(s *state) error {
			v := s.Runs[key]
			if v.Steps >= v.Limits.MaxSteps || r.Reserve.Tokens > v.Limits.MaxTokens-v.Tokens || r.Reserve.Cost > v.Limits.MaxCost-v.Cost {
				return ErrBudget
			}
			v.Steps++
			v.Tokens += r.Reserve.Tokens
			v.Cost += r.Reserve.Cost
			v.ModelPending = true
			return nil
		})
		if err != nil {
			return "", err
		}
		turn, modelErr := r.Model.Next(ctx, x.History, r.Reserve)
		if modelErr != nil {
			return "", ErrUnknown
		} // attempted request retains its reservation and quarantine
		calls, validationErr := validateTurn(x.History, turn)
		if validationErr != nil || turn.Tokens < 0 || turn.Cost < 0 || turn.Tokens > r.Reserve.Tokens || turn.Cost > r.Reserve.Cost {
			return "", ErrConflict
		}
		err = r.Engine.Store.transaction(func(s *state) error {
			v := s.Runs[key]
			v.ModelPending = false
			// Reject an oversized batch before executing its first tool.
			if len(calls) > v.Limits.MaxCalls-v.Calls {
				v.Status = "budget_blocked"
				return nil
			}
			v.History = append(v.History, turn.Items...)
			v.Pending = calls
			v.Next = 0
			if turn.Final {
				v.Status = "succeeded"
			}
			return nil
		})
		if err != nil {
			return "", err
		}
		x, err = r.Snapshot(ctx, id)
		if err != nil {
			return "", err
		}
		if x.Status == "budget_blocked" {
			return "", ErrBudget
		}
	}
}

// PublishTool routes model proposals through the same deterministic control
// plane. Repeated call IDs or new model rounds cannot bypass an unknown job.
func (e Engine) PublishTool(p Provider) func(context.Context, ToolCall) (string, error) {
	return func(ctx context.Context, c ToolCall) (string, error) {
		if c.Name != "publish" {
			return "", ErrDenied
		}
		a, err := identity(ctx)
		if err != nil {
			return "", err
		}
		if err = e.Store.transaction(func(s *state) error {
			if !allowed(s, a, c.Proposal.Binding, false) {
				return ErrDenied
			}
			return nil
		}); err != nil {
			return "", err
		}
		j, err := e.Job(c.Proposal.ID)
		if errors.Is(err, ErrConflict) {
			if err = e.Propose(ctx, c.Proposal); err != nil {
				return "", err
			}
			return "", ErrApproval
		}
		if err != nil {
			return "", err
		}
		if j.Proposal != c.Proposal {
			return "", ErrConflict
		}
		switch j.Status {
		case "succeeded":
			return j.Receipt, nil
		case "pending":
			return "", ErrApproval
		case "unknown", "manual", "claimed":
			return "", ErrUnknown
		case "ready":
			claim, err := e.Claim(ctx, j.Proposal.ID, time.Minute)
			if err != nil {
				return "", err
			}
			if err = e.Dispatch(ctx, claim, p); err != nil {
				return "", err
			}
			j, err = e.Job(j.Proposal.ID)
			if err != nil {
				return "", err
			}
			if j.Status != "succeeded" {
				return "", ErrConflict
			}
			return j.Receipt, nil
		default:
			return "", ErrConflict
		}
	}
}
