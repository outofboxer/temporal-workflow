package domain

import (
	"testing"
)

func TestMakeBillID(t *testing.T) {
	tests := []struct {
		name       string
		customerID string
		period     BillingPeriod
		expected   BillID
	}{
		{
			name:       "Valid customer and period",
			customerID: "cust-123",
			period:     BillingPeriod("2025-01"),
			expected:   BillID("bill/cust-123/2025-01"),
		},
		{
			name:       "Customer with special characters",
			customerID: "customer_123-test",
			period:     BillingPeriod("2025-12"),
			expected:   BillID("bill/customer_123-test/2025-12"),
		},
		{
			name:       "Empty customer ID",
			customerID: "",
			period:     BillingPeriod("2025-01"),
			expected:   BillID("bill//2025-01"),
		},
		{
			name:       "Empty period",
			customerID: "cust-123",
			period:     BillingPeriod(""),
			expected:   BillID("bill/cust-123/"),
		},
		{
			name:       "Both empty",
			customerID: "",
			period:     BillingPeriod(""),
			expected:   BillID("bill//"),
		},
		{
			name:       "Customer with spaces",
			customerID: "customer with spaces",
			period:     BillingPeriod("2025-06"),
			expected:   BillID("bill/customer with spaces/2025-06"),
		},
		{
			name:       "Period with different format",
			customerID: "cust-456",
			period:     BillingPeriod("2024-12"),
			expected:   BillID("bill/cust-456/2024-12"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MakeBillID(tt.customerID, tt.period)
			if result != tt.expected {
				t.Errorf("MakeBillID(%q, %q) = %q, want %q",
					tt.customerID, tt.period, result, tt.expected)
			}
		})
	}
}

func TestBillID_String(t *testing.T) {
	tests := []struct {
		name     string
		billID   BillID
		expected string
	}{
		{
			name:     "Valid bill ID",
			billID:   BillID("bill/cust-123/2025-01"),
			expected: "bill/cust-123/2025-01",
		},
		{
			name:     "Empty bill ID",
			billID:   BillID(""),
			expected: "",
		},
		{
			name:     "Bill ID with special characters",
			billID:   BillID("bill/customer_123-test/2025-12"),
			expected: "bill/customer_123-test/2025-12",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := string(tt.billID)
			if result != tt.expected {
				t.Errorf("BillID.String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestBillID_Equality(t *testing.T) {
	tests := []struct {
		name     string
		billID1  BillID
		billID2  BillID
		expected bool
	}{
		{
			name:     "Equal bill IDs",
			billID1:  BillID("bill/cust-123/2025-01"),
			billID2:  BillID("bill/cust-123/2025-01"),
			expected: true,
		},
		{
			name:     "Different customer IDs",
			billID1:  BillID("bill/cust-123/2025-01"),
			billID2:  BillID("bill/cust-456/2025-01"),
			expected: false,
		},
		{
			name:     "Different periods",
			billID1:  BillID("bill/cust-123/2025-01"),
			billID2:  BillID("bill/cust-123/2025-02"),
			expected: false,
		},
		{
			name:     "Case sensitivity",
			billID1:  BillID("bill/cust-123/2025-01"),
			billID2:  BillID("BILL/CUST-123/2025-01"),
			expected: false,
		},
		{
			name:     "Empty vs non-empty",
			billID1:  BillID(""),
			billID2:  BillID("bill/cust-123/2025-01"),
			expected: false,
		},
		{
			name:     "Both empty",
			billID1:  BillID(""),
			billID2:  BillID(""),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := (tt.billID1 == tt.billID2)
			if result != tt.expected {
				t.Errorf("BillID equality = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMakeBillID_Consistency(t *testing.T) {
	// Test that MakeBillID produces consistent results
	customerID := "cust-123"
	period := BillingPeriod("2025-01")

	// Generate the same bill ID multiple times
	id1 := MakeBillID(customerID, period)
	id2 := MakeBillID(customerID, period)
	id3 := MakeBillID(customerID, period)

	// All should be equal
	if id1 != id2 || id2 != id3 {
		t.Errorf("MakeBillID should produce consistent results: %q, %q, %q", id1, id2, id3)
	}

	// Should match expected format
	expected := BillID("bill/cust-123/2025-01")
	if id1 != expected {
		t.Errorf("MakeBillID = %q, want %q", id1, expected)
	}
}

func TestMakeBillID_Format(t *testing.T) {
	// Test that the format is always "bill/{customerID}/{period}"
	customerID := "test-customer"
	period := BillingPeriod("2025-06")

	billID := MakeBillID(customerID, period)
	billIDStr := string(billID)

	// Check format components
	expectedPrefix := "bill/"
	if !contains(billIDStr, expectedPrefix) {
		t.Errorf("BillID should start with %q, got %q", expectedPrefix, billIDStr)
	}

	expectedCustomer := "/" + customerID + "/"
	if !contains(billIDStr, expectedCustomer) {
		t.Errorf("BillID should contain %q, got %q", expectedCustomer, billIDStr)
	}

	expectedPeriod := "/" + string(period)
	if !contains(billIDStr, expectedPeriod) {
		t.Errorf("BillID should end with %q, got %q", expectedPeriod, billIDStr)
	}
}

// Helper function for string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		len(s) > len(substr) && contains(s[1:], substr)
}
