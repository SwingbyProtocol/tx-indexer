package common

import (
	"math/rand"
	"sync"
	"time"
)

var mu = sync.RWMutex{}

func GetMu() *sync.RWMutex {
	return &mu
}

func RandRange(min int, max int) int {
	return rand.Intn(max-min) + min
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
