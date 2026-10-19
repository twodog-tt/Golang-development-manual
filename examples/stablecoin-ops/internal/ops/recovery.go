package ops

import (
	"context"
	"errors"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"time"
)

// Dispatch is retryable across process loss, including a lost StartWorkflow
// response. It starts only an audit; it cannot submit another payment or swap.
func (s *Store) DispatchRecoveryMonitors(ctx context.Context, c client.Client) error {
	rows, err := s.DB.Query(ctx, "SELECT id,payment FROM ops.recovery_plans WHERE applied AND needs_monitor AND NOT monitor_started ORDER BY id")
	if err != nil {
		return err
	}
	type job struct{ id, payment string }
	var jobs []job
	for rows.Next() {
		var j job
		if err = rows.Scan(&j.id, &j.payment); err != nil {
			rows.Close()
			return err
		}
		jobs = append(jobs, j)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, j := range jobs {
		_, err = c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: "operator-audit:" + j.id, TaskQueue: Queue, WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE}, RecoveryAuditWorkflow, j.id, j.payment)
		var duplicate *serviceerror.WorkflowExecutionAlreadyStarted
		if err != nil && !errors.As(err, &duplicate) {
			return err
		}
		if s.Fault(ctx, "recovery_monitor_response_lost") {
			return errors.New("fixture: monitor StartWorkflow response lost")
		}
		if _, err = s.DB.Exec(ctx, "UPDATE ops.recovery_plans SET monitor_started=true WHERE id=$1 AND applied", j.id); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) RunRecoveryMonitors(ctx context.Context, c client.Client) {
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			call, cancel := context.WithTimeout(ctx, 5*time.Second)
			_ = s.DispatchRecoveryMonitors(call, c)
			cancel()
		}
	}
}

// This new workflow type does not change any pre-existing history. A queued
// workflow alone is insufficient evidence that recovery monitoring is healthy.
func RecoveryAuditWorkflow(ctx workflow.Context, plan, payment string) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 15 * time.Second, RetryPolicy: &temporal.RetryPolicy{InitialInterval: time.Second, MaximumAttempts: 3}})
	var state string
	if err := workflow.ExecuteActivity(ctx, "AuditChain", payment).Get(ctx, &state); err != nil {
		return err
	}
	if err := workflow.ExecuteActivity(ctx, "MarkRecoveryAudited", plan).Get(ctx, nil); err != nil {
		return err
	}
	if state == "settled" {
		return ChainPaymentWorkflow(ctx, ChainInput{Input: Input{Payment: Payment{ID: payment}}, Monitor: true})
	}
	return nil
}
func (a *Activities) MarkRecoveryAudited(ctx context.Context, plan string) error {
	_, err := a.Store.DB.Exec(ctx, "UPDATE ops.recovery_plans SET monitor_checked=true WHERE id=$1 AND applied AND needs_monitor", plan)
	return err
}
