package feesapi

import (
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/outofboxer/temporal-workflow/fees/app/views"
	"github.com/outofboxer/temporal-workflow/fees/domain"
)

func mapBillListResponse(summaries []views.BillSummary) ListBillsResponse {
	out := make([]ListBillResponse, 0, len(summaries))
	for _, s := range summaries {
		out = append(out, ListBillResponse{
			ID:            s.WorkflowID,
			CustomerID:    s.CustomerID,
			Currency:      s.Currency,
			BillingPeriod: billingPeriodNumToString(s.BillingPeriodNum),
			Status:        s.Status,
			ItemCount:     s.ItemCount,
			Total:         totalCentsToString(s.TotalCents),
		})
	}

	return ListBillsResponse{Bills: out}
}

// BillingPeriodNum (e.g., 202410) -> "YYYY-MM" (e.g., "2024-10").
func billingPeriodNumToString(n int64) string {
	if n < 100001 || n > 999912 { // quick sanity range: 0000-01 .. 9999-12
		return "<formatting error in range>"
	}
	year := n / 100  //nolint:mnd
	month := n % 100 //nolint:mnd
	if month < 1 || month > 12 {
		return "<formatting error in month>"
	}

	return fmt.Sprintf("%04d-%02d", year, month)
}

// TotalCentsToString converts 12345 -> "123.45".
func totalCentsToString(totalCents int64) string {
	const shift = 2

	return decimal.NewFromInt(totalCents).Shift(-shift).StringFixed(shift)
}

func map2BillingResponse(b domain.Bill) *BillResponse {
	lineItems := make([]BillLineItemResponse, 0, len(b.Items))
	for _, bi := range b.Items {
		lineItems = append(lineItems, BillLineItemResponse{
			IdempotencyKey: bi.IdempotencyKey,
			Description:    bi.Description,
			Amount:         bi.Amount,
			AddedAt:        bi.AddedAt,
		})
	}

	return &BillResponse{
		ID:            string(b.ID),
		CustomerID:    b.CustomerID,
		Currency:      string(b.Currency),
		BillingPeriod: string(b.BillingPeriod),
		Status:        string(b.Status),
		Items:         lineItems,
		Total:         b.Total.ToString(),
		CreatedAt:     b.CreatedAt,
		UpdatedAt:     b.UpdatedAt,
		ClosedAt:      b.FinalizedAt,
	}
}
