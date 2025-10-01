package temporal

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/operatorservice/v1"
	"go.temporal.io/api/serviceerror"
	workflowpb "go.temporal.io/api/workflow/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"

	"github.com/outofboxer/temporal-workflow/fees/app"
	"github.com/outofboxer/temporal-workflow/fees/app/views"
	"github.com/outofboxer/temporal-workflow/fees/app/workflows"
	"github.com/outofboxer/temporal-workflow/fees/domain"
	libmoney "github.com/outofboxer/temporal-workflow/libs/money"
)

// MockTemporalClient is a mock implementation of client.Client
type MockTemporalClient struct {
	mock.Mock
}

func (m *MockTemporalClient) ExecuteWorkflow(
	ctx context.Context,
	options client.StartWorkflowOptions,
	workflow interface{},
	args ...interface{},
) (client.WorkflowRun, error) {
	argsMock := m.Called(ctx, options, workflow, args)
	return argsMock.Get(0).(client.WorkflowRun), argsMock.Error(1)
}

func (m *MockTemporalClient) SignalWorkflow(
	ctx context.Context,
	workflowID string,
	runID string,
	signalName string,
	arg interface{},
) error {
	args := m.Called(ctx, workflowID, runID, signalName, arg)
	return args.Error(0)
}

func (m *MockTemporalClient) QueryWorkflow(
	ctx context.Context,
	workflowID string,
	runID string,
	queryType string,
	args ...interface{},
) (converter.EncodedValue, error) {
	mockArgs := m.Called(ctx, workflowID, runID, queryType, args)
	return mockArgs.Get(0).(converter.EncodedValue), mockArgs.Error(1)
}

func (m *MockTemporalClient) ListWorkflow(
	ctx context.Context,
	request *workflowservice.ListWorkflowExecutionsRequest,
) (*workflowservice.ListWorkflowExecutionsResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*workflowservice.ListWorkflowExecutionsResponse), args.Error(1)
}

func (m *MockTemporalClient) Close() {
	m.Called()
}

func (m *MockTemporalClient) CancelWorkflow(ctx context.Context, workflowID string, runID string) error {
	args := m.Called(ctx, workflowID, runID)
	return args.Error(0)
}

func (m *MockTemporalClient) TerminateWorkflow(ctx context.Context, workflowID string, runID string, reason string, details ...interface{}) error {
	args := m.Called(ctx, workflowID, runID, reason, details)
	return args.Error(0)
}

func (m *MockTemporalClient) GetWorkflow(ctx context.Context, workflowID string, runID string) client.WorkflowRun {
	args := m.Called(ctx, workflowID, runID)
	return args.Get(0).(client.WorkflowRun)
}

func (m *MockTemporalClient) SignalWithStartWorkflow(ctx context.Context, workflowID string, signalName string, signalArg interface{}, options client.StartWorkflowOptions, workflow interface{}, workflowArgs ...interface{}) (client.WorkflowRun, error) {
	args := m.Called(ctx, workflowID, signalName, signalArg, options, workflow, workflowArgs)
	return args.Get(0).(client.WorkflowRun), args.Error(1)
}

func (m *MockTemporalClient) GetWorkflowHistory(ctx context.Context, workflowID string, runID string, isLongPoll bool, filterType enums.HistoryEventFilterType) client.HistoryEventIterator {
	args := m.Called(ctx, workflowID, runID, isLongPoll, filterType)
	return args.Get(0).(client.HistoryEventIterator)
}

func (m *MockTemporalClient) CompleteActivity(ctx context.Context, taskToken []byte, result interface{}, err error) error {
	args := m.Called(ctx, taskToken, result, err)
	return args.Error(0)
}

func (m *MockTemporalClient) CompleteActivityByID(ctx context.Context, namespace string, workflowID string, runID string, activityID string, result interface{}, err error) error {
	args := m.Called(ctx, namespace, workflowID, runID, activityID, result, err)
	return args.Error(0)
}

func (m *MockTemporalClient) RecordActivityHeartbeat(ctx context.Context, taskToken []byte, details ...interface{}) error {
	args := m.Called(ctx, taskToken, details)
	return args.Error(0)
}

func (m *MockTemporalClient) RecordActivityHeartbeatByID(ctx context.Context, namespace string, workflowID string, runID string, activityID string, details ...interface{}) error {
	args := m.Called(ctx, namespace, workflowID, runID, activityID, details)
	return args.Error(0)
}

func (m *MockTemporalClient) ListOpenWorkflow(ctx context.Context, request *workflowservice.ListOpenWorkflowExecutionsRequest) (*workflowservice.ListOpenWorkflowExecutionsResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*workflowservice.ListOpenWorkflowExecutionsResponse), args.Error(1)
}

func (m *MockTemporalClient) ListClosedWorkflow(ctx context.Context, request *workflowservice.ListClosedWorkflowExecutionsRequest) (*workflowservice.ListClosedWorkflowExecutionsResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*workflowservice.ListClosedWorkflowExecutionsResponse), args.Error(1)
}

func (m *MockTemporalClient) ListWorkflowExecutions(ctx context.Context, request *workflowservice.ListWorkflowExecutionsRequest) (*workflowservice.ListWorkflowExecutionsResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*workflowservice.ListWorkflowExecutionsResponse), args.Error(1)
}

func (m *MockTemporalClient) ListArchivedWorkflow(ctx context.Context, request *workflowservice.ListArchivedWorkflowExecutionsRequest) (*workflowservice.ListArchivedWorkflowExecutionsResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*workflowservice.ListArchivedWorkflowExecutionsResponse), args.Error(1)
}

func (m *MockTemporalClient) ScanWorkflow(ctx context.Context, request *workflowservice.ScanWorkflowExecutionsRequest) (*workflowservice.ScanWorkflowExecutionsResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*workflowservice.ScanWorkflowExecutionsResponse), args.Error(1)
}

func (m *MockTemporalClient) CountWorkflow(ctx context.Context, request *workflowservice.CountWorkflowExecutionsRequest) (*workflowservice.CountWorkflowExecutionsResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*workflowservice.CountWorkflowExecutionsResponse), args.Error(1)
}

func (m *MockTemporalClient) GetSearchAttributes(ctx context.Context) (*workflowservice.GetSearchAttributesResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(*workflowservice.GetSearchAttributesResponse), args.Error(1)
}

func (m *MockTemporalClient) UpdateWorkerBuildIdCompatibility(ctx context.Context, options *client.UpdateWorkerBuildIdCompatibilityOptions) error {
	args := m.Called(ctx, options)
	return args.Error(0)
}

func (m *MockTemporalClient) GetWorkerBuildIdCompatibility(ctx context.Context, options *client.GetWorkerBuildIdCompatibilityOptions) (*client.WorkerBuildIDVersionSets, error) {
	args := m.Called(ctx, options)
	return args.Get(0).(*client.WorkerBuildIDVersionSets), args.Error(1)
}

func (m *MockTemporalClient) GetWorkerTaskReachability(ctx context.Context, options *client.GetWorkerTaskReachabilityOptions) (*client.WorkerTaskReachability, error) {
	args := m.Called(ctx, options)
	return args.Get(0).(*client.WorkerTaskReachability), args.Error(1)
}

func (m *MockTemporalClient) UpdateWorkflow(ctx context.Context, options client.UpdateWorkflowOptions) (client.WorkflowUpdateHandle, error) {
	args := m.Called(ctx, options)
	return args.Get(0).(client.WorkflowUpdateHandle), args.Error(1)
}

func (m *MockTemporalClient) GetWorkflowUpdateHandle(options client.GetWorkflowUpdateHandleOptions) client.WorkflowUpdateHandle {
	args := m.Called(options)
	return args.Get(0).(client.WorkflowUpdateHandle)
}

func (m *MockTemporalClient) CheckHealth(ctx context.Context, request *client.CheckHealthRequest) (*client.CheckHealthResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*client.CheckHealthResponse), args.Error(1)
}

func (m *MockTemporalClient) DeploymentClient() client.DeploymentClient {
	args := m.Called()
	return args.Get(0).(client.DeploymentClient)
}

func (m *MockTemporalClient) DescribeTaskQueue(ctx context.Context, taskQueue string, taskQueueType enums.TaskQueueType) (*workflowservice.DescribeTaskQueueResponse, error) {
	args := m.Called(ctx, taskQueue, taskQueueType)
	return args.Get(0).(*workflowservice.DescribeTaskQueueResponse), args.Error(1)
}

func (m *MockTemporalClient) DescribeTaskQueueEnhanced(ctx context.Context, options client.DescribeTaskQueueEnhancedOptions) (client.TaskQueueDescription, error) {
	args := m.Called(ctx, options)
	return args.Get(0).(client.TaskQueueDescription), args.Error(1)
}

func (m *MockTemporalClient) DescribeWorkflow(ctx context.Context, workflowID string, runID string) (*client.WorkflowExecutionDescription, error) {
	args := m.Called(ctx, workflowID, runID)
	return args.Get(0).(*client.WorkflowExecutionDescription), args.Error(1)
}

func (m *MockTemporalClient) DescribeWorkflowExecution(ctx context.Context, workflowID string, runID string) (*workflowservice.DescribeWorkflowExecutionResponse, error) {
	args := m.Called(ctx, workflowID, runID)
	return args.Get(0).(*workflowservice.DescribeWorkflowExecutionResponse), args.Error(1)
}

func (m *MockTemporalClient) GetWorkerVersioningRules(ctx context.Context, options client.GetWorkerVersioningOptions) (*client.WorkerVersioningRules, error) {
	args := m.Called(ctx, options)
	return args.Get(0).(*client.WorkerVersioningRules), args.Error(1)
}

func (m *MockTemporalClient) NewWithStartWorkflowOperation(options client.StartWorkflowOptions, workflow interface{}, args ...interface{}) client.WithStartWorkflowOperation {
	mockArgs := m.Called(options, workflow, args)
	return mockArgs.Get(0).(client.WithStartWorkflowOperation)
}

func (m *MockTemporalClient) OperatorService() operatorservice.OperatorServiceClient {
	args := m.Called()
	return args.Get(0).(operatorservice.OperatorServiceClient)
}

func (m *MockTemporalClient) QueryWorkflowWithOptions(ctx context.Context, request *client.QueryWorkflowWithOptionsRequest) (*client.QueryWorkflowWithOptionsResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*client.QueryWorkflowWithOptionsResponse), args.Error(1)
}

func (m *MockTemporalClient) ResetWorkflowExecution(ctx context.Context, request *workflowservice.ResetWorkflowExecutionRequest) (*workflowservice.ResetWorkflowExecutionResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*workflowservice.ResetWorkflowExecutionResponse), args.Error(1)
}

func (m *MockTemporalClient) ScheduleClient() client.ScheduleClient {
	args := m.Called()
	return args.Get(0).(client.ScheduleClient)
}

func (m *MockTemporalClient) UpdateWithStartWorkflow(ctx context.Context, options client.UpdateWithStartWorkflowOptions) (client.WorkflowUpdateHandle, error) {
	args := m.Called(ctx, options)
	return args.Get(0).(client.WorkflowUpdateHandle), args.Error(1)
}

func (m *MockTemporalClient) UpdateWorkerVersioningRules(ctx context.Context, options client.UpdateWorkerVersioningRulesOptions) (*client.WorkerVersioningRules, error) {
	args := m.Called(ctx, options)
	return args.Get(0).(*client.WorkerVersioningRules), args.Error(1)
}

func (m *MockTemporalClient) UpdateWorkflowExecutionOptions(ctx context.Context, options client.UpdateWorkflowExecutionOptionsRequest) (client.WorkflowExecutionOptions, error) {
	args := m.Called(ctx, options)
	return args.Get(0).(client.WorkflowExecutionOptions), args.Error(1)
}

func (m *MockTemporalClient) WorkerDeploymentClient() client.WorkerDeploymentClient {
	args := m.Called()
	return args.Get(0).(client.WorkerDeploymentClient)
}

func (m *MockTemporalClient) WorkflowService() workflowservice.WorkflowServiceClient {
	args := m.Called()
	return args.Get(0).(workflowservice.WorkflowServiceClient)
}

// MockWorkflowRun is a mock implementation of client.WorkflowRun
type MockWorkflowRun struct {
	mock.Mock
}

func (m *MockWorkflowRun) GetID() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockWorkflowRun) GetRunID() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockWorkflowRun) Get(ctx context.Context, valuePtr interface{}) error {
	args := m.Called(ctx, valuePtr)
	return args.Error(0)
}

func (m *MockWorkflowRun) GetWithOptions(ctx context.Context, valuePtr interface{}, options client.WorkflowRunGetOptions) error {
	args := m.Called(ctx, valuePtr, options)
	return args.Error(0)
}

// MockEncodedValue is a mock implementation of converter.EncodedValue
type MockEncodedValue struct {
	mock.Mock
}

func (m *MockEncodedValue) Get(valuePtr interface{}) error {
	args := m.Called(valuePtr)
	return args.Error(0)
}

func (m *MockEncodedValue) HasValue() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockEncodedValue) Size() int {
	args := m.Called()
	return args.Int(0)
}

func TestGateway_StartMonthlyBill(t *testing.T) {
	tests := []struct {
		name          string
		params        app.MonthlyFeeAccrualWorkflowParams
		mockSetup     func(*MockTemporalClient, *MockWorkflowRun)
		expectedError string
	}{
		{
			name: "successful workflow start",
			params: app.MonthlyFeeAccrualWorkflowParams{
				BillID:       domain.BillID("test-bill-123"),
				CustomerID:   "customer-123",
				Period:       domain.BillingPeriod("2025-01"),
				PeriodYYYYMM: 202501,
				Currency:     libmoney.CurrencyUSD,
			},
			mockSetup: func(mockClient *MockTemporalClient, mockRun *MockWorkflowRun) {
				mockClient.On("ExecuteWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(mockRun, nil)
			},
			expectedError: "",
		},
		{
			name: "workflow already started error",
			params: app.MonthlyFeeAccrualWorkflowParams{
				BillID:       domain.BillID("test-bill-456"),
				CustomerID:   "customer-456",
				Period:       domain.BillingPeriod("2025-02"),
				PeriodYYYYMM: 202502,
				Currency:     libmoney.CurrencyGEL,
			},
			mockSetup: func(mockClient *MockTemporalClient, mockRun *MockWorkflowRun) {
				alreadyStartedErr := &serviceerror.WorkflowExecutionAlreadyStarted{
					Message: "Workflow execution already started",
				}
				mockClient.On("ExecuteWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(mockRun, alreadyStartedErr)
			},
			expectedError: app.ErrBillWithPeriodAlreadyStarted.Error(),
		},
		{
			name: "temporal client error",
			params: app.MonthlyFeeAccrualWorkflowParams{
				BillID:       domain.BillID("test-bill-789"),
				CustomerID:   "customer-789",
				Period:       domain.BillingPeriod("2025-03"),
				PeriodYYYYMM: 202503,
				Currency:     libmoney.CurrencyUSD,
			},
			mockSetup: func(mockClient *MockTemporalClient, mockRun *MockWorkflowRun) {
				mockClient.On("ExecuteWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(mockRun, errors.New("temporal connection error"))
			},
			expectedError: "temporal workflow start error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockTemporalClient{}
			mockRun := &MockWorkflowRun{}
			tt.mockSetup(mockClient, mockRun)

			gateway := NewGateway(mockClient, "test-namespace")

			err := gateway.StartMonthlyBill(context.Background(), tt.params)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

func TestGateway_AddLineItem(t *testing.T) {
	tests := []struct {
		name          string
		billID        domain.BillID
		lineItem      domain.LineItem
		mockSetup     func(*MockTemporalClient)
		expectedError string
	}{
		{
			name:   "successful line item addition",
			billID: domain.BillID("test-bill-123"),
			lineItem: domain.LineItem{
				IdempotencyKey: "item-1",
				Description:    "Test item",
				Amount:         libmoney.NewFromInt(1000, libmoney.CurrencyUSD),
				AddedAt:        time.Now(),
			},
			mockSetup: func(mockClient *MockTemporalClient) {
				mockClient.On("SignalWorkflow", mock.Anything, "test-bill-123", "", "SignalAddLineItem", mock.Anything).
					Return(nil)
			},
			expectedError: "",
		},
		{
			name:   "signal workflow error",
			billID: domain.BillID("test-bill-456"),
			lineItem: domain.LineItem{
				IdempotencyKey: "item-2",
				Description:    "Test item 2",
				Amount:         libmoney.NewFromInt(2000, libmoney.CurrencyGEL),
				AddedAt:        time.Now(),
			},
			mockSetup: func(mockClient *MockTemporalClient) {
				mockClient.On("SignalWorkflow", mock.Anything, "test-bill-456", "", "SignalAddLineItem", mock.Anything).
					Return(errors.New("signal failed"))
			},
			expectedError: "signal failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockTemporalClient{}
			tt.mockSetup(mockClient)

			gateway := NewGateway(mockClient, "test-namespace")

			err := gateway.AddLineItem(context.Background(), tt.billID, tt.lineItem)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

func TestGateway_CloseBill(t *testing.T) {
	tests := []struct {
		name          string
		billID        domain.BillID
		mockSetup     func(*MockTemporalClient)
		expectedError string
	}{
		{
			name:   "successful bill close",
			billID: domain.BillID("test-bill-123"),
			mockSetup: func(mockClient *MockTemporalClient) {
				mockClient.On("SignalWorkflow", mock.Anything, "test-bill-123", "", "SignalCloseBill", nil).
					Return(nil)
			},
			expectedError: "",
		},
		{
			name:   "signal workflow error",
			billID: domain.BillID("test-bill-456"),
			mockSetup: func(mockClient *MockTemporalClient) {
				mockClient.On("SignalWorkflow", mock.Anything, "test-bill-456", "", "SignalCloseBill", nil).
					Return(errors.New("signal failed"))
			},
			expectedError: "signal failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockTemporalClient{}
			tt.mockSetup(mockClient)

			gateway := NewGateway(mockClient, "test-namespace")

			err := gateway.CloseBill(context.Background(), tt.billID)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

func TestGateway_QueryBill(t *testing.T) {
	tests := []struct {
		name          string
		billID        domain.BillID
		mockSetup     func(*MockTemporalClient, *MockEncodedValue)
		expectedBill  domain.Bill
		expectedError string
	}{
		{
			name:   "successful bill query",
			billID: domain.BillID("test-bill-123"),
			mockSetup: func(mockClient *MockTemporalClient, mockValue *MockEncodedValue) {
				mockClient.On("QueryWorkflow", mock.Anything, "test-bill-123", "", "CurrentBillState", mock.Anything).
					Return(mockValue, nil)

				// Mock the Get method to populate the BillDTO
				mockValue.On("Get", mock.AnythingOfType("*workflows.BillDTO")).Run(func(args mock.Arguments) {
					billDTO := args.Get(0).(*workflows.BillDTO)
					billDTO.ID = "test-bill-123"
					billDTO.CustomerID = "customer-123"
					billDTO.Currency = libmoney.CurrencyUSD
					billDTO.BillingPeriod = "2025-01"
					billDTO.Status = "OPEN"
					billDTO.Items = []workflows.LineItemDTO{
						{
							IdempotencyKey: "item-1",
							Description:    "Test item",
							Amount:         libmoney.NewFromInt(1000, libmoney.CurrencyUSD),
							AddedAt:        time.Now(),
						},
					}
					billDTO.Total = libmoney.NewFromInt(1000, libmoney.CurrencyUSD)
					billDTO.CreatedAt = time.Now()
					billDTO.UpdatedAt = time.Now()
				}).Return(nil)
			},
			expectedBill: domain.Bill{
				ID:            domain.BillID("test-bill-123"),
				CustomerID:    "customer-123",
				Currency:      libmoney.CurrencyUSD,
				BillingPeriod: domain.BillingPeriod("2025-01"),
				Status:        domain.BillStatusOpen,
				Items: []domain.LineItem{
					{
						IdempotencyKey: "item-1",
						Description:    "Test item",
						Amount:         libmoney.NewFromInt(1000, libmoney.CurrencyUSD),
						AddedAt:        time.Now(),
					},
				},
				Total:     libmoney.NewFromInt(1000, libmoney.CurrencyUSD),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			expectedError: "",
		},
		{
			name:   "bill not found",
			billID: domain.BillID("test-bill-456"),
			mockSetup: func(mockClient *MockTemporalClient, mockValue *MockEncodedValue) {
				notFoundErr := &serviceerror.NotFound{
					Message: "Workflow execution not found",
				}
				mockClient.On("QueryWorkflow", mock.Anything, "test-bill-456", "", "CurrentBillState", mock.Anything).
					Return(mockValue, notFoundErr)
			},
			expectedError: app.ErrBillNotFound.Error(),
		},
		{
			name:   "query workflow error",
			billID: domain.BillID("test-bill-789"),
			mockSetup: func(mockClient *MockTemporalClient, mockValue *MockEncodedValue) {
				mockClient.On("QueryWorkflow", mock.Anything, "test-bill-789", "", "CurrentBillState", mock.Anything).
					Return(mockValue, errors.New("query failed"))
			},
			expectedError: "query bill: query failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockTemporalClient{}
			mockValue := &MockEncodedValue{}
			tt.mockSetup(mockClient, mockValue)

			gateway := NewGateway(mockClient, "test-namespace")

			bill, err := gateway.QueryBill(context.Background(), tt.billID)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedBill.ID, bill.ID)
				assert.Equal(t, tt.expectedBill.CustomerID, bill.CustomerID)
				assert.Equal(t, tt.expectedBill.Currency, bill.Currency)
				assert.Equal(t, tt.expectedBill.BillingPeriod, bill.BillingPeriod)
				assert.Equal(t, tt.expectedBill.Status, bill.Status)
				assert.Len(t, bill.Items, len(tt.expectedBill.Items))
			}

			mockClient.AssertExpectations(t)
			mockValue.AssertExpectations(t)
		})
	}
}

func TestGateway_SearchBills(t *testing.T) {
	tests := []struct {
		name          string
		params        app.SearchBillFilter
		mockSetup     func(*MockTemporalClient)
		expectedBills []views.BillSummary
		expectedError string
	}{
		{
			name: "successful search with status filter",
			params: app.SearchBillFilter{
				CustomerID: "customer-123",
				Status:     []string{"OPEN", "PENDING"},
			},
			mockSetup: func(mockClient *MockTemporalClient) {
				// Mock search attributes with proper metadata
				searchAttrs := map[string]*commonpb.Payload{
					"CustomerID": {
						Data: []byte(`"customer-123"`),
						Metadata: map[string][]byte{
							"encoding": []byte("json/plain"),
						},
					},
					"BillingPeriodNum": {
						Data: []byte(`202501`),
						Metadata: map[string][]byte{
							"encoding": []byte("json/plain"),
						},
					},
					"BillStatus": {
						Data: []byte(`"OPEN"`),
						Metadata: map[string][]byte{
							"encoding": []byte("json/plain"),
						},
					},
					"BillCurrency": {
						Data: []byte(`"USD"`),
						Metadata: map[string][]byte{
							"encoding": []byte("json/plain"),
						},
					},
					"BillItemCount": {
						Data: []byte(`2`),
						Metadata: map[string][]byte{
							"encoding": []byte("json/plain"),
						},
					},
					"BillTotalCents": {
						Data: []byte(`5000`),
						Metadata: map[string][]byte{
							"encoding": []byte("json/plain"),
						},
					},
				}

				executionInfo := &workflowpb.WorkflowExecutionInfo{
					Execution: &commonpb.WorkflowExecution{
						WorkflowId: "test-bill-123",
						RunId:      "test-run-123",
					},
					SearchAttributes: &commonpb.SearchAttributes{
						IndexedFields: searchAttrs,
					},
				}

				response := &workflowservice.ListWorkflowExecutionsResponse{
					Executions: []*workflowpb.WorkflowExecutionInfo{executionInfo},
				}

				mockClient.On("ListWorkflow", mock.Anything, mock.MatchedBy(func(req *workflowservice.ListWorkflowExecutionsRequest) bool {
					return req.Namespace == "test-namespace" &&
						req.Query == `WorkflowType = "MonthlyFeeAccrualWorkflow" AND CustomerID = "customer-123" AND (BillStatus = "OPEN" OR BillStatus = "PENDING")`
				})).Return(response, nil)
			},
			expectedBills: []views.BillSummary{
				{
					WorkflowID:       "test-bill-123",
					RunID:            "test-run-123",
					CustomerID:       "customer-123",
					BillingPeriodNum: 202501,
					Status:           "OPEN",
					Currency:         "USD",
					ItemCount:        2,
					TotalCents:       5000,
				},
			},
			expectedError: "",
		},
		{
			name: "successful search with date range",
			params: app.SearchBillFilter{
				CustomerID: "customer-456",
				FromYYYYMM: int64Ptr(202501),
				ToYYYYMM:   int64Ptr(202512),
			},
			mockSetup: func(mockClient *MockTemporalClient) {
				response := &workflowservice.ListWorkflowExecutionsResponse{
					Executions: []*workflowpb.WorkflowExecutionInfo{},
				}

				mockClient.On("ListWorkflow", mock.Anything, mock.MatchedBy(func(req *workflowservice.ListWorkflowExecutionsRequest) bool {
					return req.Namespace == "test-namespace" &&
						req.Query == `WorkflowType = "MonthlyFeeAccrualWorkflow" AND CustomerID = "customer-456" AND BillingPeriodNum >= 202501 AND BillingPeriodNum <= 202512`
				})).Return(response, nil)
			},
			expectedBills: []views.BillSummary{},
			expectedError: "",
		},
		{
			name: "list workflow error",
			params: app.SearchBillFilter{
				CustomerID: "customer-789",
			},
			mockSetup: func(mockClient *MockTemporalClient) {
				mockClient.On("ListWorkflow", mock.Anything, mock.Anything).
					Return((*workflowservice.ListWorkflowExecutionsResponse)(nil), errors.New("list failed"))
			},
			expectedError: "list failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockTemporalClient{}
			tt.mockSetup(mockClient)

			gateway := NewGateway(mockClient, "test-namespace")

			bills, err := gateway.SearchBills(context.Background(), tt.params)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.expectedBills), len(bills))
				if len(tt.expectedBills) > 0 {
					assert.Equal(t, tt.expectedBills[0].WorkflowID, bills[0].WorkflowID)
					assert.Equal(t, tt.expectedBills[0].CustomerID, bills[0].CustomerID)
					assert.Equal(t, tt.expectedBills[0].Status, bills[0].Status)
				}
			}

			mockClient.AssertExpectations(t)
		})
	}
}

func TestVisQuote(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no special characters",
			input:    "normal-string",
			expected: "normal-string",
		},
		{
			name:     "with backslashes",
			input:    "path\\to\\file",
			expected: "path\\\\to\\\\file",
		},
		{
			name:     "with quotes",
			input:    `"quoted string"`,
			expected: `\"quoted string\"`,
		},
		{
			name:     "with both backslashes and quotes",
			input:    `path\\to\\"file"`,
			expected: `path\\\\to\\\\\"file\"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := visQuote(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMapInfoToSummary(t *testing.T) {
	tests := []struct {
		name            string
		executionInfo   *workflowpb.WorkflowExecutionInfo
		expectedSummary views.BillSummary
		expectedError   string
	}{
		{
			name: "successful mapping",
			executionInfo: &workflowpb.WorkflowExecutionInfo{
				Execution: &commonpb.WorkflowExecution{
					WorkflowId: "test-bill-123",
					RunId:      "test-run-123",
				},
				SearchAttributes: &commonpb.SearchAttributes{
					IndexedFields: map[string]*commonpb.Payload{
						"CustomerID": {
							Data: []byte(`"customer-123"`),
							Metadata: map[string][]byte{
								"encoding": []byte("json/plain"),
							},
						},
						"BillingPeriodNum": {
							Data: []byte(`202501`),
							Metadata: map[string][]byte{
								"encoding": []byte("json/plain"),
							},
						},
						"BillStatus": {
							Data: []byte(`"OPEN"`),
							Metadata: map[string][]byte{
								"encoding": []byte("json/plain"),
							},
						},
						"BillCurrency": {
							Data: []byte(`"USD"`),
							Metadata: map[string][]byte{
								"encoding": []byte("json/plain"),
							},
						},
						"BillItemCount": {
							Data: []byte(`3`),
							Metadata: map[string][]byte{
								"encoding": []byte("json/plain"),
							},
						},
						"BillTotalCents": {
							Data: []byte(`7500`),
							Metadata: map[string][]byte{
								"encoding": []byte("json/plain"),
							},
						},
					},
				},
			},
			expectedSummary: views.BillSummary{
				WorkflowID:       "test-bill-123",
				RunID:            "test-run-123",
				CustomerID:       "customer-123",
				BillingPeriodNum: 202501,
				Status:           "OPEN",
				Currency:         "USD",
				ItemCount:        3,
				TotalCents:       7500,
			},
			expectedError: "",
		},
		{
			name: "missing search attributes",
			executionInfo: &workflowpb.WorkflowExecutionInfo{
				Execution: &commonpb.WorkflowExecution{
					WorkflowId: "test-bill-456",
					RunId:      "test-run-456",
				},
				SearchAttributes: &commonpb.SearchAttributes{
					IndexedFields: map[string]*commonpb.Payload{},
				},
			},
			expectedSummary: views.BillSummary{
				WorkflowID: "test-bill-456",
				RunID:      "test-run-456",
			},
			expectedError: "nil payload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dc := converter.GetDefaultDataConverter()
			summary, err := mapInfoToSummary(dc, tt.executionInfo)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedSummary.WorkflowID, summary.WorkflowID)
				assert.Equal(t, tt.expectedSummary.RunID, summary.RunID)
				assert.Equal(t, tt.expectedSummary.CustomerID, summary.CustomerID)
				assert.Equal(t, tt.expectedSummary.BillingPeriodNum, summary.BillingPeriodNum)
				assert.Equal(t, tt.expectedSummary.Status, summary.Status)
				assert.Equal(t, tt.expectedSummary.Currency, summary.Currency)
				assert.Equal(t, tt.expectedSummary.ItemCount, summary.ItemCount)
				assert.Equal(t, tt.expectedSummary.TotalCents, summary.TotalCents)
			}
		})
	}
}

func TestDecode(t *testing.T) {
	tests := []struct {
		name          string
		payload       *commonpb.Payload
		expectedValue string
		expectedError string
	}{
		{
			name: "successful decode",
			payload: &commonpb.Payload{
				Data: []byte(`"test-value"`),
				Metadata: map[string][]byte{
					"encoding": []byte("json/plain"),
				},
			},
			expectedValue: "test-value",
			expectedError: "",
		},
		{
			name:          "nil payload",
			payload:       nil,
			expectedValue: "",
			expectedError: "nil payload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dc := converter.GetDefaultDataConverter()
			var result string
			err := decode(dc, tt.payload, &result)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue, result)
			}
		})
	}
}

// Helper function to create int64 pointer
func int64Ptr(i int64) *int64 {
	return &i
}
