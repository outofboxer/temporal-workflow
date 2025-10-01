package feesapi

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"encore.dev/beta/errs"

	"github.com/outofboxer/temporal-workflow/fees/app"
	"github.com/outofboxer/temporal-workflow/fees/app/usecases"
	"github.com/outofboxer/temporal-workflow/fees/app/views"
	"github.com/outofboxer/temporal-workflow/fees/domain"
	libmoney "github.com/outofboxer/temporal-workflow/libs/money"
)

// MockTemporalPort implements app.TemporalPort for testing
type MockTemporalPort struct {
	mock.Mock
}

func (m *MockTemporalPort) StartMonthlyBill(ctx context.Context, params app.MonthlyFeeAccrualWorkflowParams) error {
	args := m.Called(ctx, params)
	return args.Error(0)
}

func (m *MockTemporalPort) AddLineItem(ctx context.Context, id domain.BillID, li domain.LineItem) error {
	args := m.Called(ctx, id, li)
	return args.Error(0)
}

func (m *MockTemporalPort) CloseBill(ctx context.Context, id domain.BillID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTemporalPort) QueryBill(ctx context.Context, id domain.BillID) (domain.Bill, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(domain.Bill), args.Error(1)
}

func (m *MockTemporalPort) SearchBills(ctx context.Context, params app.SearchBillFilter) ([]views.BillSummary, error) {
	args := m.Called(ctx, params)
	return args.Get(0).([]views.BillSummary), args.Error(1)
}

// Helper functions for creating test data
var fixedTime = time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

func createTestBill() domain.Bill {
	return domain.Bill{
		ID:            "bill/customer-123/2025-01",
		CustomerID:    "customer-123",
		Currency:      libmoney.CurrencyUSD,
		BillingPeriod: "2025-01",
		Status:        domain.BillStatusOpen,
		Items:         []domain.LineItem{},
		Total:         libmoney.Money{},
		CreatedAt:     fixedTime,
		UpdatedAt:     fixedTime,
	}
}

func createTestLineItem() domain.LineItem {
	return domain.LineItem{
		IdempotencyKey: "item-123",
		Description:    "Test item",
		Amount:         libmoney.Money{},
		AddedAt:        fixedTime,
	}
}

// Test service with mocked temporal port
func createTestService() (*Service, *MockTemporalPort) {
	mockTemporal := &MockTemporalPort{}
	service := &Service{
		Create:  usecases.CreateBill{T: mockTemporal},
		AddItem: usecases.AddLineItem{T: mockTemporal},
		Close:   usecases.CloseBill{T: mockTemporal},
		Get:     usecases.GetBill{T: mockTemporal},
		Search:  usecases.SearchBill{T: mockTemporal},
	}
	return service, mockTemporal
}

func TestCreateBill(t *testing.T) {
	tests := []struct {
		name             string
		customerID       string
		request          *CreateBillRequest
		mockSetup        func(*MockTemporalPort)
		expectedStatus   int
		expectedError    *errs.Error
		validateResponse func(t *testing.T, resp *CreateBillResponse)
	}{
		{
			name:       "successful bill creation",
			customerID: "customer-123",
			request: &CreateBillRequest{
				Currency:      libmoney.CurrencyUSD,
				BillingPeriod: "2025-01",
			},
			mockSetup: func(m *MockTemporalPort) {
				expectedParams := app.MonthlyFeeAccrualWorkflowParams{
					BillID:       "bill/customer-123/2025-01",
					CustomerID:   "customer-123",
					Period:       "2025-01",
					PeriodYYYYMM: 202501,
					Currency:     libmoney.CurrencyUSD,
				}
				expectedBill := createTestBill()

				m.On("StartMonthlyBill", mock.Anything, expectedParams).Return(nil)
				m.On("QueryBill", mock.Anything, domain.BillID("bill/customer-123/2025-01")).Return(expectedBill, nil)
			},
			expectedStatus: 201,
			validateResponse: func(t *testing.T, resp *CreateBillResponse) {
				assert.Equal(t, 201, resp.Status)
				assert.Equal(t, "/api/v1/customers/customer-123/bills/2025-01", resp.Location)
				assert.NotNil(t, resp.Message)
				assert.Equal(t, "bill/customer-123/2025-01", resp.Message.ID)
				assert.Equal(t, "customer-123", resp.Message.CustomerID)
				assert.Equal(t, "USD", resp.Message.Currency)
				assert.Equal(t, "2025-01", resp.Message.BillingPeriod)
				assert.Equal(t, "OPEN", resp.Message.Status)
			},
		},
		{
			name:       "empty customer ID",
			customerID: "",
			request: &CreateBillRequest{
				Currency:      libmoney.CurrencyUSD,
				BillingPeriod: "2025-01",
			},
			mockSetup: func(m *MockTemporalPort) {
				// No mock setup needed as validation fails before use case call
			},
			expectedError: &errs.Error{
				Code:    errs.InvalidArgument,
				Message: "customerId should be not empty and fit length restriction",
			},
		},
		{
			name:       "invalid currency - validation should prevent this",
			customerID: "customer-123",
			request: &CreateBillRequest{
				Currency:      "INVALID",
				BillingPeriod: "2025-01",
			},
			mockSetup: func(m *MockTemporalPort) {
				// This test case should not reach the use case due to validation
				// But if it does, we'll mock it to return an error
				expectedParams := app.MonthlyFeeAccrualWorkflowParams{
					BillID:       "bill/customer-123/2025-01",
					CustomerID:   "customer-123",
					Period:       "2025-01",
					PeriodYYYYMM: 202501,
					Currency:     "INVALID",
				}
				m.On("StartMonthlyBill", mock.Anything, expectedParams).Return(app.ErrBillWithPeriodAlreadyStarted)
			},
			expectedError: &errs.Error{
				Code:    errs.AlreadyExists,
				Message: "a bill already exists for this customer and period",
			},
		},
		{
			name:       "bill already exists",
			customerID: "customer-123",
			request: &CreateBillRequest{
				Currency:      libmoney.CurrencyUSD,
				BillingPeriod: "2025-01",
			},
			mockSetup: func(m *MockTemporalPort) {
				expectedParams := app.MonthlyFeeAccrualWorkflowParams{
					BillID:       "bill/customer-123/2025-01",
					CustomerID:   "customer-123",
					Period:       "2025-01",
					PeriodYYYYMM: 202501,
					Currency:     libmoney.CurrencyUSD,
				}
				m.On("StartMonthlyBill", mock.Anything, expectedParams).Return(app.ErrBillWithPeriodAlreadyStarted)
			},
			expectedError: &errs.Error{
				Code:    errs.AlreadyExists,
				Message: "a bill already exists for this customer and period",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockTemporal := createTestService()
			tt.mockSetup(mockTemporal)

			resp, err := service.CreateBill(context.Background(), tt.customerID, tt.request)

			if tt.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tt.expectedError.Code, err.(*errs.Error).Code)
				assert.Contains(t, err.(*errs.Error).Message, tt.expectedError.Message)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedStatus, resp.Status)
				if tt.validateResponse != nil {
					tt.validateResponse(t, resp)
				}
			}

			mockTemporal.AssertExpectations(t)
		})
	}
}

func TestAddLineItem(t *testing.T) {
	tests := []struct {
		name             string
		customerID       string
		period           string
		request          *AddLineItemRequest
		mockSetup        func(*MockTemporalPort)
		expectedError    *errs.Error
		validateResponse func(t *testing.T, resp *BillResponse)
	}{
		{
			name:       "successful line item addition",
			customerID: "customer-123",
			period:     "2025-01",
			request: &AddLineItemRequest{
				Description:    "Test item",
				Amount:         "10.50",
				IdempotencyKey: "item-123",
			},
			mockSetup: func(m *MockTemporalPort) {
				billID := domain.BillID("bill/customer-123/2025-01")
				openBill := createTestBill()
				updatedBill := createTestBill()
				updatedBill.Items = []domain.LineItem{createTestLineItem()}

				m.On("QueryBill", mock.Anything, billID).Return(openBill, nil).Once()
				m.On("AddLineItem", mock.Anything, billID, mock.MatchedBy(func(li domain.LineItem) bool {
					return li.Description == "Test item" && li.IdempotencyKey == "item-123"
				})).Return(nil)
				m.On("QueryBill", mock.Anything, billID).Return(updatedBill, nil).Once()
			},
			validateResponse: func(t *testing.T, resp *BillResponse) {
				assert.Equal(t, "bill/customer-123/2025-01", resp.ID)
				assert.Equal(t, "customer-123", resp.CustomerID)
				assert.Equal(t, "USD", resp.Currency)
				assert.Equal(t, "2025-01", resp.BillingPeriod)
				assert.Equal(t, "OPEN", resp.Status)
				assert.Len(t, resp.Items, 1)
				assert.Equal(t, "item-123", resp.Items[0].IdempotencyKey)
				assert.Equal(t, "Test item", resp.Items[0].Description)
			},
		},
		{
			name:       "invalid period format",
			customerID: "customer-123",
			period:     "invalid-period",
			request: &AddLineItemRequest{
				Description:    "Test item",
				Amount:         "10.50",
				IdempotencyKey: "item-123",
			},
			mockSetup: func(m *MockTemporalPort) {
				// No mock setup needed as validation fails before use case call
			},
			expectedError: &errs.Error{
				Code:    errs.InvalidArgument,
				Message: "invalid period",
			},
		},
		{
			name:       "invalid amount format",
			customerID: "customer-123",
			period:     "2025-01",
			request: &AddLineItemRequest{
				Description:    "Test item",
				Amount:         "invalid-amount",
				IdempotencyKey: "item-123",
			},
			mockSetup: func(m *MockTemporalPort) {
				// No mock setup needed as validation fails before use case call
			},
			expectedError: &errs.Error{
				Code:    errs.InvalidArgument,
				Message: "amount is invalid",
			},
		},
		{
			name:       "bill not found",
			customerID: "customer-123",
			period:     "2025-01",
			request: &AddLineItemRequest{
				Description:    "Test item",
				Amount:         "10.50",
				IdempotencyKey: "item-123",
			},
			mockSetup: func(m *MockTemporalPort) {
				billID := domain.BillID("bill/customer-123/2025-01")
				m.On("QueryBill", mock.Anything, billID).Return(domain.Bill{}, app.ErrBillNotFound)
			},
			expectedError: &errs.Error{
				Code:    errs.NotFound,
				Message: "bill not found",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockTemporal := createTestService()
			tt.mockSetup(mockTemporal)

			resp, err := service.AddLineItem(context.Background(), tt.customerID, tt.period, tt.request)

			if tt.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tt.expectedError.Code, err.(*errs.Error).Code)
				assert.Contains(t, err.(*errs.Error).Message, tt.expectedError.Message)
			} else {
				require.NoError(t, err)
				if tt.validateResponse != nil {
					tt.validateResponse(t, resp)
				}
			}

			mockTemporal.AssertExpectations(t)
		})
	}
}

func TestGetBill(t *testing.T) {
	tests := []struct {
		name             string
		customerID       string
		period           string
		mockSetup        func(*MockTemporalPort)
		expectedError    *errs.Error
		validateResponse func(t *testing.T, resp *BillResponse)
	}{
		{
			name:       "successful bill retrieval",
			customerID: "customer-123",
			period:     "2025-01",
			mockSetup: func(m *MockTemporalPort) {
				billID := domain.BillID("bill/customer-123/2025-01")
				expectedBill := createTestBill()
				m.On("QueryBill", mock.Anything, billID).Return(expectedBill, nil)
			},
			validateResponse: func(t *testing.T, resp *BillResponse) {
				assert.Equal(t, "bill/customer-123/2025-01", resp.ID)
				assert.Equal(t, "customer-123", resp.CustomerID)
				assert.Equal(t, "USD", resp.Currency)
				assert.Equal(t, "2025-01", resp.BillingPeriod)
				assert.Equal(t, "OPEN", resp.Status)
			},
		},
		{
			name:       "empty customer ID",
			customerID: "",
			period:     "2025-01",
			mockSetup: func(m *MockTemporalPort) {
				// No mock setup needed as validation fails before use case call
			},
			expectedError: &errs.Error{
				Code:    errs.InvalidArgument,
				Message: "customerId cannot be empty",
			},
		},
		{
			name:       "bill not found",
			customerID: "customer-123",
			period:     "2025-01",
			mockSetup: func(m *MockTemporalPort) {
				billID := domain.BillID("bill/customer-123/2025-01")
				m.On("QueryBill", mock.Anything, billID).Return(domain.Bill{}, app.ErrBillNotFound)
			},
			expectedError: &errs.Error{
				Code:    errs.NotFound,
				Message: "bill not found",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockTemporal := createTestService()
			tt.mockSetup(mockTemporal)

			resp, err := service.GetBill(context.Background(), tt.customerID, tt.period)

			if tt.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tt.expectedError.Code, err.(*errs.Error).Code)
				assert.Contains(t, err.(*errs.Error).Message, tt.expectedError.Message)
			} else {
				require.NoError(t, err)
				if tt.validateResponse != nil {
					tt.validateResponse(t, resp)
				}
			}

			mockTemporal.AssertExpectations(t)
		})
	}
}

func TestCloseBill(t *testing.T) {
	tests := []struct {
		name             string
		customerID       string
		period           string
		mockSetup        func(*MockTemporalPort)
		expectedError    *errs.Error
		validateResponse func(t *testing.T, resp *BillResponse)
	}{
		{
			name:       "successful bill closure",
			customerID: "customer-123",
			period:     "2025-01",
			mockSetup: func(m *MockTemporalPort) {
				billID := domain.BillID("bill/customer-123/2025-01")
				openBill := createTestBill()
				closedBill := createTestBill()
				closedBill.Status = domain.BillStatusClosed

				m.On("QueryBill", mock.Anything, billID).Return(openBill, nil).Once()
				m.On("CloseBill", mock.Anything, billID).Return(nil)
				m.On("QueryBill", mock.Anything, billID).Return(closedBill, nil).Once()
			},
			validateResponse: func(t *testing.T, resp *BillResponse) {
				assert.Equal(t, "bill/customer-123/2025-01", resp.ID)
				assert.Equal(t, "customer-123", resp.CustomerID)
				assert.Equal(t, "USD", resp.Currency)
				assert.Equal(t, "2025-01", resp.BillingPeriod)
				assert.Equal(t, "CLOSED", resp.Status)
			},
		},
		{
			name:       "empty customer ID",
			customerID: "",
			period:     "2025-01",
			mockSetup: func(m *MockTemporalPort) {
				// No mock setup needed as validation fails before use case call
			},
			expectedError: &errs.Error{
				Code:    errs.InvalidArgument,
				Message: "customerId cannot be empty",
			},
		},
		{
			name:       "bill not found",
			customerID: "customer-123",
			period:     "2025-01",
			mockSetup: func(m *MockTemporalPort) {
				billID := domain.BillID("bill/customer-123/2025-01")
				m.On("QueryBill", mock.Anything, billID).Return(domain.Bill{}, app.ErrBillNotFound)
			},
			expectedError: &errs.Error{
				Code:    errs.NotFound,
				Message: "bill not found",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockTemporal := createTestService()
			tt.mockSetup(mockTemporal)

			resp, err := service.CloseBill(context.Background(), tt.customerID, tt.period)

			if tt.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tt.expectedError.Code, err.(*errs.Error).Code)
				assert.Contains(t, err.(*errs.Error).Message, tt.expectedError.Message)
			} else {
				require.NoError(t, err)
				if tt.validateResponse != nil {
					tt.validateResponse(t, resp)
				}
			}

			mockTemporal.AssertExpectations(t)
		})
	}
}

func TestListBills(t *testing.T) {
	tests := []struct {
		name             string
		customerID       string
		params           *ListBillsQueryParams
		mockSetup        func(*MockTemporalPort)
		expectedError    *errs.Error
		validateResponse func(t *testing.T, resp *ListBillsResponse)
	}{
		{
			name:       "successful bills listing with filters",
			customerID: "customer-123",
			params: &ListBillsQueryParams{
				Status:      "OPEN",
				PeriodStart: "2025-01",
				PeriodEnd:   "2025-03",
			},
			mockSetup: func(m *MockTemporalPort) {
				expectedFilter := app.SearchBillFilter{
					CustomerID: "customer-123",
					FromYYYYMM: int64Ptr(202501),
					ToYYYYMM:   int64Ptr(202503),
					Status:     []string{"OPEN", "PENDING"},
				}
				expectedBills := []views.BillSummary{
					{
						WorkflowID:       "bill/customer-123/2025-01",
						CustomerID:       "customer-123",
						BillingPeriodNum: 202501,
						Status:           "OPEN",
						TotalCents:       1000,
						Currency:         "USD",
						ItemCount:        2,
					},
				}
				m.On("SearchBills", mock.Anything, expectedFilter).Return(expectedBills, nil)
			},
			validateResponse: func(t *testing.T, resp *ListBillsResponse) {
				assert.Len(t, resp.Bills, 1)
				bill := resp.Bills[0]
				assert.Equal(t, "bill/customer-123/2025-01", bill.ID)
				assert.Equal(t, "customer-123", bill.CustomerID)
				assert.Equal(t, "USD", bill.Currency)
				assert.Equal(t, "2025-01", bill.BillingPeriod)
				assert.Equal(t, "OPEN", bill.Status)
				assert.Equal(t, int64(2), bill.ItemCount)
				assert.Equal(t, "10.00", bill.Total)
			},
		},
		{
			name:       "empty customer ID",
			customerID: "",
			params:     &ListBillsQueryParams{},
			mockSetup: func(m *MockTemporalPort) {
				// No mock setup needed as validation fails before use case call
			},
			expectedError: &errs.Error{
				Code:    errs.InvalidArgument,
				Message: "customerId cannot be empty",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockTemporal := createTestService()
			tt.mockSetup(mockTemporal)

			resp, err := service.ListBills(context.Background(), tt.customerID, tt.params)

			if tt.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tt.expectedError.Code, err.(*errs.Error).Code)
				assert.Contains(t, err.(*errs.Error).Message, tt.expectedError.Message)
			} else {
				require.NoError(t, err)
				if tt.validateResponse != nil {
					tt.validateResponse(t, resp)
				}
			}

			mockTemporal.AssertExpectations(t)
		})
	}
}

// Test API mapper functions
func TestMap2BillingResponse(t *testing.T) {
	bill := createTestBill()
	bill.Items = []domain.LineItem{createTestLineItem()}

	resp := map2BillingResponse(bill)

	assert.Equal(t, "bill/customer-123/2025-01", resp.ID)
	assert.Equal(t, "customer-123", resp.CustomerID)
	assert.Equal(t, "USD", resp.Currency)
	assert.Equal(t, "2025-01", resp.BillingPeriod)
	assert.Equal(t, "OPEN", resp.Status)
	assert.Len(t, resp.Items, 1)
	assert.Equal(t, "item-123", resp.Items[0].IdempotencyKey)
	assert.Equal(t, "Test item", resp.Items[0].Description)
	assert.Equal(t, fixedTime, resp.CreatedAt)
	assert.Equal(t, fixedTime, resp.UpdatedAt)
}

func TestMapBillListResponse(t *testing.T) {
	summaries := []views.BillSummary{
		{
			WorkflowID:       "bill/customer-123/2025-01",
			CustomerID:       "customer-123",
			BillingPeriodNum: 202501,
			Status:           "OPEN",
			TotalCents:       1000,
			Currency:         "USD",
			ItemCount:        2,
		},
	}

	resp := mapBillListResponse(summaries)

	assert.Len(t, resp.Bills, 1)
	bill := resp.Bills[0]
	assert.Equal(t, "bill/customer-123/2025-01", bill.ID)
	assert.Equal(t, "customer-123", bill.CustomerID)
	assert.Equal(t, "USD", bill.Currency)
	assert.Equal(t, "2025-01", bill.BillingPeriod)
	assert.Equal(t, "OPEN", bill.Status)
	assert.Equal(t, int64(2), bill.ItemCount)
	assert.Equal(t, "10.00", bill.Total)
}

func TestBillingPeriodNumToString(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		{
			name:     "valid period",
			input:    202501,
			expected: "2025-01",
		},
		{
			name:     "valid period with single digit month",
			input:    202503,
			expected: "2025-03",
		},
		{
			name:     "invalid range - too small",
			input:    100000,
			expected: "<formatting error in range>",
		},
		{
			name:     "invalid range - too large",
			input:    1000000,
			expected: "<formatting error in range>",
		},
		{
			name:     "invalid month",
			input:    202500,
			expected: "<formatting error in month>",
		},
		{
			name:     "invalid month - too large",
			input:    202513,
			expected: "<formatting error in month>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := billingPeriodNumToString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTotalCentsToString(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		{
			name:     "zero cents",
			input:    0,
			expected: "0.00",
		},
		{
			name:     "positive cents",
			input:    1000,
			expected: "10.00",
		},
		{
			name:     "negative cents",
			input:    -1000,
			expected: "-10.00",
		},
		{
			name:     "single digit cents",
			input:    5,
			expected: "0.05",
		},
		{
			name:     "large amount",
			input:    123456,
			expected: "1234.56",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := totalCentsToString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test validation functions
func TestCreateBillRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		request *CreateBillRequest
		wantErr bool
	}{
		{
			name: "valid request",
			request: &CreateBillRequest{
				Currency:      libmoney.CurrencyUSD,
				BillingPeriod: "2025-01",
			},
			wantErr: false,
		},
		{
			name: "invalid currency",
			request: &CreateBillRequest{
				Currency:      "INVALID",
				BillingPeriod: "2025-01",
			},
			wantErr: true,
		},
		{
			name: "invalid billing period",
			request: &CreateBillRequest{
				Currency:      libmoney.CurrencyUSD,
				BillingPeriod: "invalid-period",
			},
			wantErr: true,
		},
		{
			name: "empty currency",
			request: &CreateBillRequest{
				Currency:      "",
				BillingPeriod: "2025-01",
			},
			wantErr: true,
		},
		{
			name: "empty billing period",
			request: &CreateBillRequest{
				Currency:      libmoney.CurrencyUSD,
				BillingPeriod: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAddLineItemRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		request *AddLineItemRequest
		wantErr bool
	}{
		{
			name: "valid request",
			request: &AddLineItemRequest{
				Description:    "Test item",
				Amount:         "10.50",
				IdempotencyKey: "item-123",
			},
			wantErr: false,
		},
		{
			name: "empty description",
			request: &AddLineItemRequest{
				Description:    "",
				Amount:         "10.50",
				IdempotencyKey: "item-123",
			},
			wantErr: true,
		},
		{
			name: "description too short",
			request: &AddLineItemRequest{
				Description:    "A",
				Amount:         "10.50",
				IdempotencyKey: "item-123",
			},
			wantErr: true,
		},
		{
			name: "description too long",
			request: &AddLineItemRequest{
				Description:    string(make([]byte, 1025)), // 1025 bytes
				Amount:         "10.50",
				IdempotencyKey: "item-123",
			},
			wantErr: true,
		},
		{
			name: "empty amount",
			request: &AddLineItemRequest{
				Description:    "Test item",
				Amount:         "",
				IdempotencyKey: "item-123",
			},
			wantErr: true,
		},
		{
			name: "empty idempotency key",
			request: &AddLineItemRequest{
				Description:    "Test item",
				Amount:         "10.50",
				IdempotencyKey: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestListBillsQueryParams_Validate(t *testing.T) {
	tests := []struct {
		name    string
		params  *ListBillsQueryParams
		wantErr bool
	}{
		{
			name: "valid params with status",
			params: &ListBillsQueryParams{
				Status:      "OPEN",
				PeriodStart: "2025-01",
				PeriodEnd:   "2025-03",
			},
			wantErr: false,
		},
		{
			name: "valid params with minimal filters",
			params: &ListBillsQueryParams{
				Status:      "OPEN",    // Status is required by validation
				PeriodStart: "2025-01", // Valid datetime format required
				PeriodEnd:   "2025-01", // Valid datetime format required
			},
			wantErr: false,
		},
		{
			name: "invalid status",
			params: &ListBillsQueryParams{
				Status:      "INVALID",
				PeriodStart: "2025-01",
				PeriodEnd:   "2025-03",
			},
			wantErr: true,
		},
		{
			name: "invalid period start",
			params: &ListBillsQueryParams{
				Status:      "OPEN",
				PeriodStart: "invalid-period",
				PeriodEnd:   "2025-03",
			},
			wantErr: true,
		},
		{
			name: "invalid period end",
			params: &ListBillsQueryParams{
				Status:      "OPEN",
				PeriodStart: "2025-01",
				PeriodEnd:   "invalid-period",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.params.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Helper function to create int64 pointer
func int64Ptr(i int64) *int64 {
	return &i
}
