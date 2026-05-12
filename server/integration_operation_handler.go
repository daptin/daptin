package server

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

type integrationOperationRequest struct {
	OAuthTokenID interface{}            `json:"oauth_token_id"`
	CredentialID interface{}            `json:"credential_id"`
	Input        map[string]interface{} `json:"input"`
}

func CreateIntegrationOperationHandler(cruds map[string]*resource.DbResource) func(*gin.Context) {
	return func(c *gin.Context) {
		providerName := c.Param("providerName")
		operationName := integrationOperationNameParam(c)
		log.Tracef("Integration operation request received provider=[%s] operation=[%s]", providerName, operationName)
		if providerName == "" || operationName == "" {
			log.Warnf("Integration operation request missing provider or operation provider=[%s] operation=[%s]", providerName, operationName)
			c.AbortWithStatusJSON(http.StatusBadRequest, map[string]interface{}{
				"error": "provider and operation are required",
			})
			return
		}

		var body integrationOperationRequest
		if c.Request.Body != nil {
			err := json.NewDecoder(c.Request.Body).Decode(&body)
			if err != nil && !errors.Is(err, io.EOF) {
				log.Warnf("Integration operation request body parse failed provider=[%s] operation=[%s]: %v", providerName, operationName, err)
				c.AbortWithStatusJSON(http.StatusBadRequest, map[string]interface{}{
					"error": err.Error(),
				})
				return
			}
		}
		if body.Input == nil {
			body.Input = make(map[string]interface{})
		}
		if body.OAuthTokenID != nil {
			body.Input["oauth_token_id"] = body.OAuthTokenID
		}
		if body.CredentialID != nil {
			body.Input["credential_id"] = body.CredentialID
		}
		log.Debugf("Integration operation input prepared provider=[%s] operation=[%s] input_keys=%d oauth_token=%t credential=%t",
			providerName, operationName, len(body.Input), body.OAuthTokenID != nil, body.CredentialID != nil)

		user := c.Request.Context().Value("user")
		sessionUser := sessionUserFromContextValue(user)
		requestSessionUser := *sessionUser
		body.Input["sessionUser"] = sessionUser
		body.Input["requestSessionUser"] = &requestSessionUser
		body.Input["httpRequest"] = c.Request
		body.Input["httpRequestHeaders"] = map[string][]string(c.Request.Header)

		worldCrud := cruds["world"]
		if worldCrud == nil {
			log.Errorf("Integration operation cannot execute provider=[%s] operation=[%s]: world resource is not available", providerName, operationName)
			c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]interface{}{
				"error": "world resource is not available",
			})
			return
		}

		performer, ok := resource.GetActionHandler(worldCrud, providerName)
		if !ok || performer == nil {
			log.Warnf("Integration operation provider not found provider=[%s] operation=[%s]", providerName, operationName)
			c.AbortWithStatusJSON(http.StatusNotFound, map[string]interface{}{
				"error": "integration provider [" + providerName + "] is not installed or enabled",
			})
			return
		}

		transaction, err := worldCrud.Connection().Beginx()
		if err != nil {
			log.Errorf("Integration operation transaction begin failed provider=[%s] operation=[%s]: %v", providerName, operationName, err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]interface{}{
				"error": err.Error(),
			})
			return
		}

		outcome := actionresponse.Outcome{
			Type:   providerName,
			Method: operationName,
		}
		responder, actionResponses, errs := performer.DoAction(outcome, body.Input, transaction)
		if len(errs) > 0 {
			_ = transaction.Rollback()
			status := http.StatusBadRequest
			if strings.Contains(strings.ToLower(errs[0].Error()), "no such method") {
				status = http.StatusNotFound
			}
			log.Warnf("Integration operation execution failed provider=[%s] operation=[%s] status=[%d]: %v", providerName, operationName, status, errs[0])
			c.AbortWithStatusJSON(status, map[string]interface{}{
				"error": errs[0].Error(),
			})
			return
		}
		if err = transaction.Commit(); err != nil {
			log.Errorf("Integration operation transaction commit failed provider=[%s] operation=[%s]: %v", providerName, operationName, err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]interface{}{
				"error": err.Error(),
			})
			return
		}

		if responder == nil {
			log.Infof("Integration operation completed provider=[%s] operation=[%s] responses=%d", providerName, operationName, len(actionResponses))
			c.JSON(http.StatusOK, actionResponses)
			return
		}
		statusCode := responder.StatusCode()
		if statusCode == 0 {
			statusCode = http.StatusOK
		}
		log.Infof("Integration operation completed provider=[%s] operation=[%s] status=[%d]", providerName, operationName, statusCode)
		if response, ok := responder.(api2go.Response); ok {
			c.JSON(statusCode, response.Result())
			return
		}
		c.JSON(statusCode, responder.Result())
	}
}

func integrationOperationNameParam(c *gin.Context) string {
	return strings.TrimPrefix(c.Param("operationName"), "/")
}

func sessionUserFromContextValue(user interface{}) *auth.SessionUser {
	if sessionUser, ok := user.(*auth.SessionUser); ok && sessionUser != nil {
		return sessionUser
	}
	if user != nil {
		log.Warnf("Ignoring unexpected user context type [%T] for integration operation", user)
	}
	return &auth.SessionUser{}
}
