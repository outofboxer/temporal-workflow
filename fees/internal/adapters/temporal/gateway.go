package temporal

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	workflowpb "go.temporal.io/api/workflow/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/temporal"

	"github.com/outofboxer/temporal-workflow/fees/app"
	"github.com/outofboxer/temporal-workflow/fees/app/views"
	"github.com/outofboxer/temporal-workflow/fees/app/workflows"
	"github.com/outofboxer/temporal-workflow/fees/app/workflows/sa"
	"github.com/outofboxer/temporal-workflow/fees/domain"
)

const (
	taskQueue           = "FEES_TASK_QUEUE"
	pageSize            = 100
	queryTimeoutSeconds = 8
)

type Gateway struct {
	tc        client.Client
	namespace string
}

func NewGateway(tc client.Client, namespace string) *Gateway {
	return &Gateway{tc: tc, namespace: namespace}
}

func (g *Gateway) StartMonthlyBill(ctx context.Context, params app.MonthlyFeeAccrualWorkflowParams) error {
	wfID := string(params.BillID) // assume it's the same as bill id

	// Try to start the workflow for this (customer, period).
	_, err := g.tc.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        wfID,
			TaskQueue: taskQueue,
			// ensures to get AlreadyStarted on ExecuteWorkflow:
			WorkflowExecutionErrorWhenAlreadyStarted: true,
			// prevents reuse
			WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
			TypedSearchAttributes: temporal.NewSearchAttributes(
				sa.KeyCustomerID.ValueSet(params.CustomerID),
				sa.KeyBillingPeriodNum.ValueSet(params.PeriodYYYYMM),
				sa.KeyBillStatus.ValueSet(string(domain.BillStatusOpen)),
				sa.KeyBillCurrency.ValueSet(string(params.Currency)),
				sa.KeyBillItemCount.ValueSet(0),  // length of LineItems, zero at init time
				sa.KeyBillTotalCents.ValueSet(0), // zero total at init time
			),
		},
		workflows.MonthlyFeeAccrualWorkflow, // workflow definition
		params,
	)
	if err != nil {
		// If already started
		var already *serviceerror.WorkflowExecutionAlreadyStarted
		if errors.As(err, &already) {
			return app.ErrBillWithPeriodAlreadyStarted
		}

		return fmt.Errorf("temporal workflow start error, %w", err)
	}

	return err
}

func (g *Gateway) AddLineItem(ctx context.Context, id domain.BillID, li domain.LineItem) error {
	// Caution! // do not treat runID as billID, workflow could be re-run for compaction!
	runID := ""
	line := workflows.AddLineItemPayload{
		Description:    li.Description,
		Amount:         li.Amount,
		IdempotencyKey: li.IdempotencyKey,
	}

	return g.tc.SignalWorkflow(ctx, string(id), runID, workflows.SignalAddLineItem, line)
}

func (g *Gateway) CloseBill(ctx context.Context, id domain.BillID) error {
	// Caution! // do not treat runID as billID, workflow could be re-run for compaction!
	runID := ""

	return g.tc.SignalWorkflow(ctx, string(id), runID, workflows.SignalCloseBill, nil)
}

func (g *Gateway) QueryBill(ctx context.Context, id domain.BillID) (domain.Bill, error) {
	// Queries can hang if a handler is busy. Wrap ctx
	ctx, cancel := context.WithTimeout(ctx, queryTimeoutSeconds*time.Second)
	defer cancel()
	// Query by workflow ID; run ID can be "" (latest)
	runID := ""
	resp, err := g.tc.QueryWorkflow(ctx, string(id), runID, workflows.QueryState /* e.g., "CurrentBillState" */)
	if err != nil {
		var nf *serviceerror.NotFound
		if errors.As(err, &nf) {
			return domain.Bill{}, app.ErrBillNotFound
		}

		return domain.Bill{}, fmt.Errorf("query bill: %w", err)
	}
	var b workflows.BillDTO
	if err := resp.Get(&b); err != nil {
		return domain.Bill{}, err
	}

	lineItems := make([]domain.LineItem, 0, len(b.Items))
	for _, li := range b.Items {
		lineItems = append(lineItems, domain.LineItem{
			IdempotencyKey: li.IdempotencyKey,
			Description:    li.Description,
			Amount:         li.Amount,
			AddedAt:        li.AddedAt,
		})
	}

	return domain.Bill{
		ID:            domain.BillID(b.ID),
		CustomerID:    b.CustomerID,
		Currency:      b.Currency,
		BillingPeriod: domain.BillingPeriod(b.BillingPeriod),
		Status:        domain.BillStatus(b.Status),
		Items:         lineItems,
		Total:         b.Total,
		CreatedAt:     b.CreatedAt,
		UpdatedAt:     b.UpdatedAt,
		FinalizedAt:   b.ClosedAt,
	}, nil
}

func visQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)

	return s
}

func (g *Gateway) SearchBills(ctx context.Context, params app.SearchBillFilter) ([]views.BillSummary, error) {
	// We don't use ListOpenWorkflow or ListClosedWorkflow because it's not domain specific status but technical one.
	// E.g. we could have bill (i.e. Workflow in Closed domain status but workflow still executed in terms of sending
	//	out invoices via payment gateway).
	// SQL injection currently is protected by API layer validation, but for real public app here we should
	//	apply additional checks and escaping.

	// Build query with required filters
	queryParts := []string{
		fmt.Sprintf(`WorkflowType = "%s"`, workflows.WorkflowTypeMonthlyBill),
		fmt.Sprintf(`CustomerID = "%s"`, visQuote(params.CustomerID)),
	}

	// Add status filter(s) with OR logic
	if len(params.Status) > 0 {
		statusConditions := make([]string, len(params.Status))
		for i, status := range params.Status {
			statusConditions[i] = fmt.Sprintf(`BillStatus = "%s"`, status)
		}
		queryParts = append(queryParts, fmt.Sprintf("(%s)", strings.Join(statusConditions, " OR ")))
	}

	// Add optional date range filters
	if params.FromYYYYMM != nil {
		queryParts = append(queryParts, fmt.Sprintf(`BillingPeriodNum >= %d`, *params.FromYYYYMM))
	}
	if params.ToYYYYMM != nil {
		queryParts = append(queryParts, fmt.Sprintf(`BillingPeriodNum <= %d`, *params.ToYYYYMM))
	}

	q := strings.Join(queryParts, " AND ")
	var out []views.BillSummary
	var token []byte
	dc := converter.GetDefaultDataConverter()

	for {
		resp, err := g.tc.ListWorkflow(ctx, &workflowservice.ListWorkflowExecutionsRequest{
			Namespace:     g.namespace,
			Query:         q,
			PageSize:      pageSize,
			NextPageToken: token,
		})
		if err != nil {
			return nil, err
		}

		for _, info := range resp.GetExecutions() {
			sum, err := mapInfoToSummary(dc, info)
			if err != nil {
				return nil, fmt.Errorf("search attributes extraction error, %w", err)
			}
			out = append(out, sum)
		}

		if len(resp.GetNextPageToken()) == 0 {
			break
		}
		token = resp.GetNextPageToken()
	}

	return out, nil
}

func decode[T any](dc converter.DataConverter, p *commonpb.Payload, out *T) error {
	if p == nil {
		return errors.New("nil payload")
	}

	return dc.FromPayload(p, out)
}

func mapInfoToSummary(dc converter.DataConverter, info *workflowpb.WorkflowExecutionInfo) (views.BillSummary, error) {
	attrs := info.GetSearchAttributes().GetIndexedFields()
	get := func(key string) *commonpb.Payload { return attrs[key] }

	sum := views.BillSummary{
		WorkflowID: info.GetExecution().GetWorkflowId(),
		RunID:      info.GetExecution().GetRunId(),
	}
	// Decode typed SAs we expect (ignore missing ones gracefully).
	err := decode(dc, get(sa.CustomerIDName), &sum.CustomerID)
	err = errors.Join(err, decode(dc, get(sa.BillingPeriodNumName), &sum.BillingPeriodNum))
	err = errors.Join(err, decode(dc, get(sa.BillStatusName), &sum.Status))
	err = errors.Join(err, decode(dc, get(sa.BillCurrencyName), &sum.Currency))
	err = errors.Join(err, decode(dc, get(sa.BillItemCountName), &sum.ItemCount))
	err = errors.Join(err, decode(dc, get(sa.BillTotalCentsName), &sum.TotalCents))
	if err != nil {
		return views.BillSummary{}, err
	}

	// Datetime SAs decode straight into time.Time
	// err = decode(dc, get(sa.PeriodStart), &sum.PeriodStart)
	// err = decode(dc, get(sa.PeriodEnd), &sum.PeriodEnd)

	// Optional summaries to upsert in the workflow
	// err = decode(dc, get(sa.TotalCents), &sum.TotalCents)
	// err = decode(dc, get(sa.ItemCountName), &sum.ItemCount)

	return sum, nil
}
