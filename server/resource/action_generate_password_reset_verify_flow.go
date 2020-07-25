package resource

import (
	"encoding/base64"
	"fmt"
	"github.com/Masterminds/squirrel"
	"github.com/artpar/api2go"
	"github.com/artpar/go.uuid"
	"github.com/dgrijalva/jwt-go"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

type GeneratePasswordResetVerifyActionPerformer struct {
	cruds          map[string]*DbResource
	secret         []byte
	tokenLifeTime  int
	jwtTokenIssuer string
}

func (d *GeneratePasswordResetVerifyActionPerformer) Name() string {
	return "password.reset.verify"
}

func (d *GeneratePasswordResetVerifyActionPerformer) DoAction(request Outcome, inFieldMap map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)

	email := inFieldMap["email"]

	existingUsers, _, err := d.cruds[USER_ACCOUNT_TABLE_NAME].GetRowsByWhereClause("user_account", squirrel.Eq{"email": email})

	responseAttrs := make(map[string]interface{})
	if err != nil || len(existingUsers) < 1 {
		responseAttrs["type"] = "error"
		responseAttrs["message"] = "No Such account"
		responseAttrs["title"] = "Failed"
		actionResponse := NewActionResponse("client.notify", responseAttrs)
		responses = append(responses, actionResponse)
	} else {
		existingUser := existingUsers[0]

		var token = inFieldMap["token"]
		tokenString, err := base64.StdEncoding.DecodeString(token.(string))
		if err != nil {
			responseAttrs["type"] = "error"
			responseAttrs["message"] = "Invalid token"
			responseAttrs["title"] = "Failed"
			actionResponse := NewActionResponse("client.notify", responseAttrs)
			responses = append(responses, actionResponse)
		} else {

			parsedToken, err := jwt.Parse(string(tokenString), func(token *jwt.Token) (interface{}, error) {
				return d.secret, nil
			})
			if err != nil || !parsedToken.Valid {

				notificationAttrs := make(map[string]string)
				notificationAttrs["message"] = "Token has expired"
				notificationAttrs["title"] = "Failed"
				notificationAttrs["type"] = "failed"
				responses = append(responses, NewActionResponse("client.notify", notificationAttrs))

			} else {

				notificationAttrs := make(map[string]string)
				notificationAttrs["message"] = "Logged in"
				notificationAttrs["title"] = "Success"
				notificationAttrs["type"] = "success"
				responses = append(responses, NewActionResponse("client.notify", notificationAttrs))

			}

		}

	}

	return nil, responses, nil
}

func NewGeneratePasswordResetVerifyActionPerformer(configStore *ConfigStore, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	secret, _ := configStore.GetConfigValueFor("jwt.secret", "backend")

	tokenLifeTimeHours, err := configStore.GetConfigIntValueFor("jwt.token.life.hours", "backend")
	CheckErr(err, "No default jwt token life time set in configuration")
	if err != nil {
		err = configStore.SetConfigIntValueFor("jwt.token.life.hours", 24*3, "backend")
		CheckErr(err, "Failed to store default jwt token life time")
		tokenLifeTimeHours = 24 * 3 // 3 days
	}

	jwtTokenIssuer, err := configStore.GetConfigValueFor("jwt.token.issuer", "backend")
	CheckErr(err, "No default jwt token issuer set")
	if err != nil {
		uid, _ := uuid.NewV4()
		jwtTokenIssuer = "daptin-" + uid.String()[0:6]
		err = configStore.SetConfigValueFor("jwt.token.issuer", jwtTokenIssuer, "backend")
	}

	handler := GeneratePasswordResetVerifyActionPerformer{
		cruds:          cruds,
		secret:         []byte(secret),
		tokenLifeTime:  tokenLifeTimeHours,
		jwtTokenIssuer: jwtTokenIssuer,
	}

	return &handler, nil

}
