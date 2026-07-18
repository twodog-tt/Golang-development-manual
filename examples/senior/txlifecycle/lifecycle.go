// Package txlifecycle demonstrates a chain-agnostic transaction lifecycle
// boundary for version-pinned Solana, Cosmos, Aptos, and Sui adapters.
//
// Adapters normalize protocol evidence into observations; they must not erase
// chain-specific meanings such as CheckTx, commitment, effects, checkpoints,
// sequence consumption, or expiration. This package contains no network client.
package txlifecycle

import (
	"errors"
	"fmt"
	"sync"
)

var (
	ErrInvalidProfile     = errors.New("txlifecycle: invalid profile")
	ErrInvalidTransaction = errors.New("txlifecycle: transaction ID and signed digest are required")
	ErrInvalidObservation = errors.New("txlifecycle: invalid observation")
)

type Chain string

const (
	ChainSolana Chain = "solana"
	ChainCosmos Chain = "cosmos"
	ChainAptos  Chain = "aptos"
	ChainSui    Chain = "sui"
)

// Stage is deliberately semantic rather than a block-count threshold. A
// version-pinned adapter decides which chain evidence satisfies each stage.
type Stage uint8

const (
	StageExecution Stage = iota + 1
	StageBusinessFinality
)

type Profile struct {
	Chain                 Chain
	RequiredTerminalStage Stage
	Description           string
}

func SolanaProfile() Profile {
	return Profile{
		Chain:                 ChainSolana,
		RequiredTerminalStage: StageBusinessFinality,
		Description:           "adapter emits terminal success/failure only at the configured confirmed/finalized commitment",
	}
}

func CosmosProfile() Profile {
	return Profile{
		Chain:                 ChainCosmos,
		RequiredTerminalStage: StageBusinessFinality,
		Description:           "CheckTx admission is not DeliverTx/commit success or failure",
	}
}

func AptosProfile() Profile {
	return Profile{
		Chain:                 ChainAptos,
		RequiredTerminalStage: StageBusinessFinality,
		Description:           "submission is not a committed transaction result",
	}
}

func SuiProfile(requireCheckpoint bool) Profile {
	required := StageExecution
	description := "successful transaction effects satisfy this business policy"
	if requireCheckpoint {
		required = StageBusinessFinality
		description = "adapter waits for the configured checkpoint/evidence boundary"
	}
	return Profile{Chain: ChainSui, RequiredTerminalStage: required, Description: description}
}

type State string

const (
	StateSigned    State = "SIGNED"
	StateUnknown   State = "UNKNOWN"
	StatePending   State = "PENDING"
	StateSucceeded State = "SUCCEEDED"
	StateFailed    State = "FAILED"
	StateExpired   State = "EXPIRED"
	StateHold      State = "MANUAL_HOLD"
)

type Action string

const (
	ActionQuery                       Action = "QUERY"
	ActionQueryOrRebroadcastSameBytes Action = "QUERY_OR_REBROADCAST_SAME_BYTES"
	ActionStopSuccess                 Action = "STOP_SUCCESS"
	ActionStopFailure                 Action = "STOP_FAILURE"
	ActionRefreshResourcesAndRebuild  Action = "REFRESH_RESOURCES_AND_REBUILD"
	ActionManualHold                  Action = "MANUAL_HOLD"
)

type SubmitOutcome string

const (
	SubmitAccepted  SubmitOutcome = "ACCEPTED"
	SubmitAmbiguous SubmitOutcome = "AMBIGUOUS"
)

type ObservationKind string

const (
	ObservationNotFound         ObservationKind = "NOT_FOUND"
	ObservationAccepted         ObservationKind = "ACCEPTED"
	ObservationExecutedSuccess  ObservationKind = "EXECUTED_SUCCESS"
	ObservationExecutedFailure  ObservationKind = "EXECUTED_FAILURE"
	ObservationFinalizedSuccess ObservationKind = "FINALIZED_SUCCESS"
	ObservationFinalizedFailure ObservationKind = "FINALIZED_FAILURE"
	ObservationExpired          ObservationKind = "EXPIRED"
)

type Observation struct {
	Provider string
	TxID     string
	Kind     ObservationKind
	Evidence string
}

type terminalClaim uint8

const (
	claimSuccess terminalClaim = iota + 1
	claimFailure
)

type Snapshot struct {
	Chain               Chain
	TxID                string
	SignedDigest        [32]byte
	State               State
	SawExecution        bool
	SawExecutionSuccess bool
	ObservationCount    uint64
}

type Tracker struct {
	mu sync.Mutex

	profile             Profile
	txID                string
	signedDigest        [32]byte
	state               State
	sawExecution        bool
	sawExecutionSuccess bool
	observationCount    uint64
	claims              map[string]terminalClaim
}

func New(profile Profile, txID string, signedDigest [32]byte) (*Tracker, error) {
	if !validChain(profile.Chain) || (profile.RequiredTerminalStage != StageExecution &&
		profile.RequiredTerminalStage != StageBusinessFinality) {
		return nil, ErrInvalidProfile
	}
	if txID == "" || signedDigest == [32]byte{} {
		return nil, ErrInvalidTransaction
	}
	return &Tracker{
		profile:      profile,
		txID:         txID,
		signedDigest: signedDigest,
		state:        StateSigned,
		claims:       make(map[string]terminalClaim),
	}, nil
}

// RecordSubmission separates an RPC acknowledgement from an ambiguous timeout.
// Neither outcome proves execution success.
func (t *Tracker) RecordSubmission(provider string, outcome SubmitOutcome) (Action, error) {
	if provider == "" || (outcome != SubmitAccepted && outcome != SubmitAmbiguous) {
		return "", ErrInvalidObservation
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if action, terminal := t.terminalAction(); terminal {
		return action, nil
	}
	switch outcome {
	case SubmitAccepted:
		t.state = StatePending
		return ActionQuery, nil
	case SubmitAmbiguous:
		t.state = StateUnknown
		return ActionQueryOrRebroadcastSameBytes, nil
	default:
		panic("validated submit outcome")
	}
}

// Apply consumes normalized evidence. NOT_FOUND never authorizes rebuilding;
// only an adapter-proven expiration can move the tracker to EXPIRED.
func (t *Tracker) Apply(observation Observation) (Action, error) {
	if observation.Provider == "" || observation.TxID != t.txID || observation.Evidence == "" ||
		!validKind(observation.Kind) {
		return "", ErrInvalidObservation
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.observationCount++

	if t.state == StateHold {
		return ActionManualHold, nil
	}
	if t.state == StateExpired {
		switch observation.Kind {
		case ObservationNotFound, ObservationExpired:
			return ActionRefreshResourcesAndRebuild, nil
		default:
			// A proven expiration already authorized rebuilding with fresh
			// resources. Later inclusion/admission evidence is contradictory
			// and creates duplicate-effect risk, so fail closed.
			t.state = StateHold
			return ActionManualHold, nil
		}
	}
	if t.state == StateSucceeded || t.state == StateFailed {
		switch observation.Kind {
		case ObservationFinalizedSuccess:
			return t.recordClaim(observation.Provider, claimSuccess)
		case ObservationFinalizedFailure:
			return t.recordClaim(observation.Provider, claimFailure)
		case ObservationExecutedSuccess:
			if t.profile.RequiredTerminalStage == StageExecution {
				return t.recordClaim(observation.Provider, claimSuccess)
			}
		case ObservationExecutedFailure:
			if t.profile.RequiredTerminalStage == StageExecution {
				return t.recordClaim(observation.Provider, claimFailure)
			}
		}
		return t.terminalActionValue(), nil
	}
	switch observation.Kind {
	case ObservationNotFound:
		if action, terminal := t.terminalAction(); terminal {
			return action, nil
		}
		if t.state == StateSigned {
			t.state = StateUnknown
		}
		return ActionQueryOrRebroadcastSameBytes, nil

	case ObservationAccepted:
		if action, terminal := t.terminalAction(); terminal {
			return action, nil
		}
		t.state = StatePending
		return ActionQuery, nil

	case ObservationExecutedSuccess:
		t.sawExecution = true
		t.sawExecutionSuccess = true
		if t.profile.RequiredTerminalStage == StageExecution {
			return t.recordClaim(observation.Provider, claimSuccess)
		}
		t.state = StatePending
		return ActionQuery, nil

	case ObservationFinalizedSuccess:
		t.sawExecution = true
		t.sawExecutionSuccess = true
		return t.recordClaim(observation.Provider, claimSuccess)

	case ObservationExecutedFailure:
		t.sawExecution = true
		if t.profile.RequiredTerminalStage == StageBusinessFinality {
			t.state = StatePending
			return ActionQuery, nil
		}
		return t.recordClaim(observation.Provider, claimFailure)

	case ObservationFinalizedFailure:
		t.sawExecution = true
		return t.recordClaim(observation.Provider, claimFailure)

	case ObservationExpired:
		if action, terminal := t.terminalAction(); terminal {
			return action, nil
		}
		// A below-finality execution followed by expiry may be a fork
		// rollback or contradictory provider evidence. The generic tracker
		// cannot prove duplicate safety, so require chain-specific review.
		if t.sawExecution {
			t.state = StateHold
			return ActionManualHold, nil
		}
		t.state = StateExpired
		return ActionRefreshResourcesAndRebuild, nil

	default:
		panic("validated observation kind")
	}
}

func (t *Tracker) Snapshot() Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	return Snapshot{
		Chain:               t.profile.Chain,
		TxID:                t.txID,
		SignedDigest:        t.signedDigest,
		State:               t.state,
		SawExecution:        t.sawExecution,
		SawExecutionSuccess: t.sawExecutionSuccess,
		ObservationCount:    t.observationCount,
	}
}

func (t *Tracker) recordClaim(provider string, claim terminalClaim) (Action, error) {
	if previous, exists := t.claims[provider]; exists && previous != claim {
		t.state = StateHold
		return ActionManualHold, nil
	}
	t.claims[provider] = claim

	var success, failure bool
	for _, current := range t.claims {
		success = success || current == claimSuccess
		failure = failure || current == claimFailure
	}
	if success && failure {
		t.state = StateHold
		return ActionManualHold, nil
	}
	if success {
		t.state = StateSucceeded
		return ActionStopSuccess, nil
	}
	if failure {
		t.state = StateFailed
		return ActionStopFailure, nil
	}
	return "", fmt.Errorf("txlifecycle: unreachable empty claim set")
}

func (t *Tracker) terminalAction() (Action, bool) {
	switch t.state {
	case StateSucceeded:
		return ActionStopSuccess, true
	case StateFailed:
		return ActionStopFailure, true
	case StateHold:
		return ActionManualHold, true
	case StateExpired:
		return ActionRefreshResourcesAndRebuild, true
	default:
		return "", false
	}
}

func (t *Tracker) terminalActionValue() Action {
	action, terminal := t.terminalAction()
	if !terminal {
		panic("txlifecycle: expected terminal state")
	}
	return action
}

func validChain(chain Chain) bool {
	switch chain {
	case ChainSolana, ChainCosmos, ChainAptos, ChainSui:
		return true
	default:
		return false
	}
}

func validKind(kind ObservationKind) bool {
	switch kind {
	case ObservationNotFound,
		ObservationAccepted,
		ObservationExecutedSuccess,
		ObservationExecutedFailure,
		ObservationFinalizedSuccess,
		ObservationFinalizedFailure,
		ObservationExpired:
		return true
	default:
		return false
	}
}
