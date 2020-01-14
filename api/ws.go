package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

type Websocket struct {
	Pubsub  *PubSub
	listen  string
	actions []*Action
}

func NewWebsocket(conf *APIConfig) *Websocket {
	ws := &Websocket{
		Pubsub:  NewPubSub(),
		listen:  conf.ListenWS,
		actions: conf.Actions,
	}
	return ws
}

func (ws *Websocket) Publish(topic string, data interface{}) {
	ws.Pubsub.PublishJSON(topic, data)
}

func (ws *Websocket) Start() {
	go func() {
		http.HandleFunc("/ws", ws.mainHandler)
		err := http.ListenAndServe(ws.listen, nil)
		if err != nil {
			log.Fatal(err)
		}
	}()
	log.Infof("WS api listen: %s", ws.listen)
}

func (ws *Websocket) mainHandler(w http.ResponseWriter, r *http.Request) {
	ws.Pubsub.upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}
	// Time allowed to read the next pong message from the peer.
	pongWait := 30 * time.Second
	//writeWait := 30 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod := (pongWait * 9) / 10
	// Connection
	conn, err := ws.Pubsub.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Info("upgrade:", err)
		conn.Close()
		return
	}

	// Create a pubsub client
	client := Client{
		id:         uuid.Must(uuid.NewRandom()).String(),
		connection: conn,
		mu:         &ws.Pubsub.mu,
	}
	// Add PONG handler
	client.connection.SetPongHandler(func(msg string) error {
		//client.Connection.SetWriteDeadline(time.Now().Add(writeWait))
		client.connection.SetReadDeadline(time.Now().Add(pongWait))
		log.Info("Received PON from client id: ", client.id)
		return nil
	})
	// Add message handler
	client.SetMsgHandlers(func(c *Client, msg []byte) {
		// Call onAction
		ws.onAction(c, msg)
	}, func(c *Client) {
		// Remove client when ws is closed
		ws.onRemoved(c)
	})
	// Register pubsub client to pubsub manager
	ws.Pubsub.AddClient(&client)
	// Send Hello msg
	client.SendJSON(NewSuccessResponse("", "Websocket connection is succesful", nil))
	// Pubsub client
	log.Info("New Client is connected, total: ", len(ws.Pubsub.clients))

	ticker := time.NewTicker(24 * time.Second)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				ws.Pubsub.PublishPing(pingPeriod)
			}
		}
	}()
}

func (ws *Websocket) onAction(c *Client, msg []byte) {
	req := Request{}
	err := json.Unmarshal(msg, &req)
	if err != nil {
		c.SendJSON(NewErrorResponse("", err.Error()))
		return
	}
	for _, action := range ws.actions {
		if action.key == req.Action && action.method == "WS:" {
			action.HandlerWS(c, &req)
		}
	}
}

func (ws *Websocket) onRemoved(c *Client) {
	ws.Pubsub.RemoveClient(c)
	log.Infof("Client removed %s", c.id)
}
