// Package callauction implements a deterministic, limit-order-only call
// auction. Venue-specific eligibility, market orders, collars, and the final
// price tie-break are intentionally supplied as policy rather than presented
// as universal exchange rules.
package callauction

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

var ErrNoCross = errors.New("auction has no executable crossing interest")

type Side string

const (
	Buy  Side = "BUY"
	Sell Side = "SELL"
)

type FinalTieBreak string

const (
	LowerPrice  FinalTieBreak = "LOWER_PRICE"
	HigherPrice FinalTieBreak = "HIGHER_PRICE"
)

type Order struct {
	ID       string `json:"id"`
	Side     Side   `json:"side"`
	Price    int64  `json:"price"`
	Quantity int64  `json:"quantity"`
	Sequence uint64 `json:"sequence"`
}

type Policy struct {
	ReferencePrice  int64         `json:"reference_price"`
	CandidatePrices []int64       `json:"candidate_prices,omitempty"`
	FinalTieBreak   FinalTieBreak `json:"final_tie_break"`
}

type Trade struct {
	ID          uint64 `json:"id"`
	BuyOrderID  string `json:"buy_order_id"`
	SellOrderID string `json:"sell_order_id"`
	Price       int64  `json:"price"`
	Quantity    int64  `json:"quantity"`
}

type Result struct {
	ClearingPrice      int64   `json:"clearing_price"`
	ExecutableQuantity int64   `json:"executable_quantity"`
	BuyInterest        int64   `json:"buy_interest"`
	SellInterest       int64   `json:"sell_interest"`
	Imbalance          int64   `json:"imbalance"`
	ImbalanceSide      Side    `json:"imbalance_side,omitempty"`
	Trades             []Trade `json:"trades"`
}

type evaluation struct {
	price      int64
	buy        int64
	sell       int64
	executable int64
	imbalance  int64
	distance   int64
}

// Uncross selects a clearing price by:
//  1. maximizing executable quantity,
//  2. minimizing absolute imbalance,
//  3. minimizing distance to the reference price,
//  4. applying the configured final deterministic tie-break.
//
// Better-priced orders execute before at-price orders, then FIFO by Sequence.
func Uncross(orders []Order, policy Policy) (Result, error) {
	if err := validate(orders, policy); err != nil {
		return Result{}, err
	}
	candidates := candidatePrices(orders, policy)
	best, err := choosePrice(orders, candidates, policy)
	if err != nil {
		return Result{}, err
	}

	buys := eligibleOrders(orders, Buy, best.price)
	sells := eligibleOrders(orders, Sell, best.price)
	sort.Slice(buys, func(i, j int) bool {
		if buys[i].Price != buys[j].Price {
			return buys[i].Price > buys[j].Price
		}
		return buys[i].Sequence < buys[j].Sequence
	})
	sort.Slice(sells, func(i, j int) bool {
		if sells[i].Price != sells[j].Price {
			return sells[i].Price < sells[j].Price
		}
		return sells[i].Sequence < sells[j].Sequence
	})

	trades := allocate(buys, sells, best.price, best.executable)
	result := Result{
		ClearingPrice:      best.price,
		ExecutableQuantity: best.executable,
		BuyInterest:        best.buy,
		SellInterest:       best.sell,
		Imbalance:          best.imbalance,
		Trades:             trades,
	}
	switch {
	case best.buy > best.sell:
		result.ImbalanceSide = Buy
	case best.sell > best.buy:
		result.ImbalanceSide = Sell
	}
	return result, nil
}

func choosePrice(orders []Order, candidates []int64, policy Policy) (evaluation, error) {
	var best evaluation
	haveBest := false
	for _, price := range candidates {
		current, err := evaluateAt(orders, price, policy.ReferencePrice)
		if err != nil {
			return evaluation{}, err
		}
		if !haveBest || better(current, best, policy.FinalTieBreak) {
			best = current
			haveBest = true
		}
	}
	if !haveBest || best.executable == 0 {
		return evaluation{}, ErrNoCross
	}
	return best, nil
}

func evaluateAt(orders []Order, price, reference int64) (evaluation, error) {
	current := evaluation{price: price, distance: absDiff(price, reference)}
	for _, order := range orders {
		var err error
		switch {
		case order.Side == Buy && order.Price >= price:
			current.buy, err = checkedAdd(current.buy, order.Quantity)
		case order.Side == Sell && order.Price <= price:
			current.sell, err = checkedAdd(current.sell, order.Quantity)
		}
		if err != nil {
			return evaluation{}, err
		}
	}
	current.executable = min(current.buy, current.sell)
	current.imbalance = absDiff(current.buy, current.sell)
	return current, nil
}

func better(a, b evaluation, final FinalTieBreak) bool {
	if a.executable != b.executable {
		return a.executable > b.executable
	}
	if a.imbalance != b.imbalance {
		return a.imbalance < b.imbalance
	}
	if a.distance != b.distance {
		return a.distance < b.distance
	}
	if final == HigherPrice {
		return a.price > b.price
	}
	return a.price < b.price
}

func candidatePrices(orders []Order, policy Policy) []int64 {
	source := policy.CandidatePrices
	if len(source) == 0 {
		source = make([]int64, 0, len(orders)+1)
		for _, order := range orders {
			source = append(source, order.Price)
		}
		source = append(source, policy.ReferencePrice)
	}

	seen := make(map[int64]struct{}, len(source))
	out := make([]int64, 0, len(source))
	for _, price := range source {
		if _, exists := seen[price]; exists {
			continue
		}
		seen[price] = struct{}{}
		out = append(out, price)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func eligibleOrders(orders []Order, side Side, clearingPrice int64) []Order {
	var out []Order
	for _, order := range orders {
		if order.Side != side {
			continue
		}
		if side == Buy && order.Price >= clearingPrice {
			out = append(out, order)
		}
		if side == Sell && order.Price <= clearingPrice {
			out = append(out, order)
		}
	}
	return out
}

func allocate(buys, sells []Order, price, target int64) []Trade {
	buyRemaining := remainingOrders(buys, target)
	sellRemaining := remainingOrders(sells, target)

	var trades []Trade
	var buyIndex, sellIndex int
	var nextTradeID uint64 = 1
	for buyIndex < len(buyRemaining) && sellIndex < len(sellRemaining) {
		quantity := min(buyRemaining[buyIndex].remaining, sellRemaining[sellIndex].remaining)
		trades = append(trades, Trade{
			ID:          nextTradeID,
			BuyOrderID:  buyRemaining[buyIndex].order.ID,
			SellOrderID: sellRemaining[sellIndex].order.ID,
			Price:       price,
			Quantity:    quantity,
		})
		nextTradeID++
		buyRemaining[buyIndex].remaining -= quantity
		sellRemaining[sellIndex].remaining -= quantity
		if buyRemaining[buyIndex].remaining == 0 {
			buyIndex++
		}
		if sellRemaining[sellIndex].remaining == 0 {
			sellIndex++
		}
	}
	return trades
}

type orderRemaining struct {
	order     Order
	remaining int64
}

func remainingOrders(orders []Order, target int64) []orderRemaining {
	out := make([]orderRemaining, 0, len(orders))
	left := target
	for _, order := range orders {
		if left == 0 {
			break
		}
		quantity := min(order.Quantity, left)
		out = append(out, orderRemaining{order: order, remaining: quantity})
		left -= quantity
	}
	if left != 0 {
		panic("callauction: aggregate evaluation and allocation disagree")
	}
	return out
}

func validate(orders []Order, policy Policy) error {
	if len(orders) == 0 {
		return errors.New("at least one order is required")
	}
	if policy.ReferencePrice <= 0 {
		return errors.New("reference price must be positive")
	}
	if policy.FinalTieBreak != LowerPrice && policy.FinalTieBreak != HigherPrice {
		return fmt.Errorf("unsupported final tie-break %q", policy.FinalTieBreak)
	}
	for _, price := range policy.CandidatePrices {
		if price <= 0 {
			return fmt.Errorf("candidate price must be positive: %d", price)
		}
	}

	ids := make(map[string]struct{}, len(orders))
	sequences := make(map[uint64]struct{}, len(orders))
	for _, order := range orders {
		if order.ID == "" {
			return errors.New("order ID is required")
		}
		if _, exists := ids[order.ID]; exists {
			return fmt.Errorf("duplicate order ID %q", order.ID)
		}
		ids[order.ID] = struct{}{}
		if order.Side != Buy && order.Side != Sell {
			return fmt.Errorf("unsupported side %q", order.Side)
		}
		if order.Price <= 0 || order.Quantity <= 0 {
			return fmt.Errorf("order %q has non-positive price or quantity", order.ID)
		}
		if order.Sequence == 0 {
			return fmt.Errorf("order %q has zero sequence", order.ID)
		}
		if _, exists := sequences[order.Sequence]; exists {
			return fmt.Errorf("duplicate sequence %d", order.Sequence)
		}
		sequences[order.Sequence] = struct{}{}
	}
	return nil
}

func checkedAdd(a, b int64) (int64, error) {
	if b > math.MaxInt64-a {
		return 0, errors.New("aggregate auction quantity overflow")
	}
	return a + b, nil
}

func absDiff(a, b int64) int64 {
	if a >= b {
		return a - b
	}
	return b - a
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
