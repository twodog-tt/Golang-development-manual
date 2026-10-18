package ops

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const Queue = "stablecoin-ops-v1"

// Recovery only queries existing external facts; it never creates a new transfer.
func ReconcileWorkflow(ctx workflow.Context, p Payment) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Second, RetryPolicy: &temporal.RetryPolicy{InitialInterval: time.Second, MaximumAttempts: 3}})
	var state string
	if err := workflow.ExecuteActivity(ctx, "Observe", p.ID).Get(ctx, &state); err != nil {
		return err
	}
	if state == "confirmed" {
		if err := workflow.ExecuteActivity(ctx, "Settle", p.ID).Get(ctx, nil); err != nil {
			return err
		}
		return workflow.ExecuteActivity(ctx, "Notify", "settle:"+p.ID).Get(ctx, nil)
	}
	return nil
}

type Input struct {
	Payment         Payment
	ApprovalSeconds int
	ObserveSeconds  int
}
type Activities struct {
	Store   *Store
	Channel Channel
}

func PaymentWorkflow(ctx workflow.Context, in Input) error {
	return paymentWorkflow(ctx, in, false)
}

// Local upgrade fixture: prior implementation without the approval audit marker.
func LegacyPaymentWorkflow(ctx workflow.Context, in Input) error {
	return paymentWorkflow(ctx, in, true)
}
func paymentWorkflow(ctx workflow.Context, in Input, legacy bool) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Second, RetryPolicy: &temporal.RetryPolicy{InitialInterval: time.Second, MaximumInterval: 2 * time.Second, MaximumAttempts: 3}})
	// Keep this marker and its legacy branch when changing in-flight histories.
	version := workflow.DefaultVersion
	if !legacy {
		version = workflow.GetVersion(ctx, "approval-audit", workflow.DefaultVersion, 1)
	}
	var approved bool
	received, _ := workflow.GetSignalChannel(ctx, "approve").ReceiveWithTimeout(ctx, time.Duration(in.ApprovalSeconds)*time.Second, &approved)
	var err error
	if !received || !approved {
		return workflow.ExecuteActivity(ctx, "SetState", in.Payment.ID, "expired").Get(ctx, nil)
	}
	if version == 1 {
		if err = workflow.ExecuteActivity(ctx, "AuditApproval", in.Payment.ID).Get(ctx, nil); err != nil {
			return err
		}
	}
	if err = workflow.ExecuteActivity(ctx, "Submit", in.Payment).Get(ctx, nil); err != nil {
		// Exhausted retries mean unknown external outcome, never permission to refund.
		return workflow.ExecuteActivity(ctx, "SetState", in.Payment.ID, "review").Get(ctx, nil)
	}
	deadline := workflow.Now(ctx).Add(time.Duration(in.ObserveSeconds) * time.Second)
	for workflow.Now(ctx).Before(deadline) {
		var state string
		err = workflow.ExecuteActivity(ctx, "Observe", in.Payment.ID).Get(ctx, &state)
		if err == nil && state == "confirmed" {
			if err = workflow.ExecuteActivity(ctx, "Settle", in.Payment.ID).Get(ctx, nil); err != nil {
				return err
			}
			return workflow.ExecuteActivity(ctx, "Notify", "settle:"+in.Payment.ID).Get(ctx, nil)
		}
		if err == nil && state == "reversed" {
			return workflow.ExecuteActivity(ctx, "SetState", in.Payment.ID, "review").Get(ctx, nil)
		}
		if err = workflow.Sleep(ctx, time.Second); err != nil {
			return err
		}
	}
	return workflow.ExecuteActivity(ctx, "SetState", in.Payment.ID, "review").Get(ctx, nil)
}
func RefundWorkflow(ctx workflow.Context, p Payment) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Second, RetryPolicy: &temporal.RetryPolicy{InitialInterval: time.Second, MaximumInterval: 2 * time.Second, MaximumAttempts: 3}})
	if err := workflow.ExecuteActivity(ctx, "PrepareRefund", p.ID).Get(ctx, nil); err != nil {
		return err
	}
	if err := workflow.ExecuteActivity(ctx, "SendRefund", p.ID).Get(ctx, nil); err != nil {
		return err
	}
	if err := workflow.ExecuteActivity(ctx, "FinishRefund", p.ID).Get(ctx, nil); err != nil {
		return err
	}
	return workflow.ExecuteActivity(ctx, "Notify", "refund:"+p.ID).Get(ctx, nil)
}

func NotifyWorkflow(ctx workflow.Context, event string) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Second, RetryPolicy: &temporal.RetryPolicy{InitialInterval: time.Second, MaximumAttempts: 3}})
	return workflow.ExecuteActivity(ctx, "Notify", event).Get(ctx, nil)
}
func (a *Activities) Notify(ctx context.Context, event string) error {
	var raw []byte
	if err := a.Store.DB.QueryRow(ctx, "SELECT payload::text FROM lab.outbox WHERE event_id=$1", event).Scan(&raw); err != nil {
		return err
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	request, err := http.NewRequestWithContext(ctx, "POST", a.Channel.URL+"/merchant-webhook", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	request.Header.Set("X-Lab-Token", a.Channel.Token)
	request.Header.Set("X-Timestamp", timestamp)
	request.Header.Set("X-Signature", Signature(a.Channel.Token, timestamp, raw))
	response, err := (&http.Client{Timeout: 2 * time.Second}).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return fmt.Errorf("merchant delivery HTTP %d", response.StatusCode)
	}
	_, err = a.Store.DB.Exec(ctx, "UPDATE lab.outbox SET published=true WHERE event_id=$1", event)
	return err
}
func (a *Activities) AuditApproval(ctx context.Context, id string) error {
	// Approval evidence is idempotent even when the Activity response is lost.
	_, err := a.Store.DB.Exec(ctx, "INSERT INTO ops.audit(payment,action) VALUES($1,'approved') ON CONFLICT DO NOTHING", id)
	return err
}
func (a *Activities) SetState(ctx context.Context, id, state string) error {
	return a.Store.State(ctx, id, state)
}
func (a *Activities) Submit(ctx context.Context, p Payment) error {
	_, err := a.Channel.Call(ctx, "POST", "/transfers", p)
	if err != nil {
		return err
	}
	return a.Store.State(ctx, p.ID, "pending")
}
func (a *Activities) Observe(ctx context.Context, id string) (string, error) {
	event, err := a.Channel.Call(ctx, "GET", "/transfers/"+id, nil)
	if err != nil {
		return "", err
	}
	if event.Payment != id {
		return "", errors.New("channel identity mismatch")
	}
	if err = a.Store.Ingest(ctx, event); err != nil {
		return "", err
	}
	var state string
	err = a.Store.DB.QueryRow(ctx, "SELECT provider_state FROM ops.payments WHERE id=$1", id).Scan(&state)
	return state, err
}
func (a *Activities) Settle(ctx context.Context, id string) error {
	if err := a.Store.Settle(ctx, id); err != nil {
		return err
	}
	if a.Store.Fault(ctx, "settlement_response_lost") {
		return fmt.Errorf("fixture: committed, Activity response lost")
	}
	return nil
}
func (a *Activities) PrepareRefund(ctx context.Context, id string) error {
	return a.Store.RefundState(ctx, id, false)
}
func (a *Activities) SendRefund(ctx context.Context, id string) error {
	e, err := a.Channel.Call(ctx, "POST", "/refunds/"+id, nil)
	if err != nil {
		return err
	}
	if e.State != "refunded" {
		return errors.New("refund outcome unknown")
	}
	return nil
}
func (a *Activities) FinishRefund(ctx context.Context, id string) error {
	return a.Store.RefundState(ctx, id, true)
}
