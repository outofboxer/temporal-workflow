package libmoney

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

type Currency string

const (
	CurrencyNone Currency = "None" // sometimes we don't know currency or currency is depending on parent object
	CurrencyUSD  Currency = "USD"
	CurrencyGEL  Currency = "GEL"
)

type Money struct {
	value    decimal.Decimal
	currency Currency
}

func SupportedCurrency(currency Currency) bool {
	return currency == CurrencyGEL || currency == CurrencyUSD
}

func NewFromFloat[fl float32 | float64](v fl, c Currency) Money {
	v2 := float64(v)
	if math.IsNaN(v2) {
		return Money{}
	} else if math.IsInf(v2, 0) {
		return Money{}
	}

	return Money{
		value:    decimal.NewFromFloat(v2),
		currency: c,
	}
}

func NewResetCurrency(v Money, c Currency) Money {
	return Money{
		value:    v.value,
		currency: c,
	}
}

func NewFromInt[i int | int8 | int16 | int32 | int64](m i, c Currency) Money {
	return Money{
		value:    decimal.NewFromInt(int64(m)),
		currency: c,
	}
}

func NewFromString(m string, c Currency) (Money, error) {
	if m == "" {
		return Money{
			value:    decimal.Zero,
			currency: c,
		}, errors.New("empty string")
	}
	v, err := decimal.NewFromString(m)
	if err != nil {
		return Money{}, err
	}

	return Money{
		value:    v,
		currency: c,
	}, nil
}

func NewFomBigInt(i *big.Int, e int32, c Currency) Money {
	return Money{
		value:    decimal.NewFromBigInt(i, e),
		currency: c,
	}
}

func NewFomDecimal(v decimal.Decimal, c Currency) Money {
	return Money{
		value:    v,
		currency: c,
	}
}

func NewFromPgNumeric(n *pgtype.Numeric, c Currency) Money {
	if n == nil {
		return Money{}
	}
	intN := n.Int
	if intN == nil {
		return Money{}
	}

	return NewFomBigInt(intN, n.Exp, c)
}

// ToFloat64 returns the float64 representation of the Money value.
// Alias for Amount.Float64().
// TODO thing about round and precision.
func (m *Money) ToFloat64() float64 {
	v, _ := m.value.Float64()

	return v
}

func (m *Money) ToInt64() int64 {
	return m.value.IntPart()
}

func (m *Money) ToString() string {
	return m.value.String()
}

func (m *Money) ToPgNumeric() *pgtype.Numeric {
	var numeric pgtype.Numeric
	if err := numeric.Scan(m.ToString()); err != nil {
		return nil
	}

	return &numeric
}

// MarshalJSON outputs Money as JSON in the format:
//
//	{"Value":"123.45","Currency":"USD"}
//
// "Value" is emitted as a string to avoid precision loss.
func (m Money) MarshalJSON() ([]byte, error) {
	type out struct {
		Value    string `json:"Value"`
		Currency string `json:"Currency"`
	}

	return json.Marshal(out{
		Value:    m.value.String(), // or m.ToString()
		Currency: string(m.currency),
	})
}

// UnmarshalJSON supports:
//
//	{"Value":"123.45","Currency":"USD"}  ← string (safe, recommended)
//	{"Value":123.45,"Currency":"USD"}    ← number (also accepted)
func (m *Money) UnmarshalJSON(data []byte) error {
	// Decode into a light helper so we can parse Value flexibly.
	var aux struct {
		Value    json.RawMessage `json:"Value"`
		Currency string          `json:"Currency"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("money: invalid json: %w", err)
	}

	// Parse Value using decimal's own JSON parser (works for "123.45" or 123.45).
	var d decimal.Decimal
	if len(aux.Value) == 0 || string(aux.Value) == "null" {
		d = decimal.Zero
	} else if err := d.UnmarshalJSON(aux.Value); err != nil {
		return fmt.Errorf("money.value: %w", err)
	}

	*m = NewFomDecimal(d, Currency(aux.Currency))

	return nil
}

func (m *Money) Add(m2 ...Money) Money {
	res := m.value
	for _, v := range m2 {
		res = res.Add(v.value)
	}

	return Money{
		value:    res,
		currency: m.currency,
	}
}

func (m *Money) Sub(m2 ...Money) Money {
	res := m.value
	for _, v := range m2 {
		res = res.Sub(v.value)
	}

	return Money{
		value:    res,
		currency: m.currency,
	}
}

func (m *Money) Mul(m2 Money) Money {
	return Money{
		value:    m.value.Mul(m2.value),
		currency: m.currency,
	}
}

// MulOnInt multiplies the Money value by the given int64 value.
func (m *Money) MulOnInt(m2 int64) Money {
	return Money{
		value:    m.value.Mul(decimal.NewFromInt(m2)),
		currency: m.currency,
	}
}

// MulOnFloat multiplies the Money value by the given int64 value.
func (m *Money) MulOnFloat(m2 float64) Money {
	return Money{
		value:    m.value.Mul(decimal.NewFromFloat(m2)),
		currency: m.currency,
	}
}

func (m *Money) MulOnDecimal(m2 decimal.Decimal) *Money {
	return &Money{
		value:    m.value.Mul(m2),
		currency: m.currency,
	}
}

func (m *Money) IntPart() int64 {
	return m.value.IntPart()
}

func (m *Money) Round(places int32) *Money {
	return &Money{
		value:    m.value.Round(places),
		currency: m.currency,
	}
}

func (m *Money) Div(m2 Money) Money {
	res := m.value.Div(m2.value)

	return Money{
		value:    res,
		currency: m.currency,
	}
}

func (m *Money) Abs() Money {
	res := m.value.Abs()

	return Money{
		value:    res,
		currency: m.currency,
	}
}

// Cmp compares the numbers represented by d and d2 and returns:
//
//	-1 if m <  m2
//	 0 if m == m2
//	+1 if m >  m2
func (m *Money) Cmp(m2 Money) int {
	return m.value.Cmp(m2.value)
}

func (m *Money) IsPositive() bool {
	return m.value.IsPositive()
}

func (m *Money) IsNegative() bool {
	return m.value.IsNegative()
}

func (m *Money) GetPercent(percent float64) Money {
	return NewFromFloat(m.ToFloat64()*percent/100, m.currency) //nolint:mnd
}

func (m *Money) IsZero() bool {
	return m.value.IsZero()
}

// Neg returns -m.
func (m *Money) Neg() Money {
	return Money{
		value:    m.value.Neg(),
		currency: m.currency,
	}
}
