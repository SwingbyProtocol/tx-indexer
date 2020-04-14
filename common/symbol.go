package common

import "strings"

var (
	BTC   = NewSymbol("BTC")
	BNB   = NewSymbol("BNB")
	BTCB1 = NewSymbol("BTC.B")
	BTCB2 = NewSymbol("BTCB")
)

// If this list is updated please also update it in data/fees.go
var Symbols = []Symbol{BTC, BNB, BTCB1, BTCB2}

type Symbol struct {
	symbol string
}

func NewSymbol(s string) Symbol {
	return Symbol{strings.ToUpper(s)}
}

func (s *Symbol) String() string {
	return s.symbol
}
