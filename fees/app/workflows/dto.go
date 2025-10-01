package workflows

import (
	"time"

	"github.com/outofboxer/temporal-workflow/fees/domain"
	libmoney "github.com/outofboxer/temporal-workflow/libs/money"
)

const WorkflowTypeMonthlyBill = "MonthlyFeeAccrualWorkflow"

const (
	SignalAddLineItem = "SignalAddLineItem"
	SignalCloseBill   = "SignalCloseBill"
	QueryState        = "CurrentBillState"
)

// CloseBillSignal is sent when the service signals the end of the month [2].
type CloseBillSignal struct{}

type AddLineItemPayload struct {
	Description    string
	Amount         libmoney.Money
	IdempotencyKey string
}

type BillDTO struct {
	ID, CustomerID string
	Currency       libmoney.Currency
	BillingPeriod  string
	Status         string
	Items          []LineItemDTO
	Total          libmoney.Money
	CreatedAt      time.Time
	UpdatedAt      time.Time
	ClosedAt       *time.Time
}

type LineItemDTO struct {
	IdempotencyKey string
	Description    string
	Amount         libmoney.Money
	AddedAt        time.Time
}

func billToDTO(bill domain.Bill) BillDTO {
	lineItems := make([]LineItemDTO, 0, len(bill.Items))
	for _, li := range bill.Items {
		lineItems = append(lineItems, LineItemDTO{
			IdempotencyKey: li.IdempotencyKey,
			Description:    li.Description,
			Amount:         li.Amount,
			AddedAt:        li.AddedAt,
		})
	}

	return BillDTO{
		ID:            string(bill.ID),
		CustomerID:    bill.CustomerID,
		Currency:      bill.Currency,
		BillingPeriod: string(bill.BillingPeriod),
		Status:        string(bill.Status),
		Items:         lineItems,
		Total:         bill.Total,
		CreatedAt:     bill.CreatedAt,
		UpdatedAt:     bill.UpdatedAt,
		ClosedAt:      bill.FinalizedAt,
	}
}
