package main

import (
	"fmt"
	"time"
)

var count = 0

func main() {
	run()
	select {}
}

func run() {
	err := doLoop()
	if err != nil {
		fmt.Print(err)
	}
	time.Sleep(1 * time.Nanosecond)
	go run()
	return
}
func doLoop() error {
	count++
	fmt.Printf("start %d \n", count)
	return nil
}
