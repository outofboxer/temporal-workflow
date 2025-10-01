package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/outofboxer/temporal-workflow/fees/app"
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

func TestCreateBill_Handle(t *testing.T) {
	tests := []struct {
		name           string
		cmd            CreateBillCmd
		mockSetup      func(*MockTemporalPort)
		expectedError  string
		expectedResult domain.Bill
	}{
		{
			name: "successful bill creation",
			cmd: CreateBillCmd{
				CustomerID: "customer-123",
				Period:     "2025-01",
				Currency:   libmoney.CurrencyUSD,
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
			expectedResult: createTestBill(),
		},
		{
			name: "invalid period format",
			cmd: CreateBillCmd{
				CustomerID: "customer-123",
				Period:     "invalid-period",
				Currency:   libmoney.CurrencyUSD,
			},
			mockSetup: func(m *MockTemporalPort) {
				// No mock setup needed as error occurs before Temporal calls
			},
			expectedError: "period formatting error",
		},
		{
			name: "temporal start workflow error",
			cmd: CreateBillCmd{
				CustomerID: "customer-123",
				Period:     "2025-01",
				Currency:   libmoney.CurrencyUSD,
			},
			mockSetup: func(m *MockTemporalPort) {
				expectedParams := app.MonthlyFeeAccrualWorkflowParams{
					BillID:       "bill/customer-123/2025-01",
					CustomerID:   "customer-123",
					Period:       "2025-01",
					PeriodYYYYMM: 202501,
					Currency:     libmoney.CurrencyUSD,
				}

				m.On("StartMonthlyBill", mock.Anything, expectedParams).Return(errors.New("workflow start failed"))
			},
			expectedError: "workflow start failed",
		},
		{
			name: "query bill error after successful start",
			cmd: CreateBillCmd{
				CustomerID: "customer-123",
				Period:     "2025-01",
				Currency:   libmoney.CurrencyUSD,
			},
			mockSetup: func(m *MockTemporalPort) {
				expectedParams := app.MonthlyFeeAccrualWorkflowParams{
					BillID:       "bill/customer-123/2025-01",
					CustomerID:   "customer-123",
					Period:       "2025-01",
					PeriodYYYYMM: 202501,
					Currency:     libmoney.CurrencyUSD,
				}

				m.On("StartMonthlyBill", mock.Anything, expectedParams).Return(nil)
				m.On("QueryBill", mock.Anything, domain.BillID("bill/customer-123/2025-01")).Return(domain.Bill{}, errors.New("query failed"))
			},
			expectedError: "query failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTemporal := &MockTemporalPort{}
			tt.mockSetup(mockTemporal)

			uc := CreateBill{T: mockTemporal}
			result, err := uc.Handle(context.Background(), tt.cmd)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}

			mockTemporal.AssertExpectations(t)
		})
	}
}

func TestAddLineItem_Handle(t *testing.T) {
	tests := []struct {
		name           string
		cmd            AddLineItemCmd
		mockSetup      func(*MockTemporalPort)
		expectedError  string
		expectedResult domain.Bill
	}{
		{
			name: "successful line item addition",
			cmd: AddLineItemCmd{
				CustomerID: "customer-123",
				Period:     "2025-01",
				Item:       createTestLineItem(),
			},
			mockSetup: func(m *MockTemporalPort) {
				billID := domain.BillID("bill/customer-123/2025-01")
				openBill := createTestBill()
				updatedBill := createTestBill()
				updatedBill.Items = []domain.LineItem{createTestLineItem()}

				m.On("QueryBill", mock.Anything, billID).Return(openBill, nil).Once()
				m.On("AddLineItem", mock.Anything, billID, mock.MatchedBy(func(li domain.LineItem) bool {
					return li.IdempotencyKey == "item-123" && li.Description == "Test item"
				})).Return(nil)
				m.On("QueryBill", mock.Anything, billID).Return(updatedBill, nil).Once()
			},
			expectedResult: func() domain.Bill {
				bill := createTestBill()
				bill.Items = []domain.LineItem{createTestLineItem()}
				return bill
			}(),
		},
		{
			name: "bill not found",
			cmd: AddLineItemCmd{
				CustomerID: "customer-123",
				Period:     "2025-01",
				Item:       createTestLineItem(),
			},
			mockSetup: func(m *MockTemporalPort) {
				billID := domain.BillID("bill/customer-123/2025-01")
				m.On("QueryBill", mock.Anything, billID).Return(domain.Bill{}, app.ErrBillNotFound)
			},
			expectedError: app.ErrBillNotFound.Error(),
		},
		{
			name: "bill already closed",
			cmd: AddLineItemCmd{
				CustomerID: "customer-123",
				Period:     "2025-01",
				Item:       createTestLineItem(),
			},
			mockSetup: func(m *MockTemporalPort) {
				billID := domain.BillID("bill/customer-123/2025-01")
				closedBill := createTestBill()
				closedBill.Status = domain.BillStatusClosed

				m.On("QueryBill", mock.Anything, billID).Return(closedBill, nil)
			},
			expectedError: app.ErrBillAlreadyClosed.Error(),
		},
		{
			name: "line item already added (idempotency)",
			cmd: AddLineItemCmd{
				CustomerID: "customer-123",
				Period:     "2025-01",
				Item:       createTestLineItem(),
			},
			mockSetup: func(m *MockTemporalPort) {
				billID := domain.BillID("bill/customer-123/2025-01")
				billWithItem := createTestBill()
				billWithItem.Items = []domain.LineItem{createTestLineItem()}

				m.On("QueryBill", mock.Anything, billID).Return(billWithItem, nil)
			},
			expectedError: app.ErrLineItemAlreadyAdded.Error(),
		},
		{
			name: "temporal add line item error",
			cmd: AddLineItemCmd{
				CustomerID: "customer-123",
				Period:     "2025-01",
				Item:       createTestLineItem(),
			},
			mockSetup: func(m *MockTemporalPort) {
				billID := domain.BillID("bill/customer-123/2025-01")
				openBill := createTestBill()

				m.On("QueryBill", mock.Anything, billID).Return(openBill, nil).Once()
				m.On("AddLineItem", mock.Anything, billID, mock.MatchedBy(func(li domain.LineItem) bool {
					return li.IdempotencyKey == "item-123" && li.Description == "Test item"
				})).Return(errors.New("signal failed"))
			},
			expectedError: "signal failed",
		},
		{
			name: "query bill error after successful add",
			cmd: AddLineItemCmd{
				CustomerID: "customer-123",
				Period:     "2025-01",
				Item:       createTestLineItem(),
			},
			mockSetup: func(m *MockTemporalPort) {
				billID := domain.BillID("bill/customer-123/2025-01")
				openBill := createTestBill()

				m.On("QueryBill", mock.Anything, billID).Return(openBill, nil).Once()
				m.On("AddLineItem", mock.Anything, billID, mock.MatchedBy(func(li domain.LineItem) bool {
					return li.IdempotencyKey == "item-123" && li.Description == "Test item"
				})).Return(nil)
				m.On("QueryBill", mock.Anything, billID).Return(domain.Bill{}, errors.New("query failed"))
			},
			expectedError: "query failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTemporal := &MockTemporalPort{}
			tt.mockSetup(mockTemporal)

			uc := AddLineItem{T: mockTemporal}
			result, err := uc.Handle(context.Background(), tt.cmd)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}

			mockTemporal.AssertExpectations(t)
		})
	}
}

func TestCloseBill_Handle(t *testing.T) {
	tests := []struct {
		name           string
		cmd            CloseBillCmd
		mockSetup      func(*MockTemporalPort)
		expectedError  string
		expectedResult domain.Bill
	}{
		{
			name: "successful bill closure",
			cmd: CloseBillCmd{
				CustomerID: "customer-123",
				Period:     "2025-01",
			},
			mockSetup: func(m *MockTemporalPort) {
				billID := domain.BillID("bill/customer-123/2025-01")
				openBill := createTestBill()
				closedBill := createTestBill()
				closedBill.Status = domain.BillStatusClosed

				m.On("QueryBill", mock.Anything, billID).Return(openBill, nil).Once()
				m.On("CloseBill", mock.Anything, billID).Return(nil)
				m.On("QueryBill", mock.Anything, billID).Return(closedBill, nil).Once()
			},
			expectedResult: func() domain.Bill {
				bill := createTestBill()
				bill.Status = domain.BillStatusClosed
				return bill
			}(),
		},
		{
			name: "bill not found",
			cmd: CloseBillCmd{
				CustomerID: "customer-123",
				Period:     "2025-01",
			},
			mockSetup: func(m *MockTemporalPort) {
				billID := domain.BillID("bill/customer-123/2025-01")
				m.On("QueryBill", mock.Anything, billID).Return(domain.Bill{}, app.ErrBillNotFound)
			},
			expectedError: app.ErrBillNotFound.Error(),
		},
		{
			name: "bill already closed",
			cmd: CloseBillCmd{
				CustomerID: "customer-123",
				Period:     "2025-01",
			},
			mockSetup: func(m *MockTemporalPort) {
				billID := domain.BillID("bill/customer-123/2025-01")
				closedBill := createTestBill()
				closedBill.Status = domain.BillStatusClosed

				m.On("QueryBill", mock.Anything, billID).Return(closedBill, nil)
			},
			expectedError: app.ErrBillAlreadyClosed.Error(),
		},
		{
			name: "bill in pending status",
			cmd: CloseBillCmd{
				CustomerID: "customer-123",
				Period:     "2025-01",
			},
			mockSetup: func(m *MockTemporalPort) {
				billID := domain.BillID("bill/customer-123/2025-01")
				pendingBill := createTestBill()
				pendingBill.Status = domain.BillStatusPending

				m.On("QueryBill", mock.Anything, billID).Return(pendingBill, nil)
			},
			expectedError: app.ErrBillAlreadyClosed.Error(),
		},
		{
			name: "temporal close bill error",
			cmd: CloseBillCmd{
				CustomerID: "customer-123",
				Period:     "2025-01",
			},
			mockSetup: func(m *MockTemporalPort) {
				billID := domain.BillID("bill/customer-123/2025-01")
				openBill := createTestBill()

				m.On("QueryBill", mock.Anything, billID).Return(openBill, nil).Once()
				m.On("CloseBill", mock.Anything, billID).Return(errors.New("signal failed"))
			},
			expectedError: "signal failed",
		},
		{
			name: "query bill error after successful close",
			cmd: CloseBillCmd{
				CustomerID: "customer-123",
				Period:     "2025-01",
			},
			mockSetup: func(m *MockTemporalPort) {
				billID := domain.BillID("bill/customer-123/2025-01")
				openBill := createTestBill()

				m.On("QueryBill", mock.Anything, billID).Return(openBill, nil).Once()
				m.On("CloseBill", mock.Anything, billID).Return(nil)
				m.On("QueryBill", mock.Anything, billID).Return(domain.Bill{}, errors.New("query failed"))
			},
			expectedError: "query failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTemporal := &MockTemporalPort{}
			tt.mockSetup(mockTemporal)

			uc := CloseBill{T: mockTemporal}
			result, err := uc.Handle(context.Background(), tt.cmd)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}

			mockTemporal.AssertExpectations(t)
		})
	}
}

func TestGetBill_Handle(t *testing.T) {
	tests := []struct {
		name           string
		cmd            GetBillCmd
		mockSetup      func(*MockTemporalPort)
		expectedError  string
		expectedResult domain.Bill
	}{
		{
			name: "successful bill retrieval",
			cmd: GetBillCmd{
				CustomerID: "customer-123",
				Period:     "2025-01",
			},
			mockSetup: func(m *MockTemporalPort) {
				billID := domain.BillID("bill/customer-123/2025-01")
				expectedBill := createTestBill()

				m.On("QueryBill", mock.Anything, billID).Return(expectedBill, nil)
			},
			expectedResult: createTestBill(),
		},
		{
			name: "bill not found",
			cmd: GetBillCmd{
				CustomerID: "customer-123",
				Period:     "2025-01",
			},
			mockSetup: func(m *MockTemporalPort) {
				billID := domain.BillID("bill/customer-123/2025-01")
				m.On("QueryBill", mock.Anything, billID).Return(domain.Bill{}, app.ErrBillNotFound)
			},
			expectedError: app.ErrBillNotFound.Error(),
		},
		{
			name: "temporal query error",
			cmd: GetBillCmd{
				CustomerID: "customer-123",
				Period:     "2025-01",
			},
			mockSetup: func(m *MockTemporalPort) {
				billID := domain.BillID("bill/customer-123/2025-01")
				m.On("QueryBill", mock.Anything, billID).Return(domain.Bill{}, errors.New("query failed"))
			},
			expectedError: "query failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTemporal := &MockTemporalPort{}
			tt.mockSetup(mockTemporal)

			uc := GetBill{T: mockTemporal}
			result, err := uc.Handle(context.Background(), tt.cmd)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}

			mockTemporal.AssertExpectations(t)
		})
	}
}

func TestSearchBill_Handle(t *testing.T) {
	tests := []struct {
		name           string
		cmd            SearchBillCmd
		mockSetup      func(*MockTemporalPort)
		expectedError  string
		expectedResult []views.BillSummary
	}{
		{
			name: "successful search with open status",
			cmd: SearchBillCmd{
				CustomerID: "customer-123",
				PeriodFrom: "2025-01",
				PeriodTo:   "2025-03",
				Status:     string(domain.BillStatusOpen),
			},
			mockSetup: func(m *MockTemporalPort) {
				expectedFilter := app.SearchBillFilter{
					CustomerID: "customer-123",
					FromYYYYMM: int64Ptr(202501),
					ToYYYYMM:   int64Ptr(202503),
					Status:     []string{string(domain.BillStatusOpen), string(domain.BillStatusPending)},
				}
				expectedResults := []views.BillSummary{
					{
						WorkflowID:       "bill/customer-123/2025-01",
						CustomerID:       "customer-123",
						BillingPeriodNum: 202501,
						Status:           string(domain.BillStatusOpen),
						TotalCents:       1000,
						Currency:         string(libmoney.CurrencyUSD),
					},
				}

				m.On("SearchBills", mock.Anything, expectedFilter).Return(expectedResults, nil)
			},
			expectedResult: []views.BillSummary{
				{
					WorkflowID:       "bill/customer-123/2025-01",
					CustomerID:       "customer-123",
					BillingPeriodNum: 202501,
					Status:           string(domain.BillStatusOpen),
					TotalCents:       1000,
					Currency:         string(libmoney.CurrencyUSD),
				},
			},
		},
		{
			name: "successful search with closed status",
			cmd: SearchBillCmd{
				CustomerID: "customer-123",
				PeriodFrom: "2025-01",
				PeriodTo:   "2025-03",
				Status:     string(domain.BillStatusClosed),
			},
			mockSetup: func(m *MockTemporalPort) {
				expectedFilter := app.SearchBillFilter{
					CustomerID: "customer-123",
					FromYYYYMM: int64Ptr(202501),
					ToYYYYMM:   int64Ptr(202503),
					Status:     []string{string(domain.BillStatusClosed)},
				}
				expectedResults := []views.BillSummary{
					{
						WorkflowID:       "bill/customer-123/2025-01",
						CustomerID:       "customer-123",
						BillingPeriodNum: 202501,
						Status:           string(domain.BillStatusClosed),
						TotalCents:       2000,
						Currency:         string(libmoney.CurrencyUSD),
					},
				}

				m.On("SearchBills", mock.Anything, expectedFilter).Return(expectedResults, nil)
			},
			expectedResult: []views.BillSummary{
				{
					WorkflowID:       "bill/customer-123/2025-01",
					CustomerID:       "customer-123",
					BillingPeriodNum: 202501,
					Status:           string(domain.BillStatusClosed),
					TotalCents:       2000,
					Currency:         string(libmoney.CurrencyUSD),
				},
			},
		},
		{
			name: "successful search with empty periods",
			cmd: SearchBillCmd{
				CustomerID: "customer-123",
				PeriodFrom: "",
				PeriodTo:   "",
				Status:     string(domain.BillStatusOpen),
			},
			mockSetup: func(m *MockTemporalPort) {
				expectedFilter := app.SearchBillFilter{
					CustomerID: "customer-123",
					FromYYYYMM: nil,
					ToYYYYMM:   nil,
					Status:     []string{string(domain.BillStatusOpen), string(domain.BillStatusPending)},
				}
				expectedResults := []views.BillSummary{}

				m.On("SearchBills", mock.Anything, expectedFilter).Return(expectedResults, nil)
			},
			expectedResult: []views.BillSummary{},
		},
		{
			name: "invalid period from format",
			cmd: SearchBillCmd{
				CustomerID: "customer-123",
				PeriodFrom: "invalid-period",
				PeriodTo:   "2025-03",
				Status:     string(domain.BillStatusOpen),
			},
			mockSetup: func(m *MockTemporalPort) {
				// No mock setup needed as error occurs before Temporal calls
			},
			expectedError: "fromInt conversion error",
		},
		{
			name: "invalid period to format",
			cmd: SearchBillCmd{
				CustomerID: "customer-123",
				PeriodFrom: "2025-01",
				PeriodTo:   "invalid-period",
				Status:     string(domain.BillStatusOpen),
			},
			mockSetup: func(m *MockTemporalPort) {
				// No mock setup needed as error occurs before Temporal calls
			},
			expectedError: "toInt conversion error",
		},
		{
			name: "temporal search error",
			cmd: SearchBillCmd{
				CustomerID: "customer-123",
				PeriodFrom: "2025-01",
				PeriodTo:   "2025-03",
				Status:     string(domain.BillStatusOpen),
			},
			mockSetup: func(m *MockTemporalPort) {
				expectedFilter := app.SearchBillFilter{
					CustomerID: "customer-123",
					FromYYYYMM: int64Ptr(202501),
					ToYYYYMM:   int64Ptr(202503),
					Status:     []string{string(domain.BillStatusOpen), string(domain.BillStatusPending)},
				}

				m.On("SearchBills", mock.Anything, expectedFilter).Return([]views.BillSummary{}, errors.New("search failed"))
			},
			expectedError: "SearchBills UC failer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTemporal := &MockTemporalPort{}
			tt.mockSetup(mockTemporal)

			uc := SearchBill{T: mockTemporal}
			result, err := uc.Handle(context.Background(), tt.cmd)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}

			mockTemporal.AssertExpectations(t)
		})
	}
}

// Helper function to create int64 pointer
func int64Ptr(i int64) *int64 {
	return &i
}
