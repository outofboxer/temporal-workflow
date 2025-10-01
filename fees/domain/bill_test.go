package domain

import (
	"errors"
	"fmt"
	"testing"
	"time"

	libmoney "github.com/outofboxer/temporal-workflow/libs/money"
)

func TestBill_Transitions(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() Bill
		action   func(*Bill) error
		expected BillStatus
		wantErr  bool
	}{
		{
			name: "Open to Pending",
			setup: func() Bill {
				return newTestBill(t, BillStatusOpen)
			},
			action: func(b *Bill) error {
				return b.Pending(time.Now())
			},
			expected: BillStatusPending,
			wantErr:  false,
		},
		{
			name: "Pending to Closed",
			setup: func() Bill {
				return newTestBill(t, BillStatusPending)
			},
			action: func(b *Bill) error {
				return b.Close(time.Now())
			},
			expected: BillStatusClosed,
			wantErr:  false,
		},
		{
			name: "Open to Error",
			setup: func() Bill {
				return newTestBill(t, BillStatusOpen)
			},
			action: func(b *Bill) error {
				return b.Error(time.Now())
			},
			expected: BillStatusError,
			wantErr:  false,
		},
		{
			name: "Closed to Pending (invalid)",
			setup: func() Bill {
				return newTestBill(t, BillStatusClosed)
			},
			action: func(b *Bill) error {
				return b.Pending(time.Now())
			},
			expected: BillStatusClosed,
			wantErr:  true,
		},
		{
			name: "Closed to Closed (idempotent)",
			setup: func() Bill {
				return newTestBill(t, BillStatusClosed)
			},
			action: func(b *Bill) error {
				return b.Close(time.Now())
			},
			expected: BillStatusClosed,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bill := tt.setup()
			err := tt.action(&bill)

			if (err != nil) != tt.wantErr {
				t.Errorf("Bill.%s() error = %v, wantErr %v", tt.name, err, tt.wantErr)
				return
			}

			if bill.Status != tt.expected {
				t.Errorf("Bill.Status = %v, want %v", bill.Status, tt.expected)
			}
		})
	}
}

func TestBill_AddItem_Idempotency(t *testing.T) {
	bill := newTestBill(t, BillStatusOpen)
	amount, _ := libmoney.NewFromString("10.50", libmoney.CurrencyUSD)
	now := time.Now()

	// First add
	err := bill.AddItem("key1", "description", amount, now)
	if err != nil {
		t.Fatalf("First add failed: %v", err)
	}

	if len(bill.Items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(bill.Items))
	}

	// Duplicate add (should be idempotent)
	err = bill.AddItem("key1", "different description", amount, now)
	if err != nil {
		t.Fatalf("Duplicate add failed: %v", err)
	}

	// Should still have only 1 item
	if len(bill.Items) != 1 {
		t.Fatalf("Expected 1 item after duplicate, got %d", len(bill.Items))
	}

	// Original item should be unchanged
	if bill.Items[0].Description != "description" {
		t.Errorf("Expected original description, got %s", bill.Items[0].Description)
	}
}

func TestBill_AddItem_CurrencyHandling(t *testing.T) {
	tests := []struct {
		name           string
		billCurrency   libmoney.Currency
		itemCurrency   libmoney.Currency
		expectedAmount string
		shouldSucceed  bool
	}{
		{
			name:           "Same currency USD",
			billCurrency:   libmoney.CurrencyUSD,
			itemCurrency:   libmoney.CurrencyUSD,
			expectedAmount: "10.5",
			shouldSucceed:  true,
		},
		{
			name:           "Same currency GEL",
			billCurrency:   libmoney.CurrencyGEL,
			itemCurrency:   libmoney.CurrencyGEL,
			expectedAmount: "10.5",
			shouldSucceed:  true,
		},
		{
			name:           "Different currency (USD to GEL)",
			billCurrency:   libmoney.CurrencyGEL,
			itemCurrency:   libmoney.CurrencyUSD,
			expectedAmount: "10.5", // Should be converted
			shouldSucceed:  true,
		},
		{
			name:           "Different currency (GEL to USD)",
			billCurrency:   libmoney.CurrencyUSD,
			itemCurrency:   libmoney.CurrencyGEL,
			expectedAmount: "10.5", // Should be converted
			shouldSucceed:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bill := newTestBillWithCurrency(t, tt.billCurrency)
			amount, _ := libmoney.NewFromString("10.50", tt.itemCurrency)
			now := time.Now()

			err := bill.AddItem("key1", "description", amount, now)

			if (err != nil) != !tt.shouldSucceed {
				t.Errorf("AddItem() error = %v, wantErr %v", err, !tt.shouldSucceed)
				return
			}

			if tt.shouldSucceed {
				if len(bill.Items) != 1 {
					t.Fatalf("Expected 1 item, got %d", len(bill.Items))
				}

				// Check that amount was converted to bill currency
				actualAmount := bill.Items[0].Amount.ToString()
				if actualAmount != tt.expectedAmount {
					t.Errorf("Expected amount %s, got %s", tt.expectedAmount, actualAmount)
				}
			}
		})
	}
}

func TestBill_AddItem_ClosedBillRejection(t *testing.T) {
	bill := newTestBill(t, BillStatusClosed)
	amount, _ := libmoney.NewFromString("10.50", libmoney.CurrencyUSD)
	now := time.Now()

	err := bill.AddItem("key1", "description", amount, now)
	if err == nil {
		t.Fatal("Expected error when adding to closed bill")
	}

	if !errors.Is(err, ErrBillNotOpen) {
		t.Errorf("Expected ErrBillNotOpen, got %v", err)
	}

	if len(bill.Items) != 0 {
		t.Errorf("Expected 0 items, got %d", len(bill.Items))
	}
}

func TestBill_AddItem_EmptyIdempotencyKey(t *testing.T) {
	bill := newTestBill(t, BillStatusOpen)
	amount, _ := libmoney.NewFromString("10.50", libmoney.CurrencyUSD)
	now := time.Now()

	err := bill.AddItem("", "description", amount, now)
	if err == nil {
		t.Fatal("Expected error for empty idempotency key")
	}

	if !errors.Is(err, ErrEmptyIdempotencyKey) {
		t.Errorf("Expected ErrEmptyIdempotencyKey, got %v", err)
	}
}

func TestBill_IsActive(t *testing.T) {
	tests := []struct {
		name     string
		status   BillStatus
		expected bool
	}{
		{"Open is active", BillStatusOpen, true},
		{"Pending is not active", BillStatusPending, false},
		{"Closed is not active", BillStatusClosed, false},
		{"Error is not active", BillStatusError, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bill := newTestBill(t, tt.status)
			if bill.IsActive() != tt.expected {
				t.Errorf("IsActive() = %v, want %v", bill.IsActive(), tt.expected)
			}
		})
	}
}

func TestBill_IsReadyForInvoicing(t *testing.T) {
	tests := []struct {
		name     string
		status   BillStatus
		expected bool
	}{
		{"Open is not ready", BillStatusOpen, false},
		{"Pending is ready", BillStatusPending, true},
		{"Closed is not ready", BillStatusClosed, false},
		{"Error is not ready", BillStatusError, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bill := newTestBill(t, tt.status)
			if bill.IsReadyForInvoicing() != tt.expected {
				t.Errorf("IsReadyForInvoicing() = %v, want %v", bill.IsReadyForInvoicing(), tt.expected)
			}
		})
	}
}

func TestBill_RecalcTotal(t *testing.T) {
	bill := newTestBill(t, BillStatusOpen)
	now := time.Now()

	// Add multiple items
	amounts := []string{"10.50", "5.25", "2.75"}
	for i, amt := range amounts {
		amount, _ := libmoney.NewFromString(amt, libmoney.CurrencyUSD)
		err := bill.AddItem(fmt.Sprintf("key%d", i), "description", amount, now)
		if err != nil {
			t.Fatalf("AddItem failed: %v", err)
		}
	}

	// Test recalc
	recalcTotal := bill.RecalcTotal()
	expectedTotal, _ := libmoney.NewFromString("18.50", libmoney.CurrencyUSD)

	if recalcTotal.Cmp(expectedTotal) != 0 {
		t.Errorf("RecalcTotal() = %s, want %s", recalcTotal.ToString(), expectedTotal.ToString())
	}
}

func TestBill_TotalConsistency(t *testing.T) {
	bill := newTestBill(t, BillStatusOpen)
	now := time.Now()

	// Add items and verify total is updated
	amounts := []string{"10.50", "5.25", "2.75"}
	expectedTotal := 0.0

	for i, amt := range amounts {
		amount, _ := libmoney.NewFromString(amt, libmoney.CurrencyUSD)
		err := bill.AddItem(fmt.Sprintf("key%d", i), "description", amount, now)
		if err != nil {
			t.Fatalf("AddItem failed: %v", err)
		}

		expectedTotal += 10.50 + 5.25 + 2.75

		// Verify total is consistent after each addition
		recalcTotal := bill.RecalcTotal()
		if bill.Total.Cmp(recalcTotal) != 0 {
			t.Errorf("Total inconsistency after item %d: stored=%s, recalc=%s",
				i, bill.Total.ToString(), recalcTotal.ToString())
		}
	}
}

func TestBill_StatusTransitions_CompleteFlow(t *testing.T) {
	bill := newTestBill(t, BillStatusOpen)
	now := time.Now()

	// Add some items
	amount, _ := libmoney.NewFromString("10.50", libmoney.CurrencyUSD)
	err := bill.AddItem("key1", "description", amount, now)
	if err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	// Open -> Pending
	err = bill.Pending(now)
	if err != nil {
		t.Fatalf("Pending failed: %v", err)
	}
	if bill.Status != BillStatusPending {
		t.Errorf("Expected Pending, got %s", bill.Status)
	}
	if !bill.IsReadyForInvoicing() {
		t.Error("Expected ready for invoicing")
	}

	// Pending -> Closed
	err = bill.Close(now)
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if bill.Status != BillStatusClosed {
		t.Errorf("Expected Closed, got %s", bill.Status)
	}
	if bill.IsActive() {
		t.Error("Expected not active after close")
	}
	if bill.FinalizedAt == nil {
		t.Error("Expected FinalizedAt to be set")
	}
}

// Helper functions
func newTestBill(t *testing.T, status BillStatus) Bill {
	t.Helper()
	now := time.Now()
	bill, err := NewBillBuilder().
		WithID(BillID("test-bill")).
		ForCustomer("test-customer").
		ForPeriod(BillingPeriod("2025-01")).
		WithCurrency(libmoney.CurrencyUSD).
		WithCreatedAt(now).
		Build()
	if err != nil {
		t.Fatalf("Failed to create bill: %v", err)
	}

	// Set status manually for testing
	bill.Status = status
	return bill
}

func newTestBillWithCurrency(t *testing.T, currency libmoney.Currency) Bill {
	t.Helper()
	now := time.Now()
	bill, err := NewBillBuilder().
		WithID(BillID("test-bill")).
		ForCustomer("test-customer").
		ForPeriod(BillingPeriod("2025-01")).
		WithCurrency(currency).
		WithCreatedAt(now).
		Open().
		Build()
	if err != nil {
		t.Fatalf("Failed to create bill: %v", err)
	}
	return bill
}
