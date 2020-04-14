package common

import (
	"fmt"
	"math/big"
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
