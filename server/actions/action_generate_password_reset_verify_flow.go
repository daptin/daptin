package actions

import (
	"encoding/base64"
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/doug-martin/goqu/v9"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type generatePasswordResetVerifyActionPerformer struct {
	cruds          map[string]*resource.DbResource
	secret         []byte
	tokenLifeTime  int
	jwtTokenIssuer string
}

func (d *generatePasswordResetVerifyActionPerformer) Name() string {
	return "password.reset.verify"
}

func (d *generatePasswordResetVerifyActionPerformer) DoAction(request actionresponse.Outcome, inFieldMap map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	responses := make([]actionresponse.ActionResponse, 0)

	email := inFieldMap["email"]

	existingUsers, _, err := d.cruds[resource.USER_ACCOUNT_TABLE_NAME].GetRowsByWhereClause("user_account", nil, transaction, goqu.Ex{"email": email})

	responseAttrs := make(map[string]interface{})
	if err != nil || len(existingUsers) < 1 {
		responseAttrs["type"] = "error"
		responseAttrs["message"] = "No Such account"
		responseAttrs["title"] = "Failed"
		actionResponse := resource.NewActionResponse("client.notify", responseAttrs)
		responses = append(responses, actionResponse)
	} else {
		//existingUser := existingUsers[0]

		var token = inFieldMap["token"]
		tokenString, err := base64.StdEncoding.DecodeString(token.(string))
		if err != nil {
			responseAttrs["type"] = "error"
			responseAttrs["message"] = "Invalid token"
			responseAttrs["title"] = "Failed"
			actionResponse := resource.NewActionResponse("client.notify", responseAttrs)
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
				responses = append(responses, resource.NewActionResponse("client.notify", notificationAttrs))

			} else {

				notificationAttrs := make(map[string]string)
				notificationAttrs["message"] = "Logged in"
				notificationAttrs["title"] = "Success"
				notificationAttrs["type"] = "success"
				responses = append(responses, resource.NewActionResponse("client.notify", notificationAttrs))

			}
		}

	}

	return nil, responses, nil
}

func NewGeneratePasswordResetVerifyActionPerformer(configStore *resource.ConfigStore, cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {

	transaction, err := cruds["world"].Connection().Beginx()
	if err != nil {
		resource.CheckErr(err, "Failed to begin transaction [82]")
		return nil, err
	}
	defer transaction.Commit()
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

	handler := generatePasswordResetVerifyActionPerformer{
		cruds:          cruds,
		secret:         []byte(secret),
		tokenLifeTime:  tokenLifeTimeHours,
		jwtTokenIssuer: jwtTokenIssuer,
	}

	return &handler, nil

}
