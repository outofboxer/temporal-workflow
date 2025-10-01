package usecases

import (
	"context"
	"fmt"

	"github.com/outofboxer/temporal-workflow/fees/app"
	"github.com/outofboxer/temporal-workflow/fees/domain"
	libmoney "github.com/outofboxer/temporal-workflow/libs/money"
	"github.com/outofboxer/temporal-workflow/libs/time"
)

type CreateBillCmd struct {
	CustomerID string
	Period     domain.BillingPeriod
	Currency   libmoney.Currency
}

type CreateBill struct{ T app.TemporalPort }

func (uc CreateBill) Handle(ctx context.Context, c CreateBillCmd) (domain.Bill, error) {
	id := domain.MakeBillID(c.CustomerID, c.Period)
	yyyymm, err := time.ToYYYYMM(string(c.Period))
	if err != nil {
		return domain.Bill{}, fmt.Errorf("period formatting error, %w", err)
	}
	workflowParams := app.MonthlyFeeAccrualWorkflowParams{
		BillID:       id,
		CustomerID:   c.CustomerID,
		Period:       c.Period,
		PeriodYYYYMM: yyyymm,
		Currency:     c.Currency,
	}
	if err := uc.T.StartMonthlyBill(ctx, workflowParams); err != nil {
		return domain.Bill{}, err
	}

	return uc.T.QueryBill(ctx, id)
}
