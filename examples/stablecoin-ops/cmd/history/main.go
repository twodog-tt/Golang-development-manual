package main

import (
	"context"
	"encoding/json"
	"example.com/stablecoin-ops/internal/ops"
	"flag"
	enums "go.temporal.io/api/enums/v1"
	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/protobuf/encoding/protojson"
	"log"
	"os"
	"time"
)

func main() {
	id := flag.String("id", "", "workflow ID")
	out := flag.String("out", "history.json", "output")
	negative := flag.Bool("negative", false, "expect an incompatible workflow to fail replay")
	flag.Parse()
	c, err := client.Dial(client.Options{HostPort: ops.Env("TEMPORAL_ADDRESS", "127.0.0.1:7233")})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()
	iter := c.GetWorkflowHistory(context.Background(), *id, "", false, enums.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
	h := &historypb.History{}
	for iter.HasNext() {
		event, err := iter.Next()
		if err != nil {
			log.Fatal(err)
		}
		h.Events = append(h.Events, event)
	}
	data, err := protojson.Marshal(h)
	if err != nil {
		log.Fatal(err)
	}
	if err = os.WriteFile(*out, data, 0600); err != nil {
		log.Fatal(err)
	}
	replay := worker.NewWorkflowReplayer()
	if *negative {
		replay.RegisterWorkflowWithOptions(func(ctx workflow.Context, in ops.Input) error {
			return workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Second}), "UnexpectedActivity").Get(ctx, nil)
		}, workflow.RegisterOptions{Name: "PaymentWorkflow"})
		if err = replay.ReplayWorkflowHistory(nil, h); err == nil {
			log.Fatal("incompatible workflow unexpectedly replayed")
		}
		json.NewEncoder(os.Stdout).Encode(map[string]string{"incompatible_replay": "rejected"})
		return
	}
	replay.RegisterWorkflow(ops.PaymentWorkflow)
	if err = replay.ReplayWorkflowHistory(nil, h); err != nil {
		log.Fatal(err)
	}
	json.NewEncoder(os.Stdout).Encode(map[string]any{"replay": "passed", "events": len(h.Events), "workflow": *id})
}
