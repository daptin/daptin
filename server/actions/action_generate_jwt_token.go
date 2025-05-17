package actions

import (
	"fmt"
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/doug-martin/goqu/v9"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"time"
)

type generateJwtTokenActionPerformer struct {
	cruds          map[string]*resource.DbResource
	secret         []byte
	tokenLifeTime  int
	jwtTokenIssuer string
}

func (d *generateJwtTokenActionPerformer) Name() string {
	return "jwt.token"
}

func (d *generateJwtTokenActionPerformer) DoAction(request actionresponse.Outcome, inFieldMap map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	responses := make([]actionresponse.ActionResponse, 0)

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

	existingUsers, _, err := d.cruds[resource.USER_ACCOUNT_TABLE_NAME].GetRowsByWhereClauseWithTransaction("user_account", nil, transaction, goqu.Ex{"email": email})

	responseAttrs := make(map[string]interface{})
	if err != nil || len(existingUsers) < 1 {
		responseAttrs["type"] = "error"
		responseAttrs["message"] = "Invalid username or password"
		responseAttrs["title"] = "Failed"
		actionResponse := resource.NewActionResponse("client.notify", responseAttrs)
		responses = append(responses, actionResponse)
		return nil, responses, []error{fmt.Errorf("Invalid username or password")}
	} else {
		existingUser := existingUsers[0]
		if skipPasswordCheck || (existingUser["password"] != nil && resource.BcryptCheckStringHash(password, existingUser["password"].(string))) {

			// Create a new token object, specifying signing method and the claims
			// you would like it to contain.
			u, _ := uuid.NewV7()
			timeNow := time.Now().UTC()

			timeNow.Add(-2 * time.Minute) // allow clock skew of 2 minutes
			token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"email": existingUser["email"],
				"sub":   daptinid.InterfaceToDIR(existingUser["reference_id"]).String(),
				"name":  existingUser["name"],
				"nbf":   timeNow.Unix(),
				"exp":   timeNow.Add(time.Duration(d.tokenLifeTime) * time.Hour).Unix(),
				"iss":   d.jwtTokenIssuer,
				"iat":   timeNow.Unix(),
				"jti":   u.String(),
			})

			// Sign and get the complete encoded token as a string using the secret
			tokenString, err := token.SignedString(d.secret)
			//fmt.Printf("%v %v", tokenString, err)
			if err != nil {
				log.Errorf("Failed to sign string: %v", err)
				return nil, nil, []error{err}
			}

			responseAttrs = make(map[string]interface{})
			responseAttrs["value"] = string(tokenString)
			responseAttrs["key"] = "token"

			actionResponse := resource.NewActionResponse("client.store.set", responseAttrs)
			responses = append(responses, actionResponse)

			cookieResponseAttrs := make(map[string]interface{})
			cookieResponseAttrs["value"] = string(tokenString) + "; SameSite=Strict"
			cookieResponseAttrs["key"] = "token"

			actionResponse = resource.NewActionResponse("client.cookie.set", cookieResponseAttrs)
			responses = append(responses, actionResponse)

			notificationAttrs := make(map[string]string)
			notificationAttrs["message"] = "Logged in"
			notificationAttrs["title"] = "Success"
			notificationAttrs["type"] = "success"
			responses = append(responses, resource.NewActionResponse("client.notify", notificationAttrs))

			responseAttrs = make(map[string]interface{})
			responseAttrs["location"] = "/"
			responseAttrs["window"] = "self"
			responseAttrs["delay"] = 2000

			responses = append(responses, resource.NewActionResponse("client.redirect", responseAttrs))

		} else {
			responseAttrs = make(map[string]interface{})
			responseAttrs["type"] = "error"
			responseAttrs["title"] = "Failed"
			responseAttrs["message"] = "Invalid username or password"
			responses = append(responses, resource.NewActionResponse("client.notify", responseAttrs))
			return nil, responses, []error{fmt.Errorf("Invalid username or password")}
		}

	}

	return nil, responses, nil
}

func NewGenerateJwtTokenPerformer(configStore *resource.ConfigStore, cruds map[string]*resource.DbResource, transaction *sqlx.Tx) (actionresponse.ActionPerformerInterface, error) {

	secret, _ := configStore.GetConfigValueFor("jwt.secret", "backend", transaction)

	tokenLifeTimeHours, err := configStore.GetConfigIntValueFor("jwt.token.life.hours", "backend", transaction)
	resource.CheckErr(err, "No default jwt token life time set in configuration")
	if err != nil {
		err = configStore.SetConfigIntValueFor("jwt.token.life.hours", 24*3, "backend", transaction)
		resource.CheckErr(err, "Failed to store default jwt token life time")
		tokenLifeTimeHours = 24 * 3 // 3 days
	}

	jwtTokenIssuer, err := configStore.GetConfigValueFor("jwt.token.issuer", "backend", transaction)
	resource.CheckErr(err, "No default jwt token issuer set")
	if err != nil {
		uid, _ := uuid.NewV7()
		jwtTokenIssuer = "daptin-" + uid.String()[0:6]
		err = configStore.SetConfigValueFor("jwt.token.issuer", jwtTokenIssuer, "backend", transaction)
	}

	handler := generateJwtTokenActionPerformer{
		cruds:          cruds,
		secret:         []byte(secret),
		tokenLifeTime:  tokenLifeTimeHours,
		jwtTokenIssuer: jwtTokenIssuer,
	}

	return &handler, nil

}
