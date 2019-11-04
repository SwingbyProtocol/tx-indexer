package pubsub

import (
	"encoding/json"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

const (
	WATCHTXS   = "watchTxs"
	UNWATCHTXS = "unwatchTxs"
	GETTXS     = "getTxs"
)

type PubSub struct {
	Clients       []Client
	Subscriptions []Subscription
}

type Client struct {
	ID         string
	Connection *websocket.Conn
}

type Message struct {
	Action  string          `json:"action"`
	Address string          `json:"address"`
	Params  json.RawMessage `json:"params"`
}

type Subscription struct {
	Topic  string
	Client *Client
}

func (ps *PubSub) AddClient(client Client) *PubSub {
	ps.Clients = append(ps.Clients, client)
	//fmt.Println("adding new client to the list", client.Id, len(ps.Clients))
	msg := "Hello Client ID: " + client.ID
	log.Info(msg)
	payload := []byte(msg)
	client.Connection.WriteMessage(1, payload)
	return ps
}

func (ps *PubSub) RemoveClient(client Client) *PubSub {
	for index, sub := range ps.Subscriptions {
		if client.ID == sub.Client.ID {
			if len(ps.Subscriptions) == 0 {
				continue
			}
			ps.Subscriptions = append(ps.Subscriptions[:index], ps.Subscriptions[index+1:]...)
		}
	}
	for index, c := range ps.Clients {
		if c.ID == client.ID {
			ps.Clients = append(ps.Clients[:index], ps.Clients[index+1:]...)
		}
	}
	return ps
}

func (ps *PubSub) GetSubscriptions(topic string, client *Client) []Subscription {
	var subscriptionList []Subscription
	for _, subscription := range ps.Subscriptions {
		if client != nil {
			if subscription.Client.ID == client.ID && subscription.Topic == topic {
				subscriptionList = append(subscriptionList, subscription)
			}
		} else {
			if subscription.Topic == topic {
				subscriptionList = append(subscriptionList, subscription)
			}
		}
	}
	return subscriptionList
}

func (ps *PubSub) Subscribe(client *Client, topic string) *PubSub {
	clientSubs := ps.GetSubscriptions(topic, client)
	if len(clientSubs) > 0 {
		// client is subscribed this topic before
		return ps
	}
	newSubscription := Subscription{
		Topic:  topic,
		Client: client,
	}
	ps.Subscriptions = append(ps.Subscriptions, newSubscription)
	return ps
}

func (ps *PubSub) Publish(topic string, msg []byte, excludeClient *Client) {
	subscriptions := ps.GetSubscriptions(topic, nil)
	for _, sub := range subscriptions {
		log.Infof("Sending to client id %s message is %s \n", sub.Client.ID, msg)
		//sub.Client.Connection.WriteMessage(1, message)
		sub.Client.Send(msg)
	}
}
func (client *Client) Send(message []byte) error {
	return client.Connection.WriteMessage(1, message)
}

func (ps *PubSub) Unsubscribe(client *Client, topic string) *PubSub {
	//clientSubscriptions := ps.GetSubscriptions(topic, client)
	for index, sub := range ps.Subscriptions {
		if sub.Client.ID == client.ID && sub.Topic == topic {
			ps.Subscriptions = append(ps.Subscriptions[:index], ps.Subscriptions[index+1:]...)
		}
	}
	return ps
}
