package usecases

import (
	"context"

	"github.com/outofboxer/temporal-workflow/fees/app"
	"github.com/outofboxer/temporal-workflow/fees/domain"
)

type AddLineItemCmd struct {
	CustomerID string
	Period     domain.BillingPeriod
	Item       domain.LineItem
}

type AddLineItem struct{ T app.TemporalPort }

func (uc AddLineItem) Handle(ctx context.Context, c AddLineItemCmd) (domain.Bill, error) {
	billID := domain.MakeBillID(c.CustomerID, c.Period)

	bill, err := uc.T.QueryBill(ctx, billID)
	if err != nil {
		return domain.Bill{}, err
	}
	if !bill.IsActive() {
		return domain.Bill{}, app.ErrBillAlreadyClosed
	}

	for _, li := range bill.Items {
		if li.IdempotencyKey == c.Item.IdempotencyKey {
			return domain.Bill{}, app.ErrLineItemAlreadyAdded
		}
	}

	if err := uc.T.AddLineItem(ctx, billID, c.Item); err != nil {
		return domain.Bill{}, err
	}

	return uc.T.QueryBill(ctx, billID)
}
