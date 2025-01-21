package websockets

import (
	"context"
	"encoding/json"
	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/resource"
	"github.com/go-redis/redis/v8"
	uuid "github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"strings"
)

// WebSocketConnectionHandlerImpl : Each websocket connection has its own handler
type WebSocketConnectionHandlerImpl struct {
	DtopicMap        *map[string]*olric.PubSub
	subscribedTopics map[string]*redis.PubSub
	olricDb          *olric.EmbeddedClient
	cruds            map[string]*resource.DbResource
}

func (wsch *WebSocketConnectionHandlerImpl) MessageFromClient(message WebSocketPayload, client *Client) {
	switch message.Method {
	case "subscribe":
		topics, ok := message.Payload["topicName"].(string)

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
				dTopic := (*wsch.DtopicMap)[topic]
				subscription := dTopic.Subscribe(context.Background(), topic)
				wsch.subscribedTopics[topic] = subscription
				go func(pubsub *redis.PubSub, eventType string, filtersMap map[string]interface{}) {
					listenChannel := pubsub.Channel()

					for {
						msg := <-listenChannel
						if msg == nil {
							// subscription is closed
							return
						}
						var eventMessage resource.EventMessage
						err = eventMessage.UnmarshalBinary([]byte(msg.Payload))
						resource.CheckErr(err, "Failed to unmarshal eventMessage")

						eventDataMap := make(map[string]interface{})
						err = json.Unmarshal(eventMessage.EventData, &eventDataMap)
						resource.CheckErr(err, "Failed to unmarshal eventMessage.EventData")
						eventData := eventDataMap
						typeName, _ := eventData["__type"]
						tableExists := false
						if typeName != nil {
							_, tableExists = wsch.cruds[typeName.(string)]
						}

						permission := resource.PermissionInstance{Permission: auth.ALLOW_ALL_PERMISSIONS}

						if tableExists {
							tx, err := wsch.cruds["world"].Connection.Beginx()
							if err != nil {
								resource.CheckErr(err, "Failed to begin transaction [78]")
							}

							permission = wsch.cruds["world"].GetRowPermission(eventData, tx)
							err = tx.Commit()
							if err != nil {
								resource.CheckErr(err, "Failed to commit transaction [84]")
							}

						}
						if permission.CanRead(client.user.UserReferenceId, client.user.Groups, wsch.cruds["world"].AdministratorGroupId) {

							sendMessage := true
							if filtersMap != nil {

								if eventType != "" {
									if eventMessage.EventType != eventType {
										return
									}
								}

								for key, val := range filtersMap {
									if eventData[key] != val {
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

				}(subscription, eventTypeString, filtersMap)

				if err != nil {
					log.Printf("Failed to add listener to topicName: %v", err)
				}
			}
		}
	case "create-topicName":
		topicName, ok := message.Payload["name"].(string)
		if !ok {
			return
		}

		_, exists := (*wsch.DtopicMap)[topicName]
		if exists {
			log.Printf("topicName already exists: %v", topicName)
			return
		}

		newTopic, err := wsch.olricDb.NewPubSub()
		resource.CheckErr(err, "Failed to create new topicName on client request [%v]", topicName)

		topicSubscription := newTopic.Subscribe(context.Background(), "members")
		go func(pubsub *redis.PubSub) {
			channel := pubsub.Channel()
			for {
				msg := <-channel
				log.Printf("[145] Member says: " + msg.String())
			}
		}(topicSubscription)

		(*wsch.DtopicMap)[topicName] = newTopic

	case "list-topicName":
		topics := make([]string, 0)
		for t, _ := range *wsch.DtopicMap {
			topics = append(topics, t)
		}

		data, _ := json.Marshal(map[string]interface{}{
			"topics": topics,
		})

		client.ch <- resource.EventMessage{
			EventData:     data,
			MessageSource: "system",
			EventType:     "response",
			ObjectType:    "topicName-list",
		}

	case "destroy-topicName":
		topic, ok := message.Payload["name"].(string)
		if !ok {
			log.Printf("topicName does not exist: %v", topic)
			return
		}

		_, isSystemTopic := wsch.cruds[topic]
		if isSystemTopic {
			log.Printf("user can delete only user created topics: %v", topic)
			return
		}

		//sub := (*wsch.DtopicMap)[topic]
		//err := sub.Destroy()
		//resource.CheckErr(err, "failed to destroy topicName")
		delete(*wsch.DtopicMap, topic)

	case "new-message":
		var err error
		var topic *olric.PubSub
		topicName, ok := message.Payload["topicName"].(string)
		message, ok := message.Payload["message"].(map[string]interface{})

		topic, ok = (*wsch.DtopicMap)[topicName]

		if !ok {
			log.Printf("topicName does not exist: %v", topicName)
			return
		}

		messageBytes, err := json.Marshal(message)
		resource.CheckErr(err, "Failed to marshal message on topicName")
		userRef, _ := uuid.FromBytes(client.user.UserReferenceId[:])
		_, err = topic.Publish(context.Background(), topicName, resource.EventMessage{
			MessageSource: userRef.String(),
			EventType:     "new-message",
			ObjectType:    topicName,
			EventData:     messageBytes,
		})

		resource.CheckErr(err, "Failed to publish message on ["+topicName+"]")

	case "unsubscribe":
		topics := message.Payload["topicName"].(string)
		if len(topics) < 1 {
			return
		}
		topicsList := strings.Split(topics, ",")
		for _, topic := range topicsList {
			pubSubs, ok := wsch.subscribedTopics[topic]
			if ok {
				pubSubs.Close()
				//err := (*wsch.DtopicMap)[topic].RemoveListener(subscriptionId)
				delete(wsch.subscribedTopics, topic)
				//if err != nil {
				//	log.Printf("Failed to remove listener from topicName: %v", err)
				//}
			}
		}
	}
}
