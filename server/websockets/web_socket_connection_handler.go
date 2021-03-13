package websockets

import (
	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/resource"
	"log"
	"strings"
)

// Each websocket connection has its own handler
type WebSocketConnectionHandlerImpl struct {
	DtopicMap        *map[string]*olric.DTopic
	subscribedTopics map[string]uint64
	olricDb          *olric.Olric
	cruds            map[string]*resource.DbResource
}

func (wsch *WebSocketConnectionHandlerImpl) MessageFromClient(message WebSocketPayload, client *Client) {
	switch message.Method {
	case "subscribe":
		topics, ok := message.Payload["topic"].(string)

		if !ok {
			return
		}
		if len(topics) < 1 {
			return
		}
		filters, ok := message.Payload["filters"]
		var filtersMap map[string]interface{}
		if ok {
			filtersMap = filters.(map[string]interface{})
		}

		topicsList := strings.Split(topics, ",")
		for _, topic := range topicsList {
			_, ok := wsch.subscribedTopics[topic]
			if !ok {
				var err error
				eventType, ok := filtersMap["EventType"]
				eventTypeString := ""
				if ok {
					eventTypeString = eventType.(string)
					delete(filtersMap, "EventType")
				}
				wsch.subscribedTopics[topic], err = (*wsch.DtopicMap)[topic].AddListener(func(eventType string, filtersMap map[string]interface{}) func(olric.DTopicMessage) {
					return func(message olric.DTopicMessage) {
						eventMessage := message.Message.(resource.EventMessage)

						typeName, _ := eventMessage.EventData["__type"]
						tableExists := false
						if typeName != nil {
							_, tableExists = wsch.cruds[typeName.(string)]
						}

						permission := resource.PermissionInstance{Permission: auth.ALLOW_ALL_PERMISSIONS}

						if tableExists {
							permission = wsch.cruds["world"].GetRowPermission(eventMessage.EventData)

						}
						if permission.CanRead(client.user.UserReferenceId, client.user.Groups) {

							sendMessage := true
							if filtersMap != nil {

								if eventType != "" {
									if eventMessage.EventType != eventType {
										return
									}
								}

								for key, val := range filtersMap {
									if eventMessage.EventData[key] != val {
										sendMessage = false
										break
									}
								}
							}
							if sendMessage {
								client.ch <- eventMessage
							}

						}

					}
				}(eventTypeString, filtersMap))
				if err != nil {
					log.Printf("Failed to add listener to topic: %v", err)
				}
			}
		}
	case "create-topic":
		topic, ok := message.Payload["name"].(string)
		if !ok {
			return
		}

		_, exists := (*wsch.DtopicMap)[topic]
		if exists {
			log.Printf("topic already exists: %v", topic)
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
		topic, ok := message.Payload["name"].(string)
		if !ok {
			log.Printf("topic does not exist: %v", topic)
			return
		}

		_, isSystemTopic := wsch.cruds[topic]
		if isSystemTopic {
			log.Printf("user can delete only user created topics: %v", topic)
			return
		}

		err := (*wsch.DtopicMap)[topic].Destroy()
		resource.CheckErr(err, "failed to destroy topic")
		delete(*wsch.DtopicMap, topic)

	case "new-message":
		var err error
		var topic *olric.DTopic
		topicName, ok := message.Payload["topic"].(string)
		message, ok := message.Payload["message"].(map[string]interface{})

		topic, ok = (*wsch.DtopicMap)[topicName]

		if !ok {
			log.Printf("topic does not exist: {}", topicName)
			return
		}

		err = topic.Publish(resource.EventMessage{
			MessageSource: client.user.UserReferenceId,
			EventType:     "new-message",
			ObjectType:    topicName,
			EventData:     message,
		})

		resource.CheckErr(err, "Failed to publish message on topic")

	case "unsubscribe":
		topics := message.Payload["topic"].(string)
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
