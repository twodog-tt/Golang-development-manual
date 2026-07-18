package matchingengine

import (
	"testing"
)

func newLimit(seq uint64, id string, side Side, price, qty int64, tif TimeInForce, postOnly bool) Command {
	return Command{
		Sequence: seq,
		Type:     CommandNewOrder,
		NewOrder: &NewOrder{
			OrderID:       id,
			ClientOrderID: "client-" + id,
			AccountID:     "account-" + id,
			Side:          side,
			Price:         price,
			Quantity:      qty,
			TimeInForce:   tif,
			STP:           STPCancelTaker,
			PostOnly:      postOnly,
		},
	}
}

func TestPriceTimePriorityAndMakerPrice(t *testing.T) {
	engine := New()
	mustApply(t, engine, newLimit(1, "ask-1", Sell, 100, 5, GTC, false))
	mustApply(t, engine, newLimit(2, "ask-2", Sell, 100, 7, GTC, false))

	events := mustApply(t, engine, newLimit(3, "buy-1", Buy, 110, 8, IOC, false))
	trades := eventsOfType(events, EventTrade)
	if len(trades) != 2 {
		t.Fatalf("trades = %d, want 2: %+v", len(trades), events)
	}
	if trades[0].MakerOrderID != "ask-1" || trades[0].Quantity != 5 || trades[0].Price != 100 {
		t.Fatalf("first trade violates FIFO/maker price: %+v", trades[0])
	}
	if trades[1].MakerOrderID != "ask-2" || trades[1].Quantity != 3 || trades[1].Price != 100 {
		t.Fatalf("second trade violates FIFO: %+v", trades[1])
	}

	ask2, ok := engine.Order("ask-2")
	if !ok || ask2.Remaining != 4 || ask2.Status != StatusPartiallyFilled {
		t.Fatalf("ask-2 = %+v, ok=%v", ask2, ok)
	}
	buy, _ := engine.Order("buy-1")
	if buy.Status != StatusFilled || buy.Remaining != 0 {
		t.Fatalf("buy-1 = %+v", buy)
	}

	engine = New()
	mustApply(t, engine, newLimit(1, "bid-maker", Buy, 105, 2, GTC, false))
	events = mustApply(t, engine, newLimit(2, "sell-taker", Sell, 100, 1, IOC, false))
	trades = eventsOfType(events, EventTrade)
	if len(trades) != 1 || trades[0].Price != 105 {
		t.Fatalf("trade must execute at resting maker price: %+v", trades)
	}
}

func TestFOKPostOnlyAndIOCRemainder(t *testing.T) {
	engine := New()
	mustApply(t, engine, newLimit(1, "ask", Sell, 100, 3, GTC, false))

	events := mustApply(t, engine, newLimit(2, "fok", Buy, 100, 4, FOK, false))
	if got := eventsOfType(events, EventRejected); len(got) != 1 || got[0].Reason != "fok_not_fillable" {
		t.Fatalf("FOK events = %+v", events)
	}
	if asks := engine.Asks(); len(asks) != 1 || asks[0].Quantity != 3 {
		t.Fatalf("FOK must not mutate book: %+v", asks)
	}

	events = mustApply(t, engine, newLimit(3, "post", Buy, 100, 1, GTC, true))
	if got := eventsOfType(events, EventRejected); len(got) != 1 || got[0].Reason != "post_only_would_take" {
		t.Fatalf("post-only events = %+v", events)
	}

	events = mustApply(t, engine, newLimit(4, "ioc", Buy, 100, 5, IOC, false))
	trades := eventsOfType(events, EventTrade)
	cancels := eventsOfType(events, EventCanceled)
	if len(trades) != 1 || trades[0].Quantity != 3 {
		t.Fatalf("IOC trade = %+v", trades)
	}
	if len(cancels) != 1 || cancels[0].Remaining != 2 || cancels[0].Reason != "ioc_remainder" {
		t.Fatalf("IOC cancel = %+v", cancels)
	}
}

func TestCancelDuplicateAndSequence(t *testing.T) {
	engine := New()
	mustApply(t, engine, newLimit(1, "bid", Buy, 90, 5, GTC, false))

	duplicate := newLimit(2, "other-id", Buy, 80, 1, GTC, false)
	duplicate.NewOrder.ClientOrderID = "client-bid"
	events := mustApply(t, engine, duplicate)
	got := eventsOfType(events, EventDuplicate)
	if len(got) != 1 || got[0].RelatedOrderID != "bid" {
		t.Fatalf("duplicate events = %+v", events)
	}

	events = mustApply(t, engine, Command{
		Sequence:    3,
		Type:        CommandCancelOrder,
		CancelOrder: &CancelOrder{OrderID: "bid"},
	})
	if len(eventsOfType(events, EventCanceled)) != 1 || len(engine.Bids()) != 0 {
		t.Fatalf("cancel failed: events=%+v bids=%+v", events, engine.Bids())
	}

	if _, err := engine.Apply(newLimit(5, "gap", Buy, 1, 1, GTC, false)); err == nil {
		t.Fatal("expected sequence gap error")
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	engine := New()
	mustApply(t, engine, newLimit(1, "ask", Sell, 100, 10, GTC, false))
	mustApply(t, engine, newLimit(2, "buy", Buy, 100, 4, GTC, false))
	mustApply(t, engine, newLimit(3, "bid", Buy, 90, 8, GTC, false))

	before, err := engine.StateHash()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := FromSnapshot(engine.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	after, err := restored.StateHash()
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("state hash mismatch: before=%s after=%s", before, after)
	}

	events := mustApply(t, restored, newLimit(4, "sell", Sell, 90, 3, IOC, false))
	trades := eventsOfType(events, EventTrade)
	if len(trades) != 1 || trades[0].MakerOrderID != "bid" {
		t.Fatalf("restored FIFO/book state is wrong: %+v", events)
	}
}

func TestSelfTradePreventionModes(t *testing.T) {
	tests := []struct {
		name             string
		mode             STPMode
		wantMakerStatus  OrderStatus
		wantTakerStatus  OrderStatus
		wantBids         int
		wantAsks         int
		wantCancelEvents int
	}{
		{
			name:             "cancel maker",
			mode:             STPCancelMaker,
			wantMakerStatus:  StatusCanceled,
			wantTakerStatus:  StatusOpen,
			wantBids:         1,
			wantAsks:         0,
			wantCancelEvents: 1,
		},
		{
			name:             "cancel taker",
			mode:             STPCancelTaker,
			wantMakerStatus:  StatusOpen,
			wantTakerStatus:  StatusCanceled,
			wantBids:         0,
			wantAsks:         1,
			wantCancelEvents: 1,
		},
		{
			name:             "cancel both",
			mode:             STPCancelBoth,
			wantMakerStatus:  StatusCanceled,
			wantTakerStatus:  StatusCanceled,
			wantBids:         0,
			wantAsks:         0,
			wantCancelEvents: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := New()
			maker := newLimit(1, "maker", Sell, 100, 5, GTC, false)
			maker.NewOrder.AccountID = "same-account"
			mustApply(t, engine, maker)

			taker := newLimit(2, "taker", Buy, 100, 3, GTC, false)
			taker.NewOrder.AccountID = "same-account"
			taker.NewOrder.STP = tt.mode
			events := mustApply(t, engine, taker)

			if got := eventsOfType(events, EventTrade); len(got) != 0 {
				t.Fatalf("STP emitted trades: %+v", got)
			}
			if got := eventsOfType(events, EventSelfTradePrevented); len(got) != 1 || got[0].STP != tt.mode {
				t.Fatalf("STP events=%+v", events)
			}
			if got := eventsOfType(events, EventCanceled); len(got) != tt.wantCancelEvents {
				t.Fatalf("cancel events=%+v, want %d", got, tt.wantCancelEvents)
			}

			makerView, _ := engine.Order("maker")
			takerView, _ := engine.Order("taker")
			if makerView.Status != tt.wantMakerStatus || takerView.Status != tt.wantTakerStatus {
				t.Fatalf("maker=%+v taker=%+v", makerView, takerView)
			}
			if len(engine.Bids()) != tt.wantBids || len(engine.Asks()) != tt.wantAsks {
				t.Fatalf("bids=%+v asks=%+v", engine.Bids(), engine.Asks())
			}
		})
	}
}

func TestFOKPrecheckIncludesSTPPolicy(t *testing.T) {
	t.Run("cancel maker can skip self liquidity and fill external liquidity", func(t *testing.T) {
		engine := New()
		self := newLimit(1, "self", Sell, 100, 2, GTC, false)
		self.NewOrder.AccountID = "account-a"
		mustApply(t, engine, self)

		external := newLimit(2, "external", Sell, 100, 3, GTC, false)
		external.NewOrder.AccountID = "account-b"
		mustApply(t, engine, external)

		fok := newLimit(3, "fok", Buy, 100, 3, FOK, false)
		fok.NewOrder.AccountID = "account-a"
		fok.NewOrder.STP = STPCancelMaker
		events := mustApply(t, engine, fok)

		if got := eventsOfType(events, EventTrade); len(got) != 1 || got[0].MakerOrderID != "external" || got[0].Quantity != 3 {
			t.Fatalf("trades=%+v events=%+v", got, events)
		}
		selfView, _ := engine.Order("self")
		if selfView.Status != StatusCanceled {
			t.Fatalf("self maker=%+v, want canceled", selfView)
		}
	})

	t.Run("cancel taker rejects FOK without mutating maker", func(t *testing.T) {
		engine := New()
		self := newLimit(1, "self", Sell, 100, 3, GTC, false)
		self.NewOrder.AccountID = "account-a"
		mustApply(t, engine, self)

		fok := newLimit(2, "fok", Buy, 100, 3, FOK, false)
		fok.NewOrder.AccountID = "account-a"
		fok.NewOrder.STP = STPCancelTaker
		events := mustApply(t, engine, fok)
		rejections := eventsOfType(events, EventRejected)
		if len(rejections) != 1 || rejections[0].Reason != "fok_not_fillable" {
			t.Fatalf("events=%+v", events)
		}
		selfView, _ := engine.Order("self")
		if selfView.Status != StatusOpen || selfView.Remaining != 3 {
			t.Fatalf("FOK precheck mutated maker: %+v", selfView)
		}
	})
}

func mustApply(t *testing.T, engine *Engine, cmd Command) []Event {
	t.Helper()
	events, err := engine.Apply(cmd)
	if err != nil {
		t.Fatalf("Apply(%+v): %v", cmd, err)
	}
	return events
}

func eventsOfType(events []Event, typ EventType) []Event {
	var out []Event
	for _, event := range events {
		if event.Type == typ {
			out = append(out, event)
		}
	}
	return out
}
