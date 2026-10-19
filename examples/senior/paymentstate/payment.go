package paymentstate

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidPayment    = errors.New("paymentstate: payment and event IDs are required")
	ErrInvalidTransition = errors.New("paymentstate: invalid transition")
	ErrEventConflict     = errors.New("paymentstate: event ID reused with different content")
)

type State string

const (
	StateAwaitingFunds State = "awaiting_funds"
	StateObserved      State = "observed"
	StateConfirmed     State = "confirmed"
	StateSettled       State = "settled"
	StateFailed        State = "failed"
	StateReversed      State = "reversed"
)

type EventType string

const (
	EventFundsObserved  EventType = "funds_observed"
	EventFundsConfirmed EventType = "funds_confirmed"
	EventSettlement     EventType = "settlement_posted"
	EventExpired        EventType = "expired"
	EventChainOrphaned  EventType = "chain_orphaned"
	EventReversal       EventType = "reversal_posted"
)

type Event struct {
	ID        string
	Type      EventType
	Reference string
}

type Payment struct {
	ID      string
	State   State
	Version uint64
	applied map[string]Event
}

func New(id string) (*Payment, error) {
	if id == "" {
		return nil, ErrInvalidPayment
	}
	return &Payment{
		ID:      id,
		State:   StateAwaitingFunds,
		applied: make(map[string]Event),
	}, nil
}

type transitionKey struct {
	state State
	event EventType
}

var transitions = map[transitionKey]State{
	{StateAwaitingFunds, EventFundsObserved}: StateObserved,
	{StateAwaitingFunds, EventExpired}:       StateFailed,
	{StateObserved, EventFundsConfirmed}:     StateConfirmed,
	{StateObserved, EventChainOrphaned}:      StateAwaitingFunds,
	{StateConfirmed, EventSettlement}:        StateSettled,
	{StateConfirmed, EventChainOrphaned}:     StateReversed,
	{StateConfirmed, EventReversal}:          StateReversed,
	{StateSettled, EventReversal}:            StateReversed,
}

// Apply accepts an exact event retry, but rejects conflicting content under the
// same ID. A real service must persist this binding and the aggregate version
// atomically, protected by database UNIQUE/CAS.
func (p *Payment) Apply(event Event) (bool, error) {
	if p == nil || p.ID == "" || event.ID == "" {
		return false, ErrInvalidPayment
	}
	if p.applied == nil {
		p.applied = make(map[string]Event)
	}
	if previous, ok := p.applied[event.ID]; ok {
		if previous != event {
			return false, ErrEventConflict
		}
		return false, nil
	}

	next, ok := transitions[transitionKey{state: p.State, event: event.Type}]
	if !ok {
		return false, fmt.Errorf(
			"%w: state=%s event=%s",
			ErrInvalidTransition,
			p.State,
			event.Type,
		)
	}

	p.State = next
	p.Version++
	p.applied[event.ID] = event
	return true, nil
}
