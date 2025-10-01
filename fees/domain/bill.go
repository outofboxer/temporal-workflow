package domain

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	libmoney "github.com/outofboxer/temporal-workflow/libs/money"
)

type BillStatus string

const (
	BillStatusUnknown BillStatus = "" // also like a fallback from bad conversions from strings.
	BillStatusOpen    BillStatus = "OPEN"
	BillStatusPending BillStatus = "PENDING"
	BillStatusClosed  BillStatus = "CLOSED"
	BillStatusError   BillStatus = "ERROR"
)

var allowed = map[BillStatus]map[BillStatus]bool{
	BillStatusOpen:    {BillStatusPending: true, BillStatusError: true},
	BillStatusPending: {BillStatusClosed: true, BillStatusError: true},
	BillStatusClosed:  {}, // manual copy on restart
	BillStatusUnknown: {BillStatusError: true},
	BillStatusError:   {BillStatusError: true},
}

var (
	ErrInvalidTransition   = errors.New("invalid status transition")
	ErrGuardFailed         = errors.New("status guard failed")
	ErrEmptyIdempotencyKey = errors.New("empty idempotency key")
	ErrBillNotOpen         = errors.New("bill not open")
)

type LineItem struct {
	IdempotencyKey string
	Description    string
	Amount         libmoney.Money
	AddedAt        time.Time
}

type BillingPeriod string

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

func (b *Bill) Transition(to BillStatus, guards ...func(*Bill) error) error {
	// Fail-proof: in functions like Bill.Close, first I change status and then recalculate internals.
	// If execution stopped at recalculation and after status change, at the middle of Bill.Close,
	// this self-transition allowance is required to workflow replay.
	selfTransition := b.Status == to
	if !selfTransition && !allowed[b.Status][to] {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, b.Status, to)
	}
	for _, g := range guards {
		if err := g(b); err != nil {
			return fmt.Errorf("%w: %w", ErrGuardFailed, err)
		}
	}
	b.Status = to

	return nil
}

func (b *Bill) AddItem(idempotencyKey string, description string, amount libmoney.Money, updatedAt time.Time) error {
	if idempotencyKey == "" {
		return ErrEmptyIdempotencyKey
	}
	if b.Status != BillStatusOpen {
		return ErrBillNotOpen
	}
	for _, li := range b.Items {
		if li.IdempotencyKey == idempotencyKey {
			// just skip it, idempotency on the house.
			return nil
		}
	}
	amountMoney := libmoney.NewResetCurrency(amount, b.Currency)
	li := LineItem{
		IdempotencyKey: idempotencyKey,
		Description:    description,
		Amount:         amountMoney,
		AddedAt:        updatedAt,
	}

	b.Items = append(b.Items, li)
	b.Total = b.Total.Add(li.Amount)
	b.UpdatedAt = updatedAt

	return nil
}

func (b *Bill) Pending(now time.Time) error {
	err := b.Transition(BillStatusPending, func(_ *Bill) error {
		// example of guard:
		// if len(b.Items) == 0 {
		//	return fmt.Errorf("cannot close empty bill")
		// }
		return nil
	})
	if err != nil {
		return err
	}
	b.UpdatedAt = now

	return nil
}

func (b *Bill) Close(closedAt time.Time) error {
	err := b.Transition(BillStatusClosed)
	if err != nil {
		return err
	}
	b.UpdatedAt = closedAt
	b.FinalizedAt = &closedAt

	return nil
}

func (b *Bill) Error(closedAt time.Time) error {
	if !b.IsActive() { // includes in ERROR state
		return nil
	}
	b.FinalizedAt = &closedAt

	err := b.Transition(BillStatusError)
	if err != nil {
		return err
	}

	return nil
}

// IsActive Only Open means active and allows to add LineItems.
func (b *Bill) IsActive() bool {
	return b.Status == BillStatusOpen
}

func (b *Bill) IsReadyForInvoicing() bool {
	return b.Status == BillStatusPending
}

func (b *Bill) RecalcTotal() libmoney.Money {
	sum := libmoney.NewFromInt(0, b.Currency)
	for _, li := range b.Items {
		sum = sum.Add(li.Amount)
	}

	return sum
}

var reYYYYMM = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

// Bill Builder goes below

type BillBuilder struct {
	id         BillID
	customerID string
	currency   libmoney.Currency
	period     BillingPeriod
	status     BillStatus
	items      []LineItem
	createdAt  *time.Time
}

func NewBillBuilder() *BillBuilder {
	return &BillBuilder{
		status: BillStatusOpen,
		items:  make([]LineItem, 0),
	}
}

func (b *BillBuilder) WithID(id BillID) *BillBuilder {
	b.id = id

	return b
}

func (b *BillBuilder) ForCustomer(customerID string) *BillBuilder {
	b.customerID = customerID

	return b
}

func (b *BillBuilder) WithCurrency(c libmoney.Currency) *BillBuilder {
	b.currency = c

	return b
}

func (b *BillBuilder) ForPeriodYYYYMM(yyyyMM string) *BillBuilder {
	b.period = BillingPeriod(yyyyMM)

	return b
}

func (b *BillBuilder) ForPeriod(p BillingPeriod) *BillBuilder {
	b.period = p

	return b
}

func (b *BillBuilder) WithCreatedAt(t time.Time) *BillBuilder {
	b.createdAt = &t

	return b
}

func (b *BillBuilder) Open() *BillBuilder {
	b.status = BillStatusOpen

	return b
}

func (b *BillBuilder) Closed() *BillBuilder {
	b.status = BillStatusClosed

	return b
}

func (b *BillBuilder) AddItem(li LineItem) *BillBuilder {
	b.items = append(b.items, li)

	return b
}

func (b *BillBuilder) AddItems(items ...LineItem) *BillBuilder {
	for _, it := range items {
		b.AddItem(it)
	}

	return b
}

func (b *BillBuilder) Build() (Bill, error) {
	if b.id == "" {
		return Bill{}, errors.New("id is required")
	}
	if b.customerID == "" {
		return Bill{}, errors.New("customerID is required")
	}
	if !libmoney.SupportedCurrency(b.currency) {
		return Bill{}, fmt.Errorf("currency must be USD or GEL, got %q", b.currency)
	}
	if !reYYYYMM.MatchString(string(b.period)) {
		return Bill{}, fmt.Errorf("billing period must be YYYY-MM, got %s", b.period)
	}
	if b.createdAt == nil {
		return Bill{}, errors.New("createdAt is required")
	}

	total, err := libmoney.NewFromString("0", b.currency)
	if err != nil {
		return Bill{}, fmt.Errorf("total conversion error, currency: %s", b.currency)
	}
	for _, item := range b.items {
		total = total.Add(item.Amount)
	}

	return Bill{
		ID:            b.id,
		CustomerID:    b.customerID,
		Currency:      b.currency,
		BillingPeriod: b.period,
		Status:        b.status,
		Items:         append([]LineItem(nil), b.items...), // copy for safety
		Total:         total,                               // libmoney.Money{Amount: b.totalSum, Currency: b.currency},
		CreatedAt:     *b.createdAt,                        // checked for nil earlier
		UpdatedAt:     *b.createdAt,
	}, nil
}

// Convenience for tests; panic on invalid setup. DO NOT USE in PROD CODE!
func (b *BillBuilder) MustBuild() Bill {
	bill, err := b.Build()
	if err != nil {
		panic(err)
	}

	return bill
}
