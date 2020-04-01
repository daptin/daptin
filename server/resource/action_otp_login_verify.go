package resource

import (
	"context"
	"fmt"
	"github.com/artpar/go.uuid"
	"github.com/daptin/daptin/server/auth"
	"github.com/dgrijalva/jwt-go"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strings"

	//"golang.org/x/oauth2"
	"github.com/artpar/api2go"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"time"
)

type OtpLoginVerifyActionPerformer struct {
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

func (d *OtpLoginVerifyActionPerformer) Name() string {
	return "otp.login.verify"
}

func (d *OtpLoginVerifyActionPerformer) DoAction(request Outcome, inFieldMap map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {
	responses := make([]ActionResponse, 0)

	state, ok := inFieldMap["otp"].(string)
	email, ok := inFieldMap["email"]
	var userAccount map[string]interface{}
	var userOtpProfile map[string]interface{}
	var err error
	if email == nil || email == "" {
		phone, ok := inFieldMap["mobile"]
		if !ok {
			return nil, nil, []error{errors.New("email or mobile missing")}
		}
		userOtpProfile, err = d.cruds["user_otp_account"].GetObjectByWhereClause("user_otp_account", "mobile_number", phone.(string))
		if err != nil || userOtpProfile == nil {
			return nil, nil, []error{errors.New("unregistered mobile number")}
		}
		userAccount, _, err = d.cruds["user_account"].GetSingleRowByReferenceId("user_account", userOtpProfile["otp_of_account"].(string), nil)
	} else {
		userAccount, err = d.cruds["user_account"].GetUserAccountRowByEmail(email.(string))
		if err != nil {
			return nil, nil, []error{errors.New("invalid email")}
		}
		userAccountId, ok := userAccount["id"]
		if !ok {
			return nil, nil, []error{errors.New("unregistered mobile number")}
		}
		userOtpProfile, err = d.cruds["user_otp_account"].GetObjectByWhereClause("user_otp_account", "otp_of_account", userAccountId)
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

		pr := &http.Request{}
		user := &auth.SessionUser{
			UserId:          userAccount["id"].(int64),
			UserReferenceId: userAccount["reference_id"].(string),
		}
		pr = pr.WithContext(context.WithValue(context.Background(), "user", user))
		req := api2go.Request{
			PlainRequest: pr,
		}

		_, err := d.cruds["user_otp_account"].UpdateWithoutFilters(model, req)
		if err != nil {
			log.Errorf("Failed to mark user otp account as verified: %v", err)
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
		notificationAttrs["message"] = "You can now login using this number"
		notificationAttrs["title"] = "OTP Verified"
		notificationAttrs["type"] = "success"
		responses = append(responses, NewActionResponse("client.notify", notificationAttrs))

	} else {

		u, _ := uuid.NewV4()
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"email":   userAccount["email"],
			"name":    userAccount["name"],
			"nbf":     time.Now().Unix(),
			"exp":     time.Now().Add(time.Duration(d.tokenLifeTime) * time.Hour).Unix(),
			"iss":     d.jwtTokenIssuer,
			"picture": fmt.Sprintf("https://www.gravatar.com/avatar/%s&d=monsterid", GetMD5Hash(strings.ToLower(userAccount["email"].(string)))),
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

func NewOtpLoginVerifyActionPerformer(cruds map[string]*DbResource, configStore *ConfigStore) (ActionPerformerInterface, error) {

	configStore.GetConfigValueFor("jwt.secret", "backend")

	jwtSecret, err := configStore.GetConfigValueFor("jwt.secret", "backend")
	encryptionSecret, _ := configStore.GetConfigValueFor("encryption.secret", "backend")

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

	handler := OtpLoginVerifyActionPerformer{
		cruds:            cruds,
		tokenLifeTime:    tokenLifeTimeHours,
		configStore:      configStore,
		encryptionSecret: []byte(encryptionSecret),
		secret:           []byte(jwtSecret),
		jwtTokenIssuer:   jwtTokenIssuer,
	}

	return &handler, nil

}
