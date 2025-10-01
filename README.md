# Monthly Billing System

A comprehensive monthly billing system built with **Encore** and **Temporal** for progressive fee accrual and invoice management. This system demonstrates enterprise-grade patterns including Clean Architecture, Domain-Driven Design, and Temporal workflow orchestration.

## Overview

This system provides a complete billing solution that:
- **Creates monthly bills** for customers with configurable currencies (USD/GEL)
- **Accrues fees progressively** by adding line items throughout the billing period
- **Closes bills** when the billing period ends, triggering invoice generation
- **Maintains data consistency** through Temporal workflows and idempotency
- **Provides comprehensive APIs** for bill management and querying

## Architecture

### Clean Architecture Implementation

The system follows Clean Architecture principles with clear separation of concerns:

```
fees/
├── app/                    # Application Layer
│   ├── ports.go           # Interface definitions
│   ├── usecases/          # Business logic use cases
│   ├── workflows/         # Temporal workflow definitions
│   └── views/             # Data transfer objects
├── domain/                # Domain Layer
│   ├── bill.go           # Core business entities
│   └── id.go             # Domain value objects
├── internal/              # Internal Infrastructure
│   ├── adapters/         # External service adapters
│   └── validation/       # Input validation utilities
└── services/             # Service Layer
    ├── feesapi/          # REST API service
    └── worker/           # Temporal worker service
```

### Key Components

#### **Domain Layer** (`fees/domain/`)
- **`Bill`** - Core aggregate representing a monthly bill
- **`LineItem`** - Individual fee items with idempotency support
- **`BillID`** - Domain value object for bill identification
- **Business Rules**: State transitions, currency handling, total calculations

#### **Application Layer** (`fees/app/`)
- **Use Cases**: CreateBill, AddLineItem, CloseBill, GetBill, SearchBills
- **Workflows**: MonthlyFeeAccrualWorkflow (Temporal orchestration)
- **Ports**: Interfaces for external dependencies (TemporalPort)
- **DTOs**: Data transfer objects for API communication

#### **Infrastructure Layer** (`fees/internal/`)
- **Temporal Gateway**: Adapter for Temporal workflow operations
- **Validation**: Input validation using go-playground/validator
- **Money Library**: Custom currency handling with decimal precision

#### **Service Layer** (`fees/services/`)
- **FeesAPI**: RESTful API service with Encore
- **Worker**: Temporal worker for workflow execution
- **Configuration**: Environment-specific settings

## Temporal Workflow Design

### Monthly Fee Accrual Workflow

The core business process is modeled as a single Temporal workflow:

```go
MonthlyFeeAccrualWorkflow(ctx, params MonthlyFeeAccrualWorkflowParams)
```

**Workflow Lifecycle:**
1. **Initialization**: Creates a new `domain.Bill` with OPEN status
2. **Progressive Accrual**: Accepts `SignalAddLineItem` to add fees
3. **Closure**: Accepts `SignalCloseBill` to finalize the bill
4. **Invoice Processing**: Executes activities for external invoicing
5. **Completion**: Transitions bill to CLOSED status

**Key Features:**
- **Idempotency**: Duplicate line items are ignored based on idempotency keys
- **State Management**: Bill state is maintained within the workflow
- **Search Attributes**: Real-time visibility through Temporal search attributes
- **Error Handling**: Robust error handling with retry policies
- **Query Support**: Real-time bill state queries via `QueryState`

### Search Attributes

The system uses Temporal search attributes for visibility and filtering:

| Attribute | Type | Purpose |
|-----------|------|---------|
| `CustomerID` | Keyword | Filter bills by customer |
| `BillingPeriodNum` | Int | Filter by billing period (YYYYMM) |
| `BillStatus` | Keyword | Filter by bill status (OPEN/PENDING/CLOSED) |
| `BillCurrency` | Keyword | Filter by currency (USD/GEL) |
| `BillItemCount` | Int | Track number of line items |
| `BillTotalCents` | Int | Track total amount in cents |

## API Design

### RESTful Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/customers/{customerID}/bills` | Create a new monthly bill |
| `POST` | `/api/v1/customers/{customerID}/bills/{period}/items` | Add a line item to a bill |
| `POST` | `/api/v1/customers/{customerID}/bills/{period}/close` | Close a bill |
| `GET` | `/api/v1/customers/{customerID}/bills/{period}` | Get bill details |
| `GET` | `/api/v1/customers/{customerID}/bills` | List bills with filters |

### Request/Response Examples

**Create Bill:**
```json
POST /api/v1/customers/cust-1/bills
{
  "currency": "USD",
  "billingPeriod": "2025-01"
}
```

**Add Line Item:**
```json
POST /api/v1/customers/cust-1/bills/2025-01/items
{
  "description": "API usage fee",
  "amount": "10.50",
  "IdempotencyKey": "api-fee-2025-01-15"
}
```

**Bill Response:**
```json
{
  "id": "bill/cust-1/2025-01",
  "customerId": "cust-1",
  "currency": "USD",
  "billingPeriod": "2025-01",
  "status": "OPEN",
  "items": [
    {
      "idempotencyKey": "api-fee-2025-01-15",
      "description": "API usage fee",
      "amount": {"Value": "10.50", "Currency": "USD"},
      "addedAt": "2025-01-15T10:30:00Z"
    }
  ],
  "total": "10.50",
  "createdAt": "2025-01-01T00:00:00Z",
  "updatedAt": "2025-01-15T10:30:00Z"
}
```

## Data Models

### Domain Entities

**Bill Aggregate:**
```go
type Bill struct {
    ID            BillID
    CustomerID    string
    Currency      libmoney.Currency
    BillingPeriod BillingPeriod
    Status        BillStatus
    Items         []LineItem
    Total         libmoney.Money
    CreatedAt     time.Time
    UpdatedAt     time.Time
    FinalizedAt   *time.Time
}
```

**Line Item:**
```go
type LineItem struct {
    IdempotencyKey string
    Description    string
    Amount         libmoney.Money
    AddedAt        time.Time
}
```

### State Transitions

The bill follows a strict state machine:

```
OPEN → PENDING → CLOSED
  ↓       ↓
ERROR ← ERROR
```

- **OPEN**: Bill is active and accepting line items
- **PENDING**: Bill is being processed (invoicing/charging)
- **CLOSED**: Bill is finalized and no longer accepting items
- **ERROR**: Bill encountered an error during processing

## Testing Strategy

### Comprehensive Test Coverage

The system includes extensive testing at all layers:

#### **Domain Tests** (`fees/domain/`)
- ✅ Bill state transitions and business rules
- ✅ Line item idempotency and currency handling
- ✅ Total calculation and consistency checks

#### **Use Case Tests** (`fees/app/usecases/`)
- ✅ All business logic scenarios
- ✅ Error handling and edge cases
- ✅ Mock-based testing with testify

#### **Workflow Tests** (`fees/app/workflows/`)
- ✅ Temporal workflow execution
- ✅ Signal handling and state management
- ✅ Activity execution and error scenarios

#### **API Tests** (`fees/services/feesapi/`)
- ✅ REST endpoint functionality
- ✅ Request validation and error responses
- ✅ Integration with use cases

#### **Gateway Tests** (`fees/internal/adapters/temporal/`)
- ✅ Temporal client interactions
- ✅ Search attribute handling
- ✅ Error mapping and propagation

### Test Execution

```bash
# Run all tests
encore test ./...

# Run specific test suites
encore test ./fees/domain/... -v
encore test ./fees/app/usecases/... -v
encore test ./fees/services/feesapi/... -v
```

## Configuration

### Environment Setup

The system uses Encore's configuration system with environment-specific settings:

**Development Configuration:**
```cue
#Config: {
    Temporal: {
        Host:      "localhost:7233"
        Namespace: "default"
        UseTLS:    false
        UseAPIKey: false
    }
}
```

### Temporal Setup

The system requires Temporal search attributes to be registered:

```bash
temporal operator search-attribute create --namespace default --name CustomerID --type Keyword
temporal operator search-attribute create --namespace default --name BillingPeriodNum --type Int
temporal operator search-attribute create --namespace default --name BillStatus --type Keyword
temporal operator search-attribute create --namespace default --name BillCurrency --type Keyword
temporal operator search-attribute create --namespace default --name BillItemCount --type Int
temporal operator search-attribute create --namespace default --name BillTotalCents --type Int
```

## Error Handling

### Error Types and Mapping

The system implements comprehensive error handling:

| Domain Error | HTTP Status | Description |
|--------------|-------------|-------------|
| `ErrBillWithPeriodAlreadyStarted` | 409 Conflict | Bill already exists for customer/period |
| `ErrLineItemAlreadyAdded` | 409 Conflict | Line item already added (idempotency) |
| `ErrBillNotFound` | 404 Not Found | Bill does not exist |
| `ErrBillAlreadyClosed` | 412 Precondition Failed | Bill is already closed |
| Validation Errors | 400 Bad Request | Invalid input data |

### Error Response Format

```json
{
  "code": "invalid_argument",
  "message": "Validation failed for field 'Currency' with rule 'oneof'",
  "details": []
}
```

## Performance Considerations

### Scalability Features

- **Temporal Workflows**: Horizontal scaling through Temporal's distributed execution
- **Search Attributes**: Efficient querying and filtering of bills
- **Idempotency**: Safe retry mechanisms for external integrations
- **Async Processing**: Non-blocking invoice generation and charging

### Monitoring and Observability

- **Encore Dashboard**: Built-in request tracing and metrics
- **Temporal UI**: Workflow execution monitoring
- **Structured Logging**: Comprehensive logging with context
- **Error Tracking**: Detailed error reporting and stack traces

## Security Considerations

- **Input Validation**: Comprehensive validation using go-playground/validator
- **Idempotency Keys**: Prevent duplicate operations and ensure data consistency
- **Currency Handling**: Precise decimal arithmetic for financial calculations
- **Error Sanitization**: Safe error messages without sensitive data exposure

## Build from scratch

## Prerequisites 

**Install Encore:**
- **macOS:** `brew install encoredev/tap/encore`
- **Linux:** `curl -L https://encore.dev/install.sh | bash`
- **Windows:** `iwr https://encore.dev/install.ps1 | iex`
  
**Docker:**
1. [Install Docker](https://docker.com)
2. Start Docker

**Run setup.sh**

Setup currently supported at macOS and Linux only. 
It starts Temporal Dev Server (temporal.io) docker and registers search parameters in the Temporal Dev Server.

On Windows, you could run setup manually from command line:
```bash
docker run --rm -p 7233:7233 -p 8233:8233 temporalio/temporal:latest server start-dev --ip 0.0.0.0
temporal operator search-attribute create --namespace default --name CustomerID --type Keyword
temporal operator search-attribute create --namespace default --name BillingPeriodNum  --type Int
temporal operator search-attribute create --namespace default --name BillStatus --type Keyword
temporal operator search-attribute create --namespace default --name BillCurrency --type Keyword
temporal operator search-attribute create --namespace default --name BillItemCount --type Int
temporal operator search-attribute create --namespace default --name BillTotalCents --type Int
```


## Run app

Run app using command line from the root of this repository:

```bash
encore run
```

## Using the API

### API Examples

Create a bill (currency: GEL or USD, period YYYY-MM):
```bash
curl -sS -X POST 'http://127.0.0.1:4000/api/v1/customers/cust-1/bills' \
  -H 'Content-Type: application/json' \
  -d '{"currency":"USD","billingPeriod":"2025-09"}' | jq .
```

Add a line item (idempotent):
```bash
curl -sS -X POST 'http://127.0.0.1:4000/api/v1/customers/cust-1/bills/2025-09/items' \
  -H 'Content-Type: application/json' \
  -d '{"description":"api fee","amount":"2.50","IdempotencyKey":"li-1"}' | jq .
```

Close the bill:
```bash
curl -sS -X POST 'http://127.0.0.1:4000/api/v1/customers/cust-1/bills/2025-09/close' | jq .
```

Get bill:
```bash
curl -sS 'http://127.0.0.1:4000/api/v1/customers/cust-1/bills/2025-09' | jq .
```

List bills for customer (filter by status and period range):
```bash
curl -sS 'http://127.0.0.1:4000/api/v1/customers/cust-1/bills?status=OPEN&from=2025-01&to=2025-12' | jq .
```

## Open the developer dashboard

While `encore run` is running, open [http://localhost:9400/](http://localhost:9400/) to access Encore's [local developer dashboard](https://encore.dev/docs/go/observability/dev-dash).

Here you can see traces for all your requests, view your architecture diagram, and see API docs in the Service Catalog.

## Connecting to databases

You can connect to your databases via psql shell:

```bash
encore db shell <database-name> --env=local --superuser
```

Learn more in the [CLI docs](https://encore.dev/docs/go/cli/cli-reference#database-management).

## Deployment

### Self-hosting

See the [self-hosting instructions](https://encore.dev/docs/go/self-host/docker-build) for how to use `encore build docker` to create a Docker image and configure it.

## Testing

```bash
encore test ./...
```


Task queues and config:
- Worker registers workflows on task queue `FEES_TASK_QUEUE` (see `fees/services/worker/service.go`).
- API starts workflows with the same task queue via the Temporal gateway.