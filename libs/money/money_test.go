package libmoney

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestNewFromFloat(t *testing.T) {
	input := 1.23
	expected := decimal.NewFromFloat(input)
	actual := NewFromFloat(input)
	assert.Equalf(t, expected, actual.Value, "expected %v, got %v", expected, actual)
}

func TestNewFromInt(t *testing.T) {
	input := 123
	expected := decimal.NewFromInt(int64(input))
	actual := NewFromInt(input)
	assert.Equalf(t, expected, actual.Value, "expected %v, got %v", expected, actual)
}

func TestNewFromString(t *testing.T) {
	inputValue := "1.23"
	expectedValue, err := decimal.NewFromString(inputValue)
	assert.NoError(t, err)
	actualValue, err := NewFromString(inputValue)
	assert.NoError(t, err)
	assert.Equalf(t, expectedValue, actualValue.Value, "expected %v, got %v", expectedValue, actualValue)
}

func TestToFloat64(t *testing.T) {
	input := Money{Value: decimal.NewFromFloat(1.23), Currency: CurrencyUSD}
	expected := float64(1.23)
	actual := input.ToFloat64()
	assert.Equalf(t, expected, actual, "expected %v, got %v", expected, actual)
}

func TestMoney_UnmarshalJSON(t *testing.T) {
	input := `{"some_one_cypher":1.23}`
	expected := Money{Value: decimal.NewFromFloat(1.23)}
	actual := struct {
		SomeOneCypher Money `json:"some_one_cypher"`
	}{}
	err := json.Unmarshal([]byte(input), &actual)
	assert.NoError(t, err)
	assert.Equalf(t, expected.Value, actual.SomeOneCypher.Value, "expected %v, got %v", expected.Value, actual.SomeOneCypher.Value)
}

func TestMoney_UnmarshalJSONQuoted(t *testing.T) {
	input := `{"some_one_cypher":"1.23"}`
	actual := struct {
		SomeOneCypher Money `json:"some_one_cypher"`
	}{}
	err := json.Unmarshal([]byte(input), &actual)
	assert.NoError(t, err)
}

func TestMoney_UnmarshalJSONQuotedInt(t *testing.T) {
	input := `{"some_one_cypher":"1"}`
	actual := struct {
		SomeOneCypher Money `json:"some_one_cypher"`
	}{}
	err := json.Unmarshal([]byte(input), &actual)
	assert.NoError(t, err)
}

func TestMoney_UnmarshalJSONInt(t *testing.T) {
	input := `{"some_one_cypher":"1"}`
	actual := struct {
		SomeOneCypher Money `json:"some_one_cypher"`
	}{}
	err := json.Unmarshal([]byte(input), &actual)
	assert.NoError(t, err)
}

func TestMoney_UnmarshalJSONError(t *testing.T) {
	input := `{"some_one_cypher":1.123.123}`
	actual := struct {
		SomeOneCypher Money `json:"some_one_cypher"`
	}{}
	err := json.Unmarshal([]byte(input), &actual)
	assert.Error(t, err)
}

func TestMoney_UnmarshalJSONEmpty(t *testing.T) {
	input := `{"some_one_cypher":""}`
	actual := struct {
		SomeOneCypher Money `json:"some_one_cypher"`
	}{}
	err := json.Unmarshal([]byte(input), &actual)
	assert.NoError(t, err)
	assert.Equalf(t, Money{}, actual.SomeOneCypher, "expected %v, got %v", Money{}, actual.SomeOneCypher)
}

func TestMoney_UnmarshalJSONNull(t *testing.T) {
	input := `{"some_one_cypher":null}`
	actual := struct {
		SomeOneCypher Money `json:"some_one_cypher"`
	}{}
	err := json.Unmarshal([]byte(input), &actual)
	assert.NoError(t, err)
	assert.Equalf(t, Money{}, actual.SomeOneCypher, "expected %v, got %v", Money{}, actual.SomeOneCypher)
}

func TestMoney_UnmarshalJSONNotExists(t *testing.T) {
	input := `{}`
	actual := struct {
		SomeOneCypher Money `json:"some_one_cypher"`
	}{}
	err := json.Unmarshal([]byte(input), &actual)
	assert.NoError(t, err)
	assert.Equalf(t, Money{}, actual.SomeOneCypher, "expected %v, got %v", Money{}, actual.SomeOneCypher)
}

func TestAdd(t *testing.T) {
	input1 := NewFromInt(1)
	input2 := NewFromInt(2)
	expected := NewFromInt(3)
	actual := input1.Add(input2)
	assert.Equalf(t, expected, actual, "expected %v, got %v", expected, actual)
	input3 := NewFromInt(3)
	input4 := NewFromInt(4)
	expected = NewFromInt(10)
	actual = actual.Add(input3, input4)
	assert.Equalf(t, expected, actual, "expected %v, got %v", expected, actual)
}

func TestMul(t *testing.T) {
	input1 := NewFromInt(10)
	input2 := NewFromInt(2)
	expected := NewFromInt(20)
	actual := input1.Mul(input2)
	assert.Equalf(t, expected, actual, "expected %v, got %v", expected, actual)
}

func TestMulOnInt(t *testing.T) {
	input := NewFromInt(10)
	expected := NewFromInt(20)
	actual := input.MulOnInt(2)
	assert.Equalf(t, expected, actual, "expected %v, got %v", expected, actual)
}

func TestMulOnFloat(t *testing.T) {
	input := NewFromInt(4)
	expected := NewFromFloat(8.8)
	actual := input.MulOnFloat(2.2)
	assert.Equalf(t, expected, actual, "expected %v, got %v", expected, actual)
}

func TestSub(t *testing.T) {
	input1 := NewFromInt(10)
	input2 := NewFromInt(2)
	expected := NewFromInt(8)
	actual := input1.Sub(input2)
	assert.Equalf(t, expected, actual, "expected %v, got %v", expected, actual)
	input3 := NewFromInt(3)
	input4 := NewFromInt(4)
	expected = NewFromInt(1)
	actual = actual.Sub(input3, input4)
	assert.Equalf(t, expected, actual, "expected %v, got %v", expected, actual)
}

func TestGetPercent(t *testing.T) {
	input := Money{Value: decimal.NewFromFloat(120), Currency: CurrencyUSD}
	expected := Money{Value: decimal.NewFromFloat(3.6), Currency: CurrencyUSD}
	actual := input.GetPercent(3)
	assert.Equalf(t, expected.ToFloat64(), actual.ToFloat64(), "expected %v, got %v", expected.ToFloat64(), actual.ToFloat64())
}

func TestIsZero(t *testing.T) {
	input := Money{Value: decimal.NewFromFloat(0), Currency: CurrencyUSD}
	expected := true
	actual := input.IsZero()
	assert.Equalf(t, expected, actual, "expected %v, got %v", expected, actual)
	input = Money{Value: decimal.NewFromFloat(1), Currency: CurrencyUSD}
	expected = false
	actual = input.IsZero()
	assert.Equalf(t, expected, actual, "expected %v, got %v", expected, actual)
}

func TestNeg(t *testing.T) {
	input := NewFromInt(10)
	expected := NewFromInt(-10)
	actual := input.Neg()
	assert.Equalf(t, expected, actual, "expected %v, got %v", expected, actual)
}

func TestIsPositive(t *testing.T) {
	input := NewFromInt(10)
	expected := true
	actual := input.IsPositive()
	assert.Equalf(t, expected, actual, "expected %v, got %v", expected, actual)
	input = NewFromInt(-10)
	expected = false
	actual = input.IsPositive()
	assert.Equalf(t, expected, actual, "expected %v, got %v", expected, actual)
}

func TestIsNegative(t *testing.T) {
	input := NewFromInt(-10)
	expected := true
	actual := input.IsNegative()
	assert.Equalf(t, expected, actual, "expected %v, got %v", expected, actual)
	input = NewFromInt(10)
	expected = false
	actual = input.IsNegative()
	assert.Equalf(t, expected, actual, "expected %v, got %v", expected, actual)
}

func TestToInt64(t *testing.T) {
	input := NewFromFloat(10.123)
	expected := int64(10)
	actual := input.ToInt64()
	assert.Equalf(t, expected, actual, "expected %v, got %v", expected, actual)
}

func TestToPgNumeric(t *testing.T) {
	expected := 10.123
	input := NewFromFloat(expected)
	numeric := input.ToPgNumeric()
	actual, err := numeric.Float64Value()
	assert.NoError(t, err)
	assert.Equalf(t, expected, actual.Float64, "expected %v, got %v", expected, actual)
}

func TestToString(t *testing.T) {
	expected := "10.123"
	input, err := NewFromString(expected)
	assert.NoError(t, err)
	actual := input.ToString()
	assert.Equalf(t, expected, actual, "expected %v, got %v", expected, actual)
}

func TestNewFromPgNumeric(t *testing.T) {
	input := NewFromFloat(10.123)
	pgNumeric := input.ToPgNumeric()
	actual := NewFromPgNumeric(pgNumeric)
	assert.Equalf(t, input, actual, "expected %v, got %v", input, actual)
}
