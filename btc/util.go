package btc

import (
	"sync"
	"time"
)

var mu = sync.RWMutex{}

func GetMu() *sync.RWMutex {
	return &mu
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
	isExist := false
	for _, id := range array {
		if id == key {
			isExist = true
		}
	}
	return isExist
}
