package common

import (
	"fmt"
	"math"
	"math/big"
	"strings"

	log "github.com/sirupsen/logrus"
)

type Amount interface {
	fmt.Stringer
	// Adds another Amount to this one and returns a new Amount
	Add(Amount) (Amount, error)
	// Subtracts another Amount from this one and returns a new Amount
	Sub(Amount) (Amount, error)
	// Checks for amount equality and returns true if both are equal
	Equals(Amount) bool
	// Returns amount as big int
	BigInt() *big.Int
	// Returns amount in satoshi unit format
	Int() int64
	Uint() uint64
	// Returns the number of decimal precision
	DecimalCount() int
	// Floor the last n number of decimals
	// precision is calculated by Amount.Decimals - n
	// decimals = 6 & n = 3 | 0.123456 -> 0.123000
	FloorDecimals(n int) Amount
	// Returns a string with the number of decimals fixed to the number of decimals for this Amount (e.g. 0.00010000 if 8 decimals)
	StringFixedDecs() string
}

type DynamicAmount struct {
	decimals int
	value    *big.Int
	size     int64
}

type AmountUnit int

const decimalScale = 8

var _ Amount = DynamicAmount{}

func NewAmountFromFloat64(f float64) (DynamicAmount, error) {
	i := big.NewFloat(f)
	if i.Cmp(big.NewFloat(0)) == -1 {
		return DynamicAmount{}, fmt.Errorf("NewAmountFromString: amount %d should not be < 0", i)
	}
	size := math.Pow10(decimalScale)
	i2, _ := big.NewFloat(0).Mul(i, big.NewFloat(size)).Int64()
	return DynamicAmount{
		decimals: decimalScale,
		value:    big.NewInt(i2),
		size:     int64(size),
	}, nil
}

func NewAmountFromString(val string) (DynamicAmount, error) {
	i, ok := new(big.Float).SetString(val)
	if !ok {
		return DynamicAmount{}, fmt.Errorf("Unable to convert %s to big.Float", val)
	}
	if i.Cmp(big.NewFloat(0)) == -1 {
		return DynamicAmount{}, fmt.Errorf("NewAmountFromString: amount %d should not be < 0", i)
	}
	size := math.Pow10(decimalScale)
	i2, _ := big.NewFloat(0).Mul(i, big.NewFloat(size)).Int64()
	return DynamicAmount{
		decimals: decimalScale,
		value:    big.NewInt(i2),
		size:     int64(size),
	}, nil
}

func NewAmountFromInt64(val int64) (DynamicAmount, error) {
	if val < 0 {
		return DynamicAmount{}, fmt.Errorf("NewAmountFromInt: amount %d should not be < 0", val)
	}
	size := math.Pow10(decimalScale)
	return DynamicAmount{
		decimals: decimalScale,
		value:    big.NewInt(val),
		size:     int64(size),
	}, nil
}

func NewAmountFromBigInt(i *big.Int) (DynamicAmount, error) {
	return NewAmountFromInt64(i.Int64())
}

func (b DynamicAmount) Add(y Amount) (Amount, error) {
	return NewAmountFromBigInt(new(big.Int).Add(b.BigInt(), y.BigInt()))
}

func (b DynamicAmount) Sub(y Amount) (Amount, error) {
	return NewAmountFromBigInt(new(big.Int).Sub(b.BigInt(), y.BigInt()))
}

func (b DynamicAmount) Equals(y Amount) bool {
	// TODO: check that decimals are equal too? (for now they're always 8)
	return b.Int() == y.Int()
}

func (b DynamicAmount) Int() int64 {
	return b.value.Int64()
}

func (b DynamicAmount) Uint() uint64 {
	return b.value.Uint64()
}

func (b DynamicAmount) DecimalCount() int {
	return b.decimals
}

func (b DynamicAmount) BigInt() *big.Int {
	return b.value
}

func (b DynamicAmount) FloorDecimals(decimals int) Amount {
	// calculate rounding scale based on provided delta
	newScale := b.decimals - decimals
	if b.decimals < newScale {
		return b
	}
	// doing it with a string prevents rounding
	str := b.String()
	split := strings.SplitN(str, ".", 2)
	if len(split) < 2 {
		log.Warningf("FloorDecimals was unable to work on this number: %s, returning the original amount", str)
		return b
	}
	if len(split) == 1 || split[1] == "" {
		split = []string{split[0], "0"}
	}
	chars := newScale
	if len(split[1]) < chars {
		chars = len(split[1])
	}
	newStr := fmt.Sprintf("%s.%s", split[0], split[1][:chars])
	newAmt, _ := NewAmountFromString(newStr)
	return newAmt
}

func (b DynamicAmount) String() string {
	str := strings.TrimRight(b.StringFixedDecs(), "0")
	if str[len(str)-1] == '.' {
		return str + "0"
	}
	return str
}

func (b DynamicAmount) StringFixedDecs() string {
	// TODO: ugly but works
	if b.value == nil {
		return "0.0"
	}
	return fmt.Sprintf("%."+fmt.Sprintf("%d", b.decimals)+"f", b.bigFloat())
}

func (b DynamicAmount) bigFloat() *big.Float {
	// TODO: unexpected side effects with large amounts, try to limit use of this
	v := big.NewFloat(float64(b.value.Int64()))
	return v.Quo(v, big.NewFloat(float64(b.size)))
}
