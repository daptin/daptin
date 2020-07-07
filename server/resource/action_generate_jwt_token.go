package resource

import (
	"fmt"
	"github.com/Masterminds/squirrel"
	"github.com/artpar/api2go"
	"github.com/artpar/go.uuid"
	"github.com/dgrijalva/jwt-go"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

type GenerateJwtTokenActionPerformer struct {
	cruds          map[string]*DbResource
	secret         []byte
	tokenLifeTime  int
	jwtTokenIssuer string
}

func (d *GenerateJwtTokenActionPerformer) Name() string {
	return "jwt.token"
}

func (d *GenerateJwtTokenActionPerformer) DoAction(request Outcome, inFieldMap map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)

	email := inFieldMap["email"]
	var password = ""

	skipPasswordCheck := false

	skipPasswordCheckStr, ok := inFieldMap["skipPasswordCheck"]
	if ok {
		skipPasswordCheck, _ = skipPasswordCheckStr.(bool)
	}

	if !skipPasswordCheck {
		if inFieldMap["password"] != nil {
			password = inFieldMap["password"].(string)
		} else {
			return nil, nil, []error{fmt.Errorf("email or password is empty")}
		}
	}

	if email == nil || (len(password) < 1 && !skipPasswordCheck) {
		return nil, nil, []error{fmt.Errorf("email or password is empty")}
	}

	existingUsers, _, err := d.cruds[USER_ACCOUNT_TABLE_NAME].GetRowsByWhereClause("user_account", squirrel.Eq{"email": email})

	responseAttrs := make(map[string]interface{})
	if err != nil || len(existingUsers) < 1 {
		responseAttrs["type"] = "error"
		responseAttrs["message"] = "Invalid username or password"
		responseAttrs["title"] = "Failed"
		actionResponse := NewActionResponse("client.notify", responseAttrs)
		responses = append(responses, actionResponse)
	} else {
		existingUser := existingUsers[0]
		if skipPasswordCheck || (existingUser["password"] != nil && BcryptCheckStringHash(password, existingUser["password"].(string))) {

			// Create a new token object, specifying signing method and the claims
			// you would like it to contain.
			u, _ := uuid.NewV4()
			token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"email":   existingUser["email"],
				"name":    existingUser["name"],
				"nbf":     time.Now().Unix(),
				"exp":     time.Now().Add(time.Duration(d.tokenLifeTime) * time.Hour).Unix(),
				"iss":     d.jwtTokenIssuer,
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

			cookieResponseAttrs := make(map[string]interface{})
			cookieResponseAttrs["value"] = string(tokenString) + "; SameSite=Strict"
			cookieResponseAttrs["key"] = "token"

			actionResponse = NewActionResponse("client.cookie.set", cookieResponseAttrs)
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

	handler := GenerateJwtTokenActionPerformer{
		cruds:          cruds,
		secret:         []byte(secret),
		tokenLifeTime:  tokenLifeTimeHours,
		jwtTokenIssuer: jwtTokenIssuer,
	}

	return &handler, nil

}
