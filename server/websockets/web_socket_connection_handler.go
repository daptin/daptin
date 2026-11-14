package websockets

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/artpar/api2go/v2"
	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
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

// topicMeta stores ownership and permission for user-created topics in DMap
type topicMeta struct {
	Owner      string `json:"owner"`
	Permission int64  `json:"permission"`
}

// WebSocketConnectionHandlerImpl : Each websocket connection has its own handler
type WebSocketConnectionHandlerImpl struct {
	DtopicMap        *map[string]*olric.PubSub
	dtopicMapLock    *sync.RWMutex
	subscribedTopics map[string]*PubSubEntry
	olricDb          *olric.EmbeddedClient
	cruds            map[string]*resource.DbResource
	sharedPubSub     *olric.PubSub
}

const wsTopicPrefix = "ws-topic:"

func (wsch *WebSocketConnectionHandlerImpl) getTopicMeta(name string) (topicMeta, bool) {
	var meta topicMeta
	if resource.OlricCache == nil {
		return meta, false
	}
	val, err := resource.OlricCache.Get(context.Background(), wsTopicPrefix+name)
	if err != nil {
		return meta, false
	}
	var data []byte
	err = val.Scan(&data)
	if err != nil {
		return meta, false
	}
	err = json.Unmarshal(data, &meta)
	if err != nil {
		return meta, false
	}
	return meta, true
}

func (wsch *WebSocketConnectionHandlerImpl) putTopicMeta(name string, meta topicMeta) {
	if resource.OlricCache == nil {
		return
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return
	}
	resource.OlricCache.Put(context.Background(), wsTopicPrefix+name, data)
}

func (wsch *WebSocketConnectionHandlerImpl) deleteTopicMeta(name string) {
	if resource.OlricCache == nil {
		return
	}
	_, _ = resource.OlricCache.Delete(context.Background(), wsTopicPrefix+name)
}

func (wsch *WebSocketConnectionHandlerImpl) Close() {
	for topic, entry := range wsch.subscribedTopics {
		entry.cancel()
		entry.pubsub.Close()
		delete(wsch.subscribedTopics, topic)
	}
}

func sendResponse(client *Client, id string, method string, ok bool, data map[string]interface{}, errMsg string) {
	msg := resource.WsOutMessage{
		Type:   "response",
		Id:     id,
		Method: method,
	}
	msg.Ok = &ok
	if ok {
		msg.Data, _ = json.Marshal(data)
	} else {
		msg.Error = errMsg
	}
	client.Write(msg)
}

func (wsch *WebSocketConnectionHandlerImpl) isAdmin(client *Client) bool {
	adminGroupId := wsch.cruds["world"].AdministratorGroupId
	for _, g := range client.user.Groups {
		if g.GroupReferenceId == adminGroupId {
			return true
		}
	}
	return false
}

func (wsch *WebSocketConnectionHandlerImpl) MessageFromClient(message WebSocketPayload, client *Client) {
	reqId := message.Id

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

		eventTypeString := ""
		if filtersMap != nil {
			if eventType, ok := filtersMap["EventType"]; ok {
				eventTypeString, _ = eventType.(string)
				delete(filtersMap, "EventType")
			}
		}

		topicsList := strings.Split(topics, ",")
		for _, topic := range topicsList {
			_, alreadySubscribed := wsch.subscribedTopics[topic]
			if alreadySubscribed {
				continue
			}

			adminGroupId := wsch.cruds["world"].AdministratorGroupId
			_, isSystemTopic := wsch.cruds[topic]

			if isSystemTopic {
				// System topic: check table-level CanPeek
				tx, err := wsch.cruds["world"].Connection().Beginx()
				if err != nil {
					resource.CheckErr(err, "Failed to begin transaction for subscribe permission check")
					sendResponse(client, reqId, "subscribe", false, nil, "internal error")
					continue
				}
				tablePerm := wsch.cruds["world"].GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", topic, tx)
				tx.Commit()

				if !tablePerm.CanPeek(client.user.UserReferenceId, client.user.Groups, adminGroupId) {
					sendResponse(client, reqId, "subscribe", false, nil, "permission denied: "+topic)
					continue
				}

				wsch.dtopicMapLock.RLock()
				dTopic := (*wsch.DtopicMap)[topic]
				wsch.dtopicMapLock.RUnlock()

				if dTopic == nil {
					log.Warnf("topic not found, skipping subscribe: %v", topic)
					sendResponse(client, reqId, "subscribe", false, nil, "topic not found: "+topic)
					continue
				}

				ctx, cancel := context.WithCancel(context.Background())
				subscription := dTopic.Subscribe(ctx, topic)
				wsch.subscribedTopics[topic] = &PubSubEntry{
					pubsub: subscription,
					cancel: cancel,
				}
				ready := make(chan struct{})
				go wsch.systemTopicListener(topic, subscription, eventTypeString, filtersMap, client, ready)
				<-ready
			} else {
				// User topic: check DMap meta CanRead
				meta, found := wsch.getTopicMeta(topic)
				if !found {
					sendResponse(client, reqId, "subscribe", false, nil, "topic not found: "+topic)
					continue
				}

				metaPerm := permission.PermissionInstance{
					UserId:     daptinid.InterfaceToDIR(meta.Owner),
					Permission: auth.AuthPermission(meta.Permission),
				}
				if !metaPerm.CanRead(client.user.UserReferenceId, client.user.Groups, adminGroupId) {
					sendResponse(client, reqId, "subscribe", false, nil, "permission denied: "+topic)
					continue
				}

				ctx, cancel := context.WithCancel(context.Background())
				subscription := wsch.sharedPubSub.Subscribe(ctx, topic)
				wsch.subscribedTopics[topic] = &PubSubEntry{
					pubsub: subscription,
					cancel: cancel,
				}
				ready := make(chan struct{})
				go wsch.userTopicListener(subscription, eventTypeString, filtersMap, client, ready)
				<-ready
			}

			sendResponse(client, reqId, "subscribe", true, map[string]interface{}{
				"topic": topic,
			}, "")
		}

	case "create-topicName":
		topicName, ok := message.Payload["name"].(string)
		if !ok {
			return
		}

		// Block system topic names
		_, isSystemTopic := wsch.cruds[topicName]
		if isSystemTopic {
			sendResponse(client, reqId, "create-topicName", false, nil, "cannot create topic with reserved name")
			return
		}

		// Check DMap for existing user topic
		_, exists := wsch.getTopicMeta(topicName)
		if exists {
			log.Printf("topicName already exists: %v", topicName)
			sendResponse(client, reqId, "create-topicName", false, nil, "topic already exists")
			return
		}

		// Store topic metadata in distributed DMap
		userRef, _ := uuid.FromBytes(client.user.UserReferenceId[:])
		wsch.putTopicMeta(topicName, topicMeta{
			Owner:      userRef.String(),
			Permission: int64(auth.UserCRUD | auth.UserExecute),
		})

		sendResponse(client, reqId, "create-topicName", true, map[string]interface{}{
			"topicName": topicName,
			"created":   true,
		}, "")

	case "destroy-topicName":
		topic, ok := message.Payload["name"].(string)
		if !ok {
			log.Printf("destroy-topicName: missing or invalid name")
			return
		}

		_, isSystemTopic := wsch.cruds[topic]
		if isSystemTopic {
			log.Printf("user can delete only user created topics: %v", topic)
			sendResponse(client, reqId, "destroy-topicName", false, nil, "cannot delete system topic")
			return
		}

		meta, found := wsch.getTopicMeta(topic)
		if !found {
			sendResponse(client, reqId, "destroy-topicName", false, nil, "topic not found")
			return
		}

		adminGroupId := wsch.cruds["world"].AdministratorGroupId
		metaPerm := permission.PermissionInstance{
			UserId:     daptinid.InterfaceToDIR(meta.Owner),
			Permission: auth.AuthPermission(meta.Permission),
		}
		if !metaPerm.CanDelete(client.user.UserReferenceId, client.user.Groups, adminGroupId) {
			sendResponse(client, reqId, "destroy-topicName", false, nil, "permission denied")
			return
		}

		wsch.deleteTopicMeta(topic)

		sendResponse(client, reqId, "destroy-topicName", true, map[string]interface{}{
			"topicName": topic,
			"destroyed": true,
		}, "")

	case "new-message":
		topicName, ok := message.Payload["topicName"].(string)
		if !ok {
			log.Printf("new-message: missing or invalid topicName")
			sendResponse(client, reqId, "new-message", false, nil, "missing or invalid topicName")
			return
		}
		msgPayload, ok := message.Payload["message"].(map[string]interface{})
		if !ok {
			log.Printf("new-message: missing or invalid message payload")
			sendResponse(client, reqId, "new-message", false, nil, "missing or invalid message")
			return
		}

		adminGroupId := wsch.cruds["world"].AdministratorGroupId
		_, isSystemTopic := wsch.cruds[topicName]

		if isSystemTopic {
			// System topic: check table-level CanCreate
			tx, err := wsch.cruds["world"].Connection().Beginx()
			if err != nil {
				resource.CheckErr(err, "Failed to begin transaction for new-message permission check")
				sendResponse(client, reqId, "new-message", false, nil, "internal error")
				return
			}
			tablePerm := wsch.cruds["world"].GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", topicName, tx)
			tx.Commit()

			if !tablePerm.CanCreate(client.user.UserReferenceId, client.user.Groups, adminGroupId) {
				sendResponse(client, reqId, "new-message", false, nil, "permission denied: "+topicName)
				return
			}

			wsch.dtopicMapLock.RLock()
			dTopic := (*wsch.DtopicMap)[topicName]
			wsch.dtopicMapLock.RUnlock()

			if dTopic == nil {
				sendResponse(client, reqId, "new-message", false, nil, "topic does not exist: "+topicName)
				return
			}

			messageBytes, err := json.Marshal(msgPayload)
			resource.CheckErr(err, "Failed to marshal message on topicName")
			userRef, _ := uuid.FromBytes(client.user.UserReferenceId[:])
			_, err = dTopic.Publish(context.Background(), topicName, resource.WsOutMessage{
				Type:   "event",
				Topic:  topicName,
				Event:  "new-message",
				Source: userRef.String(),
				Data:   messageBytes,
			})
			resource.CheckErr(err, "Failed to publish message on ["+topicName+"]")
		} else {
			// User topic: check DMap meta CanExecute
			meta, found := wsch.getTopicMeta(topicName)
			if !found {
				sendResponse(client, reqId, "new-message", false, nil, "topic does not exist: "+topicName)
				return
			}

			metaPerm := permission.PermissionInstance{
				UserId:     daptinid.InterfaceToDIR(meta.Owner),
				Permission: auth.AuthPermission(meta.Permission),
			}
			if !metaPerm.CanExecute(client.user.UserReferenceId, client.user.Groups, adminGroupId) {
				sendResponse(client, reqId, "new-message", false, nil, "permission denied: "+topicName)
				return
			}

			messageBytes, err := json.Marshal(msgPayload)
			resource.CheckErr(err, "Failed to marshal message on topicName")
			userRef, _ := uuid.FromBytes(client.user.UserReferenceId[:])
			_, err = wsch.sharedPubSub.Publish(context.Background(), topicName, resource.WsOutMessage{
				Type:   "event",
				Topic:  topicName,
				Event:  "new-message",
				Source: userRef.String(),
				Data:   messageBytes,
			})
			resource.CheckErr(err, "Failed to publish message on ["+topicName+"]")
		}

	case "set-topic-permission":
		topicName, ok := message.Payload["topicName"].(string)
		if !ok {
			sendResponse(client, reqId, "set-topic-permission", false, nil, "missing or invalid topicName")
			return
		}

		permValue, ok := message.Payload["permission"]
		if !ok {
			sendResponse(client, reqId, "set-topic-permission", false, nil, "missing permission value")
			return
		}
		permFloat, ok := permValue.(float64)
		if !ok {
			sendResponse(client, reqId, "set-topic-permission", false, nil, "invalid permission value")
			return
		}

		_, isSystemTopic := wsch.cruds[topicName]
		if isSystemTopic {
			sendResponse(client, reqId, "set-topic-permission", false, nil, "cannot modify system topic permissions")
			return
		}

		meta, found := wsch.getTopicMeta(topicName)
		if !found {
			sendResponse(client, reqId, "set-topic-permission", false, nil, "topic not found")
			return
		}

		userRef, _ := uuid.FromBytes(client.user.UserReferenceId[:])
		if meta.Owner != userRef.String() && !wsch.isAdmin(client) {
			sendResponse(client, reqId, "set-topic-permission", false, nil, "only owner or admin can change permissions")
			return
		}

		meta.Permission = int64(permFloat)
		wsch.putTopicMeta(topicName, meta)

		sendResponse(client, reqId, "set-topic-permission", true, map[string]interface{}{
			"topicName":  topicName,
			"permission": meta.Permission,
		}, "")

	case "get-topic-permission":
		topicName, ok := message.Payload["topicName"].(string)
		if !ok {
			sendResponse(client, reqId, "get-topic-permission", false, nil, "missing or invalid topicName")
			return
		}

		adminGroupId := wsch.cruds["world"].AdministratorGroupId
		_, isSystemTopic := wsch.cruds[topicName]

		if isSystemTopic {
			tx, err := wsch.cruds["world"].Connection().Beginx()
			if err != nil {
				resource.CheckErr(err, "Failed to begin transaction for get-topic-permission")
				sendResponse(client, reqId, "get-topic-permission", false, nil, "internal error")
				return
			}
			tablePerm := wsch.cruds["world"].GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", topicName, tx)
			tx.Commit()

			if !tablePerm.CanPeek(client.user.UserReferenceId, client.user.Groups, adminGroupId) {
				sendResponse(client, reqId, "get-topic-permission", false, nil, "permission denied")
				return
			}

			sendResponse(client, reqId, "get-topic-permission", true, map[string]interface{}{
				"topicName":  topicName,
				"permission": int64(tablePerm.Permission),
				"type":       "system",
			}, "")
		} else {
			meta, found := wsch.getTopicMeta(topicName)
			if !found {
				sendResponse(client, reqId, "get-topic-permission", false, nil, "topic not found")
				return
			}

			metaPerm := permission.PermissionInstance{
				UserId:     daptinid.InterfaceToDIR(meta.Owner),
				Permission: auth.AuthPermission(meta.Permission),
			}
			if !metaPerm.CanPeek(client.user.UserReferenceId, client.user.Groups, adminGroupId) {
				sendResponse(client, reqId, "get-topic-permission", false, nil, "permission denied")
				return
			}

			sendResponse(client, reqId, "get-topic-permission", true, map[string]interface{}{
				"topicName":  topicName,
				"owner":      meta.Owner,
				"permission": meta.Permission,
				"type":       "user",
			}, "")
		}

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
				sendResponse(client, reqId, "unsubscribe", true, map[string]interface{}{
					"topic": topic,
				}, "")
			}
		}
	default:
		sendResponse(client, reqId, message.Method, false, nil, "no such method")
	}
}

// systemTopicListener handles messages for system topics with per-row CanRead checks
func (wsch *WebSocketConnectionHandlerImpl) systemTopicListener(
	topic string, pubsub *redis.PubSub, eventType string, filtersMap map[string]interface{}, client *Client, ready chan struct{},
) {
	// Wait for Olric to confirm the subscription before signaling ready.
	// Without this, the SUBSCRIBE command may still be in-flight when PUBLISH fires.
	_, err := pubsub.Receive(context.Background())
	if err != nil {
		log.Errorf("systemTopicListener: subscribe confirmation failed: %v", err)
	}
	listenChannel := pubsub.Channel()
	close(ready)
	table := wsch.cruds[topic]
	if table == nil {
		return
	}
	for msg := range listenChannel {
		var eventMessage resource.WsOutMessage
		if msg.Channel != topic || eventMessage.UnmarshalBinary([]byte(msg.Payload)) != nil {
			continue
		}
		eventDataMap, ok := systemEventData(topic, eventMessage)
		if !ok {
			continue
		}
		if eventMessage.Event != "new-message" {
			var perm permission.PermissionInstance
			if eventMessage.Event == "delete" {
				// The resource lifecycle captures this before deleting the row and
				// its group relationships. A lookup after deletion loses that ACL.
				snapshot, ok := eventDataMap["__permission"].(map[string]interface{})
				_, groupsPresent := snapshot["UserGroupId"]
				if !ok || snapshot["Permission"] == nil || snapshot["UserId"] == nil || !groupsPresent {
					continue
				}
				encoded, err := json.Marshal(snapshot)
				if err != nil || json.Unmarshal(encoded, &perm) != nil {
					continue
				}
			} else {
				tx, err := table.Connection().Beginx()
				if err != nil {
					continue
				}
				row, err := table.GetReferenceIdToObjectWithTransaction(topic, daptinid.InterfaceToDIR(eventDataMap["reference_id"]), tx)
				if err != nil {
					tx.Rollback()
					continue
				}
				perm = table.GetRowPermission(row, tx)
				if tx.Commit() != nil {
					continue
				}
			}
			if !perm.CanRead(client.user.UserReferenceId, client.user.Groups, table.AdministratorGroupId) {
				continue
			}
			// Apply the same column visibility rules as public resource responses.
			delete(eventDataMap, "id")
			delete(eventDataMap, "__permission")
			info := table.TableInfo()
			model := api2go.NewApi2GoModelWithData(topic, info.Columns, int64(info.DefaultPermission), info.Relations, eventDataMap)
			eventDataMap = model.GetAttributes()
			eventMessage.Data, err = json.Marshal(eventDataMap)
			if err != nil {
				continue
			}
		}

		sendMessage := true
		if filtersMap != nil {
			if eventType != "" && eventMessage.Event != eventType {
				continue
			}
			for key, val := range filtersMap {
				if eventDataMap[key] != val {
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

// System topics carry database lifecycle events and explicitly published messages.
// Payload attributes cannot select a different resource or event authorization path.
func systemEventData(topic string, event resource.WsOutMessage) (map[string]interface{}, bool) {
	if event.Type != "event" || event.Topic != topic {
		return nil, false
	}
	if event.Event == "new-message" {
		if _, err := uuid.Parse(event.Source); err != nil || !json.Valid(event.Data) {
			return nil, false
		}
		var message map[string]interface{}
		_ = json.Unmarshal(event.Data, &message)
		return message, true
	}
	if event.Source != "database" || (event.Event != "create" && event.Event != "update" && event.Event != "delete") {
		return nil, false
	}
	var row map[string]interface{}
	if json.Unmarshal(event.Data, &row) != nil || row == nil {
		return nil, false
	}
	if row["__type"] != topic || daptinid.InterfaceToDIR(row["reference_id"]) == daptinid.NullReferenceId {
		return nil, false
	}
	return row, true
}

// userTopicListener handles messages for user-created topics (no per-row permission check)
func (wsch *WebSocketConnectionHandlerImpl) userTopicListener(
	pubsub *redis.PubSub, eventType string, filtersMap map[string]interface{}, client *Client, ready chan struct{},
) {
	// Wait for Olric to confirm the subscription before signaling ready.
	// Without this, the SUBSCRIBE command may still be in-flight when PUBLISH fires.
	_, err := pubsub.Receive(context.Background())
	if err != nil {
		log.Errorf("userTopicListener: subscribe confirmation failed: %v", err)
	}
	listenChannel := pubsub.Channel()
	close(ready)
	for msg := range listenChannel {
		var eventMessage resource.WsOutMessage
		err := eventMessage.UnmarshalBinary([]byte(msg.Payload))
		resource.CheckErr(err, "Failed to unmarshal eventMessage")

		sendMessage := true
		if filtersMap != nil {
			if eventType != "" && eventMessage.Event != eventType {
				continue
			}
			eventDataMap := make(map[string]interface{})
			err = json.Unmarshal(eventMessage.Data, &eventDataMap)
			if err == nil {
				for key, val := range filtersMap {
					if eventDataMap[key] != val {
						sendMessage = false
						break
					}
				}
			}
		}
		if sendMessage {
			client.Write(eventMessage)
		}
	}
}
