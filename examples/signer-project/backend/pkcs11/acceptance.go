package pkcs11backend

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/twodog-tt/Golang-development-manual/examples/signer-project/fence"
)

type AcceptanceOptions struct {
	Concurrency                int
	Signatures                 int
	RequireAttributeEvidence   bool
	RequireExpectedFingerprint bool
}

type AcceptanceReport struct {
	StartedAt             time.Time  `json:"started_at"`
	FinishedAt            time.Time  `json:"finished_at"`
	Inspection            Inspection `json:"inspection"`
	Identity              Identity   `json:"identity"`
	ExistingKeyOnly       bool       `json:"existing_key_only"`
	FingerprintPinned     bool       `json:"fingerprint_pinned"`
	AttributeEvidenceMode string     `json:"attribute_evidence_mode"`
	Concurrency           int        `json:"concurrency"`
	Signatures            int        `json:"signatures"`
	LatencyP50            string     `json:"latency_p50"`
	LatencyP95            string     `json:"latency_p95"`
	ReopenIdentityStable  bool       `json:"reopen_identity_stable"`
	AllSignaturesVerified bool       `json:"all_signatures_verified"`
}

// RunAcceptance validates one existing PKCS#11 P-256 key. It never creates,
// rotates, deletes, wraps, or exports a key. A successful report is application
// path evidence, not a substitute for the vendor's hardware inventory,
// firmware/FIPS validation, operator ceremony, audit export, or HA tests.
func RunAcceptance(
	ctx context.Context,
	config Config,
	options AcceptanceOptions,
) (AcceptanceReport, error) {
	if ctx == nil {
		return AcceptanceReport{}, errors.New("pkcs11 acceptance: context is nil")
	}
	if config.CreateIfMissing {
		return AcceptanceReport{}, errors.New(
			"pkcs11 acceptance: CreateIfMissing must be false",
		)
	}
	if options.RequireExpectedFingerprint && len(config.ExpectedPublicKeySHA256) == 0 {
		return AcceptanceReport{}, errors.New(
			"pkcs11 acceptance: an independently recorded public-key fingerprint is required",
		)
	}
	if options.Concurrency == 0 {
		options.Concurrency = 4
	}
	if options.Signatures == 0 {
		options.Signatures = 16
	}
	if options.Concurrency < 1 || options.Concurrency > 256 {
		return AcceptanceReport{}, errors.New(
			"pkcs11 acceptance: concurrency must be between 1 and 256",
		)
	}
	if options.Signatures < options.Concurrency || options.Signatures > 1_000_000 {
		return AcceptanceReport{}, errors.New(
			"pkcs11 acceptance: signatures must be at least concurrency and at most 1000000",
		)
	}

	report := AcceptanceReport{
		StartedAt:         time.Now().UTC(),
		ExistingKeyOnly:   true,
		FingerprintPinned: len(config.ExpectedPublicKeySHA256) != 0,
		Concurrency:       options.Concurrency,
		Signatures:        options.Signatures,
	}
	if options.RequireAttributeEvidence {
		report.AttributeEvidenceMode = "required"
	} else {
		report.AttributeEvidenceMode = "best-effort"
	}
	inspection, err := Inspect(config)
	if err != nil {
		return AcceptanceReport{}, fmt.Errorf("pkcs11 acceptance: inspect: %w", err)
	}
	if err = inspection.ValidateSecureKey(options.RequireAttributeEvidence); err != nil {
		return AcceptanceReport{}, err
	}
	report.Inspection = inspection

	backend, err := Open(config)
	if err != nil {
		return AcceptanceReport{}, fmt.Errorf("pkcs11 acceptance: open backend: %w", err)
	}
	identity := backend.Identity()
	report.Identity = identity

	latencies, err := exerciseConcurrentSigning(
		ctx,
		backend,
		config.LogicalKeyID,
		identity,
		options.Concurrency,
		options.Signatures,
	)
	closeErr := backend.Close()
	if err != nil {
		return AcceptanceReport{}, err
	}
	if closeErr != nil {
		return AcceptanceReport{}, fmt.Errorf("pkcs11 acceptance: close backend: %w", closeErr)
	}

	reopened, err := Open(config)
	if err != nil {
		return AcceptanceReport{}, fmt.Errorf("pkcs11 acceptance: reopen backend: %w", err)
	}
	reopenedIdentity := reopened.Identity()
	if reopenedIdentity.PublicKeySHA256 != identity.PublicKeySHA256 ||
		!bytes.Equal(reopenedIdentity.PublicKeyDER, identity.PublicKeyDER) {
		_ = reopened.Close()
		return AcceptanceReport{}, fmt.Errorf(
			"%w after reconnect: before=%s after=%s",
			ErrIdentityMismatch,
			identity.PublicKeySHA256,
			reopenedIdentity.PublicKeySHA256,
		)
	}
	reopenDigest := fence.DigestPayload([]byte("pkcs11-hardware-acceptance/reopen/v1"))
	reopenResult, err := reopened.Sign(ctx, config.LogicalKeyID, reopenDigest)
	closeErr = reopened.Close()
	if err != nil {
		return AcceptanceReport{}, fmt.Errorf("pkcs11 acceptance: sign after reopen: %w", err)
	}
	if !VerifyResult(reopenResult, reopenDigest) {
		return AcceptanceReport{}, errors.New(
			"pkcs11 acceptance: invalid signature after reopen",
		)
	}
	if closeErr != nil {
		return AcceptanceReport{}, fmt.Errorf(
			"pkcs11 acceptance: close reopened backend: %w",
			closeErr,
		)
	}

	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})
	report.LatencyP50 = percentile(latencies, 50).String()
	report.LatencyP95 = percentile(latencies, 95).String()
	report.ReopenIdentityStable = true
	report.AllSignaturesVerified = true
	report.FinishedAt = time.Now().UTC()
	return report, nil
}

func exerciseConcurrentSigning(
	ctx context.Context,
	backend *Backend,
	logicalKeyID string,
	identity Identity,
	concurrency int,
	signatures int,
) ([]time.Duration, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var next atomic.Int64
	latencies := make([]time.Duration, signatures)
	firstError := make(chan error, 1)
	var workers sync.WaitGroup
	for worker := 0; worker < concurrency; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for {
				index := int(next.Add(1) - 1)
				if index >= signatures {
					return
				}
				if err := ctx.Err(); err != nil {
					return
				}
				digest := fence.DigestPayload(
					[]byte(fmt.Sprintf("pkcs11-hardware-acceptance/sign/v1/%d", index)),
				)
				started := time.Now()
				result, err := backend.Sign(ctx, logicalKeyID, digest)
				latencies[index] = time.Since(started)
				if err == nil && !VerifyResult(result, digest) {
					err = errors.New("token returned a signature that did not verify")
				}
				if err == nil {
					actual := backend.Identity()
					if actual.PublicKeySHA256 != identity.PublicKeySHA256 {
						err = fmt.Errorf(
							"public key changed during acceptance: expected=%s actual=%s",
							identity.PublicKeySHA256,
							actual.PublicKeySHA256,
						)
					}
				}
				if err != nil {
					select {
					case firstError <- fmt.Errorf("signature %d: %w", index, err):
					default:
					}
					cancel()
					return
				}
			}
		}()
	}
	workers.Wait()
	select {
	case err := <-firstError:
		return nil, fmt.Errorf("pkcs11 acceptance: concurrent signing: %w", err)
	default:
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("pkcs11 acceptance: concurrent signing: %w", err)
	}
	return latencies, nil
}

func percentile(values []time.Duration, percentile int) time.Duration {
	if len(values) == 0 {
		return 0
	}
	index := (len(values)*percentile + 99) / 100
	if index < 1 {
		index = 1
	}
	if index > len(values) {
		index = len(values)
	}
	return values[index-1]
}
