# Monthly Billing System


## Build from scratch

Setup currently supported at macOS and Linux only.

## Prerequisites

**1. Install Encore:**
- **macOS:** `brew install encoredev/tap/encore`
- **Linux:** `curl -L https://encore.dev/install.sh | bash`
- **Windows:** `iwr https://encore.dev/install.ps1 | iex`

**2. Docker:**
1. [Install Docker](https://docker.com)
2. Start Docker

**3. Configure temporal:**

3a. If you have local Temporal dev server
https://learn.temporal.io/getting_started/typescript/dev_environment/

The project configs assume ir should be available:
* The Temporal Service be available on localhost:7233.
* The Temporal Web UI be available at http://localhost:8233.

Run the shell command at the project root dir for local Temporal to setup search attributes:
```bash
make init-temporal
```

3b. If you don't want to install dev Temporal
Run it in docker by this command, this starts Temporal Dev server and configures search attributes:
```bash
make init-temporal-docker
```

## Testing

Download dependencies and run test (this will run 'encore run'):
```bash
make compile
```

## Run app

Run app using command line from the root of this repository:

```bash
encore run
```


## Using the API (test cases)

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

Get bill:
```bash
curl -sS 'http://127.0.0.1:4000/api/v1/customers/cust-1/bills/2025-09' | jq .
```

List bills for customer (filter by status and period range):
```bash
curl -sS 'http://127.0.0.1:4000/api/v1/customers/cust-1/bills?status=OPEN&from=2025-01&to=2025-12' | jq .
```

Close the bill:
```bash
curl -sS -X POST 'http://127.0.0.1:4000/api/v1/customers/cust-1/bills/2025-09/close' | jq .
```

## Open the developer dashboard

While `encore run` is running, open [http://localhost:9400/](http://localhost:9400/) to access Encore's [local developer dashboard](https://encore.dev/docs/go/observability/dev-dash).

Here you can see traces for all your requests, view your architecture diagram, and see API docs in the Service Catalog.


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