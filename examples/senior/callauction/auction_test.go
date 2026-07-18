package callauction

import (
	"errors"
	"reflect"
	"testing"
)

func TestPriceSelectionPriority(t *testing.T) {
	t.Run("maximize executable quantity", func(t *testing.T) {
		result, err := Uncross([]Order{
			{ID: "b1", Side: Buy, Price: 105, Quantity: 5, Sequence: 1},
			{ID: "b2", Side: Buy, Price: 100, Quantity: 5, Sequence: 2},
			{ID: "s1", Side: Sell, Price: 95, Quantity: 4, Sequence: 3},
			{ID: "s2", Side: Sell, Price: 103, Quantity: 5, Sequence: 4},
		}, Policy{ReferencePrice: 100, FinalTieBreak: LowerPrice})
		if err != nil {
			t.Fatal(err)
		}
		if result.ClearingPrice != 103 || result.ExecutableQuantity != 5 {
			t.Fatalf("result=%+v, want price=103 quantity=5", result)
		}
	})

	t.Run("minimize imbalance before reference distance", func(t *testing.T) {
		result, err := Uncross([]Order{
			{ID: "b1", Side: Buy, Price: 100, Quantity: 9, Sequence: 1},
			{ID: "b2", Side: Buy, Price: 99, Quantity: 1, Sequence: 2},
			{ID: "s1", Side: Sell, Price: 90, Quantity: 8, Sequence: 3},
		}, Policy{
			ReferencePrice:  99,
			CandidatePrices: []int64{99, 100},
			FinalTieBreak:   LowerPrice,
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.ClearingPrice != 100 || result.Imbalance != 1 {
			t.Fatalf("result=%+v, want price=100 imbalance=1", result)
		}
	})
}

func TestFinalTieBreakIsExplicitVenuePolicy(t *testing.T) {
	orders := []Order{
		{ID: "b", Side: Buy, Price: 110, Quantity: 10, Sequence: 1},
		{ID: "s", Side: Sell, Price: 90, Quantity: 10, Sequence: 2},
	}
	for _, tt := range []struct {
		name  string
		final FinalTieBreak
		want  int64
	}{
		{name: "lower", final: LowerPrice, want: 99},
		{name: "higher", final: HigherPrice, want: 101},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Uncross(orders, Policy{
				ReferencePrice:  100,
				CandidatePrices: []int64{99, 101},
				FinalTieBreak:   tt.final,
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.ClearingPrice != tt.want {
				t.Fatalf("price=%d, want %d", result.ClearingPrice, tt.want)
			}
		})
	}
}

func TestAllocationUsesBetterPriceThenFIFO(t *testing.T) {
	result, err := Uncross([]Order{
		{ID: "buy-better", Side: Buy, Price: 110, Quantity: 2, Sequence: 2},
		{ID: "buy-first-at", Side: Buy, Price: 100, Quantity: 2, Sequence: 1},
		{ID: "buy-later-at", Side: Buy, Price: 100, Quantity: 2, Sequence: 3},
		{ID: "sell-better", Side: Sell, Price: 90, Quantity: 3, Sequence: 4},
		{ID: "sell-at", Side: Sell, Price: 100, Quantity: 1, Sequence: 5},
	}, Policy{
		ReferencePrice:  100,
		CandidatePrices: []int64{100},
		FinalTieBreak:   LowerPrice,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []Trade{
		{ID: 1, BuyOrderID: "buy-better", SellOrderID: "sell-better", Price: 100, Quantity: 2},
		{ID: 2, BuyOrderID: "buy-first-at", SellOrderID: "sell-better", Price: 100, Quantity: 1},
		{ID: 3, BuyOrderID: "buy-first-at", SellOrderID: "sell-at", Price: 100, Quantity: 1},
	}
	if !reflect.DeepEqual(result.Trades, want) {
		t.Fatalf("trades:\n got=%+v\nwant=%+v", result.Trades, want)
	}
}

func TestNoCrossAndValidation(t *testing.T) {
	_, err := Uncross([]Order{
		{ID: "b", Side: Buy, Price: 99, Quantity: 1, Sequence: 1},
		{ID: "s", Side: Sell, Price: 101, Quantity: 1, Sequence: 2},
	}, Policy{ReferencePrice: 100, FinalTieBreak: LowerPrice})
	if !errors.Is(err, ErrNoCross) {
		t.Fatalf("err=%v, want ErrNoCross", err)
	}

	_, err = Uncross([]Order{
		{ID: "b", Side: Buy, Price: 100, Quantity: 1, Sequence: 1},
		{ID: "s", Side: Sell, Price: 100, Quantity: 1, Sequence: 1},
	}, Policy{ReferencePrice: 100, FinalTieBreak: LowerPrice})
	if err == nil {
		t.Fatal("expected duplicate sequence validation error")
	}
}
