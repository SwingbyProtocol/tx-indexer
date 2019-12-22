package pubsub

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

type Client struct {
	ID         string
	Connection *websocket.Conn
	Mu         *sync.Mutex
}

func (client *Client) SendJSON(message interface{}) error {
	bytes, err := json.Marshal(message)
	if err != nil {
		log.Info(err)
		return err
	}
	return client.Send(bytes)
}

func (client *Client) Send(message []byte) error {
	// protect concurrent write
	client.Mu.Lock()
	defer client.Mu.Unlock()
	return client.Connection.WriteMessage(1, message)
}

func (client *Client) Ping(writeWait time.Duration) error {
	client.Connection.SetWriteDeadline(time.Now().Add(writeWait))
	err := client.Connection.WriteMessage(websocket.PingMessage, []byte("Ping"))
	if err != nil {
		return err
	}
	return nil
}

func (client *Client) SetMsgHandlers(handler func(c *Client, msg []byte) error) {
	go func() {
		for {
			_, message, err := client.Connection.ReadMessage()
			if err != nil {
				log.Info("WS:error:", err)
				break
			}
			err = handler(client, message)
			if err != nil {
				log.Info("ws:error:", err)
			}
		}
	}()
}

func (client *Client) ReadPump(ps *PubSub, onAction func(c *Client, msg Message), onStop func()) {
	for {
		_, message, err := client.Connection.ReadMessage()
		if err != nil {
			log.Info("WS:error:", err)
			go onStop()
			break
		}
		msg := Message{}
		err = json.Unmarshal(message, &msg)
		if err != nil {
			errMsg := "Error: This is not correct message payload"
			log.Info(errMsg)
			//sendMsg(&client, msg.Action, errMsg)
			continue
		}
	}
	//go onAction(client, msg)
}
