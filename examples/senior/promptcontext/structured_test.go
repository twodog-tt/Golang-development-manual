package promptcontext

import (
	"context"
	"encoding/json"
	"strings"
	"td-homework/examples/senior/llmclient"
	"testing"
)

func TestActualClientInterfaceAndStrictIntent(t *testing.T) {
	reply := `{"action":"query","order_id":"ORD-1"}`
	got, e := Extract(context.Background(), LocalFixture{Model: &llmclient.MockClient{Reply: reply}, Status: "completed", Version: PromptVersion}, "查订单")
	if e != nil || got.Action != "query" || got.OrderID != "ORD-1" {
		t.Fatal(got, e)
	}
	var schema map[string]any
	if e = json.Unmarshal([]byte(OrderSchema), &schema); e != nil || schema["additionalProperties"] != false {
		t.Fatal(e)
	}
}
func TestRefusalIncompleteEmptyAndVersion(t *testing.T) {
	for _, v := range []Envelope{{Version: PromptVersion, Status: "incomplete", Content: `{"action":"refund","order_id":"ORD-1"}`}, {Version: PromptVersion, Status: "completed", Refusal: "declined", Content: `{"action":"refund","order_id":"ORD-1"}`}, {Version: PromptVersion, Status: "failed"}, {Version: PromptVersion, Status: "completed"}, {Version: "old", Status: "completed", Content: `{"action":"query","order_id":"ORD-1"}`}} {
		if _, e := Extract(context.Background(), LocalFixture{Model: &llmclient.MockClient{Reply: v.Content}, Status: v.Status, Refusal: v.Refusal, Version: v.Version}, "query"); e == nil {
			t.Fatal("non-success accepted", v)
		}
	}
}
func TestMalformedJSONAndBusinessValues(t *testing.T) {
	cases := []string{`null`, `{}`, `{"action":"query","action":"refund","order_id":"ORD-1"}`, `{"action":"query","\u0061ction":"refund","order_id":"ORD-1"}`, `{"Action":"query","order_id":"ORD-1"}`, `{"action":null,"order_id":"ORD-1"}`, `{"action":"query","order_id":"ORD-1","admin":true}`, `{"action":"delete","order_id":"ORD-1"}`, `{"action":"query","order_id":"../secret"}`, `{"action":"query","order_id":"ORD-1"} {}`, `{"action":"query","order_id":1}`, strings.Repeat(" ", 17000)}
	for _, raw := range cases {
		if _, e := ParseIntent(raw); e == nil {
			t.Fatal("accepted", raw)
		}
	}
}
func TestCanceledExtractionDoesNotReturnIntent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Extract(ctx, LocalFixture{Model: &llmclient.MockClient{Reply: "x"}, Status: "completed", Version: PromptVersion}, "query"); e == nil {
		t.Fatal("canceled success")
	}
}

type resultFixture struct{ v Envelope }

func (f resultFixture) CompleteStructured(context.Context, []llmclient.Message) (Envelope, error) {
	return f.v, nil
}
func TestEmptyResultAndModelCannotSpoofStatus(t *testing.T) {
	for _, v := range []Envelope{{Version: PromptVersion, Status: "completed"}, {Version: PromptVersion, Status: "incomplete", Content: `{"action":"query","order_id":"ORD-1"}`}, {Version: PromptVersion, Status: "completed", Content: `{"status":"completed","action":"refund","order_id":"ORD-1"}`}} {
		if _, e := Extract(context.Background(), resultFixture{v}, "query"); e == nil {
			t.Fatal("accepted unsafe result", v)
		}
	}
}
