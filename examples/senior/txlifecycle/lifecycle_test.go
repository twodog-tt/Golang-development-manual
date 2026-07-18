package txlifecycle

import (
	"crypto/sha256"
	"sync"
	"testing"
)

func TestSubmitTimeoutAndNotFoundNeverAuthorizeImmediateRebuild(t *testing.T) {
	tracker := newTracker(t, SolanaProfile())
	action, err := tracker.RecordSubmission("rpc-a", SubmitAmbiguous)
	if err != nil || action != ActionQueryOrRebroadcastSameBytes {
		t.Fatalf("submit action=%s err=%v", action, err)
	}
	action, err = tracker.Apply(observation("rpc-b", ObservationNotFound))
	if err != nil || action != ActionQueryOrRebroadcastSameBytes {
		t.Fatalf("not found action=%s err=%v", action, err)
	}
	if state := tracker.Snapshot().State; state != StateUnknown {
		t.Fatalf("state=%s, want UNKNOWN", state)
	}
}

func TestSolanaSuccessRequiresConfiguredBusinessFinality(t *testing.T) {
	tracker := newTracker(t, SolanaProfile())
	_, _ = tracker.RecordSubmission("rpc-a", SubmitAccepted)
	action, err := tracker.Apply(observation("rpc-a", ObservationExecutedSuccess))
	if err != nil || action != ActionQuery || tracker.Snapshot().State != StatePending {
		t.Fatalf("execution action=%s state=%s err=%v", action, tracker.Snapshot().State, err)
	}
	action, err = tracker.Apply(observation("rpc-b", ObservationFinalizedSuccess))
	if err != nil || action != ActionStopSuccess || tracker.Snapshot().State != StateSucceeded {
		t.Fatalf("finality action=%s state=%s err=%v", action, tracker.Snapshot().State, err)
	}
}

func TestCosmosAdmissionIsNotExecution(t *testing.T) {
	tracker := newTracker(t, CosmosProfile())
	action, err := tracker.Apply(observation("grpc-a", ObservationAccepted))
	if err != nil || action != ActionQuery || tracker.Snapshot().State != StatePending {
		t.Fatalf("CheckTx action=%s state=%s err=%v", action, tracker.Snapshot().State, err)
	}
	action, err = tracker.Apply(observation("grpc-a", ObservationFinalizedFailure))
	if err != nil || action != ActionStopFailure || tracker.Snapshot().State != StateFailed {
		t.Fatalf("DeliverTx failure action=%s state=%s err=%v", action, tracker.Snapshot().State, err)
	}
}

func TestPreFinalityFailureDoesNotBypassConfiguredFinality(t *testing.T) {
	tracker := newTracker(t, SolanaProfile())
	action, err := tracker.Apply(observation("rpc-a", ObservationExecutedFailure))
	if err != nil || action != ActionQuery || tracker.Snapshot().State != StatePending {
		t.Fatalf("execution failure action=%s state=%s err=%v", action, tracker.Snapshot().State, err)
	}
	action, err = tracker.Apply(observation("rpc-a", ObservationFinalizedFailure))
	if err != nil || action != ActionStopFailure || tracker.Snapshot().State != StateFailed {
		t.Fatalf("final failure action=%s state=%s err=%v", action, tracker.Snapshot().State, err)
	}
}

func TestSuiCheckpointRequirementIsPolicyControlled(t *testing.T) {
	effectsOnly := newTracker(t, SuiProfile(false))
	action, err := effectsOnly.Apply(observation("grpc-a", ObservationExecutedSuccess))
	if err != nil || action != ActionStopSuccess {
		t.Fatalf("effects policy action=%s err=%v", action, err)
	}

	checkpoint := newTracker(t, SuiProfile(true))
	action, err = checkpoint.Apply(observation("grpc-a", ObservationExecutedSuccess))
	if err != nil || action != ActionQuery || checkpoint.Snapshot().State != StatePending {
		t.Fatalf("checkpoint pending action=%s state=%s err=%v", action, checkpoint.Snapshot().State, err)
	}
	action, err = checkpoint.Apply(observation("graphql-b", ObservationFinalizedSuccess))
	if err != nil || action != ActionStopSuccess {
		t.Fatalf("checkpoint action=%s err=%v", action, err)
	}
}

func TestExpirationRequiresResourceRefreshBeforeRebuild(t *testing.T) {
	tracker := newTracker(t, AptosProfile())
	action, err := tracker.Apply(observation("rest-a", ObservationExpired))
	if err != nil || action != ActionRefreshResourcesAndRebuild || tracker.Snapshot().State != StateExpired {
		t.Fatalf("expiration action=%s state=%s err=%v", action, tracker.Snapshot().State, err)
	}
}

func TestExpiryAfterPreFinalityExecutionEntersManualHold(t *testing.T) {
	tracker := newTracker(t, SolanaProfile())
	if _, err := tracker.Apply(observation("rpc-a", ObservationExecutedSuccess)); err != nil {
		t.Fatal(err)
	}
	action, err := tracker.Apply(observation("rpc-b", ObservationExpired))
	if err != nil || action != ActionManualHold || tracker.Snapshot().State != StateHold {
		t.Fatalf("expiry-after-execution action=%s state=%s err=%v", action, tracker.Snapshot().State, err)
	}
}

func TestInclusionAfterProvenExpirationEntersManualHold(t *testing.T) {
	tracker := newTracker(t, AptosProfile())
	if _, err := tracker.Apply(observation("rest-a", ObservationExpired)); err != nil {
		t.Fatal(err)
	}
	action, err := tracker.Apply(observation("rest-b", ObservationFinalizedSuccess))
	if err != nil || action != ActionManualHold || tracker.Snapshot().State != StateHold {
		t.Fatalf("contradiction action=%s state=%s err=%v", action, tracker.Snapshot().State, err)
	}
}

func TestWeakObservationCannotDowngradeTerminalState(t *testing.T) {
	tracker := newTracker(t, SolanaProfile())
	if _, err := tracker.Apply(observation("rpc-a", ObservationFinalizedFailure)); err != nil {
		t.Fatal(err)
	}
	action, err := tracker.Apply(observation("rpc-b", ObservationExecutedSuccess))
	if err != nil || action != ActionStopFailure || tracker.Snapshot().State != StateFailed {
		t.Fatalf("weak observation action=%s state=%s err=%v", action, tracker.Snapshot().State, err)
	}
}

func TestConflictingTerminalProvidersEnterManualHold(t *testing.T) {
	tracker := newTracker(t, SolanaProfile())
	if action, err := tracker.Apply(observation("rpc-a", ObservationFinalizedSuccess)); err != nil || action != ActionStopSuccess {
		t.Fatalf("success action=%s err=%v", action, err)
	}
	action, err := tracker.Apply(observation("rpc-b", ObservationFinalizedFailure))
	if err != nil || action != ActionManualHold || tracker.Snapshot().State != StateHold {
		t.Fatalf("conflict action=%s state=%s err=%v", action, tracker.Snapshot().State, err)
	}
}

func TestConcurrentProviderDisagreementFailsClosed(t *testing.T) {
	tracker := newTracker(t, AptosProfile())
	observations := []Observation{
		observation("rest-a", ObservationFinalizedSuccess),
		observation("rest-b", ObservationFinalizedFailure),
	}
	var wg sync.WaitGroup
	for _, item := range observations {
		item := item
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := tracker.Apply(item); err != nil {
				t.Errorf("Apply: %v", err)
			}
		}()
	}
	wg.Wait()
	if state := tracker.Snapshot().State; state != StateHold {
		t.Fatalf("state=%s, want MANUAL_HOLD", state)
	}
}

func newTracker(t *testing.T, profile Profile) *Tracker {
	t.Helper()
	tracker, err := New(profile, "tx-42", sha256.Sum256([]byte("signed-transaction-bytes")))
	if err != nil {
		t.Fatal(err)
	}
	return tracker
}

func observation(provider string, kind ObservationKind) Observation {
	return Observation{
		Provider: provider,
		TxID:     "tx-42",
		Kind:     kind,
		Evidence: string(kind) + ":evidence",
	}
}
