package ops

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// Invoked by the localnet harness against its fresh database. This calls the
// actual Activities without adding privileged fault endpoints to the HTTP API.
func TestChainReviewIntegration(t *testing.T) {
	mode := os.Getenv("LAB_CHAIN_REVIEW_CASE")
	if mode == "" {
		t.Skip("requires isolated payment-e2e environment")
	}
	s := Open()
	defer s.DB.Close()
	a := &Activities{Store: s, Channel: Channel{URL: os.Getenv("CHANNEL_URL"), Token: os.Getenv("LAB_CHANNEL_SECRET")}}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	id := os.Getenv("LAB_PAYMENT_ID")
	switch mode {
	case "create-hold":
		var r ConversionRequest
		if err := json.Unmarshal([]byte(os.Getenv("LAB_CONVERSION_REQUEST")), &r); err != nil {
			t.Fatal(err)
		}
		if err := s.CreateConversion(ctx, os.Getenv("LAB_MERCHANT"), id, r); err != nil {
			t.Fatal(err)
		}
	case "observe":
		state, err := a.Observe(ctx, id)
		if err != nil || state != "confirmed" {
			t.Fatalf("observe: %s %v", state, err)
		}
	case "settle-rejected":
		if err := s.Settle(ctx, id); err == nil {
			t.Fatal("settled using stale deposit evidence")
		}
		var n int
		if err := s.DB.QueryRow(ctx, "SELECT count(*) FROM lab.journals WHERE id=$1", "settle:"+id).Scan(&n); err != nil || n != 0 {
			t.Fatalf("stale deposit credited: %d %v", n, err)
		}
	case "stale-finish":
		var e ChainEvidence
		if err := json.Unmarshal([]byte(os.Getenv("LAB_OLD_EVIDENCE")), &e); err != nil {
			t.Fatal(err)
		}
		if err := a.FinishConversion(ctx, id, e); err == nil {
			t.Fatal("accepted stale Activity evidence")
		}
	case "submit-rejected":
		if err := a.SubmitConversion(ctx, id); err == nil {
			t.Fatal("quarantined merchant submitted a conversion")
		}
	case "audit":
		var before, after int
		if err := s.DB.QueryRow(ctx, "SELECT count(*) FROM lab.journals").Scan(&before); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			state, err := a.AuditChain(ctx, id)
			if err != nil || state != os.Getenv("LAB_EXPECT_STATE") {
				t.Fatalf("audit: %s %v", state, err)
			}
			if err = s.DB.QueryRow(ctx, "SELECT count(*) FROM lab.journals").Scan(&after); err != nil {
				t.Fatal(err)
			}
			if i == 1 && after != before {
				t.Fatal("retry posted another journal")
			}
			before = after
		}
	default:
		t.Fatalf("unknown integration case %q", mode)
	}
}
