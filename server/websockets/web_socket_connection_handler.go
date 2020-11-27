package websockets

import (
	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/resource"
	"log"
	"strings"
)

type WebSocketConnectionHandlerImpl struct {
	DtopicMap        *map[string]*olric.DTopic
	subscribedTopics map[string]uint64
}

func (wsch *WebSocketConnectionHandlerImpl) MessageFromClient(message WebSocketPayload, client *Client) {
	switch message.Method {
	case "subscribe":
		topics := message.Payload.Attributes["topics"].(string)
		if len(topics) < 1 {
			return
		}
		topicsList := strings.Split(topics, ",")
		for _, topic := range topicsList {
			_, ok := wsch.subscribedTopics[topic]
			if !ok {
				var err error
				wsch.subscribedTopics[topic], err = (*wsch.DtopicMap)[topic].AddListener(func(message olric.DTopicMessage) {
					eventMessage := message.Message.(resource.EventMessage)
					client.ch <- eventMessage
				})
				if err != nil {
					log.Printf("Failed to add listener to topic: %v", err)
				}
			}
		}
	case "unsubscribe":
		topics := message.Payload.Attributes["topics"].(string)
		if len(topics) < 1 {
			return
		}
		topicsList := strings.Split(topics, ",")
		for _, topic := range topicsList {
			subscriptionId, ok := wsch.subscribedTopics[topic]
			if ok {
				err := (*wsch.DtopicMap)[topic].RemoveListener(subscriptionId)
				delete(wsch.subscribedTopics, topic)
				if err != nil {
					log.Printf("Failed to remove listener from topic: %v", err)
				}
			}
		}
	}
}
