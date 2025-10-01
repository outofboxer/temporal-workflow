// Service url takes URLs, generates random short IDs, and stores the URLs in a database.
package feesapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"encore.dev/beta/errs"
	"encore.dev/middleware"
	"encore.dev/rlog"

	"github.com/outofboxer/temporal-workflow/fees/app"
	"github.com/outofboxer/temporal-workflow/fees/app/usecases"
	"github.com/outofboxer/temporal-workflow/fees/domain"
	"github.com/outofboxer/temporal-workflow/fees/internal/validation"

	libmoney "github.com/outofboxer/temporal-workflow/libs/money"
)

//encore:middleware target=tag:validation
func ValidationMiddleware(req middleware.Request, next middleware.Next) middleware.Response {
	// If the payload has a Validate method, use it to validate the request.
	payload := req.Data().Payload
	if validator, ok := payload.(interface{ Validate() error }); ok {
		if err := validator.Validate(); err != nil {
			// If the validation fails, return an InvalidArgument error.
			err = errs.WrapCode(err, errs.InvalidArgument, "validation failed")

			return middleware.Response{Err: err}
		}
	}

	return next(req)
}

// CreateBillRequest is the request body for creating a new bill.
type CreateBillRequest struct {
	Currency      libmoney.Currency `json:"currency" validate:"required,oneof=GEL USD"`
	BillingPeriod string            `json:"billingPeriod" validate:"required,datetime=2006-01"` // Validates YYYY-MM format
}

func (cbr *CreateBillRequest) Validate() error {
	// Use the helper to validate the query parameter struct.
	if err := validation.Struct(cbr); err != nil {
		return err
	}

	return nil
}

type BillResponse struct {
	ID            string                 `json:"id"`
	CustomerID    string                 `json:"customerId"`
	Currency      string                 `json:"currency"`
	BillingPeriod string                 `json:"billingPeriod"`
	Status        string                 `json:"status"`
	Items         []BillLineItemResponse `json:"items"`
	Total         string                 `json:"total"`
	CreatedAt     time.Time              `json:"createdAt"`
	UpdatedAt     time.Time              `json:"updatedAt"`
	ClosedAt      *time.Time             `json:"closedAt,omitempty"`
}

type BillLineItemResponse struct {
	IdempotencyKey string         `json:"idempotencyKey"`
	Description    string         `json:"description"`
	Amount         libmoney.Money `json:"amount"`
	AddedAt        time.Time      `json:"addedAt"`
}

type CreateBillResponse struct {
	Message  *BillResponse `json:"message"`
	Status   int           `encore:"httpstatus"`
	Location string        `header:"Location"`
}

// CreateBill initiates a new Temporal Workflow to represent a new monthly bill.
// encore:api public method=POST path=/api/v1/customers/:customerID/bills tag:validation
func (s *Service) CreateBill(
	ctx context.Context,
	customerID string,
	req *CreateBillRequest,
) (*CreateBillResponse, error) {
	// Add a simple manual check for the path parameter.
	if customerID == "" || len(customerID) > 1024 {
		return nil, &errs.Error{
			Code:    errs.InvalidArgument,
			Message: "customerId should be not empty and fit length restriction",
		}
	}

	b, err := s.Create.Handle(ctx, usecases.CreateBillCmd{
		CustomerID: customerID, Period: domain.BillingPeriod(req.BillingPeriod), Currency: req.Currency,
	})
	if err != nil {
		rlog.Error("Create.Handle", "err", err)
		if errors.Is(err, app.ErrBillWithPeriodAlreadyStarted) {
			// this code also sets 409 Conflict
			return nil, errs.B().Code(errs.AlreadyExists).Msg("a bill already exists for this customer and period").Err()
		}
		// map adapter error strings/types to HTTP codes as needed
		return nil, errs.B().Code(errs.Internal).Cause(err).Msg("create bill error in api").Err()
	}
	loc := fmt.Sprintf("/api/v1/customers/%s/bills/%s", customerID, req.BillingPeriod) // make it RESTful

	return &CreateBillResponse{
		Message:  map2BillingResponse(b),
		Status:   http.StatusCreated,
		Location: loc,
	}, nil
}

type AddLineItemRequest struct {
	Description    string `json:"description" validate:"required,min=2,max=1024"`
	Amount         string `json:"amount" validate:"required,min=1,max=100"`
	IdempotencyKey string `json:"IdempotencyKey" validate:"required,min=1,max=1024"`
	// currency enforced in workflow to match bill currency
}

func (cbr *AddLineItemRequest) Validate() error {
	// Use the helper to validate the query parameter struct.
	if err := validation.Struct(cbr); err != nil {
		return err
	}

	return nil
}

// AddLineItem sends a Temporal Signal to an open bill's workflow to add a new fee.
// encore:api public method=POST path=/api/v1/customers/:customerID/bills/:period/items tag:validation
func (s *Service) AddLineItem(
	ctx context.Context,
	customerID string,
	period string,
	req *AddLineItemRequest,
) (*BillResponse, error) {
	if _, err := time.Parse("2006-01", period); err != nil {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("invalid period").Cause(err).Err()
	}
	// currency enforced in workflow as derived from Bill Currency
	amount, err := libmoney.NewFromString(req.Amount, libmoney.CurrencyNone)
	if err != nil {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("amount is invalid").Err()
	}

	item := domain.LineItem{
		Description:    req.Description,
		Amount:         amount,
		IdempotencyKey: req.IdempotencyKey,
	}
	b, err := s.AddItem.Handle(ctx, usecases.AddLineItemCmd{
		CustomerID: customerID, Period: domain.BillingPeriod(period), Item: item,
	})
	if err != nil {
		rlog.Error("AddItem.Handle", "err", err)
		if errors.Is(err, app.ErrLineItemAlreadyAdded) {
			// this code also sets 409 Conflict
			return nil, errs.B().Code(errs.AlreadyExists).Msg("the line item already added").Err()
		}
		if errors.Is(err, app.ErrBillNotFound) {
			return nil, errs.B().Code(errs.NotFound).Msg("bill not found").Err()
		}
		if errors.Is(err, app.ErrBillAlreadyClosed) {
			return nil, errs.B().Code(errs.FailedPrecondition).Msg("bill already closed").Err()
		}

		return nil, errs.B().Cause(err).Msg("add item").Err()
	}

	return map2BillingResponse(b), nil
}

// ListBillsQueryParams defines the query parameters for the ListBills endpoint.
type ListBillsQueryParams struct {
	// Filter results by bill status (OPEN or CLOSED).
	// This must be a pointer to a built-in type, like *string.
	Status      string `query:"status" validate:"oneof=OPEN CLOSED"`
	PeriodStart string `query:"from" validate:"datetime=2006-01"` // Validates YYYY-MM format
	PeriodEnd   string `query:"to" validate:"datetime=2006-01"`   // Validates YYYY-MM format
}

func (cbr *ListBillsQueryParams) Validate() error {
	// Use the helper to validate the query parameter struct.
	if err := validation.Struct(cbr); err != nil {
		return err
	}

	return nil
}

// ListBillsResponse defines the structure for the list response.
type ListBillsResponse struct {
	Bills []ListBillResponse `json:"bills"`
}

type ListBillResponse struct {
	ID            string `json:"id"`
	CustomerID    string `json:"customerId"`
	Currency      string `json:"currency"`
	BillingPeriod string `json:"billingPeriod"`
	Status        string `json:"status"`
	ItemCount     int64  `json:"itemCount"`
	Total         string `json:"total"`
}

// ListBills retrieves a list of bills (open or closed) for a customer.
// This would query Temporal for workflows associated with the customer.
// GET /api/v1/customers/:customerId/bills?status=OPEN|CLOSED
// encore:api public method=GET path=/api/v1/customers/:customerID/bills tag:validation
func (s *Service) ListBills(
	ctx context.Context,
	customerID string,
	params *ListBillsQueryParams,
) (*ListBillsResponse, error) {
	if customerID == "" {
		return nil, &errs.Error{Code: errs.InvalidArgument, Message: "customerId cannot be empty"}
	}

	// Use the optional 'status' parameter to filter the query.
	if err := validation.Struct(params); err != nil {
		rlog.Error("validation.Struct", "err", err)

		return nil, errs.B().Code(errs.InvalidArgument).Cause(err).Msg("body is invalid").Err()
	}

	bills, err := s.Search.Handle(ctx, usecases.SearchBillCmd{
		CustomerID: customerID,
		PeriodFrom: domain.BillingPeriod(params.PeriodStart),
		PeriodTo:   domain.BillingPeriod(params.PeriodEnd),
		Status:     params.Status,
	})
	if err != nil {
		rlog.Error("Search.Handle", "err", err)

		return nil, &errs.Error{Code: errs.Internal, Message: "calling search from api"}
	}
	resp := mapBillListResponse(bills)

	return &resp, nil
}

// GetBill retrieves the detailed state of a specific bill by its period.
// This would use a Temporal Query to get the current state of a running or completed workflow.
// encore:api public method=GET path=/api/v1/customers/:customerID/bills/:period
func (s *Service) GetBill(ctx context.Context, customerID string, period string) (*BillResponse, error) {
	if customerID == "" {
		return nil, &errs.Error{Code: errs.InvalidArgument, Message: "customerId cannot be empty"}
	}
	if period == "" {
		return nil, &errs.Error{Code: errs.InvalidArgument, Message: "period cannot be empty"}
	}
	if _, err := time.Parse("2006-01", period); err != nil {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("period must be YYYY-MM").Err()
	}

	b, err := s.Get.Handle(ctx, usecases.GetBillCmd{
		CustomerID: customerID, Period: domain.BillingPeriod(period),
	})
	if err != nil {
		rlog.Error("Get.Handle", "err", err)
		if errors.Is(err, app.ErrBillNotFound) {
			return nil, errs.B().Code(errs.NotFound).Msg("bill not found").Err()
		}
		// map adapter error strings/types to HTTP codes as needed
		return nil, errs.B().Cause(err).Msg("create bill").Err()
	}

	return map2BillingResponse(b), nil
}

// CloseBill sends a Temporal Signal to finalize and close an active bill.
// encore:api public method=POST path=/api/v1/customers/:customerID/bills/:period/close
func (s *Service) CloseBill(ctx context.Context, customerID string, period string) (*BillResponse, error) {
	if customerID == "" {
		return nil, &errs.Error{Code: errs.InvalidArgument, Message: "customerId cannot be empty"}
	}
	if _, err := time.Parse("2006-01", period); err != nil {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("period must be YYYY-MM").Err()
	}

	b, err := s.Close.Handle(ctx, usecases.CloseBillCmd{CustomerID: customerID, Period: domain.BillingPeriod(period)})
	if err != nil {
		rlog.Error("Close.Handle", "err", err)
		if errors.Is(err, app.ErrBillNotFound) {
			return nil, errs.B().Code(errs.NotFound).Msg("bill not found").Err()
		}
		if errors.Is(err, app.ErrBillAlreadyClosed) {
			return nil, errs.B().Code(errs.FailedPrecondition).Msg("bill already closed").Err()
		}

		return nil, errs.B().Cause(err).Msg("close bill").Err()
	}

	return map2BillingResponse(b), nil
}
