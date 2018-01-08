package resource

import (
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"gopkg.in/Masterminds/squirrel.v1"
	"strings"
	"time"
	"github.com/artpar/api2go"
)

type GenerateJwtTokenActionPerformer struct {
	cruds  map[string]*DbResource
	secret []byte
}

func (d *GenerateJwtTokenActionPerformer) Name() string {
	return "jwt.token"
}

func (d *GenerateJwtTokenActionPerformer) DoAction(request ActionRequest, inFieldMap map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)

	email := inFieldMap["email"]
	password := inFieldMap["password"]

	existingUsers, _, err := d.cruds["user"].GetRowsByWhereClause("user", squirrel.Eq{"email": email})

	responseAttrs := make(map[string]interface{})
	if err != nil || len(existingUsers) < 1 {
		responseAttrs["type"] = "error"
		responseAttrs["message"] = "Invalid username or password"
		responseAttrs["title"] = "Failed"
		actionResponse := NewActionResponse("client.notify", responseAttrs)
		responses = append(responses, actionResponse)
	} else {
		existingUser := existingUsers[0]
		if BcryptCheckStringHash(password.(string), existingUser["password"].(string)) {

			// Create a new token object, specifying signing method and the claims
			// you would like it to contain.
			u, _ := uuid.NewV4()
			token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"email":   existingUser["email"],
				"name":    existingUser["name"],
				"nbf":     time.Now().Unix(),
				"exp":     time.Now().Add(60 * time.Minute).Unix(),
				"iss":     "daptin",
				"picture": fmt.Sprintf("https://www.gravatar.com/avatar/%s&d=monsterid", GetMD5Hash(strings.ToLower(existingUser["email"].(string)))),
				"iat":     time.Now(),
				"jti":     u.String(),
			})

			// Sign and get the complete encoded token as a string using the secret
			tokenString, err := token.SignedString(d.secret)
			fmt.Printf("%v %v", tokenString, err)
			if err != nil {
				log.Errorf("Failed to sign string: %v", err)
				return nil, nil, []error{err}
			}

			responseAttrs = make(map[string]interface{})
			responseAttrs["value"] = string(tokenString)
			responseAttrs["key"] = "token"
			actionResponse := NewActionResponse("client.store.set", responseAttrs)
			responses = append(responses, actionResponse)

			notificationAttrs := make(map[string]string)
			notificationAttrs["message"] = "Logged in"
			notificationAttrs["title"] = "Success"
			notificationAttrs["type"] = "success"
			responses = append(responses, NewActionResponse("client.notify", notificationAttrs))

			responseAttrs = make(map[string]interface{})
			responseAttrs["location"] = "/"
			responseAttrs["window"] = "self"
			responseAttrs["delay"] = 2000

			responses = append(responses, NewActionResponse("client.redirect", responseAttrs))

		} else {
			responseAttrs = make(map[string]interface{})
			responseAttrs["type"] = "error"
			responseAttrs["title"] = "Failed"
			responseAttrs["message"] = "Invalid username or password"
			responses = append(responses, NewActionResponse("client.notify", responseAttrs))

		}

	}

	return nil, responses, nil
}

func NewGenerateJwtTokenPerformer(configStore *ConfigStore, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	secret, _ := configStore.GetConfigValueFor("jwt.secret", "backend")

	handler := GenerateJwtTokenActionPerformer{
		secret: []byte(secret),
		cruds:  cruds,
	}

	return &handler, nil

}
