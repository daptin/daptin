package resource

import (
	"context"
	"fmt"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/golang-jwt/jwt/v4"
	uuid "github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/url"
	"strings"

	//"golang.org/x/oauth2"
	"github.com/artpar/api2go"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"time"
)

type otpLoginVerifyActionPerformer struct {
	responseAttrs    map[string]interface{}
	cruds            map[string]*DbResource
	configStore      *ConfigStore
	encryptionSecret []byte
	tokenLifeTime    int
	jwtTokenIssuer   string
	otpKey           string
	secret           []byte
	totpSecret       string
}

func (d *otpLoginVerifyActionPerformer) Name() string {
	return "otp.login.verify"
}

func (d *otpLoginVerifyActionPerformer) DoAction(request Outcome, inFieldMap map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []ActionResponse, []error) {
	responses := make([]ActionResponse, 0)
	var err error

	state, ok := inFieldMap["otp"].(string)
	if !ok {
		stateInt, ok := inFieldMap["otp"]
		if ok {
			state = fmt.Sprintf("%v", stateInt)
		}
	}
	email, ok := inFieldMap["email"]
	var userAccount map[string]interface{}
	var userOtpProfile map[string]interface{}
	if email == nil || email == "" {
		phone, ok := inFieldMap["mobile"]
		if !ok {
			return nil, nil, []error{errors.New("email or mobile missing")}
		}
		userOtpProfile, err = d.cruds["user_otp_account"].GetObjectByWhereClauseWithTransaction("user_otp_account", "mobile_number", phone.(string), transaction)
		if err != nil || userOtpProfile == nil {
			return nil, nil, []error{errors.New("unregistered mobile number")}
		}
		userAccount, _, err = d.cruds["user_account"].GetSingleRowByReferenceIdWithTransaction(
			"user_account", daptinid.InterfaceToDIR(userOtpProfile["otp_of_account"]), nil, transaction)
	} else {
		userAccount, err = d.cruds["user_account"].GetUserAccountRowByEmailWithTransaction(email.(string), transaction)
		if err != nil {
			return nil, nil, []error{errors.New("invalid email")}
		}
		userAccountId, ok := userAccount["id"]
		if !ok {
			return nil, nil, []error{errors.New("unregistered mobile number")}
		}
		userOtpProfile, err = d.cruds["user_otp_account"].GetObjectByWhereClauseWithTransaction(
			"user_otp_account", "otp_of_account", userAccountId, transaction)
	}

	if err != nil || userOtpProfile == nil {
		return nil, nil, []error{errors.New("Invalid OTP")}
	}

	key, _ := Decrypt(d.encryptionSecret, userOtpProfile["otp_secret"].(string))

	ok, err = totp.ValidateCustom(state, key, time.Now().UTC(), totp.ValidateOpts{
		Period:    300,
		Skew:      1,
		Digits:    4,
		Algorithm: otp.AlgorithmSHA1,
	})
	if !ok {
		log.Errorf("Failed to validate otp key")
		return nil, nil, []error{errors.New("Invalid OTP")}
	}

	if userOtpProfile["verified"].(int64) == 0 {
		model := api2go.NewApi2GoModelWithData("user_otp_account", nil, 0, nil, userOtpProfile)
		model.SetAttributes(map[string]interface{}{
			"verified": 1,
		})
		ur, _ := url.Parse("/user_otp_account")

		pr := &http.Request{
			URL: ur,
		}
		user := &auth.SessionUser{
			UserId:          userAccount["id"].(int64),
			UserReferenceId: daptinid.InterfaceToDIR(userAccount["reference_id"]),
		}
		pr = pr.WithContext(context.WithValue(context.Background(), "user", user))
		req := api2go.Request{
			PlainRequest: pr,
		}

		_, err = d.cruds["user_otp_account"].UpdateWithoutFilters(model, req, transaction)
		if err != nil {
			log.Errorf("Failed to mark user otp account as verified: %v", err)
			return nil, nil, []error{err}
		}

		//userModel := api2go.NewApi2GoModelWithData("user_account", nil, 0, nil, userAccount)
		//userModel.SetAttributes(map[string]interface{}{
		//	"user_otp_account_id": userOtpProfile["reference_id"],
		//})
		//_, err = d.cruds["user_account"].UpdateWithoutFilters(userModel, req)
		//if err != nil {
		//	log.Errorf("Failed to associate verified otp account with user account: %v", err)
		//}
		notificationAttrs := make(map[string]string)
		notificationAttrs["message"] = "OTP Verified"
		notificationAttrs["title"] = "OTP Verified"
		notificationAttrs["type"] = "success"
		responses = append(responses, NewActionResponse("client.notify", notificationAttrs))

	} else {

		u, _ := uuid.NewV7()
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"email":   userAccount["email"],
			"name":    userAccount["name"],
			"nbf":     time.Now().Unix(),
			"exp":     time.Now().Add(time.Duration(d.tokenLifeTime) * time.Hour).Unix(),
			"iss":     d.jwtTokenIssuer,
			"picture": fmt.Sprintf("https://www.gravatar.com/avatar/%s&d=monsterid", GetMD5HashString(strings.ToLower(userAccount["email"].(string)))),
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

		responseAttrs := make(map[string]interface{})
		responseAttrs["value"] = string(tokenString)
		responseAttrs["key"] = "token"
		actionResponse := NewActionResponse("client.store.set", responseAttrs)
		responses = append(responses, actionResponse)

		notificationAttrs := make(map[string]string)
		notificationAttrs["message"] = "Logged in"
		notificationAttrs["title"] = "Success"
		notificationAttrs["type"] = "success"
		responses = append(responses, NewActionResponse("client.notify", notificationAttrs))
	}

	responseAttrs := make(map[string]interface{})
	responseAttrs = make(map[string]interface{})
	responseAttrs["location"] = "/"
	responseAttrs["window"] = "self"
	responseAttrs["delay"] = 2000

	responses = append(responses, NewActionResponse("client.redirect", responseAttrs))

	return nil, responses, nil
}

func NewOtpLoginVerifyActionPerformer(cruds map[string]*DbResource, configStore *ConfigStore, transaction *sqlx.Tx) (ActionPerformerInterface, error) {

	configStore.GetConfigValueFor("jwt.secret", "backend", transaction)

	jwtSecret, err := configStore.GetConfigValueFor("jwt.secret", "backend", transaction)
	encryptionSecret, _ := configStore.GetConfigValueFor("encryption.secret", "backend", transaction)

	tokenLifeTimeHours, err := configStore.GetConfigIntValueFor("jwt.token.life.hours", "backend", transaction)
	CheckErr(err, "No default jwt token life time set in configuration")
	if err != nil {
		err = configStore.SetConfigIntValueFor("jwt.token.life.hours", 24*3, "backend", transaction)
		CheckErr(err, "Failed to store default jwt token life time")
		tokenLifeTimeHours = 24 * 3 // 3 days
	}

	jwtTokenIssuer, err := configStore.GetConfigValueFor("jwt.token.issuer", "backend", transaction)
	CheckErr(err, "No default jwt token issuer set")
	if err != nil {
		uid, _ := uuid.NewV7()
		jwtTokenIssuer = "daptin-" + uid.String()[0:6]
		err = configStore.SetConfigValueFor("jwt.token.issuer", jwtTokenIssuer, "backend", transaction)
	}

	handler := otpLoginVerifyActionPerformer{
		cruds:            cruds,
		tokenLifeTime:    tokenLifeTimeHours,
		configStore:      configStore,
		encryptionSecret: []byte(encryptionSecret),
		secret:           []byte(jwtSecret),
		jwtTokenIssuer:   jwtTokenIssuer,
	}

	return &handler, nil

}
