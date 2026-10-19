package mediaio

import "testing"

func TestGeneratedAndPlaybackCompletionAreDifferent(t *testing.T) {
	v, e := NewVoice("session1")
	if e != nil {
		t.Fatal(e)
	}
	if e = v.Start("r1", "item1"); e != nil {
		t.Fatal(e)
	}
	if e = v.Append("session1", "r1", 1, 0, make([]byte, 480)); e != nil {
		t.Fatal(e)
	}
	if e = v.Generated("session1", "r1", 1); e != nil {
		t.Fatal(e)
	}
	if e = v.Start("r2", "item2"); e == nil {
		t.Fatal("started while old playback pending")
	}
	if e = v.Played("session1", "r1", 1, 480); e != nil {
		t.Fatal(e)
	}
	if e = v.Start("r2", "item2"); e != nil {
		t.Fatal(e)
	}
}
func TestInterruptUsesPlayedTimeAndFencesLateAudio(t *testing.T) {
	v, _ := NewVoice("s")
	if e := v.Start("r", "item"); e != nil {
		t.Fatal(e)
	}
	if e := v.Append("s", "r", 1, 0, make([]byte, 48000)); e != nil {
		t.Fatal(e)
	}
	if e := v.Played("s", "r", 1, 24000); e != nil {
		t.Fatal(e)
	}
	cut, e := v.Interrupt()
	if e != nil || cut.AudioEndMS != 500 || cut.ItemID != "item" || v.Queued() != 0 {
		t.Fatal(cut, e)
	}
	if e = v.Append("s", "r", 1, 1, make([]byte, 48)); e == nil {
		t.Fatal("late audio played")
	}
	if e = v.Start("r2", "item2"); e == nil {
		t.Fatal("lost truncate ACK ignored")
	}
	wrong := cut
	wrong.ResponseID = "other"
	if e = v.Acknowledge(wrong); e == nil {
		t.Fatal("wrong ACK")
	}
	if e = v.Acknowledge(cut); e != nil {
		t.Fatal(e)
	}
	if e = v.Start("r", "item"); e == nil {
		t.Fatal("generation ID reused")
	}
	if e = v.Start("r2", "item2"); e != nil {
		t.Fatal(e)
	}
}
func TestDisconnectCannotAssumeResumeOrReplay(t *testing.T) {
	v, _ := NewVoice("old")
	_ = v.Start("r1", "i1")
	_ = v.Append("old", "r1", 1, 0, make([]byte, 480))
	cut, e := v.Interrupt()
	if e != nil {
		t.Fatal(e)
	}
	v.Disconnect()
	if e = v.Acknowledge(cut); e == nil {
		t.Fatal("disconnected ACK accepted")
	}
	if e = v.Reconnect("old"); e == nil {
		t.Fatal("assumed same session resume")
	}
	if e = v.Start("r2", "i2"); e == nil {
		t.Fatal("no handshake")
	}
	if e = v.Reconnect("new"); e != nil {
		t.Fatal(e)
	}
	if v.Epoch() != 2 || v.Queued() != 0 {
		t.Fatal("did not fence old connection")
	}
	_ = v.Start("r1", "i1")
	if e = v.Append("old", "r1", 1, 0, make([]byte, 48)); e == nil {
		t.Fatal("old connection replay")
	}
	if e = v.Acknowledge(cut); e == nil {
		t.Fatal("old truncate ACK")
	}
}
func TestVoiceLimitsSequenceAndZeroSession(t *testing.T) {
	var zero Voice
	if e := zero.Start("r", "i"); e == nil {
		t.Fatal("zero state resumed")
	}
	v, _ := NewVoice("s")
	_ = v.Start("r", "i")
	for _, n := range []int{1, 48002} {
		if e := v.Append("s", "r", 1, 0, make([]byte, n)); e == nil {
			t.Fatal("bad frame")
		}
	}
	if e := v.Append("s", "r", 1, 1, make([]byte, 48)); e == nil {
		t.Fatal("sequence gap")
	}
	for i := 0; i < 2; i++ {
		if e := v.Append("s", "r", 1, i, make([]byte, 48000)); e != nil {
			t.Fatal(e)
		}
	}
	if e := v.Append("s", "r", 1, 2, make([]byte, 48)); e == nil {
		t.Fatal("unbounded playback queue")
	}
	if e := v.Played("s", "r", 1, 96002); e == nil {
		t.Fatal("playback overcount")
	}
}

func TestStalePlaybackAndHistoricalSessionReuseRejected(t *testing.T) {
	v, _ := NewVoice("a")
	_ = v.Start("old", "i1")
	v.Disconnect()
	if err := v.Reconnect("b"); err != nil {
		t.Fatal(err)
	}
	_ = v.Start("new", "i2")
	_ = v.Append("b", "new", 2, 0, make([]byte, 480))
	for _, ack := range []struct {
		session, response string
		epoch             uint64
	}{
		{"a", "old", 1}, {"b", "old", 2}, {"b", "new", 1},
	} {
		if err := v.Played(ack.session, ack.response, ack.epoch, 480); err == nil || v.Queued() != 480 {
			t.Fatal("stale playback changed queue", err)
		}
	}
	if err := v.Played("b", "new", 2, 480); err != nil {
		t.Fatal(err)
	}
	cut, err := v.Interrupt()
	if err != nil || cut.AudioEndMS != 10 {
		t.Fatal(cut, err)
	}
	if err = v.Acknowledge(cut); err != nil {
		t.Fatal(err)
	}
	if err = v.Start("next", "i3"); err != nil {
		t.Fatal(err)
	}
	_ = v.Append("b", "next", 2, 0, make([]byte, 480))
	if err = v.Played("b", "new", 2, 480); err == nil || v.Queued() != 480 {
		t.Fatal("old response callback accepted")
	}
	v.Disconnect()
	if err = v.Reconnect("a"); err == nil {
		t.Fatal("historical session reused")
	}
}
