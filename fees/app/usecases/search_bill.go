package usecases

import (
	"context"
	"fmt"

	"github.com/outofboxer/temporal-workflow/fees/app"
	"github.com/outofboxer/temporal-workflow/fees/app/views"
	"github.com/outofboxer/temporal-workflow/fees/domain"
	"github.com/outofboxer/temporal-workflow/libs/time"
)

type SearchBillCmd struct {
	CustomerID string
	PeriodFrom domain.BillingPeriod
	PeriodTo   domain.BillingPeriod
	Status     string
}

type SearchBill struct{ T app.TemporalPort }

func (uc SearchBill) Handle(ctx context.Context, c SearchBillCmd) ([]views.BillSummary, error) {
	fromInt, err := time.ToYYYYMMNullable(string(c.PeriodFrom))
	if err != nil {
		return nil, fmt.Errorf("fromInt conversion error, %w", err)
	}
	toInt, err := time.ToYYYYMMNullable(string(c.PeriodTo))
	if err != nil {
		return nil, fmt.Errorf("toInt conversion error, %w", err)
	}
	// the logic assumes OPEN and PENDING statuses should be fetched as the same logically opened for search only statuses.
	statuses := []string{c.Status}
	if c.Status == string(domain.BillStatusOpen) {
		statuses = append(statuses, string(domain.BillStatusPending))
	}
	filter := app.SearchBillFilter{
		CustomerID: c.CustomerID,
		FromYYYYMM: fromInt,
		ToYYYYMM:   toInt,
		Status:     statuses,
	}

	bills, err := uc.T.SearchBills(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("SearchBills UC failer, %w", err)
	}

	return bills, nil
}
