package api

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

type Client struct {
	id         string
	connection *websocket.Conn
	mu         *sync.Mutex
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
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.connection.WriteMessage(1, message)
}

func (client *Client) Ping(writeWait time.Duration) error {
	client.connection.SetWriteDeadline(time.Now().Add(writeWait))
	err := client.connection.WriteMessage(websocket.PingMessage, []byte("Ping"))
	if err != nil {
		return err
	}
	return nil
}

func (client *Client) SetMsgHandlers(handler func(c *Client, msg []byte), onError func(c *Client)) {
	go func() {
		for {
			_, message, err := client.connection.ReadMessage()
			if err != nil {
				onError(client)
				break
			}
			handler(client, message)
		}
	}()
}

func (client *Client) ID() string {
	return client.id
}

func (client *Client) Conn() *websocket.Conn {
	return client.connection
}
