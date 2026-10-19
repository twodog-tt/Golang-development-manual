package agenttrust

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"testing"
)

var reviewer = common.HexToAddress("0x300")
var repRegistry = common.HexToAddress("0x400")
var valRegistry = common.HexToAddress("0x500")
var validator = common.HexToAddress("0x600")

func projection(t *testing.T) *Projection {
	t.Helper()
	p, e := NewProjection(ref, owner, repRegistry, valRegistry)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func hash(s string) common.Hash { return crypto.Keccak256Hash([]byte(s)) }
func TestFeedbackExactDecimalsRevocationDuplicateAndReorg(t *testing.T) {
	p := projection(t)
	b := Block{1, hash("b1"), common.Hash{}, []Event{
		{Kind: "feedback", Registry: repRegistry, Ref: ref, Actor: reviewer, Index: 1, Value: "100", Decimals: 2, Tag: "quality"},
		{Kind: "feedback", Registry: repRegistry, Ref: ref, Actor: reviewer, Index: 2, Value: "-1", Tag: "quality"},
		{Kind: "feedback", Registry: repRegistry, Ref: ref, Actor: reviewer, Index: 3, Value: "999", Tag: "latency"},
	}}
	if e := p.Apply(b); e != nil {
		t.Fatal(e)
	}
	if e := p.Apply(b); e != nil {
		t.Fatal(e)
	}
	value, n, e := p.Mean([]common.Address{reviewer}, "quality")
	if e != nil || value != "0" || n != 2 {
		t.Fatal(value, n, e)
	}
	revoke := Block{2, hash("b2"), b.Hash, []Event{{Kind: "revoke", Registry: repRegistry, Ref: ref, Actor: reviewer, Index: 1}}}
	if e = p.Apply(revoke); e != nil {
		t.Fatal(e)
	}
	value, n, e = p.Mean([]common.Address{reviewer}, "quality")
	if e != nil || value != "-1" || n != 1 {
		t.Fatal(value, n, e)
	}
	if e = p.Rewind(b.Hash); e != nil {
		t.Fatal(e)
	}
	value, n, e = p.Mean([]common.Address{reviewer}, "quality")
	if e != nil || value != "0" || n != 2 {
		t.Fatal(value, n, e)
	}
	if _, _, e = p.Mean(nil, "quality"); e == nil {
		t.Fatal("unfiltered summary")
	}
}
func TestSignalInvalidEventsAreAtomicAndBounded(t *testing.T) {
	valid := Event{Kind: "feedback", Registry: repRegistry, Ref: ref, Actor: reviewer, Index: 1, Value: "1", Tag: "q"}
	for _, mode := range []string{"registry", "identity", "owner", "decimals", "overflow", "duplicate", "orphan-revoke"} {
		t.Run(mode, func(t *testing.T) {
			p := projection(t)
			bad := valid
			switch mode {
			case "registry":
				bad.Registry = valRegistry
			case "identity":
				bad.Ref.AgentID = "2"
			case "owner":
				bad.Actor = owner
			case "decimals":
				bad.Decimals = 19
			case "overflow":
				bad.Value = "170141183460469231731687303715884105728"
			case "orphan-revoke":
				bad.Kind = "revoke"
				bad.Index = 2
			}
			if e := p.Apply(Block{1, hash(mode), common.Hash{}, []Event{valid, bad}}); e == nil {
				t.Fatal("invalid block applied")
			}
			if _, _, e := p.Mean([]common.Address{reviewer}, "q"); e == nil {
				t.Fatal("partially applied invalid block")
			}
		})
	}
}
func TestValidationCallerIdentityDowngradeAndCanonicalReplay(t *testing.T) {
	p := projection(t)
	payload := `{"task":"one","code":"v1","output":"digest"}`
	requestHash := hash(payload)
	b1 := Block{1, hash("b1"), common.Hash{}, []Event{{Kind: "request", Registry: valRegistry, Ref: ref, Actor: owner, Validator: validator, RequestHash: requestHash, Payload: payload}}}
	for _, mode := range []string{"request-owner", "request-payload", "request-validator"} {
		bad := b1.Events[0]
		switch mode {
		case "request-owner":
			bad.Actor = reviewer
		case "request-payload":
			bad.Payload = "different task"
		case "request-validator":
			bad.Validator = common.Address{}
		}
		if err := p.Apply(Block{1, hash(mode), common.Hash{}, []Event{bad}}); err == nil {
			t.Fatal("invalid request accepted", mode)
		}
	}
	if e := p.Apply(b1); e != nil {
		t.Fatal(e)
	}
	if err := p.Apply(Block{2, hash("duplicate-request"), b1.Hash, b1.Events}); err == nil {
		t.Fatal("request commitment reused")
	}

	response := Event{Kind: "response", Registry: valRegistry, Ref: ref, Actor: validator, RequestHash: requestHash, Score: 100, Tag: "soft"}
	b2 := Block{2, hash("b2"), b1.Hash, []Event{response}}
	if e := p.Apply(b2); e != nil {
		t.Fatal(e)
	}
	response.Score = 0
	response.Tag = "invalidated"
	if e := p.Apply(Block{3, hash("b3"), b2.Hash, []Event{response}}); e != nil {
		t.Fatal(e)
	}
	v, e := p.Validation(requestHash)
	if e != nil || v.Score != 0 {
		t.Fatal("old positive response retained", v, e)
	}
	if e = p.Rewind(b2.Hash); e != nil {
		t.Fatal(e)
	}
	v, e = p.Validation(requestHash)
	if e != nil || v.Score != 100 {
		t.Fatal(v, e)
	}
	if e = p.Rewind(b1.Hash); e != nil {
		t.Fatal(e)
	}
	if _, e = p.Validation(requestHash); e == nil {
		t.Fatal("orphaned response still authorizes")
	}
	for _, mode := range []string{"caller", "registry", "identity", "request", "score"} {
		bad := response
		switch mode {
		case "caller":
			bad.Actor = reviewer
		case "registry":
			bad.Registry = repRegistry
		case "identity":
			bad.Ref.AgentID = "2"
		case "request":
			bad.RequestHash = hash("another task")
		case "score":
			bad.Score = 101
		}
		if e = p.Apply(Block{2, hash(mode), b1.Hash, []Event{bad}}); e == nil {
			t.Fatal(mode)
		}
	}
	response.Score = 80
	if e = p.Apply(Block{2, hash("canonical"), b1.Hash, []Event{response}}); e != nil {
		t.Fatal(e)
	}
	v, e = p.Validation(requestHash)
	if e != nil || v.Score != 80 {
		t.Fatal(v, e)
	}
}
func TestProjectionLineageConflictAndCallerMutation(t *testing.T) {
	p := projection(t)
	b := Block{1, hash("one"), common.Hash{}, []Event{{Kind: "feedback", Registry: repRegistry, Ref: ref, Actor: reviewer, Index: 1, Value: "1", Tag: "q"}}}
	if e := p.Apply(b); e != nil {
		t.Fatal(e)
	}
	b.Events[0].Value = "99"
	if e := p.Apply(b); e == nil {
		t.Fatal("same hash changed content")
	}
	v, _, _ := p.Mean([]common.Address{reviewer}, "q")
	if v != "1" {
		t.Fatal("caller rewrote accepted evidence")
	}
	if e := p.Apply(Block{2, hash("two"), hash("wrong"), nil}); e == nil {
		t.Fatal("wrong ancestry")
	}
	if e := p.Rewind(hash("missing")); e == nil {
		t.Fatal("guessed ancestor")
	}
}
