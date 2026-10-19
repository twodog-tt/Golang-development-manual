package agentplatform

import "sync"

// Principal is authenticated by trusted ingress, NOT taken from request JSON.
type Principal struct{ Tenant, Subject string }
type Offer struct {
	ID, Revision, Seller, Capability, Permissions, Asset, Unit, Terms string
	Price, Quota                                                      int64
}
type Quote struct {
	ID      string
	Buyer   Principal
	Offer   Offer
	Expires int64
}

// PaymentEvidence comes only from a trusted receipt adapter. Domain includes
// chain/asset/provider; Transaction+Event is the unique settled payment identity.
type PaymentEvidence struct {
	Domain, Transaction, Event, QuoteDigest, Block string
	Canonical, Final, Refunded                     bool
}
type PaymentReader interface {
	Lookup(string) (PaymentEvidence, error)
}
type purchase struct {
	Quote   Quote
	Receipt string
	Used    int64
}
type Market struct {
	mu          sync.Mutex
	offers      map[string]Offer
	quotes      map[string]Quote
	buys        map[string]purchase
	payments    map[string]string
	invocations map[string]bool
	reader      PaymentReader
}

func NewMarket(r PaymentReader) *Market {
	return &Market{offers: map[string]Offer{}, quotes: map[string]Quote{}, buys: map[string]purchase{}, payments: map[string]string{}, invocations: map[string]bool{}, reader: r}
}
func offerKey(o Offer) string { return hash([]string{o.Seller, o.ID, o.Revision}) }
func (m *Market) Publish(o Offer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if o.ID == "" || o.Revision == "" || o.Seller == "" || o.Capability == "" || o.Permissions == "" || o.Asset == "" || o.Unit == "" || o.Terms == "" || o.Price <= 0 || o.Quota <= 0 {
		return ErrDenied
	}
	k := offerKey(o)
	if old, ok := m.offers[k]; ok && old != o {
		return ErrDenied
	}
	m.offers[k] = o
	return nil
}
func (m *Market) Quote(id string, buyer Principal, o Offer, expiry, now int64) (Quote, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	published, exists := m.offers[offerKey(o)]
	if id == "" || buyer.Tenant == "" || buyer.Subject == "" || expiry <= now || expiry-now > 300 || !exists || published != o {
		return Quote{}, ErrDenied
	}
	q := Quote{id, buyer, o, expiry}
	if old, ok := m.quotes[id]; ok && old != q {
		return Quote{}, ErrDenied
	}
	m.quotes[id] = q
	return q, nil
}
func QuoteDigest(q Quote) string { return hash([]any{"local-agent-quote-v1", q}) }
func proofValid(q Quote, e PaymentEvidence) bool {
	// Domain/quote digest pins the exact asset, amount, buyer and offer terms.
	return e.Domain == q.Offer.Asset && e.QuoteDigest == QuoteDigest(q) && e.Transaction != "" && e.Event != "" && e.Block != "" && e.Canonical && e.Final && !e.Refunded
}
func (m *Market) Buy(who Principal, id, receipt string, now int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	q, ok := m.quotes[id]
	if !ok || q.Buyer != who || receipt == "" || m.reader == nil {
		return ErrDenied
	}
	e, err := m.reader.Lookup(receipt)
	if err != nil || !proofValid(q, e) {
		return ErrUnknown
	}
	if old, ok := m.buys[id]; ok {
		if old.Receipt == receipt {
			return nil
		}
		return ErrDenied
	}
	if q.Expires <= now {
		return ErrDenied
	}
	pk := hash([]string{e.Domain, e.Transaction, e.Event})
	if _, used := m.payments[pk]; used {
		return ErrDenied
	}
	m.payments[pk] = id
	m.buys[id] = purchase{Quote: q, Receipt: receipt}
	return nil
}
func (m *Market) Invoke(who Principal, id, intent string, actual Offer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.buys[id]
	if !ok || intent == "" || p.Quote.Buyer != who || p.Quote.Offer != actual || m.reader == nil {
		return ErrDenied
	}
	e, err := m.reader.Lookup(p.Receipt)
	if err != nil || !proofValid(p.Quote, e) {
		return ErrUnknown
	}
	ik := hash([]string{id, intent})
	if m.invocations[ik] {
		return nil
	}
	if p.Used >= p.Quote.Offer.Quota {
		return ErrDenied
	}
	p.Used++
	m.buys[id] = p
	m.invocations[ik] = true
	return nil
}

// Reputation is a separate projection. A unique job/payment remains consumed
// after refund/reorg; removing a signal never frees evidence for another review.
type ReviewEvidence struct {
	Job, BuyerActor, SellerActor, PaymentKey    string
	Paid, Delivered, Canonical, Final, Refunded bool
	Revision                                    uint64
}
type Review struct {
	ID, ReviewerActor, Job string
	Value                  int
}
type Reputation struct {
	mu       sync.Mutex
	evidence map[string]ReviewEvidence
	reviews  map[string]Review
	jobs     map[string]string
	payments map[string]string
}

func NewReputation() *Reputation {
	return &Reputation{evidence: map[string]ReviewEvidence{}, reviews: map[string]Review{}, jobs: map[string]string{}, payments: map[string]string{}}
}

// Observe is a trusted settlement/delivery projection input, not a buyer claim.
func (r *Reputation) Observe(e ReviewEvidence) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e.Job == "" || e.BuyerActor == "" || e.SellerActor == "" || e.PaymentKey == "" || e.Revision == 0 {
		return ErrDenied
	}
	if old, ok := r.evidence[e.Job]; ok && (old.BuyerActor != e.BuyerActor || old.SellerActor != e.SellerActor || old.PaymentKey != e.PaymentKey) {
		return ErrDenied
	}
	if old, ok := r.evidence[e.Job]; ok && e.Revision <= old.Revision {
		if old == e {
			return nil
		}
		return ErrDenied
	}
	if job, ok := r.payments[e.PaymentKey]; ok && job != e.Job {
		return ErrDenied
	}
	r.payments[e.PaymentKey] = e.Job
	r.evidence[e.Job] = e
	return nil
}
func reviewable(e ReviewEvidence) bool {
	return e.Paid && e.Delivered && e.Canonical && e.Final && !e.Refunded && e.BuyerActor != e.SellerActor
}
func (r *Reputation) Add(v Review) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.evidence[v.Job]
	if !ok || !reviewable(e) || v.ID == "" || v.ReviewerActor != e.BuyerActor || v.Value < 0 || v.Value > 5 {
		return ErrDenied
	}
	if old, ok := r.reviews[v.ID]; ok {
		if old == v {
			return nil
		}
		return ErrDenied
	}
	if _, ok := r.jobs[v.Job]; ok {
		return ErrDenied
	}
	r.jobs[v.Job] = v.ID
	r.reviews[v.ID] = v
	return nil
}
func (r *Reputation) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, v := range r.reviews {
		if reviewable(r.evidence[v.Job]) {
			n++
		}
	}
	return n
}
