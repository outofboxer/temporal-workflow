package domain

import "fmt"

type BillID string

func MakeBillID(customerID string, period BillingPeriod) BillID {
	return BillID(fmt.Sprintf("bill/%s/%s", customerID, period))
}
