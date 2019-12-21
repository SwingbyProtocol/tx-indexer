package common

import (
	"math/rand"
	"time"
)

func RandRange(min int, max int) int {
	return rand.Intn(max-min) + min
}

func GetMaxMin(ranks map[string]uint64) (uint64, uint64, string, []string) {
	top := uint64(0)
	min := uint64(1000000)
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

func loop(f func() error, t time.Duration) {
	inv := time.NewTicker(t)
	call(f)
	go func() {
		for {
			select {
			case <-inv.C:
				call(f)
			}
		}
	}()
}

func call(f func() error) {
	go func() {
		err := f()
		if err != nil {
			//log.Info(err)
		}
	}()
}

func checkExist(key string, array []string) bool {
	isexist := false
	for _, id := range array {
		if id == key {
			isexist = true
		}
	}
	return isexist
}
