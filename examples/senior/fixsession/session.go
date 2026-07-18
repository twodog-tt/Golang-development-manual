// Package fixsession implements an educational, single-writer FIX session
// sequencing and recovery state machine.
//
// It does not implement tag-value encoding, authentication, TLS, comp IDs, or
// the complete FIX state matrix. Its purpose is to make MsgSeqNum, resend,
// retransmission, gap-fill, and persisted sequence counters executable.
package fixsession

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

var (
	ErrSequenceTooLow  = errors.New("incoming MsgSeqNum is lower than NextNumIn")
	ErrSequenceReset   = errors.New("SequenceReset cannot decrease NextNumIn")
	ErrConflictingCopy = errors.New("conflicting messages use the same MsgSeqNum")
)

type MsgType string

const (
	MsgHeartbeat     MsgType = "0"
	MsgTestRequest   MsgType = "1"
	MsgResendRequest MsgType = "2"
	MsgReject        MsgType = "3"
	MsgSequenceReset MsgType = "4"
	MsgLogout        MsgType = "5"
	MsgLogon         MsgType = "A"

	// Application messages use their normal FIX MsgType. These two constants
	// are sufficient for the tests and examples.
	MsgNewOrderSingle     MsgType = "D"
	MsgCancelReplaceOrder MsgType = "G"
)

type Message struct {
	SeqNum          uint64    `json:"seq_num"`
	Type            MsgType   `json:"type"`
	SendingTime     time.Time `json:"sending_time"`
	PossDup         bool      `json:"poss_dup,omitempty"`
	OrigSendingTime time.Time `json:"orig_sending_time,omitempty"`
	GapFill         bool      `json:"gap_fill,omitempty"`
	NewSeqNo        uint64    `json:"new_seq_no,omitempty"`
	Body            string    `json:"body,omitempty"`
}

type State struct {
	NextNumIn  uint64 `json:"next_num_in"`
	NextNumOut uint64 `json:"next_num_out"`
}

type ActionType string

const (
	ActionDeliverApplication ActionType = "DELIVER_APPLICATION"
	ActionProcessSession     ActionType = "PROCESS_SESSION"
	ActionSendResendRequest  ActionType = "SEND_RESEND_REQUEST"
	ActionIgnoreDuplicate    ActionType = "IGNORE_DUPLICATE"
	ActionGapFilled          ActionType = "GAP_FILLED"
	ActionWarning            ActionType = "WARNING"
	ActionReject             ActionType = "REJECT"
)

type Action struct {
	Type       ActionType `json:"type"`
	Message    Message    `json:"message,omitempty"`
	BeginSeqNo uint64     `json:"begin_seq_no,omitempty"`
	EndSeqNo   uint64     `json:"end_seq_no,omitempty"`
	Reason     string     `json:"reason,omitempty"`
}

type Session struct {
	nextNumIn         uint64
	nextNumOut        uint64
	outbound          map[uint64]Message
	pending           map[uint64]Message
	resendOutstanding bool
}

func New() *Session {
	return &Session{
		nextNumIn:  1,
		nextNumOut: 1,
		outbound:   make(map[uint64]Message),
		pending:    make(map[uint64]Message),
	}
}

// Restore recreates sequence counters and the durable outbound retransmission
// store. A production FIX engine persists both across TCP reconnects.
func Restore(state State, outbound []Message) (*Session, error) {
	if state.NextNumIn == 0 || state.NextNumOut == 0 {
		return nil, errors.New("NextNumIn and NextNumOut must be positive")
	}
	session := &Session{
		nextNumIn:  state.NextNumIn,
		nextNumOut: state.NextNumOut,
		outbound:   make(map[uint64]Message, len(outbound)),
		pending:    make(map[uint64]Message),
	}
	for _, message := range outbound {
		if message.SeqNum == 0 || message.SeqNum >= state.NextNumOut {
			return nil, fmt.Errorf("outbound MsgSeqNum %d is outside persisted range", message.SeqNum)
		}
		if message.SendingTime.IsZero() {
			return nil, fmt.Errorf("outbound MsgSeqNum %d has no SendingTime", message.SeqNum)
		}
		if _, exists := session.outbound[message.SeqNum]; exists {
			return nil, fmt.Errorf("duplicate outbound MsgSeqNum %d", message.SeqNum)
		}
		session.outbound[message.SeqNum] = message
	}
	return session, nil
}

func (s *Session) State() State {
	return State{NextNumIn: s.nextNumIn, NextNumOut: s.nextNumOut}
}

func (s *Session) OutboundStore() []Message {
	out := make([]Message, 0, len(s.outbound))
	for _, message := range s.outbound {
		out = append(out, message)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].SeqNum < out[j].SeqNum
	})
	return out
}

// Send assigns NextNumOut and stores the original message for later session
// retransmission. Sending a new application-level retry is a separate concern.
func (s *Session) Send(typ MsgType, body string, now time.Time) (Message, error) {
	if typ == "" {
		return Message{}, errors.New("MsgType is required")
	}
	if now.IsZero() {
		return Message{}, errors.New("SendingTime is required")
	}
	message := Message{
		SeqNum:      s.nextNumOut,
		Type:        typ,
		SendingTime: now,
		Body:        body,
	}
	s.outbound[message.SeqNum] = message
	s.nextNumOut++
	return message, nil
}

// Recover builds a response to ResendRequest. Replayable application messages
// retain their original MsgSeqNum and are marked PossDup. Session messages are
// skipped with SequenceReset-GapFill.
func (s *Session) Recover(beginSeqNo, endSeqNo uint64, now time.Time) ([]Message, error) {
	if beginSeqNo == 0 {
		return nil, errors.New("BeginSeqNo must be positive")
	}
	if endSeqNo != 0 && endSeqNo < beginSeqNo {
		return nil, errors.New("EndSeqNo must be zero or >= BeginSeqNo")
	}
	if now.IsZero() {
		return nil, errors.New("SendingTime is required")
	}

	lastSent := s.nextNumOut - 1
	if beginSeqNo > lastSent {
		return nil, fmt.Errorf("BeginSeqNo %d is above last sent %d", beginSeqNo, lastSent)
	}
	last := endSeqNo
	if last == 0 || last > lastSent {
		last = lastSent
	}

	var out []Message
	var gapStart uint64
	var gapOrigTime time.Time
	flushGap := func(newSeqNo uint64) {
		if gapStart == 0 {
			return
		}
		out = append(out, Message{
			SeqNum:          gapStart,
			Type:            MsgSequenceReset,
			SendingTime:     now,
			PossDup:         true,
			OrigSendingTime: gapOrigTime,
			GapFill:         true,
			NewSeqNo:        newSeqNo,
		})
		gapStart = 0
		gapOrigTime = time.Time{}
	}

	for seq := beginSeqNo; seq <= last; seq++ {
		original, exists := s.outbound[seq]
		if !exists {
			return nil, fmt.Errorf("outbound store is missing MsgSeqNum %d", seq)
		}
		if !isReplayableApplication(original.Type) {
			if gapStart == 0 {
				gapStart = seq
				gapOrigTime = original.SendingTime
			}
			continue
		}
		flushGap(seq)
		retransmission := original
		retransmission.SendingTime = now
		retransmission.PossDup = true
		retransmission.OrigSendingTime = original.SendingTime
		out = append(out, retransmission)
	}
	flushGap(last + 1)
	return out, nil
}

// Receive enforces ordered processing. Higher messages are queued, and the
// caller is instructed to send ResendRequest. They are not delivered before
// the missing range is recovered.
func (s *Session) Receive(message Message) ([]Action, error) {
	if err := validateInbound(message); err != nil {
		return nil, err
	}

	// SequenceReset-Reset is the exceptional recovery form: its MsgSeqNum is
	// ignored, but it must never move NextNumIn backwards.
	if message.Type == MsgSequenceReset && !message.GapFill {
		if message.NewSeqNo < s.nextNumIn {
			return []Action{{
				Type:    ActionReject,
				Message: message,
				Reason:  "NewSeqNo is lower than NextNumIn",
			}}, fmt.Errorf("%w: next=%d new=%d", ErrSequenceReset, s.nextNumIn, message.NewSeqNo)
		}
		if message.NewSeqNo == s.nextNumIn {
			return []Action{{
				Type:    ActionWarning,
				Message: message,
				Reason:  "SequenceReset did not advance NextNumIn",
			}}, nil
		}
		s.nextNumIn = message.NewSeqNo
		s.dropPendingBelowNext()
		actions, err := s.drainPending()
		return append([]Action{{
			Type:    ActionGapFilled,
			Message: message,
			Reason:  "SequenceReset-Reset advanced NextNumIn",
		}}, actions...), err
	}

	switch {
	case message.SeqNum < s.nextNumIn:
		if message.PossDup {
			return []Action{{
				Type:    ActionIgnoreDuplicate,
				Message: message,
				Reason:  "MsgSeqNum was already processed",
			}}, nil
		}
		return []Action{{
			Type:    ActionReject,
			Message: message,
			Reason:  "MsgSeqNum is too low without PossDupFlag",
		}}, fmt.Errorf("%w: got=%d next=%d", ErrSequenceTooLow, message.SeqNum, s.nextNumIn)

	case message.SeqNum > s.nextNumIn:
		if existing, exists := s.pending[message.SeqNum]; exists {
			if !sameMessage(existing, message) {
				return nil, fmt.Errorf("%w: seq=%d", ErrConflictingCopy, message.SeqNum)
			}
			return []Action{{
				Type:    ActionIgnoreDuplicate,
				Message: message,
				Reason:  "same out-of-order message already queued",
			}}, nil
		}
		s.pending[message.SeqNum] = message
		if s.resendOutstanding {
			return nil, nil
		}
		s.resendOutstanding = true
		return []Action{{
			Type:       ActionSendResendRequest,
			BeginSeqNo: s.nextNumIn,
			EndSeqNo:   0,
			Reason:     "incoming MsgSeqNum is above NextNumIn",
		}}, nil

	default:
		actions, err := s.processExpected(message)
		if err != nil {
			return actions, err
		}
		more, err := s.drainPending()
		actions = append(actions, more...)
		if len(s.pending) == 0 {
			s.resendOutstanding = false
		}
		return actions, err
	}
}

func (s *Session) processExpected(message Message) ([]Action, error) {
	if message.SeqNum != s.nextNumIn {
		return nil, fmt.Errorf("internal sequence error: got=%d next=%d", message.SeqNum, s.nextNumIn)
	}
	if message.Type == MsgSequenceReset && message.GapFill {
		if message.NewSeqNo <= s.nextNumIn {
			return []Action{{
				Type:    ActionReject,
				Message: message,
				Reason:  "GapFill NewSeqNo must advance NextNumIn",
			}}, fmt.Errorf("%w: next=%d new=%d", ErrSequenceReset, s.nextNumIn, message.NewSeqNo)
		}
		s.nextNumIn = message.NewSeqNo
		s.dropPendingBelowNext()
		return []Action{{
			Type:    ActionGapFilled,
			Message: message,
			Reason:  "SequenceReset-GapFill skipped non-replayed messages",
		}}, nil
	}

	s.nextNumIn++
	actionType := ActionDeliverApplication
	if isSessionMessage(message.Type) {
		actionType = ActionProcessSession
	}
	return []Action{{Type: actionType, Message: message}}, nil
}

func (s *Session) drainPending() ([]Action, error) {
	var out []Action
	for {
		message, exists := s.pending[s.nextNumIn]
		if !exists {
			break
		}
		delete(s.pending, s.nextNumIn)
		actions, err := s.processExpected(message)
		out = append(out, actions...)
		if err != nil {
			return out, err
		}
	}
	if len(s.pending) == 0 {
		s.resendOutstanding = false
	}
	return out, nil
}

func (s *Session) dropPendingBelowNext() {
	for seq := range s.pending {
		if seq < s.nextNumIn {
			delete(s.pending, seq)
		}
	}
}

func validateInbound(message Message) error {
	if message.SeqNum == 0 {
		return errors.New("MsgSeqNum must be positive")
	}
	if message.Type == "" {
		return errors.New("MsgType is required")
	}
	if message.SendingTime.IsZero() {
		return errors.New("SendingTime is required")
	}
	if message.PossDup {
		if message.OrigSendingTime.IsZero() {
			return errors.New("PossDupFlag requires OrigSendingTime")
		}
		if message.OrigSendingTime.After(message.SendingTime) {
			return errors.New("OrigSendingTime cannot be after SendingTime")
		}
	}
	if message.Type == MsgSequenceReset && message.NewSeqNo == 0 {
		return errors.New("SequenceReset requires positive NewSeqNo")
	}
	if message.Type != MsgSequenceReset && (message.GapFill || message.NewSeqNo != 0) {
		return errors.New("GapFill and NewSeqNo are only valid on SequenceReset")
	}
	return nil
}

// sameMessage compares protocol fields rather than Go's struct representation.
// time.Time's == operator also compares location and monotonic-clock metadata,
// neither of which changes the FIX timestamp value on the wire.
func sameMessage(a, b Message) bool {
	return a.SeqNum == b.SeqNum &&
		a.Type == b.Type &&
		a.SendingTime.Equal(b.SendingTime) &&
		a.PossDup == b.PossDup &&
		a.OrigSendingTime.Equal(b.OrigSendingTime) &&
		a.GapFill == b.GapFill &&
		a.NewSeqNo == b.NewSeqNo &&
		a.Body == b.Body
}

func isSessionMessage(typ MsgType) bool {
	switch typ {
	case MsgHeartbeat, MsgTestRequest, MsgResendRequest, MsgReject,
		MsgSequenceReset, MsgLogout, MsgLogon:
		return true
	default:
		return false
	}
}

func isReplayableApplication(typ MsgType) bool {
	// FIX permits a small number of session messages such as Reject to be
	// retransmitted, but this teaching implementation gap-fills every session
	// message and retransmits only application messages.
	return !isSessionMessage(typ)
}
