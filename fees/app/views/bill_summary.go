package views

type BillSummary struct {
	WorkflowID string
	RunID      string
	Status     string
	Currency   string
	// From Search Attributes:
	CustomerID       string
	BillingPeriodNum int64
	TotalCents       int64
	ItemCount        int64
}
