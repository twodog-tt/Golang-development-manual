package fixsession

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestRecoverRetransmitsApplicationsAndGapFillsSessionMessages(t *testing.T) {
	session := New()
	base := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	mustSend(t, session, MsgLogon, "", base)
	app1 := mustSend(t, session, MsgNewOrderSingle, "order-1", base.Add(time.Second))
	mustSend(t, session, MsgHeartbeat, "", base.Add(2*time.Second))
	app2 := mustSend(t, session, MsgCancelReplaceOrder, "replace-1", base.Add(3*time.Second))

	now := base.Add(time.Minute)
	got, err := session.Recover(1, 0, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("messages=%d, want 4: %+v", len(got), got)
	}
	assertGapFill(t, got[0], 1, 2)
	assertRetransmission(t, got[1], app1, now)
	assertGapFill(t, got[2], 3, 4)
	assertRetransmission(t, got[3], app2, now)

	if state := session.State(); state.NextNumOut != 5 {
		t.Fatalf("recovery must not consume new sequence numbers: %+v", state)
	}
}

func TestReceiveQueuesHigherSequenceAndDrainsInOrder(t *testing.T) {
	session := New()
	base := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)

	actions, err := session.Receive(Message{
		SeqNum:      2,
		Type:        MsgNewOrderSingle,
		SendingTime: base.Add(time.Second),
		Body:        "order-2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 ||
		actions[0].Type != ActionSendResendRequest ||
		actions[0].BeginSeqNo != 1 ||
		actions[0].EndSeqNo != 0 {
		t.Fatalf("unexpected gap actions: %+v", actions)
	}

	actions, err = session.Receive(Message{
		SeqNum:      1,
		Type:        MsgLogon,
		SendingTime: base,
	})
	if err != nil {
		t.Fatal(err)
	}
	gotTypes := actionTypes(actions)
	wantTypes := []ActionType{ActionProcessSession, ActionDeliverApplication}
	if !reflect.DeepEqual(gotTypes, wantTypes) {
		t.Fatalf("actions=%+v, want types=%+v", actions, wantTypes)
	}
	if state := session.State(); state.NextNumIn != 3 {
		t.Fatalf("state=%+v, want NextNumIn=3", state)
	}
}

func TestEquivalentOutOfOrderCopyUsesProtocolTimeEquality(t *testing.T) {
	session := New()
	utc := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	original := Message{
		SeqNum:      2,
		Type:        MsgNewOrderSingle,
		SendingTime: utc,
		Body:        "order-2",
	}
	if _, err := session.Receive(original); err != nil {
		t.Fatal(err)
	}

	equivalent := original
	equivalent.SendingTime = utc.In(time.FixedZone("UTC+8", 8*60*60))
	actions, err := session.Receive(equivalent)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].Type != ActionIgnoreDuplicate {
		t.Fatalf("actions=%+v, want equivalent queued copy to be ignored", actions)
	}

	conflicting := equivalent
	conflicting.Body = "different-order"
	if _, err := session.Receive(conflicting); !errors.Is(err, ErrConflictingCopy) {
		t.Fatalf("err=%v, want ErrConflictingCopy", err)
	}
}

func TestGapFillAdvancesAndReleasesQueuedMessage(t *testing.T) {
	session := New()
	base := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)

	if _, err := session.Receive(Message{
		SeqNum:      4,
		Type:        MsgNewOrderSingle,
		SendingTime: base.Add(4 * time.Second),
	}); err != nil {
		t.Fatal(err)
	}

	actions, err := session.Receive(Message{
		SeqNum:          1,
		Type:            MsgSequenceReset,
		SendingTime:     base.Add(time.Minute),
		PossDup:         true,
		OrigSendingTime: base,
		GapFill:         true,
		NewSeqNo:        4,
	})
	if err != nil {
		t.Fatal(err)
	}
	gotTypes := actionTypes(actions)
	wantTypes := []ActionType{ActionGapFilled, ActionDeliverApplication}
	if !reflect.DeepEqual(gotTypes, wantTypes) {
		t.Fatalf("actions=%+v, want types=%+v", actions, wantTypes)
	}
	if state := session.State(); state.NextNumIn != 5 {
		t.Fatalf("state=%+v, want NextNumIn=5", state)
	}
}

func TestLowSequenceRequiresPossDupAndOrigSendingTime(t *testing.T) {
	session := New()
	base := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	if _, err := session.Receive(Message{
		SeqNum:      1,
		Type:        MsgLogon,
		SendingTime: base,
	}); err != nil {
		t.Fatal(err)
	}

	_, err := session.Receive(Message{
		SeqNum:      1,
		Type:        MsgLogon,
		SendingTime: base.Add(time.Second),
	})
	if !errors.Is(err, ErrSequenceTooLow) {
		t.Fatalf("err=%v, want ErrSequenceTooLow", err)
	}

	actions, err := session.Receive(Message{
		SeqNum:          1,
		Type:            MsgLogon,
		SendingTime:     base.Add(2 * time.Second),
		PossDup:         true,
		OrigSendingTime: base,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].Type != ActionIgnoreDuplicate {
		t.Fatalf("actions=%+v", actions)
	}
}

func TestSequenceResetCannotMoveBackwards(t *testing.T) {
	session, err := Restore(State{NextNumIn: 10, NextNumOut: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	actions, err := session.Receive(Message{
		SeqNum:      99,
		Type:        MsgSequenceReset,
		SendingTime: base,
		NewSeqNo:    9,
	})
	if !errors.Is(err, ErrSequenceReset) {
		t.Fatalf("err=%v, want ErrSequenceReset", err)
	}
	if len(actions) != 1 || actions[0].Type != ActionReject {
		t.Fatalf("actions=%+v", actions)
	}
	if session.State().NextNumIn != 10 {
		t.Fatalf("reset moved sequence backwards: %+v", session.State())
	}
}

func TestRestorePersistsCountersAndOutboundStore(t *testing.T) {
	session := New()
	base := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	mustSend(t, session, MsgLogon, "", base)
	mustSend(t, session, MsgNewOrderSingle, "order", base.Add(time.Second))

	restored, err := Restore(session.State(), session.OutboundStore())
	if err != nil {
		t.Fatal(err)
	}
	next := mustSend(t, restored, MsgHeartbeat, "", base.Add(2*time.Second))
	if next.SeqNum != 3 {
		t.Fatalf("SeqNum=%d, want 3", next.SeqNum)
	}
}

func TestRejectsSequenceResetFieldsOnApplicationMessage(t *testing.T) {
	session := New()
	_, err := session.Receive(Message{
		SeqNum:      1,
		Type:        MsgNewOrderSingle,
		SendingTime: time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC),
		GapFill:     true,
		NewSeqNo:    2,
	})
	if err == nil {
		t.Fatal("expected invalid SequenceReset fields to be rejected")
	}
}

func mustSend(t *testing.T, session *Session, typ MsgType, body string, at time.Time) Message {
	t.Helper()
	message, err := session.Send(typ, body, at)
	if err != nil {
		t.Fatal(err)
	}
	return message
}

func assertGapFill(t *testing.T, message Message, seq, newSeq uint64) {
	t.Helper()
	if message.Type != MsgSequenceReset ||
		!message.GapFill ||
		!message.PossDup ||
		message.SeqNum != seq ||
		message.NewSeqNo != newSeq ||
		message.OrigSendingTime.IsZero() {
		t.Fatalf("invalid gap fill: %+v", message)
	}
}

func assertRetransmission(t *testing.T, got, original Message, now time.Time) {
	t.Helper()
	if got.SeqNum != original.SeqNum ||
		got.Type != original.Type ||
		got.Body != original.Body ||
		!got.PossDup ||
		got.OrigSendingTime != original.SendingTime ||
		got.SendingTime != now {
		t.Fatalf("invalid retransmission:\n got=%+v\nwant original=%+v", got, original)
	}
}

func actionTypes(actions []Action) []ActionType {
	out := make([]ActionType, len(actions))
	for i, action := range actions {
		out[i] = action.Type
	}
	return out
}
