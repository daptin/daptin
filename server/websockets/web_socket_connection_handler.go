package websockets

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/permission"
	"github.com/daptin/daptin/server/resource"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"strings"
)

// PubSubEntry tracks a subscription and its cancel function
type PubSubEntry struct {
	pubsub *redis.PubSub
	cancel context.CancelFunc
}

// WebSocketConnectionHandlerImpl : Each websocket connection has its own handler
type WebSocketConnectionHandlerImpl struct {
	DtopicMap        *map[string]*olric.PubSub
	dtopicMapLock    *sync.RWMutex
	subscribedTopics map[string]*PubSubEntry
	olricDb          *olric.EmbeddedClient
	cruds            map[string]*resource.DbResource
}

func (wsch *WebSocketConnectionHandlerImpl) Close() {
	for topic, entry := range wsch.subscribedTopics {
		entry.cancel()
		entry.pubsub.Close()
		delete(wsch.subscribedTopics, topic)
	}
}

func sendError(client *Client, method string, message string) {
	data, _ := json.Marshal(map[string]interface{}{
		"error": message,
	})
	client.Write(resource.EventMessage{
		EventType:     "error",
		ObjectType:    method,
		EventData:     data,
		MessageSource: "system",
	})
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
			filtersMap, ok = filters.(map[string]interface{})
			if !ok {
				filtersMap = nil
			}
		}

		topicsList := strings.Split(topics, ",")
		for _, topic := range topicsList {
			_, ok := wsch.subscribedTopics[topic]
			if !ok {
				eventType, ok := filtersMap["EventType"]
				eventTypeString := ""
				if ok {
					eventTypeString, _ = eventType.(string)
					delete(filtersMap, "EventType")
				}

				wsch.dtopicMapLock.RLock()
				dTopic := (*wsch.DtopicMap)[topic]
				wsch.dtopicMapLock.RUnlock()

				if dTopic == nil {
					log.Warnf("topic not found, skipping subscribe: %v", topic)
					sendError(client, "subscribe", "topic not found: "+topic)
					continue
				}

				ctx, cancel := context.WithCancel(context.Background())
				subscription := dTopic.Subscribe(ctx, topic)
				wsch.subscribedTopics[topic] = &PubSubEntry{
					pubsub: subscription,
					cancel: cancel,
				}
				go func(pubsub *redis.PubSub, eventType string, filtersMap map[string]interface{}) {
					listenChannel := pubsub.Channel()

					for {
						msg := <-listenChannel
						if msg == nil {
							// subscription is closed
							return
						}
						var eventMessage resource.EventMessage
						err := eventMessage.UnmarshalBinary([]byte(msg.Payload))
						resource.CheckErr(err, "Failed to unmarshal eventMessage")

						eventDataMap := make(map[string]interface{})
						err = json.Unmarshal(eventMessage.EventData, &eventDataMap)
						resource.CheckErr(err, "Failed to unmarshal eventMessage.EventData")
						eventData := eventDataMap
						typeName, _ := eventData["__type"]
						tableExists := false
						if typeName != nil {
							typeStr, ok := typeName.(string)
							if ok {
								_, tableExists = wsch.cruds[typeStr]
							}
						}

						permission := permission.PermissionInstance{Permission: auth.ALLOW_ALL_PERMISSIONS}

						if tableExists {
							tx, err := wsch.cruds["world"].Connection().Beginx()
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
										continue
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
								client.Write(eventMessage)
							}

						}

					}

				}(subscription, eventTypeString, filtersMap)
				client.Write(resource.EventMessage{
					EventType:     "subscribed",
					ObjectType:    topic,
					MessageSource: "system",
				})
			}
		}
	case "create-topicName":
		topicName, ok := message.Payload["name"].(string)
		if !ok {
			return
		}

		wsch.dtopicMapLock.RLock()
		_, exists := (*wsch.DtopicMap)[topicName]
		wsch.dtopicMapLock.RUnlock()
		if exists {
			log.Printf("topicName already exists: %v", topicName)
			sendError(client, "create-topicName", "topic already exists")
			return
		}

		newTopic, err := wsch.olricDb.NewPubSub()
		resource.CheckErr(err, "Failed to create new topicName on client request [%v]", topicName)

		topicSubscription := newTopic.Subscribe(context.Background(), "members")
		go func(pubsub *redis.PubSub) {
			channel := pubsub.Channel()
			for {
				msg := <-channel
				if msg == nil {
					return
				}
				log.Println("[145] Member says: " + msg.String())
			}
		}(topicSubscription)

		wsch.dtopicMapLock.Lock()
		(*wsch.DtopicMap)[topicName] = newTopic
		wsch.dtopicMapLock.Unlock()

	case "list-topicName":
		wsch.dtopicMapLock.RLock()
		topics := make([]string, 0, len(*wsch.DtopicMap))
		for t := range *wsch.DtopicMap {
			topics = append(topics, t)
		}
		wsch.dtopicMapLock.RUnlock()

		data, _ := json.Marshal(map[string]interface{}{
			"topics": topics,
		})

		client.Write(resource.EventMessage{
			EventData:     data,
			MessageSource: "system",
			EventType:     "response",
			ObjectType:    "topicName-list",
		})

	case "destroy-topicName":
		topic, ok := message.Payload["name"].(string)
		if !ok {
			log.Printf("topicName does not exist: %v", topic)
			return
		}

		_, isSystemTopic := wsch.cruds[topic]
		if isSystemTopic {
			log.Printf("user can delete only user created topics: %v", topic)
			sendError(client, "destroy-topicName", "cannot delete system topic")
			return
		}

		wsch.dtopicMapLock.Lock()
		delete(*wsch.DtopicMap, topic)
		wsch.dtopicMapLock.Unlock()

	case "new-message":
		topicName, ok := message.Payload["topicName"].(string)
		if !ok {
			log.Printf("new-message: missing or invalid topicName")
			sendError(client, "new-message", "missing or invalid topicName")
			return
		}
		msgPayload, ok := message.Payload["message"].(map[string]interface{})
		if !ok {
			log.Printf("new-message: missing or invalid message payload")
			sendError(client, "new-message", "missing or invalid message")
			return
		}

		wsch.dtopicMapLock.RLock()
		topic, ok := (*wsch.DtopicMap)[topicName]
		wsch.dtopicMapLock.RUnlock()

		if !ok {
			log.Printf("topicName does not exist: %v", topicName)
			sendError(client, "new-message", "topic does not exist: "+topicName)
			return
		}

		messageBytes, err := json.Marshal(msgPayload)
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
		topics, ok := message.Payload["topicName"].(string)
		if !ok {
			return
		}
		if len(topics) < 1 {
			return
		}
		topicsList := strings.Split(topics, ",")
		for _, topic := range topicsList {
			entry, ok := wsch.subscribedTopics[topic]
			if ok {
				entry.cancel()
				entry.pubsub.Close()
				delete(wsch.subscribedTopics, topic)
				client.Write(resource.EventMessage{
					EventType:     "unsubscribed",
					ObjectType:    topic,
					MessageSource: "system",
				})
			}
		}
	}
}
