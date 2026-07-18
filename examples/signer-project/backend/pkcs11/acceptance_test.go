package pkcs11backend

import (
	"context"
	"testing"
	"time"
)

func TestInspectionValidateSecureKey(t *testing.T) {
	yes := true
	no := false
	inspection := Inspection{
		Mechanism: MechanismEvidence{Supported: true, CanSign: true},
		Key: KeyAttributes{
			Token:            &yes,
			Private:          &yes,
			Sensitive:        &yes,
			AlwaysSensitive:  &yes,
			Extractable:      &no,
			NeverExtractable: &yes,
			CanSign:          &yes,
		},
	}
	if err := inspection.ValidateSecureKey(true); err != nil {
		t.Fatal(err)
	}

	inspection.Key.Extractable = &yes
	if err := inspection.ValidateSecureKey(true); err == nil {
		t.Fatal("accepted an extractable private key")
	}
}

func TestInspectionRequiresEvidenceInStrictMode(t *testing.T) {
	inspection := Inspection{
		Mechanism: MechanismEvidence{Supported: true, CanSign: true},
	}
	if err := inspection.ValidateSecureKey(false); err != nil {
		t.Fatalf("non-strict validation rejected unavailable attributes: %v", err)
	}
	if err := inspection.ValidateSecureKey(true); err == nil {
		t.Fatal("strict validation accepted unavailable attributes")
	}
}

func TestRunAcceptanceNeverCreatesKeys(t *testing.T) {
	_, err := RunAcceptance(context.Background(), Config{
		CreateIfMissing: true,
	}, AcceptanceOptions{})
	if err == nil {
		t.Fatal("RunAcceptance accepted CreateIfMissing")
	}
}

func TestRunAcceptanceCanRequireIndependentFingerprint(t *testing.T) {
	_, err := RunAcceptance(context.Background(), Config{}, AcceptanceOptions{
		RequireExpectedFingerprint: true,
	})
	if err == nil {
		t.Fatal("RunAcceptance accepted an unpinned public key")
	}
}

func TestPercentileNearestRank(t *testing.T) {
	values := []time.Duration{
		time.Millisecond,
		2 * time.Millisecond,
		3 * time.Millisecond,
		4 * time.Millisecond,
	}
	if got := percentile(values, 50); got != 2*time.Millisecond {
		t.Fatalf("p50=%s, want 2ms", got)
	}
	if got := percentile(values, 95); got != 4*time.Millisecond {
		t.Fatalf("p95=%s, want 4ms", got)
	}
}
