package usecases

import (
	"context"

	"github.com/outofboxer/temporal-workflow/fees/app"
	"github.com/outofboxer/temporal-workflow/fees/domain"
)

type GetBillCmd struct {
	CustomerID string
	Period     domain.BillingPeriod
}

type GetBill struct{ T app.TemporalPort }

func (uc GetBill) Handle(ctx context.Context, c GetBillCmd) (domain.Bill, error) {
	id := domain.MakeBillID(c.CustomerID, c.Period)

	return uc.T.QueryBill(ctx, id)
}
