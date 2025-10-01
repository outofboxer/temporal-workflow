package worker

import (
	"context"
	// Encore.
	"encore.dev/beta/errs"
	"encore.dev/config"
	"go.temporal.io/sdk/workflow"

	// Temporal.
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	// Worker service.
	"github.com/outofboxer/temporal-workflow/fees/app/workflows"
	"github.com/outofboxer/temporal-workflow/fees/internal/adapters/temporal/activities"
)

//nolint:unused
var cfg *Config = config.Load[*Config]()

//nolint:unused
const taskQueue = "FEES_TASK_QUEUE"

//encore:service
type Service struct {
	tc client.Client
	w  worker.Worker
}

//nolint:unused
func initService() (*Service, error) {
	// (Optionally read host/namespace from Encore config)
	tc, err := client.Dial(client.Options{
		HostPort:  cfg.Temporal.Host(),
		Namespace: cfg.Temporal.Namespace(),
		// DataConverter: custom if you use one
	})
	if err != nil {
		return nil, errs.B().Cause(err).Msg("temporal dial").Err()
	}

	// Create a worker bound to your task queue
	w := worker.New(tc, taskQueue, worker.Options{
		// Tune as needed:
		// MaxConcurrentActivityExecutionSize: 100,
		// MaxConcurrentWorkflowTaskExecutionSize: 50,
	})

	// Register workflows (function or method receiver)
	w.RegisterWorkflowWithOptions(workflows.MonthlyFeeAccrualWorkflow,
		workflow.RegisterOptions{Name: workflows.WorkflowTypeMonthlyBill})

	w.RegisterActivity(activities.ProcessInvoiceAndChargeActivity)

	// Start non-blocking, return service so Encore can manage lifecycle
	if err := w.Start(); err != nil {
		tc.Close()

		return nil, errs.B().Cause(err).Msg("worker start").Err()
	}

	return &Service{tc: tc, w: w}, nil
}

func (s *Service) Shutdown(_ context.Context) {
	// Graceful stop
	s.w.Stop()
	s.tc.Close()
}
