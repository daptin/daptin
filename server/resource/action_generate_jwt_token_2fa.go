package resource

import (
	"context"
	"errors"
	"fmt"
	"github.com/Masterminds/squirrel"
	"github.com/artpar/api2go"
	"github.com/artpar/go.uuid"
	"github.com/daptin/daptin/server/auth"
	"github.com/dgrijalva/jwt-go"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strings"
	"time"
)

type Generate2FAJwtTokenActionPerformer struct {
	cruds            map[string]*DbResource
	secret           []byte
	tokenLifeTime    int
	jwtTokenIssuer   string
	encryptionSecret []byte
	totpLength       int
}

func (d *Generate2FAJwtTokenActionPerformer) Name() string {
	return "2fa.jwt.token"
}

func (d *Generate2FAJwtTokenActionPerformer) DoAction(request Outcome, inFieldMap map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)

	email := inFieldMap["email"]
	stateVar, ok := inFieldMap["otp"]
	if !ok {
		return nil, nil, []error{fmt.Errorf("otp is empty")}
	}
	state, ok := stateVar.(string)
	if !ok {
		state = fmt.Sprintf("%v", stateVar)
	}
	var password = ""

	if inFieldMap["password"] != nil {
		password = inFieldMap["password"].(string)
	} else {
		return nil, nil, []error{fmt.Errorf("email or password is empty")}
	}

	if len(state) == 0 {
		return nil, nil, []error{fmt.Errorf("otp is empty")}
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
		if existingUser["password"] != nil && BcryptCheckStringHash(password, existingUser["password"].(string)) {

			userAccountId, _ := existingUser["id"]
			userOtpProfile, err := d.cruds["user_otp_account"].GetObjectByWhereClause("user_otp_account", "otp_of_account", userAccountId)
			if err != nil {
				return nil, nil, []error{fmt.Errorf("user otp profile does not exist")}
			}

			key, _ := Decrypt(d.encryptionSecret, userOtpProfile["otp_secret"].(string))

			ok, err := totp.ValidateCustom(state, key, time.Now().UTC(), totp.ValidateOpts{
				Period:    300,
				Skew:      1,
				Digits:    otp.Digits(d.totpLength),
				Algorithm: otp.AlgorithmSHA1,
			})
			if !ok {
				log.Errorf("Failed to validate otp key")
				return nil, nil, []error{errors.New("invalid otp")}
			}


			if userOtpProfile["verified"].(int64) == 0 {
				model := api2go.NewApi2GoModelWithData("user_otp_account", nil, 0, nil, userOtpProfile)
				model.SetAttributes(map[string]interface{}{
					"verified": 1,
				})

				pr := &http.Request{}
				user := &auth.SessionUser{
					UserId:          existingUser["id"].(int64),
					UserReferenceId: existingUser["reference_id"].(string),
				}
				pr = pr.WithContext(context.WithValue(context.Background(), "user", user))
				req := api2go.Request{
					PlainRequest: pr,
				}

				_, err := d.cruds["user_otp_account"].UpdateWithoutFilters(model, req)
				if err != nil {
					log.Errorf("Failed to mark user otp account as verified: %v", err)
				}

			}

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
			actionResponse = NewActionResponse("client.cookie.set", responseAttrs)
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

func NewGenerate2FAJwtTokenPerformer(configStore *ConfigStore, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

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
	encryptionSecret, _ := configStore.GetConfigValueFor("encryption.secret", "backend")
	totpLength, err := configStore.GetConfigIntValueFor("totp.length", "backend")
	if err != nil {
		totpLength = 6
		configStore.SetConfigValueFor("totp.length", "6", "backend")
	}

	handler := Generate2FAJwtTokenActionPerformer{
		cruds:            cruds,
		secret:           []byte(secret),
		tokenLifeTime:    tokenLifeTimeHours,
		encryptionSecret: []byte(encryptionSecret),
		totpLength:       totpLength,
		jwtTokenIssuer:   jwtTokenIssuer,
	}

	return &handler, nil

}
