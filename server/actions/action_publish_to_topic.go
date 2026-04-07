package actions

import (
	"context"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"github.com/daptin/daptin/server/resource"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

const wsTopicPrefix = "ws-topic:"

type topicMeta struct {
	Owner      string `json:"owner"`
	Permission int64  `json:"permission"`
}

type publishToTopicActionPerformer struct {
	cruds map[string]*resource.DbResource
}

func (d *publishToTopicActionPerformer) Name() string {
	return "__publish_to_topic"
}

func (d *publishToTopicActionPerformer) DoAction(request actionresponse.Outcome, inFieldMap map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	topicName, ok := inFieldMap["topicName"].(string)
	if !ok || topicName == "" {
		return nil, nil, []error{errors.New("missing or invalid topicName")}
	}

	messageRaw, ok := inFieldMap["message"]
	if !ok || messageRaw == nil {
		return nil, nil, []error{errors.New("missing or invalid message")}
	}

	sessionUserRaw := request.Attributes["user"]
	if sessionUserRaw == nil {
		return nil, nil, []error{errors.New("Unauthorized")}
	}
	sessionUser := sessionUserRaw.(*auth.SessionUser)

	userDIR := sessionUser.UserReferenceId
	userUUID, _ := uuid.FromBytes(userDIR[:])
	userGroups := sessionUser.Groups

	pubSub := d.cruds["world"].PubSub
	if pubSub == nil {
		return nil, nil, []error{errors.New("PubSub not available")}
	}

	adminGroupId := d.cruds["world"].AdministratorGroupId
	_, isSystemTopic := d.cruds[topicName]

	if isSystemTopic {
		tablePerm := d.cruds["world"].GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", topicName, transaction)
		if !tablePerm.CanCreate(userDIR, userGroups, adminGroupId) {
			return nil, nil, []error{errors.New("permission denied: " + topicName)}
		}
	} else {
		if resource.OlricCache == nil {
			return nil, nil, []error{errors.New("topic not found: " + topicName)}
		}
		val, err := resource.OlricCache.Get(context.Background(), wsTopicPrefix+topicName)
		if err != nil {
			return nil, nil, []error{errors.New("topic not found: " + topicName)}
		}
		var data []byte
		err = val.Scan(&data)
		if err != nil {
			return nil, nil, []error{errors.New("topic not found: " + topicName)}
		}
		var meta topicMeta
		err = json.Unmarshal(data, &meta)
		if err != nil {
			return nil, nil, []error{errors.New("topic not found: " + topicName)}
		}

		metaPerm := permission.PermissionInstance{
			UserId:     daptinid.InterfaceToDIR(meta.Owner),
			Permission: auth.AuthPermission(meta.Permission),
		}
		if !metaPerm.CanExecute(userDIR, userGroups, adminGroupId) {
			return nil, nil, []error{errors.New("permission denied: " + topicName)}
		}
	}

	messageBytes, err := json.Marshal(messageRaw)
	if err != nil {
		return nil, nil, []error{errors.Wrap(err, "failed to marshal message")}
	}

	_, err = pubSub.Publish(context.Background(), topicName, resource.WsOutMessage{
		Type:   "event",
		Topic:  topicName,
		Event:  "new-message",
		Source: userUUID.String(),
		Data:   messageBytes,
	})
	if err != nil {
		return nil, nil, []error{errors.Wrap(err, "failed to publish message")}
	}

	responseAttrs := map[string]interface{}{
		"message": "published",
		"topic":   topicName,
	}

	return nil, []actionresponse.ActionResponse{
		resource.NewActionResponse("client.notify", resource.NewClientNotification("success", "Message published to "+topicName, "Success")),
		resource.NewActionResponse("publish_to_topic", responseAttrs),
	}, nil
}

func NewPublishToTopicPerformer(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {
	return &publishToTopicActionPerformer{
		cruds: cruds,
	}, nil
}
