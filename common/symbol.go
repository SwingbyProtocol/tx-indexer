package common

import "strings"

var (
	BTC   = NewSymbol("BTC", 8)
	BNB   = NewSymbol("BNB", 8)
	BTCB1 = NewSymbol("BTC.B", 8)
	BTCB2 = NewSymbol("BTCB", 8)
	ETH   = NewSymbol("ETH", 18)
)

// If this list is updated please also update it in data/fees.go
var Symbols = []Symbol{BTC, BNB, BTCB1, BTCB2}

type Symbol struct {
	symbol   string
	decimals int
}

func NewSymbol(s string, decimals int) Symbol {
	return Symbol{strings.ToUpper(s), decimals}
}

func (s *Symbol) Decimlas() int {
	return s.decimals
}

func (s *Symbol) String() string {
	return s.symbol
}
