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
	olricDb          *olric.Olric
}

func (wsch *WebSocketConnectionHandlerImpl) MessageFromClient(message WebSocketPayload, client *Client) {
	switch message.Method {
	case "subscribe":
		topics, ok := message.Payload.Attributes["topics"].(string)
		if !ok {
			return
		}
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
	case "create-topic":
		topic, ok := message.Payload.Attributes["name"].(string)
		if !ok {
			return
		}

		newTopic, err := wsch.olricDb.NewDTopic(topic, 4, 1)
		resource.CheckErr(err, "Failed to create new topic on client request [%v]", topic)

		(*wsch.DtopicMap)[topic] = newTopic

	case "list-topic":
		topics := make([]string, 0)
		for t, _ := range *wsch.DtopicMap {
			topics = append(topics, t)
		}

		client.ch <- resource.EventMessage{
			EventData: map[string]interface{}{
				"topics": topics,
			},
			MessageSource: "system",
			EventType:     "response",
			ObjectType:    "topic-list",
		}

	case "destroy-topic":
		topic, ok := message.Payload.Attributes["name"].(string)
		if !ok {
			return
		}

		_ = (*wsch.DtopicMap)[topic].Destroy()

	case "new-message":
		var err error
		var topic *olric.DTopic
		topicName, ok := message.Payload.Attributes["topic"].(string)
		message, ok := message.Payload.Attributes["message"].(map[string]interface{})

		topic, ok = (*wsch.DtopicMap)[topicName]
		if !ok {
			topic, err = wsch.olricDb.NewDTopic(topicName, 4, 1)
			resource.CheckErr(err, "Failed to create the topic")
			if err != nil {
				return
			}
			(*wsch.DtopicMap)[topicName] = topic
		}
		err = topic.Publish(resource.EventMessage{
			MessageSource: "client",
			EventType:     "new-message",
			ObjectType:    topicName,
			EventData:     message,
		})
		resource.CheckErr(err, "Failed to publish message on topic")

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
