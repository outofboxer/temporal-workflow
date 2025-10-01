package sa

// Typed search attribute keys.
import "go.temporal.io/sdk/temporal"

const (
	CustomerIDName       = "CustomerID"
	BillingPeriodNumName = "BillingPeriodNum"
	BillStatusName       = "BillStatus"
	BillCurrencyName     = "BillCurrency"
	BillItemCountName    = "BillItemCount"
	BillTotalCentsName   = "BillTotalCents"
)

var (
	KeyCustomerID       = temporal.NewSearchAttributeKeyKeyword(CustomerIDName)
	KeyBillingPeriodNum = temporal.NewSearchAttributeKeyInt64(BillingPeriodNumName) // e.g. 202410
	KeyBillStatus       = temporal.NewSearchAttributeKeyKeyword(BillStatusName)     // "OPEN" | "CLOSED"
	KeyBillCurrency     = temporal.NewSearchAttributeKeyKeyword(BillCurrencyName)
	KeyBillItemCount    = temporal.NewSearchAttributeKeyInt64(BillItemCountName)
	KeyBillTotalCents   = temporal.NewSearchAttributeKeyInt64(BillTotalCentsName)
)
