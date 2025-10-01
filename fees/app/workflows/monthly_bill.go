// Package workflows Made using Clean Architecture + Encore + Temporal, treat a workflow as an application-layer
// use case (orchestration), not domain and not infrastructure.
package workflows

import (
	"time"

	"github.com/shopspring/decimal"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/outofboxer/temporal-workflow/fees/app"
	"github.com/outofboxer/temporal-workflow/fees/app/workflows/sa"
	"github.com/outofboxer/temporal-workflow/fees/domain"
	"github.com/outofboxer/temporal-workflow/fees/internal/adapters/temporal/activities"
	libmoney "github.com/outofboxer/temporal-workflow/libs/money"
)

// This execution is on a single thread–while this means we don’t have to worry about parallelism,
//
//	we do need to worry about concurrency if we have written Signal and Update handlers that can block.
//
// MonthlyFeeAccrualWorkflow manages the entire bill lifecycle for one customer/month.
//
//nolint:funlen
func MonthlyFeeAccrualWorkflow(ctx workflow.Context, params app.MonthlyFeeAccrualWorkflowParams) (domain.Bill, error) {
	logger := workflow.GetLogger(ctx) // workflow replay safe logger

	// future optimization, At the start of the workflow, if params.Snapshot != nil,
	// restore bw.bill from it instead of building a fresh one, then re‐upsert the SAs to keep visibility correct.
	// Also, Continue-As-New, re-upsert any “static” SAs (customer, period, currency) on the new run for consistency.
	bill, err := newBillBuilderFromWorkflow(ctx).
		WithID(params.BillID).
		ForCustomer(params.CustomerID).
		ForPeriod(params.Period).
		WithCurrency(params.Currency).
		WithCreatedAt(workflow.Now(ctx)).
		Open().
		Build()
	if err != nil {
		return domain.Bill{}, err
	}

	// Define Signal and Query Handlers (Progressive Accrual Phase)

	// Register Query Handler
	if errQuery := workflow.SetQueryHandler(ctx, QueryState, func() (BillDTO, error) {
		// DO NOT log each query fact in PROD code, it can be too many logs. Just for demo.
		if !workflow.IsReplaying(ctx) {
			logger.Info("Starting Bill Query Handler processing")
			defer logger.Info("Finished Bill Query Handler processing")
		}

		return billToDTO(bill), nil
	}); errQuery != nil {
		logger.Error("SetQueryHandler failed", "errQuery", errQuery)

		return domain.Bill{}, errQuery
	}

	// Define channel to receive the Close Signal
	addItemCh := workflow.GetSignalChannel(ctx, SignalAddLineItem)
	closeCh := workflow.GetSignalChannel(ctx, SignalCloseBill)
	sel := workflow.NewSelector(ctx)

	sel.AddReceive(addItemCh, func(c workflow.ReceiveChannel, _ bool) {
		logger.Info("Starting addItem processing")
		defer logger.Info("Finished addItem processing")

		var pl AddLineItemPayload
		c.Receive(ctx, &pl)
		if !bill.IsActive() {
			logger.Info("discarding a Line Item after bill is finalized", "lineItem", pl)
			// ignore gracefully; API layer prevents this; idempotent sink
			return
		}
		err := bill.AddItem(pl.IdempotencyKey, pl.Description, pl.Amount, workflow.Now(ctx))
		if err != nil {
			logger.Error("Couldn't add Line Item", "err", err)

			return
		}
		logger.Info("added item", "lineItem", pl)
		// Temporal will retry it in case of failure of SA upsert
		err = UpdateInsertItemSearchAttributes(ctx, bill)
		if err != nil {
			logger.Error("UpdateInsertItemSearchAttributes upsert failed", "error", err)

			return
		}
		logger.Info("UpdateInsertItemSearchAttributes ok")

		// future optimization, use compaction of LineItems, persist if in offline storage,
		//	remove it from Temporal Workflow, Continue As New for the workflow
		/*if len(bill.Items) >= maxItemsPerRun { // const maxItemsPerRun = 100
			next := params
			next.Snapshot = ToSnapshot(bill) // compact DTO (no maps of huge data if you can avoid)
			// For method-style workflows, prefer calling by name to avoid receiver capture:
			return domain.Bill{}, workflow.NewContinueAsNewError(ctx,
				"BillWorkflow.MonthlyFeeAccrualWorkflow", next)
			// If you register a function workflow instead, pass the func identifier directly.
		}*/
	})

	sel.AddReceive(closeCh, func(c workflow.ReceiveChannel, _ bool) {
		logger.Info("Starting closing processing")
		defer logger.Info("Finished closing processing")

		var nothing struct{}
		c.Receive(ctx, &nothing)
		if !bill.IsActive() {
			logger.Info("discarding Close signal as bill is not active", "status", bill.Status)
			// this is idempotent processing
			return
		}

		err := bill.Pending(workflow.Now(ctx))
		if err != nil {
			logger.Error("bill.Pending failed", "err", err.Error())

			return
		}
		logger.Info("moved into Pending")

		// Temporal does retry on failure by temporal automatically
		err = UpdateBillStatusSearchAttributes(ctx, bill.Status)
		if err != nil {
			logger.Error("UpdateBillStatusSearchAttributes upsert failed", "error", err)
			// I prefer not to fail-fast, rely on Temporal retries. But it depends on Org policies.
			// return domain.Bill{}, fmt.Errorf("failed to update search attributes: %w", err)
		}
		logger.Info("UpdateBillStatusSearchAttributes ok")
	})

	// Event loop until closing or error
	for bill.IsActive() {
		sel.Select(ctx)
	}

	if !bill.IsReadyForInvoicing() {
		logger.Info("exiting since bill is not ready to invoicing", "status", bill.Status)

		return bill, err
	}
	logger.Info("Starting Invoicing activity ")

	if err := DoInvoicesActivities(ctx, bill); err != nil {
		logger.Error("Finalization failed.", "error", err)

		errStatus := bill.Error(workflow.Now(ctx))
		if errStatus != nil {
			logger.Error("bill.Error transition failed.", "error", err)
		}

		return bill, err
	}
	err = bill.Close(workflow.Now(ctx))
	if err != nil {
		logger.Error("bill.Error() failed", "err", err.Error())
	}
	// Retried automatically on failure by Temporal
	err = UpdateBillStatusSearchAttributes(ctx, bill.Status)
	if err != nil {
		logger.Error("UpdateBillStatusSearchAttributes upsert failed", "error", err)
		// I prefer not to fail-fast, rely on Temporal retries. But it depends on Org policies.
		// return domain.Bill{}, fmt.Errorf("failed to update search attributes: %w", err)
	}
	// Workflow completes—final bill is queryable from history.
	// For future: keep it running until periodEnd using timers, but these are tricky requirements to be clarified.

	return bill, nil
}

func DoInvoicesActivities(ctx workflow.Context, bill domain.Bill) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		//nolint:mnd
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			MaximumAttempts:    5,
			BackoffCoefficient: 2.0,
			MaximumInterval:    30 * time.Second,
			// USE NonRetryableErrorTypes for validation/domain errors.
			// Sample error, we don't have it in the demo.
			NonRetryableErrorTypes: []string{"ValidationError", "BusinessRuleError"},
		},
	}
	finalizationCtx := workflow.WithActivityOptions(ctx, ao)

	return workflow.ExecuteActivity(finalizationCtx, activities.ProcessInvoiceAndChargeActivity, bill).
		Get(finalizationCtx, nil)
}

// the side effect is possibly updated bill.status, set to error!
func UpdateInsertItemSearchAttributes(ctx workflow.Context, bill domain.Bill) error {
	// in case of error Temporal will retry this automatically, and replay the addReceive function
	return workflow.UpsertTypedSearchAttributes(ctx,
		sa.KeyBillTotalCents.ValueSet(moneyToCents(bill.Total)),
		sa.KeyBillItemCount.ValueSet(int64(len(bill.Items))),
	)
}

// the side effect is possibly updated bill.status, set to error!
func UpdateBillStatusSearchAttributes(ctx workflow.Context, status domain.BillStatus) error {
	// in case of error Temporal will retry this automatically
	return workflow.UpsertTypedSearchAttributes(ctx, sa.KeyBillStatus.ValueSet(string(status)))
}

func moneyToCents(m libmoney.Money) int64 {
	scale := 2
	factor := decimal.New(1, int32(scale)) // 10^scale

	return m.MulOnDecimal(factor).Round(0).IntPart() // half-away-from-zero
}

func newBillBuilderFromWorkflow(ctx workflow.Context) *domain.BillBuilder {
	runID := workflow.GetInfo(ctx).WorkflowExecution.RunID

	return domain.NewBillBuilder().WithID(domain.BillID(runID))
}
