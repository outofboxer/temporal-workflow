package app

import (
	"context"
	"errors"

	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"

	"github.com/outofboxer/temporal-workflow/fees/app/views"
	"github.com/outofboxer/temporal-workflow/fees/domain"
	libmoney "github.com/outofboxer/temporal-workflow/libs/money"
)

var (
	ErrBillWithPeriodAlreadyStarted = errors.New("a bill already exists for this customer and period")
	ErrLineItemAlreadyAdded         = errors.New("the line item already added")
	ErrBillNotFound                 = errors.New("bill not found")
	ErrBillAlreadyClosed            = errors.New("bill already closed")
)

type Kafka interface {
	PublishAudit()
}

type MonthlyFeeAccrualWorkflowParams struct {
	BillID       domain.BillID
	CustomerID   string
	Period       domain.BillingPeriod
	PeriodYYYYMM int64
	Currency     libmoney.Currency
}

type SearchBillFilter struct {
	CustomerID string
	FromYYYYMM *int64
	ToYYYYMM   *int64
	Status     []string
}

type TemporalPort interface {
	StartMonthlyBill(ctx context.Context, params MonthlyFeeAccrualWorkflowParams) error
	AddLineItem(ctx context.Context, id domain.BillID, li domain.LineItem) error
	CloseBill(ctx context.Context, id domain.BillID) error
	QueryBill(ctx context.Context, id domain.BillID) (domain.Bill, error)
	SearchBills(ctx context.Context, params SearchBillFilter) ([]views.BillSummary, error)
}

type TemporalClient interface {
	ExecuteWorkflow(
		ctx context.Context,
		options client.StartWorkflowOptions,
		workflow interface{},
		args ...interface{},
	) (client.WorkflowRun, error)
	SignalWorkflow(
		ctx context.Context,
		workflowID string,
		runID string,
		signalName string,
		arg interface{},
	) error
	QueryWorkflow(
		ctx context.Context,
		workflowID string,
		runID string,
		queryType string,
		args ...interface{},
	) (converter.EncodedValue, error)
	ListWorkflow(
		ctx context.Context,
		request *workflowservice.ListWorkflowExecutionsRequest,
	) (*workflowservice.ListWorkflowExecutionsResponse, error)
	Close()
}
