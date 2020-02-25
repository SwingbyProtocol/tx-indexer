package utils

import (
	"math/rand"

	"github.com/btcsuite/btcd/wire"
	"github.com/shopspring/decimal"
)

const WitnessScaleFactor = 4

func RandRange(min int, max int) int {
	return rand.Intn(max-min) + min
}

func GetMaxMin(ranks map[string]uint64) (uint64, uint64, string, []string) {
	top := uint64(0)
	min := uint64(^uint(0) >> 1)
	topAddr := ""
	olders := []string{}
	sorted := []string{}
	for addr, p := range ranks {
		if top < p {
			top = p
			topAddr = addr
		}
		if min > p {
			min = p
			if len(ranks) > 0 {
				olders = append(olders, addr)
			}
		}
	}
	for i := len(olders) - 1; i >= 0; i-- {
		sorted = append(sorted, olders[i])
	}
	return top, min, topAddr, sorted
}

func CheckExist(key string, array []string) bool {
	isexist := false
	for _, id := range array {
		if id == key {
			isexist = true
		}
	}
	return isexist
}

func GetTransactionWeight(msgTx *wire.MsgTx) int64 {
	baseSize := msgTx.SerializeSizeStripped()
	totalSize := msgTx.SerializeSize()
	// (baseSize * 3) + totalSize
	return int64((baseSize * (WitnessScaleFactor - 1)) + totalSize)
}

func ValueSat(value interface{}) float64 {
	x := decimal.NewFromFloat(value.(float64))
	y := decimal.NewFromFloat(100000000)
	sat, _ := x.Mul(y).Float64()
	return sat
}
