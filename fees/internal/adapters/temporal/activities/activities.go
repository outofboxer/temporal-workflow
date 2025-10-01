package activities

import (
	"context"

	"go.temporal.io/sdk/activity"

	"github.com/outofboxer/temporal-workflow/fees/domain"
)

// ProcessInvoiceAndChargeActivity handles the finalization and external charging steps.
// Should send to payment gateway: total amount.
func ProcessInvoiceAndChargeActivity(ctx context.Context, bill domain.Bill) error {
	log := activity.GetLogger(ctx)

	log.Info("processing invoice",
		"bill_id", bill.ID,
		"customer_id", bill.CustomerID,
		"period", bill.BillingPeriod,
		"status", bill.Status,
		"total", bill.Total.ToString(),
		"items", len(bill.Items),
	)

	// 1. Generate Invoice (External API call). Use idempotency keys to payment gateways because activities are retried.
	// 2. Submit charge to payment gateway (External API call)
	// 3. Final persistence/state change (Database update)
	// 4. Error typing to leverage NonRetryableErrorTypes.
	// 5. Apply tracing spans for external calls in the activity.

	// The Activity input (state) indicates the total amount] and all line items being charged.
	// Any failure here will result in the Activity being retried by Temporal.

	return nil
}
