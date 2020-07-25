package resource

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/Masterminds/squirrel"
	"github.com/artpar/api2go"
	"github.com/artpar/go-guerrilla/backends"
	"github.com/artpar/go-guerrilla/mail"
	"github.com/artpar/go.uuid"
	"github.com/dgrijalva/jwt-go"
	log "github.com/sirupsen/logrus"
	"net/textproto"
	"os"
	"strings"
	"time"
)

type GeneratePasswordResetActionPerformer struct {
	cruds                  map[string]*DbResource
	secret                 []byte
	tokenLifeTime          int
	jwtTokenIssuer         string
	passwordResetEmailFrom string
}

func (d *GeneratePasswordResetActionPerformer) Name() string {
	return "password.reset.begin"
}

func (d *GeneratePasswordResetActionPerformer) DoAction(request Outcome, inFieldMap map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

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

		// Create a new token object, specifying signing method and the claims
		// you would like it to contain.
		u, _ := uuid.NewV4()
		email := existingUser["email"].(string)
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"email":   email,
			"name":    existingUser["name"],
			"nbf":     time.Now().Unix(),
			"exp":     time.Now().Add(30 * time.Minute).Unix(),
			"iss":     d.jwtTokenIssuer,
			"picture": fmt.Sprintf("https://www.gravatar.com/avatar/%s&d=monsterid", GetMD5Hash(strings.ToLower(email))),
			"iat":     time.Now(),
			"jti":     u.String(),
		})

		// Sign and get the complete encoded token as a string using the secret
		tokenString, err := token.SignedString(d.secret)
		tokenStringBase64 := base64.StdEncoding.EncodeToString([]byte(tokenString))
		fmt.Printf("%v %v", tokenStringBase64, err)
		if err != nil {
			log.Errorf("Failed to sign string: %v", err)
			return nil, nil, []error{err}
		}

		mailBody := "Reset your password by clicking on this link: " + tokenStringBase64

		bodyDaya := bytes.NewBuffer([]byte(mailBody))
		mailEnvelop := mail.Envelope{
			Subject: "Reset password for account " + email,
			RcptTo: []mail.Address{
				{
					User: strings.Split(email, "@")[0],
					Host: strings.Split(email, "@")[1],
				},
			},
			MailFrom: mail.Address{
				User: strings.Split(d.passwordResetEmailFrom, "@")[0],
				Host: strings.Split(d.passwordResetEmailFrom, "@")[1],
			},
			Header: textproto.MIMEHeader{
				"Date": []string{time.Now().String()},
			},
			Data: *bodyDaya,
		}

		mailResult, err := d.cruds["mail"].MailSender(&mailEnvelop, backends.TaskSaveMail)
		if mailResult != nil {
			log.Infof("Password reset mail result:  {}", mailResult.String())
			notificationAttrs := make(map[string]string)
			notificationAttrs["message"] = "Password reset mail sent"
			notificationAttrs["title"] = "Success"
			notificationAttrs["type"] = "success"
			responses = append(responses, NewActionResponse("client.notify", notificationAttrs))
		} else {
			log.Errorf("Failed to sent password reset email {}", err)
			notificationAttrs := make(map[string]string)
			notificationAttrs["message"] = "Failed to send password reset mail"
			notificationAttrs["title"] = "Failed"
			notificationAttrs["type"] = "failed"
			responses = append(responses, NewActionResponse("client.notify", notificationAttrs))
		}

	}

	return nil, responses, nil
}

func NewGeneratePasswordResetActionPerformer(configStore *ConfigStore, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

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

	passwordResetEmailFrom, err := configStore.GetConfigValueFor("password.reset.email.from", "backend")
	CheckErr(err, "No default password reset email from set")
	if err != nil {
		hostname, err := configStore.GetConfigValueFor("hostname", "backend")
		if err != nil {
			hostname, err = os.Hostname()
		}
		jwtTokenIssuer = "no-reply@" + hostname
		err = configStore.SetConfigValueFor("password.reset.email.from", hostname, "backend")
	}

	handler := GeneratePasswordResetActionPerformer{
		cruds:                  cruds,
		secret:                 []byte(secret),
		tokenLifeTime:          tokenLifeTimeHours,
		passwordResetEmailFrom: passwordResetEmailFrom,

		jwtTokenIssuer: jwtTokenIssuer,
	}

	return &handler, nil

}
