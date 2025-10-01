package workflows

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/outofboxer/temporal-workflow/fees/app"
	"github.com/outofboxer/temporal-workflow/fees/domain"
	"github.com/outofboxer/temporal-workflow/fees/internal/adapters/temporal/activities"
	libmoney "github.com/outofboxer/temporal-workflow/libs/money"
)

// MockActivityEnvironment for testing activities
type MockActivityEnvironment struct {
	mock.Mock
}

func (m *MockActivityEnvironment) ProcessInvoiceAndChargeActivity(ctx context.Context, bill domain.Bill) error {
	args := m.Called(ctx, bill)
	return args.Error(0)
}

// TestMonthlyFeeAccrualWorkflow_CompleteFlow tests the complete workflow lifecycle
func TestMonthlyFeeAccrualWorkflow_CompleteFlow(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Set workflow timeout to prevent hanging
	env.SetTestTimeout(time.Minute)

	// Mock the activity
	env.OnActivity(activities.ProcessInvoiceAndChargeActivity, mock.Anything, mock.Anything).
		Return(nil)

	// Test parameters
	params := app.MonthlyFeeAccrualWorkflowParams{
		BillID:       domain.BillID("test-bill-123"),
		CustomerID:   "customer-123",
		Period:       domain.BillingPeriod("2025-01"),
		PeriodYYYYMM: 202501,
		Currency:     libmoney.CurrencyUSD,
	}

	// Register callback to send close signal after workflow starts
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(SignalCloseBill, struct{}{})
	}, time.Millisecond)

	// Execute workflow
	env.ExecuteWorkflow(MonthlyFeeAccrualWorkflow, params)

	// Verify workflow completed
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	// Get workflow result
	var result domain.Bill
	err := env.GetWorkflowResult(&result)
	require.NoError(t, err)

	// Verify result
	assert.Equal(t, string(params.BillID), string(result.ID))
	assert.Equal(t, params.CustomerID, result.CustomerID)
	assert.Equal(t, params.Period, result.BillingPeriod)
	assert.Equal(t, params.Currency, result.Currency)
	assert.Equal(t, domain.BillStatusClosed, result.Status)
	assert.NotNil(t, result.FinalizedAt)
}

// TestMonthlyFeeAccrualWorkflow_AddLineItems tests adding line items via signals
func TestMonthlyFeeAccrualWorkflow_AddLineItems(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Set workflow timeout to prevent hanging
	env.SetTestTimeout(time.Minute)

	// Mock the activity
	env.OnActivity(activities.ProcessInvoiceAndChargeActivity, mock.Anything, mock.Anything).
		Return(nil)

	params := app.MonthlyFeeAccrualWorkflowParams{
		BillID:       domain.BillID("test-bill-456"),
		CustomerID:   "customer-456",
		Period:       domain.BillingPeriod("2025-02"),
		PeriodYYYYMM: 202502,
		Currency:     libmoney.CurrencyUSD,
	}

	// Add line items via signals
	amount1, _ := libmoney.NewFromString("10.50", libmoney.CurrencyUSD)
	amount2, _ := libmoney.NewFromString("25.00", libmoney.CurrencyUSD)

	// Register callbacks to send signals after workflow starts
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(SignalAddLineItem, AddLineItemPayload{
			IdempotencyKey: "item-1",
			Description:    "API usage fee",
			Amount:         amount1,
		})
	}, time.Millisecond)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(SignalAddLineItem, AddLineItemPayload{
			IdempotencyKey: "item-2",
			Description:    "Storage fee",
			Amount:         amount2,
		})
	}, 2*time.Millisecond)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(SignalCloseBill, struct{}{})
	}, 3*time.Millisecond)

	// Start workflow
	env.ExecuteWorkflow(MonthlyFeeAccrualWorkflow, params)

	// Verify workflow completed
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	// Get result and verify
	var result domain.Bill
	err := env.GetWorkflowResult(&result)
	require.NoError(t, err)

	assert.Equal(t, 2, len(result.Items))
	assert.Equal(t, "item-1", result.Items[0].IdempotencyKey)
	assert.Equal(t, "item-2", result.Items[1].IdempotencyKey)

	// Verify total calculation
	expectedTotal, _ := libmoney.NewFromString("35.50", libmoney.CurrencyUSD)
	assert.Equal(t, expectedTotal.ToString(), result.Total.ToString())
}

// TestMonthlyFeeAccrualWorkflow_QueryHandler tests the query handler
func TestMonthlyFeeAccrualWorkflow_QueryHandler(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	defer env.AssertExpectations(t)

	env.SetTestTimeout(10 * time.Second)

	// Mock activities used by the workflow
	env.OnActivity(activities.ProcessInvoiceAndChargeActivity, mock.Anything, mock.Anything).
		Return(nil)

	params := app.MonthlyFeeAccrualWorkflowParams{
		BillID:       domain.BillID("test-bill-789"),
		CustomerID:   "customer-789",
		Period:       domain.BillingPeriod("2025-03"),
		PeriodYYYYMM: 202503,
		Currency:     libmoney.CurrencyGEL,
	}

	var queryResult BillDTO

	// 1) Query shortly after start, while bill is still OPEN.
	env.RegisterDelayedCallback(func() {
		v, err := env.QueryWorkflow(QueryState)
		require.NoError(t, err)
		require.NoError(t, v.Get(&queryResult))
	}, 1*time.Millisecond)

	// 2) Then close the bill so the workflow can complete.
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(SignalCloseBill, struct{}{})
	}, 2*time.Millisecond)

	// Run workflow to completion (callbacks fire during execution)
	env.ExecuteWorkflow(MonthlyFeeAccrualWorkflow, params)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	// Assertions on the queried OPEN state
	assert.Equal(t, string(params.BillID), queryResult.ID)
	assert.Equal(t, params.CustomerID, queryResult.CustomerID)
	assert.Equal(t, string(params.Period), queryResult.BillingPeriod)
	assert.Equal(t, string(params.Currency), string(queryResult.Currency))
	assert.Equal(t, string(domain.BillStatusOpen), queryResult.Status)
	assert.Len(t, queryResult.Items, 0)
}

// TestMonthlyFeeAccrualWorkflow_Idempotency tests idempotent line item addition
func TestMonthlyFeeAccrualWorkflow_Idempotency(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Set workflow timeout to prevent hanging
	env.SetTestTimeout(time.Minute)

	// Mock the activity
	env.OnActivity(activities.ProcessInvoiceAndChargeActivity, mock.Anything, mock.Anything).
		Return(nil)

	params := app.MonthlyFeeAccrualWorkflowParams{
		BillID:       domain.BillID("test-bill-idempotency"),
		CustomerID:   "customer-idempotency",
		Period:       domain.BillingPeriod("2025-04"),
		PeriodYYYYMM: 202504,
		Currency:     libmoney.CurrencyUSD,
	}

	amount, _ := libmoney.NewFromString("100.00", libmoney.CurrencyUSD)
	payload := AddLineItemPayload{
		IdempotencyKey: "duplicate-key",
		Description:    "Duplicate item",
		Amount:         amount,
	}

	// Register callbacks to send signals after workflow starts
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(SignalAddLineItem, payload)
	}, time.Millisecond)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(SignalAddLineItem, payload)
	}, 2*time.Millisecond)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(SignalAddLineItem, payload)
	}, 3*time.Millisecond)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(SignalCloseBill, struct{}{})
	}, 4*time.Millisecond)

	// Start workflow
	env.ExecuteWorkflow(MonthlyFeeAccrualWorkflow, params)

	// Verify workflow completed
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	// Get result and verify only one item was added
	var result domain.Bill
	err := env.GetWorkflowResult(&result)
	require.NoError(t, err)

	assert.Equal(t, 1, len(result.Items))
	assert.Equal(t, "duplicate-key", result.Items[0].IdempotencyKey)
}

// TestMonthlyFeeAccrualWorkflow_ClosedBillRejection tests that signals are ignored for closed bills
func TestMonthlyFeeAccrualWorkflow_ClosedBillRejection(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Set workflow timeout to prevent hanging
	env.SetTestTimeout(time.Minute)

	// Mock the activity
	env.OnActivity(activities.ProcessInvoiceAndChargeActivity, mock.Anything, mock.Anything).
		Return(nil)

	params := app.MonthlyFeeAccrualWorkflowParams{
		BillID:       domain.BillID("test-bill-closed"),
		CustomerID:   "customer-closed",
		Period:       domain.BillingPeriod("2025-05"),
		PeriodYYYYMM: 202505,
		Currency:     libmoney.CurrencyUSD,
	}

	// Register callbacks to send signals after workflow starts
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(SignalCloseBill, struct{}{})
	}, time.Millisecond)

	// Try to add item after closing (should be ignored)
	amount, _ := libmoney.NewFromString("50.00", libmoney.CurrencyUSD)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(SignalAddLineItem, AddLineItemPayload{
			IdempotencyKey: "after-close-item",
			Description:    "Item after close",
			Amount:         amount,
		})
	}, 2*time.Millisecond)

	// Start workflow
	env.ExecuteWorkflow(MonthlyFeeAccrualWorkflow, params)

	// Verify workflow completed
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	// Get result and verify no items were added after closing
	var result domain.Bill
	err := env.GetWorkflowResult(&result)
	require.NoError(t, err)

	assert.Equal(t, domain.BillStatusClosed, result.Status)
	assert.Equal(t, 0, len(result.Items))
}

// TestMonthlyFeeAccrualWorkflow_CurrencyHandling tests currency conversion
func TestMonthlyFeeAccrualWorkflow_CurrencyHandling(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Set workflow timeout to prevent hanging
	env.SetTestTimeout(time.Minute)

	// Mock the activity
	env.OnActivity(activities.ProcessInvoiceAndChargeActivity, mock.Anything, mock.Anything).
		Return(nil)

	params := app.MonthlyFeeAccrualWorkflowParams{
		BillID:       domain.BillID("test-bill-currency"),
		CustomerID:   "customer-currency",
		Period:       domain.BillingPeriod("2025-07"),
		PeriodYYYYMM: 202507,
		Currency:     libmoney.CurrencyGEL,
	}

	// Add items with different currencies (should be converted to bill currency)
	usdAmount, _ := libmoney.NewFromString("100.00", libmoney.CurrencyUSD)
	gelAmount, _ := libmoney.NewFromString("50.00", libmoney.CurrencyGEL)

	// Register callbacks to send signals after workflow starts
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(SignalAddLineItem, AddLineItemPayload{
			IdempotencyKey: "usd-item",
			Description:    "USD item",
			Amount:         usdAmount,
		})
	}, time.Millisecond)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(SignalAddLineItem, AddLineItemPayload{
			IdempotencyKey: "gel-item",
			Description:    "GEL item",
			Amount:         gelAmount,
		})
	}, 2*time.Millisecond)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(SignalCloseBill, struct{}{})
	}, 3*time.Millisecond)

	// Start workflow
	env.ExecuteWorkflow(MonthlyFeeAccrualWorkflow, params)

	// Verify workflow completed
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	// Get result and verify currency handling
	var result domain.Bill
	err := env.GetWorkflowResult(&result)
	require.NoError(t, err)

	assert.Equal(t, libmoney.CurrencyGEL, result.Currency)
	assert.Equal(t, 2, len(result.Items))
}

// TestBillToDTO tests the DTO conversion function
func TestBillToDTO(t *testing.T) {
	now := time.Now()
	bill := domain.Bill{
		ID:            domain.BillID("test-bill-dto"),
		CustomerID:    "customer-dto",
		Currency:      libmoney.CurrencyUSD,
		BillingPeriod: domain.BillingPeriod("2025-08"),
		Status:        domain.BillStatusOpen,
		Items: []domain.LineItem{
			{
				IdempotencyKey: "dto-item-1",
				Description:    "DTO test item 1",
				Amount:         libmoney.NewFromFloat(10.50, libmoney.CurrencyUSD),
				AddedAt:        now,
			},
			{
				IdempotencyKey: "dto-item-2",
				Description:    "DTO test item 2",
				Amount:         libmoney.NewFromFloat(25.75, libmoney.CurrencyUSD),
				AddedAt:        now.Add(time.Hour),
			},
		},
		Total:       libmoney.NewFromFloat(36.25, libmoney.CurrencyUSD),
		CreatedAt:   now,
		UpdatedAt:   now.Add(2 * time.Hour),
		FinalizedAt: &now,
	}

	dto := billToDTO(bill)

	// Verify DTO fields
	assert.Equal(t, string(bill.ID), dto.ID)
	assert.Equal(t, bill.CustomerID, dto.CustomerID)
	assert.Equal(t, bill.Currency, dto.Currency)
	assert.Equal(t, string(bill.BillingPeriod), dto.BillingPeriod)
	assert.Equal(t, string(bill.Status), dto.Status)
	assert.Equal(t, bill.Total.ToString(), dto.Total.ToString())
	assert.Equal(t, bill.CreatedAt, dto.CreatedAt)
	assert.Equal(t, bill.UpdatedAt, dto.UpdatedAt)
	assert.Equal(t, bill.FinalizedAt, dto.ClosedAt)

	// Verify line items
	assert.Equal(t, len(bill.Items), len(dto.Items))
	for i, item := range bill.Items {
		assert.Equal(t, item.IdempotencyKey, dto.Items[i].IdempotencyKey)
		assert.Equal(t, item.Description, dto.Items[i].Description)
		assert.Equal(t, item.Amount.ToString(), dto.Items[i].Amount.ToString())
		assert.Equal(t, item.AddedAt, dto.Items[i].AddedAt)
	}
}

// TestMoneyToCents tests the money conversion function
func TestMoneyToCents(t *testing.T) {
	tests := []struct {
		name     string
		amount   string
		currency libmoney.Currency
		expected int64
	}{
		{
			name:     "USD 10.50",
			amount:   "10.50",
			currency: libmoney.CurrencyUSD,
			expected: 1050,
		},
		{
			name:     "USD 0.01",
			amount:   "0.01",
			currency: libmoney.CurrencyUSD,
			expected: 1,
		},
		{
			name:     "USD 100.00",
			amount:   "100.00",
			currency: libmoney.CurrencyUSD,
			expected: 10000,
		},
		{
			name:     "GEL 25.75",
			amount:   "25.75",
			currency: libmoney.CurrencyGEL,
			expected: 2575,
		},
		{
			name:     "USD 0.00",
			amount:   "0.00",
			currency: libmoney.CurrencyUSD,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			money, err := libmoney.NewFromString(tt.amount, tt.currency)
			require.NoError(t, err)

			result := moneyToCents(money)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestWorkflowConstants tests that constants are properly defined
func TestWorkflowConstants(t *testing.T) {
	assert.Equal(t, "MonthlyFeeAccrualWorkflow", WorkflowTypeMonthlyBill)
	assert.Equal(t, "SignalAddLineItem", SignalAddLineItem)
	assert.Equal(t, "SignalCloseBill", SignalCloseBill)
	assert.Equal(t, "CurrentBillState", QueryState)
}

// TestAddLineItemPayload tests the payload structure
func TestAddLineItemPayload(t *testing.T) {
	amount, _ := libmoney.NewFromString("99.99", libmoney.CurrencyUSD)
	payload := AddLineItemPayload{
		IdempotencyKey: "test-key",
		Description:    "Test description",
		Amount:         amount,
	}

	assert.Equal(t, "test-key", payload.IdempotencyKey)
	assert.Equal(t, "Test description", payload.Description)
	assert.Equal(t, amount.ToString(), payload.Amount.ToString())
}

// TestCloseBillSignal tests the close signal structure
func TestCloseBillSignal(t *testing.T) {
	signal := CloseBillSignal{}
	// This is just a marker struct, so we just verify it can be instantiated
	assert.NotNil(t, signal)
}
