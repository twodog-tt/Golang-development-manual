package paymentstate

import (
	"errors"
	"testing"
)

func TestPaymentHappyPathAndDuplicateEvent(t *testing.T) {
	payment, err := New("pay_1")
	if err != nil {
		t.Fatal(err)
	}

	events := []Event{
		{ID: "evt_observed", Type: EventFundsObserved},
		{ID: "evt_confirmed", Type: EventFundsConfirmed},
		{ID: "evt_settled", Type: EventSettlement},
	}
	for _, event := range events {
		changed, err := payment.Apply(event)
		if err != nil {
			t.Fatal(err)
		}
		if !changed {
			t.Fatalf("event %s did not change state", event.ID)
		}
	}
	if payment.State != StateSettled || payment.Version != 3 {
		t.Fatalf("payment = %+v", payment)
	}

	changed, err := payment.Apply(events[2])
	if err != nil {
		t.Fatal(err)
	}
	if changed || payment.Version != 3 {
		t.Fatal("duplicate event changed aggregate")
	}
}

func TestObservedChainOrphanReturnsToAwaiting(t *testing.T) {
	payment, _ := New("pay_1")
	_, _ = payment.Apply(Event{ID: "evt_observed", Type: EventFundsObserved})

	changed, err := payment.Apply(Event{ID: "evt_orphan", Type: EventChainOrphaned})
	if err != nil {
		t.Fatal(err)
	}
	if !changed || payment.State != StateAwaitingFunds {
		t.Fatalf("payment = %+v", payment)
	}
}

func TestDuplicateIDBindsEventTypeAndReference(t *testing.T) {
	p, _ := New("pay_conflict")
	original := Event{ID: "evt_1", Type: EventFundsObserved, Reference: "tx_A"}
	if _, err := p.Apply(original); err != nil {
		t.Fatal(err)
	}
	for _, changed := range []Event{
		{ID: original.ID, Type: EventReversal, Reference: original.Reference},
		{ID: original.ID, Type: original.Type, Reference: "tx_B"},
	} {
		if applied, err := p.Apply(changed); applied || !errors.Is(err, ErrEventConflict) {
			t.Fatalf("conflicting event accepted: applied=%v err=%v", applied, err)
		}
		if p.State != StateObserved || p.Version != 1 {
			t.Fatal("conflict changed aggregate")
		}
	}
	if applied, err := p.Apply(original); applied || err != nil {
		t.Fatalf("exact retry rejected: applied=%v err=%v", applied, err)
	}
	if applied, err := p.Apply(Event{ID: "evt_2", Type: EventFundsConfirmed, Reference: "tx_A"}); !applied || err != nil {
		t.Fatalf("conflict poisoned later events: applied=%v err=%v", applied, err)
	}
}

func TestSettledPaymentRequiresReversalEvent(t *testing.T) {
	payment, _ := New("pay_1")
	for _, event := range []Event{
		{ID: "evt_1", Type: EventFundsObserved},
		{ID: "evt_2", Type: EventFundsConfirmed},
		{ID: "evt_3", Type: EventSettlement},
	} {
		_, _ = payment.Apply(event)
	}

	_, err := payment.Apply(Event{ID: "evt_4", Type: EventChainOrphaned})
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("error = %v, want invalid transition", err)
	}

	changed, err := payment.Apply(Event{ID: "evt_5", Type: EventReversal})
	if err != nil {
		t.Fatal(err)
	}
	if !changed || payment.State != StateReversed {
		t.Fatalf("payment = %+v", payment)
	}
}
