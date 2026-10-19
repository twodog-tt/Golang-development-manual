package agentplatform

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

type payments struct {
	mu   sync.Mutex
	rows map[string]PaymentEvidence
	err  error
}

func (p *payments) Lookup(id string) (PaymentEvidence, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.rows[id], p.err
}
func (p *payments) put(id string, e PaymentEvidence) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rows[id] = e
}
func offer() Offer {
	return Offer{"report", "v1", "seller", digest([]byte("capability-v1")), digest([]byte("read")), "eip155:84532/local-usd", "call", "terms-v1", 10, 4}
}
func market(t *testing.T) (*Market, *payments, Quote) {
	t.Helper()
	p := &payments{rows: map[string]PaymentEvidence{}}
	m := NewMarket(p)
	o := offer()
	must(t, m.Publish(o))
	q, err := m.Quote("quote-1", Principal{"tenant-a", "buyer"}, o, 200, 100)
	must(t, err)
	p.put("receipt-1", PaymentEvidence{o.Asset, "tx", "log-0", QuoteDigest(q), "block-1", true, true, false})
	return m, p, q
}

func TestQuotePinsBuyerVersionPricePermissionsAndExpiry(t *testing.T) {
	t.Run("unpublished-offer", func(t *testing.T) {
		m := NewMarket(nil)
		for i, o := range []Offer{{}, offer()} {
			_, err := m.Quote(fmt.Sprint("unpublished-", i), Principal{"tenant-a", "buyer"}, o, 200, 100)
			denied(t, err)
		}
	})
	m, _, q := market(t)
	o := q.Offer
	changed := o
	changed.Price++
	denied(t, m.Publish(changed))
	changed.Revision = "v2"
	changed.Capability = "new-code"
	changed.Permissions = "sign"
	must(t, m.Publish(changed))
	must(t, m.Buy(q.Buyer, q.ID, "receipt-1", 199))
	must(t, m.Buy(q.Buyer, q.ID, "receipt-1", 250)) // purchase replay doesn't recharge
	denied(t, m.Invoke(q.Buyer, q.ID, "new-version", changed))
	must(t, m.Invoke(q.Buyer, q.ID, "old-version", o))
	for _, who := range []Principal{{"other", "buyer"}, {"tenant-a", "other"}} {
		denied(t, m.Buy(who, q.ID, "receipt-1", 100))
		denied(t, m.Invoke(who, q.ID, "cross", o))
	}
	m, _, q = market(t)
	denied(t, m.Buy(q.Buyer, q.ID, "receipt-1", 200))
}

func TestMarketReceiptFinalityBindingRefundAndReuse(t *testing.T) {
	for _, name := range []string{"domain", "digest", "transaction", "event", "block", "canonical", "final", "refund"} {
		t.Run(name, func(t *testing.T) {
			m, p, q := market(t)
			e, _ := p.Lookup("receipt-1")
			switch name {
			case "domain":
				e.Domain = "other"
			case "digest":
				e.QuoteDigest = "other"
			case "transaction":
				e.Transaction = ""
			case "event":
				e.Event = ""
			case "block":
				e.Block = ""
			case "canonical":
				e.Canonical = false
			case "final":
				e.Final = false
			case "refund":
				e.Refunded = true
			}
			p.put("bad", e)
			denied(t, m.Buy(q.Buyer, q.ID, "bad", 100))
		})
	}
	m, p, q := market(t)
	must(t, m.Buy(q.Buyer, q.ID, "receipt-1", 100))
	must(t, m.Invoke(q.Buyer, q.ID, "once", q.Offer))
	e, _ := p.Lookup("receipt-1")
	e.Refunded = true
	p.put("receipt-1", e)
	denied(t, m.Buy(q.Buyer, q.ID, "receipt-1", 100))
	denied(t, m.Invoke(q.Buyer, q.ID, "once", q.Offer))
	denied(t, m.Invoke(q.Buyer, q.ID, "new", q.Offer))
	e.Refunded = false
	e.Canonical = false
	p.put("receipt-1", e)
	denied(t, m.Invoke(q.Buyer, q.ID, "new", q.Offer))
	// Even a faulty adapter associating the same receipt with another quote must
	// not grant a second entitlement from that payment event.
	q2, err := m.Quote("quote-2", q.Buyer, q.Offer, 200, 100)
	must(t, err)
	e.Canonical = true
	e.QuoteDigest = QuoteDigest(q2)
	p.put("alias", e)
	denied(t, m.Buy(q2.Buyer, q2.ID, "alias", 100))
}

func TestMarketConcurrentQuotaAndIdempotentInvocation(t *testing.T) {
	m, _, q := market(t)
	must(t, m.Buy(q.Buyer, q.ID, "receipt-1", 100))
	var wg sync.WaitGroup
	var won atomic.Int64
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if m.Invoke(q.Buyer, q.ID, fmt.Sprint(i), q.Offer) == nil {
				won.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if won.Load() != 4 {
		t.Fatal("quota admission", won.Load())
	}
	m, _, q = market(t)
	must(t, m.Buy(q.Buyer, q.ID, "receipt-1", 100))
	for i := 0; i < 32; i++ {
		must(t, m.Invoke(q.Buyer, q.ID, "same", q.Offer))
	}
	if m.buys[q.ID].Used != 1 {
		t.Fatal("retry charged quota twice")
	}
}

func TestReputationUniqueEconomicEvidenceRefundAndReorg(t *testing.T) {
	r := NewReputation()
	e := ReviewEvidence{"job", "buyer-actor", "seller-actor", "chain/tx/event-1", true, true, true, true, false, 1}
	must(t, r.Observe(e))
	v := Review{"review", "buyer-actor", "job", 5}
	must(t, r.Add(v))
	must(t, r.Add(v))
	denied(t, r.Add(Review{"new-review", "buyer-actor", "job", 5}))
	denied(t, r.Add(Review{"other-address", "other-actor", "job", 5}))
	reused := e
	reused.Job = "another-job"
	denied(t, r.Observe(reused))
	if r.Count() != 1 {
		t.Fatal("duplicate count")
	}
	e.Refunded = true
	e.Revision++
	must(t, r.Observe(e))
	if r.Count() != 0 {
		t.Fatal("refund retained weight")
	}
	e.Refunded = false
	e.Canonical = false
	e.Revision++
	must(t, r.Observe(e))
	if r.Count() != 0 {
		t.Fatal("orphan retained weight")
	}
	e.Canonical = true
	e.Revision++
	must(t, r.Observe(e))
	if r.Count() != 1 {
		t.Fatal("canonical recovery did not restore original signal")
	}
	denied(t, r.Add(Review{"reuse", "buyer-actor", "job", 5}))
	stale := e
	stale.Revision--
	stale.Refunded = true
	denied(t, r.Observe(stale))
	if r.Count() != 1 {
		t.Fatal("stale evidence rewrote projection")
	}
	e.Job = "self"
	e.PaymentKey = "chain/tx/event-2"
	e.BuyerActor = e.SellerActor
	must(t, r.Observe(e))
	denied(t, r.Add(Review{"self-review", e.BuyerActor, e.Job, 5}))
	e.Job = "unpaid"
	e.PaymentKey = "chain/tx/event-3"
	e.BuyerActor = "buyer"
	e.Paid = false
	must(t, r.Observe(e))
	denied(t, r.Add(Review{"unpaid-review", e.BuyerActor, e.Job, 5}))
}
