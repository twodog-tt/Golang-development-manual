// Package matchingengine implements a deterministic, single-writer limit order book.
//
// It intentionally uses simple sorted slices instead of a production tree/skip-list:
// the interview focus is ordering, exact arithmetic, replay, and invariants.
package matchingengine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

type Side string

const (
	Buy  Side = "BUY"
	Sell Side = "SELL"
)

type TimeInForce string

const (
	GTC TimeInForce = "GTC"
	IOC TimeInForce = "IOC"
	FOK TimeInForce = "FOK"
)

type STPMode string

const (
	STPCancelMaker STPMode = "CANCEL_MAKER"
	STPCancelTaker STPMode = "CANCEL_TAKER"
	STPCancelBoth  STPMode = "CANCEL_BOTH"
)

type CommandType string

const (
	CommandNewOrder    CommandType = "NEW_ORDER"
	CommandCancelOrder CommandType = "CANCEL_ORDER"
)

type Command struct {
	Sequence    uint64       `json:"sequence"`
	Type        CommandType  `json:"type"`
	NewOrder    *NewOrder    `json:"new_order,omitempty"`
	CancelOrder *CancelOrder `json:"cancel_order,omitempty"`
}

type NewOrder struct {
	OrderID       string      `json:"order_id"`
	ClientOrderID string      `json:"client_order_id"`
	AccountID     string      `json:"account_id"`
	Side          Side        `json:"side"`
	Price         int64       `json:"price"`
	Quantity      int64       `json:"quantity"`
	TimeInForce   TimeInForce `json:"time_in_force"`
	STP           STPMode     `json:"stp"`
	PostOnly      bool        `json:"post_only,omitempty"`
}

type CancelOrder struct {
	OrderID string `json:"order_id"`
}

type OrderStatus string

const (
	StatusOpen            OrderStatus = "OPEN"
	StatusPartiallyFilled OrderStatus = "PARTIALLY_FILLED"
	StatusFilled          OrderStatus = "FILLED"
	StatusCanceled        OrderStatus = "CANCELED"
	StatusRejected        OrderStatus = "REJECTED"
)

type EventType string

const (
	EventAccepted           EventType = "ORDER_ACCEPTED"
	EventRejected           EventType = "ORDER_REJECTED"
	EventDuplicate          EventType = "ORDER_DUPLICATE"
	EventTrade              EventType = "TRADE"
	EventSelfTradePrevented EventType = "SELF_TRADE_PREVENTED"
	EventRested             EventType = "ORDER_RESTED"
	EventFilled             EventType = "ORDER_FILLED"
	EventCanceled           EventType = "ORDER_CANCELED"
	EventCancelRejected     EventType = "CANCEL_REJECTED"
)

type Event struct {
	ID             string    `json:"id"`
	Sequence       uint64    `json:"sequence"`
	Index          int       `json:"index"`
	Type           EventType `json:"type"`
	OrderID        string    `json:"order_id,omitempty"`
	MakerOrderID   string    `json:"maker_order_id,omitempty"`
	TakerOrderID   string    `json:"taker_order_id,omitempty"`
	TradeID        uint64    `json:"trade_id,omitempty"`
	Price          int64     `json:"price,omitempty"`
	Quantity       int64     `json:"quantity,omitempty"`
	Remaining      int64     `json:"remaining,omitempty"`
	RelatedOrderID string    `json:"related_order_id,omitempty"`
	Reason         string    `json:"reason,omitempty"`
	STP            STPMode   `json:"stp,omitempty"`
}

type OrderView struct {
	OrderID       string      `json:"order_id"`
	ClientOrderID string      `json:"client_order_id"`
	AccountID     string      `json:"account_id"`
	Side          Side        `json:"side"`
	Price         int64       `json:"price"`
	Quantity      int64       `json:"quantity"`
	Remaining     int64       `json:"remaining"`
	TimeInForce   TimeInForce `json:"time_in_force"`
	STP           STPMode     `json:"stp"`
	PostOnly      bool        `json:"post_only,omitempty"`
	Status        OrderStatus `json:"status"`
	Sequence      uint64      `json:"sequence"`
}

type LevelView struct {
	Price    int64 `json:"price"`
	Quantity int64 `json:"quantity"`
	Orders   int   `json:"orders"`
}

type Snapshot struct {
	Version      int         `json:"version"`
	LastSequence uint64      `json:"last_sequence"`
	NextTradeID  uint64      `json:"next_trade_id"`
	Orders       []OrderView `json:"orders"`
}

type order struct {
	OrderView
}

type priceLevel struct {
	price  int64
	orders []*order
}

type bookSide struct {
	descending bool
	levels     []*priceLevel
}

type Engine struct {
	lastSequence  uint64
	nextTradeID   uint64
	bids          bookSide
	asks          bookSide
	orders        map[string]*order
	clientOrderID map[string]string
}

func New() *Engine {
	return &Engine{
		nextTradeID:   1,
		bids:          bookSide{descending: true},
		asks:          bookSide{descending: false},
		orders:        make(map[string]*order),
		clientOrderID: make(map[string]string),
	}
}

func (e *Engine) LastSequence() uint64 {
	return e.lastSequence
}

func (e *Engine) Order(orderID string) (OrderView, bool) {
	o, ok := e.orders[orderID]
	if !ok {
		return OrderView{}, false
	}
	return o.OrderView, true
}

func (e *Engine) Bids() []LevelView {
	return e.bids.depth()
}

func (e *Engine) Asks() []LevelView {
	return e.asks.depth()
}

// ValidateCommand validates the durable command envelope. Business rejections,
// such as duplicate IDs or an unfillable FOK, are deterministic events and are
// therefore handled by Apply instead of being returned as Go errors.
func ValidateCommand(cmd Command, expectedSequence uint64) error {
	if cmd.Sequence != expectedSequence {
		return fmt.Errorf("sequence: got %d, want %d", cmd.Sequence, expectedSequence)
	}
	switch cmd.Type {
	case CommandNewOrder:
		if cmd.NewOrder == nil || cmd.CancelOrder != nil {
			return errors.New("new-order command must contain only new_order")
		}
		o := cmd.NewOrder
		if strings.TrimSpace(o.OrderID) == "" ||
			strings.TrimSpace(o.ClientOrderID) == "" ||
			strings.TrimSpace(o.AccountID) == "" {
			return errors.New("order_id, client_order_id, and account_id are required")
		}
		if o.Side != Buy && o.Side != Sell {
			return fmt.Errorf("unsupported side %q", o.Side)
		}
		if o.Price <= 0 || o.Quantity <= 0 {
			return errors.New("price and quantity must be positive fixed-point integers")
		}
		if o.TimeInForce != GTC && o.TimeInForce != IOC && o.TimeInForce != FOK {
			return fmt.Errorf("unsupported time_in_force %q", o.TimeInForce)
		}
		if o.STP != STPCancelMaker && o.STP != STPCancelTaker && o.STP != STPCancelBoth {
			return fmt.Errorf("unsupported stp mode %q", o.STP)
		}
		if o.PostOnly && o.TimeInForce != GTC {
			return errors.New("post-only is only valid with GTC in this example")
		}
	case CommandCancelOrder:
		if cmd.CancelOrder == nil || cmd.NewOrder != nil {
			return errors.New("cancel command must contain only cancel_order")
		}
		if strings.TrimSpace(cmd.CancelOrder.OrderID) == "" {
			return errors.New("cancel order_id is required")
		}
	default:
		return fmt.Errorf("unsupported command type %q", cmd.Type)
	}
	return nil
}

func (e *Engine) Apply(cmd Command) ([]Event, error) {
	if err := ValidateCommand(cmd, e.lastSequence+1); err != nil {
		return nil, err
	}

	var events []Event
	emit := func(event Event) {
		event.Sequence = cmd.Sequence
		event.Index = len(events)
		event.ID = fmt.Sprintf("%d:%d", cmd.Sequence, event.Index)
		events = append(events, event)
	}

	switch cmd.Type {
	case CommandNewOrder:
		e.applyNew(*cmd.NewOrder, emit)
	case CommandCancelOrder:
		e.applyCancel(*cmd.CancelOrder, emit)
	}
	e.lastSequence = cmd.Sequence
	return events, nil
}

func (e *Engine) applyNew(in NewOrder, emit func(Event)) {
	if _, exists := e.orders[in.OrderID]; exists {
		emit(Event{Type: EventRejected, OrderID: in.OrderID, Reason: "duplicate_order_id"})
		return
	}
	if original, exists := e.clientOrderID[in.ClientOrderID]; exists {
		emit(Event{
			Type:           EventDuplicate,
			OrderID:        in.OrderID,
			RelatedOrderID: original,
			Reason:         "client_order_id_already_seen",
		})
		return
	}

	incoming := &order{OrderView: OrderView{
		OrderID:       in.OrderID,
		ClientOrderID: in.ClientOrderID,
		AccountID:     in.AccountID,
		Side:          in.Side,
		Price:         in.Price,
		Quantity:      in.Quantity,
		Remaining:     in.Quantity,
		TimeInForce:   in.TimeInForce,
		STP:           in.STP,
		PostOnly:      in.PostOnly,
		Status:        StatusOpen,
		Sequence:      e.lastSequence + 1,
	}}
	e.orders[in.OrderID] = incoming
	e.clientOrderID[in.ClientOrderID] = in.OrderID

	if in.PostOnly && e.wouldCross(in.Side, in.Price) {
		incoming.Status = StatusRejected
		emit(Event{Type: EventRejected, OrderID: in.OrderID, Remaining: in.Quantity, Reason: "post_only_would_take"})
		return
	}
	if in.TimeInForce == FOK && !e.fokFillable(in) {
		incoming.Status = StatusRejected
		emit(Event{Type: EventRejected, OrderID: in.OrderID, Remaining: in.Quantity, Reason: "fok_not_fillable"})
		return
	}

	emit(Event{Type: EventAccepted, OrderID: in.OrderID, Remaining: in.Quantity})
	opposite := &e.asks
	if in.Side == Sell {
		opposite = &e.bids
	}

	for incoming.Remaining > 0 {
		level := opposite.best()
		if level == nil || !crosses(in.Side, in.Price, level.price) {
			break
		}
		maker := level.orders[0]
		if maker.AccountID == incoming.AccountID {
			preventedQuantity := min(incoming.Remaining, maker.Remaining)
			emit(Event{
				Type:         EventSelfTradePrevented,
				OrderID:      incoming.OrderID,
				MakerOrderID: maker.OrderID,
				TakerOrderID: incoming.OrderID,
				Quantity:     preventedQuantity,
				Remaining:    incoming.Remaining,
				Reason:       string(incoming.STP),
				STP:          incoming.STP,
			})
			switch incoming.STP {
			case STPCancelMaker:
				maker.Status = StatusCanceled
				opposite.removeFront()
				emit(Event{
					Type:      EventCanceled,
					OrderID:   maker.OrderID,
					Remaining: maker.Remaining,
					Reason:    "stp_cancel_maker",
					STP:       incoming.STP,
				})
				continue
			case STPCancelTaker:
				incoming.Status = StatusCanceled
				emit(Event{
					Type:      EventCanceled,
					OrderID:   incoming.OrderID,
					Remaining: incoming.Remaining,
					Reason:    "stp_cancel_taker",
					STP:       incoming.STP,
				})
				return
			case STPCancelBoth:
				maker.Status = StatusCanceled
				opposite.removeFront()
				emit(Event{
					Type:      EventCanceled,
					OrderID:   maker.OrderID,
					Remaining: maker.Remaining,
					Reason:    "stp_cancel_both",
					STP:       incoming.STP,
				})
				incoming.Status = StatusCanceled
				emit(Event{
					Type:      EventCanceled,
					OrderID:   incoming.OrderID,
					Remaining: incoming.Remaining,
					Reason:    "stp_cancel_both",
					STP:       incoming.STP,
				})
				return
			default:
				panic("matchingengine: validated order has unsupported STP mode")
			}
		}
		fillQty := min(incoming.Remaining, maker.Remaining)
		incoming.Remaining -= fillQty
		maker.Remaining -= fillQty

		tradeID := e.nextTradeID
		e.nextTradeID++
		emit(Event{
			Type:         EventTrade,
			OrderID:      incoming.OrderID,
			MakerOrderID: maker.OrderID,
			TakerOrderID: incoming.OrderID,
			TradeID:      tradeID,
			Price:        maker.Price,
			Quantity:     fillQty,
			Remaining:    incoming.Remaining,
		})

		if maker.Remaining == 0 {
			maker.Status = StatusFilled
			opposite.removeFront()
		} else {
			maker.Status = StatusPartiallyFilled
		}
	}

	switch {
	case incoming.Remaining == 0:
		incoming.Status = StatusFilled
		emit(Event{Type: EventFilled, OrderID: incoming.OrderID})
	case in.TimeInForce == IOC:
		incoming.Status = StatusCanceled
		emit(Event{
			Type:      EventCanceled,
			OrderID:   incoming.OrderID,
			Remaining: incoming.Remaining,
			Reason:    "ioc_remainder",
		})
	case in.TimeInForce == FOK:
		// The precheck and the match loop inspect the same single-writer state.
		// Reaching this branch would indicate an engine invariant violation.
		panic("matchingengine: FOK precheck succeeded but order was not fully filled")
	default:
		if incoming.Remaining < incoming.Quantity {
			incoming.Status = StatusPartiallyFilled
		} else {
			incoming.Status = StatusOpen
		}
		e.side(incoming.Side).add(incoming)
		emit(Event{
			Type:      EventRested,
			OrderID:   incoming.OrderID,
			Price:     incoming.Price,
			Remaining: incoming.Remaining,
		})
	}
}

func (e *Engine) applyCancel(in CancelOrder, emit func(Event)) {
	o, ok := e.orders[in.OrderID]
	if !ok {
		emit(Event{Type: EventCancelRejected, OrderID: in.OrderID, Reason: "order_not_found"})
		return
	}
	if o.Status != StatusOpen && o.Status != StatusPartiallyFilled {
		emit(Event{
			Type:      EventCancelRejected,
			OrderID:   in.OrderID,
			Remaining: o.Remaining,
			Reason:    "order_not_open",
		})
		return
	}
	if !e.side(o.Side).remove(o.OrderID, o.Price) {
		panic("matchingengine: open order missing from book")
	}
	o.Status = StatusCanceled
	emit(Event{Type: EventCanceled, OrderID: o.OrderID, Remaining: o.Remaining, Reason: "user_requested"})
}

func (e *Engine) side(side Side) *bookSide {
	if side == Buy {
		return &e.bids
	}
	return &e.asks
}

func (e *Engine) wouldCross(side Side, price int64) bool {
	opposite := &e.asks
	if side == Sell {
		opposite = &e.bids
	}
	best := opposite.best()
	return best != nil && crosses(side, price, best.price)
}

func (e *Engine) fokFillable(in NewOrder) bool {
	opposite := &e.asks
	if in.Side == Sell {
		opposite = &e.bids
	}
	remaining := in.Quantity
	for _, level := range opposite.levels {
		if !crosses(in.Side, in.Price, level.price) {
			break
		}
		for _, maker := range level.orders {
			if maker.AccountID == in.AccountID {
				switch in.STP {
				case STPCancelMaker:
					continue
				case STPCancelTaker, STPCancelBoth:
					return false
				default:
					panic("matchingengine: validated order has unsupported STP mode")
				}
			}
			if maker.Remaining >= remaining {
				return true
			}
			remaining -= maker.Remaining
		}
	}
	return false
}

func crosses(takerSide Side, takerPrice, makerPrice int64) bool {
	if takerSide == Buy {
		return takerPrice >= makerPrice
	}
	return takerPrice <= makerPrice
}

func (s *bookSide) best() *priceLevel {
	if len(s.levels) == 0 {
		return nil
	}
	return s.levels[0]
}

func (s *bookSide) add(o *order) {
	index := sort.Search(len(s.levels), func(i int) bool {
		if s.descending {
			return s.levels[i].price <= o.Price
		}
		return s.levels[i].price >= o.Price
	})
	if index < len(s.levels) && s.levels[index].price == o.Price {
		s.levels[index].orders = append(s.levels[index].orders, o)
		return
	}
	level := &priceLevel{price: o.Price, orders: []*order{o}}
	s.levels = append(s.levels, nil)
	copy(s.levels[index+1:], s.levels[index:])
	s.levels[index] = level
}

func (s *bookSide) removeFront() {
	if len(s.levels) == 0 {
		panic("matchingengine: removeFront on empty side")
	}
	level := s.levels[0]
	level.orders = level.orders[1:]
	if len(level.orders) == 0 {
		s.levels = s.levels[1:]
	}
}

func (s *bookSide) remove(orderID string, price int64) bool {
	for levelIndex, level := range s.levels {
		if level.price != price {
			continue
		}
		for orderIndex, o := range level.orders {
			if o.OrderID != orderID {
				continue
			}
			level.orders = append(level.orders[:orderIndex], level.orders[orderIndex+1:]...)
			if len(level.orders) == 0 {
				s.levels = append(s.levels[:levelIndex], s.levels[levelIndex+1:]...)
			}
			return true
		}
		return false
	}
	return false
}

func (s *bookSide) depth() []LevelView {
	out := make([]LevelView, 0, len(s.levels))
	for _, level := range s.levels {
		view := LevelView{Price: level.price, Orders: len(level.orders)}
		for _, o := range level.orders {
			view.Quantity += o.Remaining
		}
		out = append(out, view)
	}
	return out
}

func (e *Engine) Snapshot() Snapshot {
	orders := make([]OrderView, 0, len(e.orders))
	for _, o := range e.orders {
		orders = append(orders, o.OrderView)
	}
	sort.Slice(orders, func(i, j int) bool {
		if orders[i].Sequence == orders[j].Sequence {
			return orders[i].OrderID < orders[j].OrderID
		}
		return orders[i].Sequence < orders[j].Sequence
	})
	return Snapshot{
		Version:      2,
		LastSequence: e.lastSequence,
		NextTradeID:  e.nextTradeID,
		Orders:       orders,
	}
}

func FromSnapshot(snapshot Snapshot) (*Engine, error) {
	if snapshot.Version != 2 {
		return nil, fmt.Errorf("unsupported snapshot version %d", snapshot.Version)
	}
	if snapshot.NextTradeID == 0 {
		return nil, errors.New("next_trade_id must be positive")
	}
	e := New()
	e.lastSequence = snapshot.LastSequence
	e.nextTradeID = snapshot.NextTradeID

	active := make([]*order, 0)
	for _, view := range snapshot.Orders {
		if view.OrderID == "" || view.ClientOrderID == "" || view.AccountID == "" {
			return nil, errors.New("snapshot contains an order without IDs")
		}
		if _, exists := e.orders[view.OrderID]; exists {
			return nil, fmt.Errorf("duplicate order_id %q in snapshot", view.OrderID)
		}
		if _, exists := e.clientOrderID[view.ClientOrderID]; exists {
			return nil, fmt.Errorf("duplicate client_order_id %q in snapshot", view.ClientOrderID)
		}
		if view.Sequence == 0 || view.Sequence > snapshot.LastSequence {
			return nil, fmt.Errorf("invalid sequence for order %q", view.OrderID)
		}
		if view.Quantity <= 0 || view.Price <= 0 || view.Remaining < 0 || view.Remaining > view.Quantity {
			return nil, fmt.Errorf("invalid quantity/price for order %q", view.OrderID)
		}
		if view.Side != Buy && view.Side != Sell {
			return nil, fmt.Errorf("invalid side for order %q", view.OrderID)
		}
		if view.TimeInForce != GTC && view.TimeInForce != IOC && view.TimeInForce != FOK {
			return nil, fmt.Errorf("invalid time_in_force for order %q", view.OrderID)
		}
		if view.STP != STPCancelMaker && view.STP != STPCancelTaker && view.STP != STPCancelBoth {
			return nil, fmt.Errorf("invalid stp mode for order %q", view.OrderID)
		}
		o := &order{OrderView: view}
		e.orders[view.OrderID] = o
		e.clientOrderID[view.ClientOrderID] = view.OrderID
		switch view.Status {
		case StatusOpen, StatusPartiallyFilled:
			if view.Remaining == 0 {
				return nil, fmt.Errorf("open order %q has zero remaining", view.OrderID)
			}
			active = append(active, o)
		case StatusFilled:
			if view.Remaining != 0 {
				return nil, fmt.Errorf("filled order %q has remaining quantity", view.OrderID)
			}
		case StatusCanceled, StatusRejected:
		default:
			return nil, fmt.Errorf("unsupported status %q", view.Status)
		}
	}
	sort.Slice(active, func(i, j int) bool {
		if active[i].Sequence == active[j].Sequence {
			return active[i].OrderID < active[j].OrderID
		}
		return active[i].Sequence < active[j].Sequence
	})
	for _, o := range active {
		e.side(o.Side).add(o)
	}
	return e, nil
}

func (e *Engine) StateHash() (string, error) {
	payload, err := json.Marshal(e.Snapshot())
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
