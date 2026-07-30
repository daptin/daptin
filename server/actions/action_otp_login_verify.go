package actions

import (
	"context"
	"fmt"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/url"
	"strings"

	//"golang.org/x/oauth2"
	"github.com/artpar/api2go/v2"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"time"
)

type otpLoginVerifyActionPerformer struct {
	responseAttrs    map[string]interface{}
	cruds            map[string]*resource.DbResource
	configStore      *resource.ConfigStore
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

func (d *otpLoginVerifyActionPerformer) DoAction(request actionresponse.Outcome, inFieldMap map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	responses := make([]actionresponse.ActionResponse, 0)
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
	accountID, ok := userAccount["id"].(int64)
	if !ok || accountID == 0 {
		return nil, nil, []error{errors.New("Invalid OTP")}
	}
	purpose, _ := inFieldMap["purpose"].(string)
	if purpose == "" {
		purpose = "login"
	}
	verified := otpProfileVerified(userOtpProfile["verified"])
	sessionUser, _ := inFieldMap["sessionUser"].(*auth.SessionUser)
	if purpose == "enrollment" || purpose == "login" {
		if sessionUser == nil || sessionUser.UserId == 0 || sessionUser.UserId != accountID {
			return nil, nil, []error{errors.New("OTP verification requires the account owner")}
		}
	}
	if purpose != "enrollment" && !verified {
		return nil, nil, []error{errors.New("OTP is not enrolled for this account")}
	}
	req, _ := inFieldMap["httpRequest"].(*http.Request)
	source := otpSource(req)
	if err := consumeOTPAttempt(accountID, source); err != nil {
		return nil, nil, []error{otpProtectionHTTPError(err)}
	}

	key, decryptErr := resource.Decrypt(d.encryptionSecret, userOtpProfile["otp_secret"].(string))
	if decryptErr != nil {
		return nil, nil, []error{errors.New("Invalid OTP")}
	}

	timeInstance := time.Now().UTC()
	ok, err = totp.ValidateCustom(state, key, timeInstance, totp.ValidateOpts{
		Period:    otpPeriodSeconds,
		Skew:      0,
		Digits:    6,
		Algorithm: otp.AlgorithmSHA1,
	})
	if !ok {
		log.Warnf("Failed to validate OTP")
		return nil, nil, []error{errors.New("Invalid OTP")}
	}
	if err := consumeOTPCode(accountID, timeInstance); err != nil {
		return nil, nil, []error{otpProtectionHTTPError(err)}
	}
	clearOTPAttempts(accountID, source)

	if purpose == "enrollment" && !verified {
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

	}
	if purpose != "login" {
		return nil, responses, nil
	}

	tokenString, err := newAuthSessionToken(d.secret, d.tokenLifeTime, d.jwtTokenIssuer, userAccount, timeInstance, map[string]interface{}{
		"picture": fmt.Sprintf("https://www.gravatar.com/avatar/%s&d=monsterid", resource.GetMD5HashString(strings.ToLower(userAccount["email"].(string)))),
	})

	if err != nil {
		log.Errorf("Failed to sign string: %v", err)
		return nil, nil, []error{err}
	}

	responseAttrs := make(map[string]interface{})
	responseAttrs["value"] = tokenString
	responseAttrs["key"] = "token"
	actionResponse := resource.NewActionResponse("client.store.set", responseAttrs)
	responses = append(responses, actionResponse)

	return nil, responses, nil
}

func otpProfileVerified(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case int64:
		return typed == 1
	case int:
		return typed == 1
	case string:
		return typed == "true" || typed == "1"
	default:
		return fmt.Sprintf("%v", value) == "1"
	}
}

func NewOtpLoginVerifyActionPerformer(cruds map[string]*resource.DbResource, configStore *resource.ConfigStore, transaction *sqlx.Tx) (actionresponse.ActionPerformerInterface, error) {

	configStore.GetConfigValueFor("jwt.secret", "backend", transaction)

	jwtSecret, err := configStore.GetConfigValueFor("jwt.secret", "backend", transaction)
	encryptionSecret, _ := configStore.GetConfigValueFor("encryption.secret", "backend", transaction)

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
