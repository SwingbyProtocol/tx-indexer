// Copyright 2015 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

package main

import (
	"flag"
	"log"
	"net/url"

	"github.com/gorilla/websocket"
)

//var uri = "btc-testnet-368e99527ef714a6.elb.ap-southeast-1.amazonaws.com"
var uri = "localhost:9099"
var addr = flag.String("addr", uri, "http service address")

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

func main() {
	flag.Parse()
	log.SetFlags(0)

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/ws"}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				break
			}
			log.Printf("recv: %s", message)
			//c.SetWriteDeadline(time.Now().Add(20 * time.Second))
		}
	}()

	str := `{"action":"watchTxs","address":"2MsM67NLa71fHvTUBqNENW15P68nHB2vVXb", "params":"message test"}`
	err = c.WriteMessage(websocket.TextMessage, []byte(str))
	if err != nil {
		log.Println("write:", err)
	}

	select {}
}
