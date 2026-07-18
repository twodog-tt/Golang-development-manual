package matchingengine

import (
	"strconv"
	"testing"
)

// BenchmarkMatchOneMaker measures the state-machine hot path while excluding
// fixture construction. It is a microbenchmark, not an end-to-end latency SLO.
func BenchmarkMatchOneMaker(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		engine := New()
		maker := newLimit(1, "maker", Sell, 100, 1, GTC, false)
		if _, err := engine.Apply(maker); err != nil {
			b.Fatal(err)
		}
		taker := newLimit(2, "taker", Buy, 100, 1, IOC, false)
		b.StartTimer()

		if _, err := engine.Apply(taker); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRestSamePriceBatch1024 measures a fixed workload: constructing an
// engine and resting 1,024 orders at one price. A fixed batch avoids the
// non-stationary benchmark mistake of growing one book with b.N.
func BenchmarkRestSamePriceBatch1024(b *testing.B) {
	const batchSize = 1024
	commands := make([]Command, batchSize)
	for i := range commands {
		id := strconv.Itoa(i)
		commands[i] = newLimit(uint64(i+1), id, Buy, 100, 1, GTC, false)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine := New()
		for _, command := range commands {
			if _, err := engine.Apply(command); err != nil {
				b.Fatal(err)
			}
		}
	}
}
