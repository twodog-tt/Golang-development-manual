package agentcommerce

import (
	"errors"
	"sync"
)

// Job models normative lifecycle rules plus a stricter local no-funding-after-
// expiry policy. It is an in-memory atomic model, not an EVM/ERC20 contract.
type Job struct {
	mu    sync.Mutex
	state JobState
	hook  HookFault
}
type JobState struct {
	Client, Provider, Evaluator, Status       string
	Budget, ExpiredAt, Escrow, Paid, Refunded int64
}

// HookFault only injects before/after reverts. It is not arbitrary external code.
type HookFault struct{ Before, After bool }

func NewJob(client, provider, evaluator string, expiredAt, now int64) (*Job, error) {
	if client == "" || evaluator == "" || expiredAt <= now {
		return nil, ErrDenied
	}
	return &Job{state: JobState{Client: client, Provider: provider, Evaluator: evaluator, ExpiredAt: expiredAt, Status: "open"}}, nil
}
func (j *Job) Snapshot() JobState     { j.mu.Lock(); defer j.mu.Unlock(); return j.state }
func (j *Job) InjectHook(h HookFault) { j.mu.Lock(); defer j.mu.Unlock(); j.hook = h }
func (j *Job) action(hookable bool, apply func(*JobState) error) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	s := j.state
	if hookable && j.hook.Before {
		return errors.New("hook before reverted")
	}
	if err := apply(&s); err != nil {
		return err
	}
	if hookable && j.hook.After {
		return errors.New("hook after reverted; all state rolled back")
	}
	j.state = s
	return nil
}
func (j *Job) SetProvider(caller, provider string) error {
	return j.action(true, func(s *JobState) error {
		if s.Status != "open" || caller != s.Client || s.Provider != "" || provider == "" {
			return ErrDenied
		}
		s.Provider = provider
		return nil
	})
}
func (j *Job) SetBudget(caller string, amount int64) error {
	return j.action(true, func(s *JobState) error {
		if s.Status != "open" || (caller != s.Client && caller != s.Provider) || amount < 0 || amount > 1000000 {
			return ErrDenied
		}
		s.Budget = amount
		return nil
	})
}
func (j *Job) Fund(caller string, expectedBudget, now int64) error {
	return j.action(true, func(s *JobState) error {
		if s.Status != "open" || caller != s.Client || s.Provider == "" || s.Budget <= 0 || s.Budget != expectedBudget || now >= s.ExpiredAt {
			return ErrDenied
		}
		s.Escrow = s.Budget
		s.Status = "funded"
		return nil
	})
}
func (j *Job) Submit(caller, deliverable string) error {
	return j.action(true, func(s *JobState) error {
		if s.Status != "funded" || caller != s.Provider || deliverable == "" {
			return ErrDenied
		}
		s.Status = "submitted"
		return nil
	})
}
func (j *Job) Complete(caller string) error {
	return j.action(true, func(s *JobState) error {
		if s.Status != "submitted" || caller != s.Evaluator {
			return ErrDenied
		}
		s.Paid = s.Escrow
		s.Escrow = 0
		s.Status = "completed"
		return nil
	})
}
func (j *Job) Reject(caller string) error {
	return j.action(true, func(s *JobState) error {
		if (s.Status == "open" && caller == s.Client) || ((s.Status == "funded" || s.Status == "submitted") && caller == s.Evaluator) {
			s.Refunded = s.Escrow
			s.Escrow = 0
			s.Status = "rejected"
			return nil
		}
		return ErrDenied
	})
}

// Expiry makes refund callable, not automatic. Complete remains possible until
// a terminal transition wins; caller of Refund never becomes its beneficiary.
func (j *Job) Refund(caller string, now int64) error {
	return j.action(false, func(s *JobState) error {
		if caller == "" || (s.Status != "funded" && s.Status != "submitted") || now < s.ExpiredAt {
			return ErrDenied
		}
		s.Refunded = s.Escrow
		s.Escrow = 0
		s.Status = "expired"
		return nil
	})
}
