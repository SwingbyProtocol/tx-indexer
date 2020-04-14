package api

import (
	"errors"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

type PubSub struct {
	clients       []*Client
	subscriptions map[string][]*Subscription
	mu            sync.Mutex
	upgrader      websocket.Upgrader
}

type Subscription struct {
	topic  string
	client *Client
}

func NewPubSub() *PubSub {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	ps := &PubSub{
		subscriptions: make(map[string][]*Subscription),
		upgrader:      upgrader,
	}
	return ps
}

func (ps *PubSub) AddClient(c *Client) *PubSub {
	ps.clients = append(ps.clients, c)
	//fmt.Println("adding new client to the list", client.Id, len(ps.Clients))
	return ps
}

func (ps *PubSub) RemoveClient(client *Client) *PubSub {
	ps.mu.Lock()
	delete(ps.subscriptions, client.id)
	for index, cli := range ps.clients {
		if cli.id == client.id {
			ps.clients = append(ps.clients[:index], ps.clients[index+1:]...)
		}
	}
	ps.mu.Unlock()
	return ps
}

func (ps *PubSub) GetClientSubs(topic string, client *Client) []*Subscription {
	var subscriptionList []*Subscription
	for _, sub := range ps.subscriptions[client.id] {
		if sub.client.id == client.id && sub.topic == topic {
			subscriptionList = append(subscriptionList, sub)
		}
	}
	return subscriptionList
}

func (ps *PubSub) Subscribe(client *Client, topic string) error {
	clientSubs := ps.GetClientSubs(topic, client)
	if len(clientSubs) > 0 {
		// client is subscribed this topic before
		return errors.New("Error this subscribe is already exist")
	}
	newSubscription := &Subscription{
		topic:  topic,
		client: client,
	}
	ps.subscriptions[client.id] = append(ps.subscriptions[client.id], newSubscription)
	return nil
}

func (ps *PubSub) Unsubscribe(client *Client, topic string) *PubSub {
	for _, subs := range ps.subscriptions {
		for index, sub := range subs {
			if sub.client.id == client.id && sub.topic == topic {
				ps.subscriptions[client.id] = append(ps.subscriptions[client.id][:index], ps.subscriptions[client.id][index+1:]...)
			}
		}
	}
	return ps
}

func (ps *PubSub) Publish(topic string, msg []byte) {
	subscriptions := ps.getTopicSubs(topic, nil)
	for _, sub := range subscriptions {
		log.Infof("Sending to client id %s msg is %s \n", sub.client.id, msg[:30])
		//sub.Client.Connection.WriteMessage(1, message)
		sub.client.Send(msg)
	}
}

func (ps *PubSub) PublishJSON(topic string, data interface{}) {
	subscriptions := ps.getTopicSubs(topic, nil)
	for _, sub := range subscriptions {
		log.Infof("Sending to client id %s \n", sub.client.id)
		//sub.Client.Connection.WriteMessage(1, message)
		sub.client.SendJSON(data)
	}
}

func (ps *PubSub) getTopicSubs(topic string, client *Client) []*Subscription {
	var subscriptionList []*Subscription
	for _, subs := range ps.subscriptions {
		for _, sub := range subs {
			if sub.topic == topic {
				subscriptionList = append(subscriptionList, sub)
			}
		}
	}
	return subscriptionList
}

func (ps *PubSub) PublishPing(writeWait time.Duration) {
	for _, subs := range ps.subscriptions {
		for _, sub := range subs {
			log.Infof("Sending PING to client id: %s", sub.client.id)
			//sub.Client.Connection.WriteMessage(1, message)
			sub.client.Ping(writeWait)
		}
	}
}
