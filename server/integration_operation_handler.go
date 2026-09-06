package server

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
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
		sanitizeProviderScopedIntegrationInput(body.Input)
		if body.OAuthTokenID != nil {
			body.Input["oauth_token_id"] = body.OAuthTokenID
		}
		if body.CredentialID != nil {
			body.Input["credential_id"] = body.CredentialID
		}
		log.Debugf("Integration operation input prepared provider=[%s] operation=[%s] input_keys=%d oauth_token=%t credential=%t",
			providerName, operationName, len(body.Input), body.OAuthTokenID != nil, body.CredentialID != nil)

		actionResponses, err := executeIntegrationOperationAction(cruds, providerName, operationName, body.Input, c.Request)
		if err != nil {
			status := integrationOperationErrorStatus(err)
			log.Warnf("Integration operation execution failed provider=[%s] operation=[%s] status=[%d]: %v", providerName, operationName, status, err)
			c.AbortWithStatusJSON(status, map[string]interface{}{"error": err.Error()})
			return
		}
		result, statusCode, err := integrationOperationActionResult(providerName, operationName, actionResponses)
		if err != nil {
			log.Errorf("Integration operation response failed provider=[%s] operation=[%s]: %v", providerName, operationName, err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			return
		}
		log.Infof("Integration operation completed provider=[%s] operation=[%s] status=[%d]", providerName, operationName, statusCode)
		c.JSON(statusCode, result)
	}
}

func executeIntegrationOperationAction(cruds map[string]*resource.DbResource, providerName string, operationName string, input map[string]interface{}, request *http.Request) ([]actionresponse.ActionResponse, error) {
	integrationCrud := cruds["integration"]
	if integrationCrud == nil {
		return nil, errors.New("integration resource is not available")
	}
	actionName, err := resource.IntegrationOperationActionName(providerName, operationName)
	if err != nil {
		return nil, err
	}
	transaction, err := integrationCrud.Connection().Beginx()
	if err != nil {
		return nil, err
	}
	responses, err := integrationCrud.HandleActionRequest(actionresponse.ActionRequest{
		Type:       "integration",
		Action:     actionName,
		Attributes: input,
	}, api2go.Request{PlainRequest: request}, transaction)
	if err != nil {
		_ = transaction.Rollback()
		return nil, err
	}
	if err := transaction.Commit(); err != nil {
		return nil, err
	}
	return responses, nil
}

func integrationOperationActionResult(providerName string, operationName string, responses []actionresponse.ActionResponse) (interface{}, int, error) {
	responseType := providerName + "." + operationName + ".response"
	statusType := providerName + "." + operationName + ".statusCode"
	var result interface{}
	status := 0
	for _, response := range responses {
		switch response.ResponseType {
		case responseType:
			result = response.Attributes
		case statusType:
			switch value := response.Attributes.(type) {
			case int:
				status = value
			case int64:
				status = int(value)
			case float64:
				status = int(value)
			}
		}
	}
	if result == nil || status == 0 {
		return nil, 0, fmt.Errorf("integration action [%s/%s] returned no provider response", providerName, operationName)
	}
	return result, status, nil
}

func integrationOperationErrorStatus(err error) int {
	if httpError, ok := err.(api2go.HTTPError); ok {
		return httpError.Status()
	}
	return http.StatusBadRequest
}

func integrationOperationNameParam(c *gin.Context) string {
	return strings.TrimPrefix(c.Param("operationName"), "/")
}

func sanitizeProviderScopedIntegrationInput(input map[string]interface{}) {
	for _, key := range []string{"oauth_token_id", "credential_id", "sessionUser", "httpRequest", "httpRequestHeaders"} {
		delete(input, key)
	}
}
