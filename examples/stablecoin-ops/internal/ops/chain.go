package ops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func ChainMode() bool { return Env("CHAIN_GATEWAY_URL", "") != "" }

type ChainEvidence struct {
	Reference   string `json:"reference"`
	State       string `json:"state"`
	Amount      string `json:"amount"`
	Input       string `json:"input"`
	Token       string `json:"token"`
	Transaction string `json:"transaction"`
	BlockHash   string `json:"block_hash"`
	Height      int64  `json:"height"`
	Indexed     int64  `json:"indexed"`
	Epoch       int64  `json:"epoch"`
}
type ConversionRequest struct {
	MinOut   string `json:"min_out"`
	Deadline int64  `json:"deadline"`
}
type Conversion struct {
	Payment  string          `json:"payment"`
	Intent   string          `json:"intent"`
	Request  json.RawMessage `json:"request"`
	State    string          `json:"state"`
	Output   *string         `json:"output_atomic"`
	Evidence json.RawMessage `json:"evidence"`
}

func chainCall(ctx context.Context, method, path string, body, out any) error {
	if !ChainMode() {
		return errors.New("chain mode disabled")
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, Env("CHAIN_GATEWAY_URL", "")+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return fmt.Errorf("chain gateway HTTP %d", response.StatusCode)
	}
	if out == nil {
		_, err = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		return err
	}
	return json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(out)
}

// Compare the financial fact, not the moving head/epoch metadata. A replacement
// inclusion is a new observation even when the transaction hash is unchanged.
func sameObservation(a, b ChainEvidence) bool {
	return a.State == b.State && a.Reference == b.Reference && a.Amount == b.Amount && a.Input == b.Input && strings.EqualFold(a.Token, b.Token) && a.Transaction == b.Transaction && a.BlockHash == b.BlockHash && a.Height == b.Height
}
func confirmedDeposit(e ChainEvidence, id, amount string) bool {
	return e.Reference == id && e.State == "confirmed" && e.Amount == amount && strings.EqualFold(e.Token, Env("CHAIN_TOKEN_IN", "")) && e.BlockHash != "" && e.Transaction != "" && e.Height > 0 && e.Indexed-e.Height+1 >= 2
}
func currentDeposit(ctx context.Context, id, amount string) (ChainEvidence, error) {
	if err := chainCall(ctx, "POST", "/index/sync", struct{}{}, nil); err != nil {
		return ChainEvidence{}, err
	}
	e, err := proof(ctx, "deposits", id)
	if err == nil && !confirmedDeposit(e, id, amount) {
		err = ErrConflict
	}
	return e, err
}

// All authorization and quarantine paths take the merchant lock before payment
// row locks. Another invoice cannot start a spend while reorg review commits.
func lockMerchant(ctx context.Context, tx pgx.Tx, id string) error {
	// Operator recovery takes the exclusive maintenance gate before either DB's
	// writer locks. All online financial paths take its shared side first.
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock_shared(31337009)"); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(merchant,31337)) FROM ops.payments WHERE id=$1", id)
	return err
}
func merchantBlocked(ctx context.Context, tx pgx.Tx, merchant string) (bool, error) {
	var blocked bool
	err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM ops.payments WHERE merchant=$1 AND state='review_chain')", merchant).Scan(&blocked)
	return blocked, err
}
func proof(ctx context.Context, kind, id string) (ChainEvidence, error) {
	var out ChainEvidence
	err := chainCall(ctx, "GET", "/evidence/"+kind+"/"+id, nil, &out)
	return out, err
}
func (s *Store) ChainEvent(ctx context.Context, id string) (Event, error) {
	if err := chainCall(ctx, "POST", "/index/sync", struct{}{}, nil); err != nil {
		return Event{}, err
	}
	evidence, err := proof(ctx, "deposits", id)
	if err != nil {
		return Event{}, err
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Event{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock_shared(31337009)"); err != nil {
		return Event{}, err
	}
	var e Event
	var n int64
	var previous string
	var stored []byte
	e.Payment = id
	if err = tx.QueryRow(ctx, "SELECT amount,asset,state,sequence,evidence FROM ops.channel WHERE id=$1 FOR UPDATE", id).Scan(&n, &e.Asset, &e.State, &e.Sequence, &stored); err != nil {
		return e, err
	}
	e.Amount = strconv.FormatInt(n, 10)
	previous = e.State
	if evidence.State == "confirmed" {
		if evidence.Amount != e.Amount || !strings.EqualFold(evidence.Token, Env("CHAIN_TOKEN_IN", "")) {
			return e, ErrConflict
		}
		if e.State != "reversed" {
			e.State = "confirmed"
		}
	} else if evidence.State == "orphaned" {
		e.State = "reversed"
	} else if e.State == "confirmed" {
		return e, ErrConflict
	}
	raw, _ := json.Marshal(evidence)
	// A state sequence identifies immutable event content. Moving indexer head
	// metadata alone must not mutate a previously delivered event's payload.
	if previous == e.State && len(stored) > 0 {
		raw = stored
	}
	if err = json.Unmarshal(raw, &evidence); err != nil {
		return e, err
	}
	e.Evidence = &evidence
	if err = tx.QueryRow(ctx, `UPDATE ops.channel SET sequence=sequence+CASE WHEN state<>$2 THEN 1 ELSE 0 END,state=$2,evidence=$3 WHERE id=$1 RETURNING sequence`, id, e.State, raw).Scan(&e.Sequence); err != nil {
		return e, err
	}
	e.ID = "chain:" + id + ":" + strconv.FormatInt(e.Sequence, 10)
	return e, tx.Commit(ctx)
}

func (s *Store) Conversion(ctx context.Context, merchant, id string) (Conversion, error) {
	var out Conversion
	err := s.DB.QueryRow(ctx, `SELECT c.payment,c.intent,c.request,c.state,c.output::text,c.evidence FROM ops.conversions c JOIN ops.payments p ON p.id=c.payment WHERE p.id=$1 AND p.merchant=$2`, id, merchant).Scan(&out.Payment, &out.Intent, &out.Request, &out.State, &out.Output, &out.Evidence)
	return out, err
}
func (s *Store) CreateConversion(ctx context.Context, merchant, id string, request ConversionRequest) error {
	if !ChainMode() {
		return ErrConflict
	}
	if _, err := Amount(request.MinOut); err != nil {
		return err
	}
	raw, _ := json.Marshal(request)
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = lockMerchant(ctx, tx, id); err != nil {
		return err
	}

	var state string
	var amount int64
	if err = tx.QueryRow(ctx, "SELECT state,amount FROM ops.payments WHERE id=$1 AND merchant=$2 FOR UPDATE", id, merchant).Scan(&state, &amount); err != nil {
		return err
	}
	if state != "settled" {
		return ErrConflict
	}
	if blocked, e := merchantBlocked(ctx, tx, merchant); e != nil {
		return e
	} else if blocked {
		return ErrConflict
	}
	var same bool
	err = tx.QueryRow(ctx, "SELECT request=$2::jsonb FROM ops.conversions WHERE payment=$1", id, raw).Scan(&same)
	if err == nil {
		if !same {
			return ErrConflict
		}
		return tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if request.Deadline <= time.Now().Unix() {
		return errors.New("expired conversion")
	}
	if _, err = tx.Exec(ctx, "INSERT INTO lab.accounts VALUES($1,$2,0,0)", "held:"+id, LedgerAsset()); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO ops.conversions(payment,intent,request) VALUES($1,$2,$3)", id, "convert_"+id, raw); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "SELECT lab.transfer($1,$2,$3,$4,$5)", "hold:"+id, merchant, "held:"+id, LedgerAsset(), amount); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (a *Activities) SubmitConversion(ctx context.Context, id string) error {
	// Serialize local authorization with reorg quarantine. The gateway still
	// persists signed bytes in its own database before any broadcast.
	tx, err := a.Store.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = lockMerchant(ctx, tx, id); err != nil {
		return err
	}

	var amount, merchant, pstate string
	var stored, request []byte
	var intent, state string
	if err = tx.QueryRow(ctx, `SELECT amount::text,merchant,state,settlement_evidence FROM ops.payments WHERE id=$1 FOR UPDATE`, id).Scan(&amount, &merchant, &pstate, &stored); err != nil {
		return err
	}
	if err = tx.QueryRow(ctx, `SELECT request,intent,state FROM ops.conversions WHERE payment=$1 FOR UPDATE`, id).Scan(&request, &intent, &state); err != nil {
		return err
	}
	if pstate != "settled" {
		return ErrConflict
	}
	if blocked, e := merchantBlocked(ctx, tx, merchant); e != nil {
		return e
	} else if blocked {
		return ErrConflict
	}
	if state != "held" && state != "review" {
		return tx.Commit(ctx)
	}
	deposit, err := currentDeposit(ctx, id, amount)
	if err != nil {
		return err
	}
	var original ChainEvidence
	if json.Unmarshal(stored, &original) != nil || !sameObservation(original, deposit) {
		return ErrConflict
	}
	var r ConversionRequest
	if err = json.Unmarshal(request, &r); err != nil {
		return err
	}
	if err = chainCall(ctx, "POST", "/intents/"+intent, map[string]any{"amount": amount, "min_out": r.MinOut, "deadline": r.Deadline}, nil); err != nil {
		return err
	}
	evidence, err := proof(ctx, "executions", intent)
	if err != nil {
		return err
	}
	if evidence.State == "pending" {
		if err = chainCall(ctx, "POST", "/intents/"+intent+"/broadcast", struct{}{}, nil); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, "UPDATE ops.conversions SET state='executing' WHERE payment=$1 AND state='held'", id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (a *Activities) ObserveConversion(ctx context.Context, id string) (ChainEvidence, error) {
	if err := chainCall(ctx, "POST", "/index/sync", struct{}{}, nil); err != nil {
		return ChainEvidence{}, err
	}
	return proof(ctx, "executions", "convert_"+id)
}

func (a *Activities) FinishConversion(ctx context.Context, id string, e ChainEvidence) error {
	tx, err := a.Store.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = lockMerchant(ctx, tx, id); err != nil {
		return err
	}

	var paymentState, merchant, state string
	var amount int64
	var raw, settled []byte
	if err = tx.QueryRow(ctx, "SELECT state,merchant,amount,settlement_evidence FROM ops.payments WHERE id=$1 FOR UPDATE", id).Scan(&paymentState, &merchant, &amount, &settled); err != nil {
		return err
	}
	if err = tx.QueryRow(ctx, "SELECT state,request FROM ops.conversions WHERE payment=$1 FOR UPDATE", id).Scan(&state, &raw); err != nil {
		return err
	}
	// Temporal retries carry the original Activity argument; it may now be an
	// orphan. Re-query the gateway before posting or accepting an idempotent retry.
	fresh, err := a.ObserveConversion(ctx, id)
	if err != nil {
		return err
	}
	if !sameObservation(e, fresh) || fresh.Reference != "convert_"+id || fresh.Height <= 0 || fresh.Indexed-fresh.Height+1 < 2 || fresh.BlockHash == "" || fresh.Transaction == "" || !strings.EqualFold(fresh.Token, Env("CHAIN_TOKEN_OUT", "")) {
		return ErrConflict
	}
	deposit, err := currentDeposit(ctx, id, strconv.FormatInt(amount, 10))
	if err != nil {
		return err
	}
	var originalDeposit ChainEvidence
	if json.Unmarshal(settled, &originalDeposit) != nil || !sameObservation(originalDeposit, deposit) || fresh.Epoch != deposit.Epoch {
		return ErrConflict
	}
	if state == "settled" || state == "released" {
		var original []byte
		if err = tx.QueryRow(ctx, "SELECT evidence FROM ops.conversions WHERE payment=$1", id).Scan(&original); err != nil {
			return err
		}
		var old ChainEvidence
		if json.Unmarshal(original, &old) != nil || !sameObservation(old, fresh) {
			return ErrConflict
		}
		return tx.Commit(ctx)
	}
	if paymentState != "settled" || (state != "held" && state != "executing" && state != "review") {
		return ErrConflict
	}
	evidence, _ := json.Marshal(e)
	if e.State == "reverted" {
		if _, err = tx.Exec(ctx, "SELECT lab.reverse_transfer($1,$2)", "release:"+id, "hold:"+id); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "UPDATE ops.conversions SET state='released',evidence=$2 WHERE payment=$1", id, evidence)
	} else {
		output, errAmount := Amount(e.Amount)
		if errAmount != nil {
			return errAmount
		}
		var request ConversionRequest
		if err = json.Unmarshal(raw, &request); err != nil {
			return err
		}
		min, _ := Amount(request.MinOut)
		if e.State != "confirmed" || e.Input != strconv.FormatInt(amount, 10) || output < min || !strings.EqualFold(e.Token, Env("CHAIN_TOKEN_OUT", "")) || e.Indexed-e.Height+1 < 2 {
			return ErrConflict
		}
		if _, err = tx.Exec(ctx, "SELECT lab.transfer($1,$2,'dex_input',$3,$4)", "swap-in:"+id, "held:"+id, LedgerAsset(), amount); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "SELECT lab.transfer($1,'dex_output',$2,$3,$4)", "swap-out:"+id, merchant+":out", Env("OUTPUT_LEDGER_ASSET", "LABDAI"), output); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "UPDATE ops.conversions SET state='settled',output=$2,evidence=$3 WHERE payment=$1", id, output, evidence)
	}
	if err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	if a.Store.Fault(ctx, "conversion_commit_response_lost") {
		return errors.New("fixture: conversion COMMIT response lost")
	}
	return nil
}

func (a *Activities) ReviewConversion(ctx context.Context, id string) error {
	_, err := a.Store.DB.Exec(ctx, "UPDATE ops.conversions SET state='review' WHERE payment=$1 AND state IN ('held','executing')", id)
	return err
}

func ConversionWorkflow(ctx workflow.Context, id string) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 15 * time.Second, RetryPolicy: &temporal.RetryPolicy{InitialInterval: time.Second, MaximumAttempts: 3}})
	// A failed send is ambiguous. Observation may still recover an existing hash.
	_ = workflow.ExecuteActivity(ctx, "SubmitConversion", id).Get(ctx, nil)
	for i := 0; i < 60; i++ {
		var e ChainEvidence
		if err := workflow.ExecuteActivity(ctx, "ObserveConversion", id).Get(ctx, &e); err == nil {
			if e.State == "confirmed" || e.State == "reverted" {
				if err = workflow.ExecuteActivity(ctx, "FinishConversion", id, e).Get(ctx, nil); err != nil {
					return workflow.ExecuteActivity(ctx, "ReviewConversion", id).Get(ctx, nil)
				}
				prefixes := []string{"hold:", "swap-in:", "swap-out:"}
				if e.State == "reverted" {
					prefixes = []string{"hold:", "release:"}
				}
				for _, prefix := range prefixes {
					if err = workflow.ExecuteActivity(ctx, "Notify", prefix+id).Get(ctx, nil); err != nil {
						return err
					}
				}
				return nil
			}
			if e.State == "orphaned" {
				break
			}
		}
		if err := workflow.Sleep(ctx, time.Second); err != nil {
			return err
		}
	}
	return workflow.ExecuteActivity(ctx, "ReviewConversion", id).Get(ctx, nil)
}

// Audit reads existing observations only. It never submits a swap, refunds or
// mints a new nonce. The row lock serializes compensation with settlement.
func (a *Activities) auditChain(ctx context.Context, id string) (string, error) {
	tx, err := a.Store.DB.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if err = lockMerchant(ctx, tx, id); err != nil {
		return "", err
	}

	var pstate, amount string
	var settled []byte
	if err = tx.QueryRow(ctx, "SELECT state,amount::text,settlement_evidence FROM ops.payments WHERE id=$1 FOR UPDATE", id).Scan(&pstate, &amount, &settled); err != nil {
		return "", err
	}
	if pstate != "settled" {
		return pstate, nil
	}
	var cstate string
	var previous []byte
	err = tx.QueryRow(ctx, "SELECT state,evidence FROM ops.conversions WHERE payment=$1 FOR UPDATE", id).Scan(&cstate, &previous)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	// Read external evidence after locking the local financial phase. Otherwise a
	// pre-lock pending proof could quarantine a conversion that just committed.
	hasConversion := cstate != ""
	if err = chainCall(ctx, "POST", "/index/sync", struct{}{}, nil); err != nil {
		return "", err
	}
	deposit, err := proof(ctx, "deposits", id)
	if err != nil {
		return "", err
	}
	var execution ChainEvidence
	var executionErr error
	if hasConversion {
		execution, executionErr = proof(ctx, "executions", "convert_"+id)
	}
	var original ChainEvidence
	if json.Unmarshal(settled, &original) != nil {
		return "", ErrConflict
	}
	observations := map[string]any{"deposit": deposit, "execution": execution, "execution_error": fmt.Sprint(executionErr)}
	raw, _ := json.Marshal(observations)
	quarantine := func() (string, error) {
		if _, err = tx.Exec(ctx, "UPDATE ops.payments SET state='review_chain',audit_evidence=$2 WHERE id=$1", id, raw); err != nil {
			return "", err
		}
		if _, err = tx.Exec(ctx, "UPDATE ops.conversions SET audit_evidence=$2 WHERE payment=$1", id, raw); err != nil {
			return "", err
		}
		return "review_chain", tx.Commit(ctx)
	}
	depositChanged := !confirmedDeposit(deposit, id, amount) || !sameObservation(original, deposit)
	// Missing intent, pending replacement and mixed epochs cannot imply returned
	// money. They must not hide an already detected loss of the deposit either.
	if depositChanged && (deposit.State != "orphaned" || (cstate != "" && executionErr != nil)) {
		return quarantine()
	}
	if executionErr != nil {
		return "", executionErr
	}
	if hasConversion && deposit.Epoch != execution.Epoch {
		return "", ErrConflict
	}
	if cstate == "recovered_cancelled" {
		var old ChainEvidence
		if json.Unmarshal(previous, &old) != nil || !sameObservation(old, execution) || (execution.State != "cancelled" && execution.State != "retired") {
			return quarantine()
		}
	}
	if cstate == "settled" || cstate == "released" {
		var old ChainEvidence
		if json.Unmarshal(previous, &old) != nil {
			return "", ErrConflict
		}
		if execution.State != "orphaned" && !sameObservation(old, execution) {
			return quarantine()
		}
		// A failure receipt that was compensated is no longer authoritative after a
		// reorg. Do not release again, retry the spend, or resurrect the old invoice.
		if cstate == "released" && execution.State == "orphaned" {
			return quarantine()
		}
	}
	if deposit.State == "orphaned" || execution.State == "orphaned" {
		if cstate == "settled" && execution.State != "orphaned" {
			return quarantine()
		}
		if cstate != "" && cstate != "settled" && cstate != "released" && cstate != "review_reversed" {
			return quarantine()
		}
		// Failed compensation (e.g. consumed output) rolls back every financial
		// change, then records a durable quarantine in the same outer transaction.
		if _, err = tx.Exec(ctx, "SAVEPOINT compensation"); err != nil {
			return "", err
		}
		compensate := func() error {
			reverse := func(journal, original string) error {
				_, e := tx.Exec(ctx, "SELECT lab.reverse_transfer($1,$2)", journal, original)
				return e
			}
			if cstate == "settled" {
				for _, pair := range [][2]string{{"undo-out:", "swap-out:"}, {"undo-in:", "swap-in:"}, {"release:", "hold:"}} {
					if e := reverse(pair[0]+id, pair[1]+id); e != nil {
						return e
					}
				}
				if _, e := tx.Exec(ctx, "UPDATE ops.conversions SET state='review_reversed',audit_evidence=$2 WHERE payment=$1", id, raw); e != nil {
					return e
				}
			}
			if deposit.State == "orphaned" {
				if e := reverse("reorg:"+id, "settle:"+id); e != nil {
					return e
				}
				if _, e := tx.Exec(ctx, "UPDATE ops.payments SET state='review_reversed',provider_state='reversed',sequence=sequence+1,audit_evidence=$2 WHERE id=$1", id, raw); e != nil {
					return e
				}
				pstate = "review_reversed"
			} else {
				// The deposit survived but a signed swap may reappear. Keep this merchant
				// quarantined rather than allowing another spend of the returned input.
				if _, e := tx.Exec(ctx, "UPDATE ops.payments SET state='review_chain',audit_evidence=$2 WHERE id=$1", id, raw); e != nil {
					return e
				}
				pstate = "review_chain"
			}
			return nil
		}
		if err = compensate(); err != nil {
			observations["compensation_error"] = err.Error()
			raw, _ = json.Marshal(observations)
			if _, rollbackErr := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT compensation"); rollbackErr != nil {
				return "", rollbackErr
			}
			return quarantine()
		}
	}
	return pstate, tx.Commit(ctx)
}

func (a *Activities) AuditChain(ctx context.Context, id string) (string, error) {
	if a.Store.Fault(ctx, "chain_audit_unavailable") {
		return "", errors.New("fixture: chain audit dependency unavailable")
	}
	state, err := a.auditChain(ctx, id)
	if err != nil {
		return state, err
	}
	return state, a.DeliverChainEvents(ctx, id)
}
func (a *Activities) DeliverChainEvents(ctx context.Context, id string) error {
	// Drain durable state after every audit, including retries after compensation
	// already committed. Merchant delivery does not alter financial facts.
	ids := []string{}
	for _, prefix := range []string{"settle:", "hold:", "swap-in:", "swap-out:", "release:", "undo-out:", "undo-in:", "reorg:"} {
		ids = append(ids, prefix+id)
	}
	rows, err := a.Store.DB.Query(ctx, "SELECT event_id FROM lab.outbox WHERE NOT published AND event_id=ANY($1::text[]) ORDER BY created_at,event_id", ids)
	if err != nil {
		return err
	}
	var pending []string
	for rows.Next() {
		var event string
		if err = rows.Scan(&event); err != nil {
			rows.Close()
			return err
		}
		pending = append(pending, event)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, event := range pending {
		if err = a.Notify(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

type ChainInput struct {
	Input   Input
	Monitor bool
}

func ChainReconcileWorkflow(ctx workflow.Context, p Payment) error {
	v := workflow.GetVersion(ctx, "chain-reconcile-after-notify", workflow.DefaultVersion, 1)
	if err := reconcileWorkflow(ctx, p, v != workflow.DefaultVersion); err != nil {
		return err
	}
	return ChainPaymentWorkflow(ctx, ChainInput{Input: Input{Payment: p}, Monitor: true})
}

func ChainPaymentWorkflow(ctx workflow.Context, in ChainInput) error {
	if !in.Monitor {
		// Old histories keep their command path. New executions continue the
		// financial audit even after notification retries have exhausted.
		v := workflow.GetVersion(ctx, "chain-monitor-after-notify", workflow.DefaultVersion, 1)
		if err := runPaymentWorkflow(ctx, in.Input, false, v != workflow.DefaultVersion); err != nil {
			return err
		}
	}
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 15 * time.Second, RetryPolicy: &temporal.RetryPolicy{InitialInterval: time.Second, MaximumAttempts: 3}})
	for i := 0; i < 50; i++ {
		var state string
		if err := workflow.ExecuteActivity(ctx, "AuditChain", in.Input.Payment.ID).Get(ctx, &state); err != nil {
			// Node/indexer outages retain existing facts, never imply a reversal.
		} else if state != "settled" {
			return nil
		}
		if err := workflow.Sleep(ctx, time.Second); err != nil {
			return err
		}
	}
	in.Monitor = true
	return workflow.NewContinueAsNewError(ctx, ChainPaymentWorkflow, in)
}
