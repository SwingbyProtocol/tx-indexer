package pubsub

import (
	"errors"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

type PubSub struct {
	Clients       []Client
	Subscriptions map[string][]Subscription
	Mu            sync.Mutex
	Upgrader      websocket.Upgrader
}

type Subscription struct {
	Topic  string
	Client *Client
}

func NewPubSub() *PubSub {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	ps := &PubSub{
		Subscriptions: make(map[string][]Subscription),
		Upgrader:      upgrader,
	}
	return ps
}

func (ps *PubSub) AddClient(client Client) *PubSub {
	ps.Clients = append(ps.Clients, client)
	//fmt.Println("adding new client to the list", client.Id, len(ps.Clients))
	return ps
}

func (ps *PubSub) RemoveClient(client *Client) *PubSub {
	ps.Mu.Lock()
	delete(ps.Subscriptions, client.ID)
	for index, cli := range ps.Clients {
		if cli.ID == client.ID {
			ps.Clients = append(ps.Clients[:index], ps.Clients[index+1:]...)
		}
	}
	ps.Mu.Unlock()
	return ps
}

func (ps *PubSub) GetTopicSubs(topic string, client *Client) []Subscription {
	var subscriptionList []Subscription
	for _, subs := range ps.Subscriptions {
		for _, sub := range subs {
			if sub.Topic == topic {
				subscriptionList = append(subscriptionList, sub)
			}
		}
	}
	return subscriptionList
}

func (ps *PubSub) GetClientSubs(topic string, client *Client) []Subscription {
	var subscriptionList []Subscription
	for _, sub := range ps.Subscriptions[client.ID] {
		if sub.Client.ID == client.ID && sub.Topic == topic {
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
	newSubscription := Subscription{
		Topic:  topic,
		Client: client,
	}
	ps.Subscriptions[client.ID] = append(ps.Subscriptions[client.ID], newSubscription)
	return nil
}

func (ps *PubSub) Unsubscribe(client *Client, topic string) *PubSub {
	//clientSubscriptions := ps.GetSubscriptions(topic, client)
	for _, subs := range ps.Subscriptions {
		for index, sub := range subs {
			if sub.Client.ID == client.ID && sub.Topic == topic {
				ps.Subscriptions[client.ID] = append(ps.Subscriptions[client.ID][:index], ps.Subscriptions[client.ID][index+1:]...)
			}
		}
	}
	return ps
}

func (ps *PubSub) Publish(topic string, msg []byte) {
	subscriptions := ps.GetTopicSubs(topic, nil)
	for _, sub := range subscriptions {
		log.Infof("Sending to client id %s msg is %s \n", sub.Client.ID, msg[:30])
		//sub.Client.Connection.WriteMessage(1, message)
		sub.Client.Send(msg)
	}
}

func (ps *PubSub) PublishJSON(topic string, data interface{}) {
	subscriptions := ps.GetTopicSubs(topic, nil)
	for _, sub := range subscriptions {
		log.Infof("Sending to client id %s \n", sub.Client.ID)
		//sub.Client.Connection.WriteMessage(1, message)
		sub.Client.SendJSON(data)
	}
}

func (ps *PubSub) PublishPing(writeWait time.Duration) {
	for _, subs := range ps.Subscriptions {
		for _, sub := range subs {
			log.Infof("Sending PING to client id: %s", sub.Client.ID)
			//sub.Client.Connection.WriteMessage(1, message)
			sub.Client.Ping(writeWait)
		}
	}
}
