package server

import (
	"fmt"
	"net/http"

	"github.com/daptin/daptin/server/apiblueprint"
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func CreateIntegrationOperationsHandler(cruds map[string]*resource.DbResource) func(*gin.Context) {
	return func(c *gin.Context) {
		withIntegrationDiscoveryRecovery(c, "list_operations", func() {
			providerName := c.Param("providerName")
			log.Tracef("Integration operation discovery list request provider=[%s]", providerName)
			log.Printf("Integration operation discovery list provider=[%s]", providerName)
			integration, ok := resolveIntegrationForDiscovery(c, cruds, providerName, "list_operations")
			if !ok {
				return
			}

			document, err := apiblueprint.ListIntegrationOperations(integration)
			if err != nil {
				log.Errorf("Integration operation discovery list failed provider=[%s]: %v", providerName, err)
				c.AbortWithStatusJSON(http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
				return
			}
			log.Infof("Integration operation discovery list completed provider=[%s] operations=%d", providerName, len(document.Operations))
			c.JSON(http.StatusOK, document)
		})
	}
}

func CreateIntegrationOperationDescribeHandler(cruds map[string]*resource.DbResource) func(*gin.Context) {
	return func(c *gin.Context) {
		withIntegrationDiscoveryRecovery(c, "describe_operation", func() {
			providerName := c.Param("providerName")
			operationName := integrationOperationNameParam(c)
			log.Tracef("Integration operation discovery describe request provider=[%s] operation=[%s]", providerName, operationName)
			integration, ok := resolveIntegrationForDiscovery(c, cruds, providerName, "describe_operation")
			if !ok {
				return
			}

			document, err := apiblueprint.DescribeIntegrationOperation(integration, operationName)
			if err != nil {
				log.Warnf("Integration operation discovery describe failed provider=[%s] operation=[%s]: %v", providerName, operationName, err)
				c.AbortWithStatusJSON(http.StatusNotFound, map[string]interface{}{"error": err.Error()})
				return
			}
			log.Infof("Integration operation discovery describe completed provider=[%s] operation=[%s]", providerName, operationName)
			c.JSON(http.StatusOK, document)
		})
	}
}

func CreateIntegrationOpenAPIHandler(cruds map[string]*resource.DbResource) func(*gin.Context) {
	return func(c *gin.Context) {
		withIntegrationDiscoveryRecovery(c, "openapi_yaml", func() {
			providerName := c.Param("providerName")
			log.Tracef("Integration operation scoped OpenAPI request provider=[%s]", providerName)
			integration, ok := resolveIntegrationForDiscovery(c, cruds, providerName, "openapi_yaml")
			if !ok {
				return
			}

			document, err := apiblueprint.BuildIntegrationOpenAPI(integration)
			if err != nil {
				log.Errorf("Integration operation scoped OpenAPI failed provider=[%s]: %v", providerName, err)
				c.AbortWithStatusJSON(http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
				return
			}
			log.Infof("Integration operation scoped OpenAPI completed provider=[%s] bytes=%d", providerName, len(document))
			c.Data(http.StatusOK, "application/yaml; charset=utf-8", []byte(document))
		})
	}
}

func resolveIntegrationForDiscovery(c *gin.Context, cruds map[string]*resource.DbResource, providerName string, action string) (resource.Integration, bool) {
	var zero resource.Integration
	if providerName == "" {
		log.Warnf("Integration operation discovery missing provider action=[%s]", action)
		c.AbortWithStatusJSON(http.StatusBadRequest, map[string]interface{}{"error": "provider is required"})
		return zero, false
	}

	worldCrud := cruds["world"]
	if worldCrud == nil {
		log.Errorf("Integration operation discovery cannot run action=[%s] provider=[%s]: world resource is not available", action, providerName)
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]interface{}{"error": "world resource is not available"})
		return zero, false
	}
	transaction, err := worldCrud.Connection().Beginx()
	if err != nil {
		log.Errorf("Integration operation discovery transaction begin failed action=[%s] provider=[%s]: %v", action, providerName, err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
		return zero, false
	}
	defer transaction.Rollback()

	integrations, err := worldCrud.GetActiveIntegrations(transaction)
	if err != nil {
		log.Errorf("Integration operation discovery integration lookup failed action=[%s] provider=[%s]: %v", action, providerName, err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
		return zero, false
	}
	log.Debugf("Integration operation discovery loaded active integrations action=[%s] provider=[%s] count=%d", action, providerName, len(integrations))
	for _, integration := range integrations {
		if integration.Name == providerName && integration.Enable {
			log.Debugf("Integration operation discovery matched provider action=[%s] provider=[%s] auth=[%s]", action, providerName, integration.AuthenticationType)
			return integration, true
		}
	}
	log.Warnf("Integration operation discovery provider not found action=[%s] provider=[%s]", action, providerName)
	c.AbortWithStatusJSON(http.StatusNotFound, map[string]interface{}{
		"error": fmt.Sprintf("integration provider [%s] is not installed or enabled", providerName),
	})
	return zero, false
}

func withIntegrationDiscoveryRecovery(c *gin.Context, action string, fn func()) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Errorf("Recovered panic in integration operation discovery action=[%s]: %v", action, recovered)
			if !c.Writer.Written() {
				c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]interface{}{"error": "integration operation discovery failed"})
			}
		}
	}()
	fn()
}
