package main

import (
	"example.com/stablecoin-ops/internal/ops"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"log"
)

func main() {
	s := ops.Open()
	defer s.DB.Close()
	c, err := client.Dial(client.Options{HostPort: ops.Env("TEMPORAL_ADDRESS", "127.0.0.1:7233")})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()
	w := worker.New(c, ops.Queue, worker.Options{})
	if ops.Env("LAB_LEGACY_WORKER", "") == "1" {
		w.RegisterWorkflowWithOptions(ops.LegacyPaymentWorkflow, workflow.RegisterOptions{Name: "PaymentWorkflow"})
	} else {
		w.RegisterWorkflow(ops.PaymentWorkflow)
	}
	w.RegisterWorkflow(ops.RefundWorkflow)
	w.RegisterWorkflow(ops.ReconcileWorkflow)
	w.RegisterWorkflow(ops.NotifyWorkflow)
	w.RegisterWorkflow(ops.ChainPaymentWorkflow)
	w.RegisterWorkflow(ops.ChainReconcileWorkflow)
	w.RegisterWorkflow(ops.ConversionWorkflow)
	w.RegisterActivity(&ops.Activities{Store: s, Channel: ops.Channel{URL: ops.Env("CHANNEL_URL", "http://127.0.0.1:8082"), Token: ops.Env("LAB_CHANNEL_SECRET", "")}})
	if err = w.Run(worker.InterruptCh()); err != nil {
		log.Fatal(err)
	}
}
