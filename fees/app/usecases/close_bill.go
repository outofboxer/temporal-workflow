package usecases

import (
	"context"

	"github.com/outofboxer/temporal-workflow/fees/app"
	"github.com/outofboxer/temporal-workflow/fees/domain"
)

type CloseBillCmd struct {
	CustomerID string
	Period     domain.BillingPeriod
}

type CloseBill struct{ T app.TemporalPort }

// This is actually idempotant at Workflow level.
func (uc CloseBill) Handle(ctx context.Context, c CloseBillCmd) (domain.Bill, error) {
	id := domain.MakeBillID(c.CustomerID, c.Period)
	bill, err := uc.T.QueryBill(ctx, id)
	if err != nil {
		return domain.Bill{}, err
	}
	if !bill.IsActive() {
		return domain.Bill{}, app.ErrBillAlreadyClosed
	}
	if err := uc.T.CloseBill(ctx, id); err != nil {
		return domain.Bill{}, err
	}

	return uc.T.QueryBill(ctx, id)
}
