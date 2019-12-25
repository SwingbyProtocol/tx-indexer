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

func (client *Client) SetMsgHandlers(handler func(c *Client, msg []byte), onError func(c *Client)) {
	go func() {
		for {
			_, message, err := client.Connection.ReadMessage()
			if err != nil {
				onError(client)
				break
			}
			handler(client, message)
		}
	}()
}
