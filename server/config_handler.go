package server

import (
	"fmt"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func CreateConfigHandler(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource, configStore *resource.ConfigStore) func(*gin.Context) {
	return func(c *gin.Context) {

		user := c.Request.Context().Value("user")
		sessionUser := &auth.SessionUser{}

		if user != nil {
			sessionUser = user.(*auth.SessionUser)
		}

		userAccountTableCrud := cruds[resource.USER_ACCOUNT_TABLE_NAME]
		transaction, err := userAccountTableCrud.Connection.Beginx()
		if err != nil {
			resource.CheckErr(err, "Failed to begin transaction [24]")
			return
		}

		defer transaction.Commit()

		if !resource.IsAdminWithTransaction(sessionUser, transaction) {
			c.AbortWithError(403, fmt.Errorf("unauthorized"))
			return
		}
		log.Tracef("User [%v] has access to config", sessionUser.UserReferenceId)

		if c.Request.Method == "GET" {

			key := c.Param("key")

			if key == "" {
				c.AbortWithStatusJSON(200, configStore.GetAllConfig(transaction))
			} else {
				end := c.Param("end")
				val, err := configStore.GetConfigValueFor(key, end, transaction)
				if err != nil {
					c.AbortWithStatus(404)
					return
				}
				c.String(200, "%v", val)
			}

		} else if c.Request.Method == "POST" {

			key := c.Param("key")
			end := c.Param("end")

			if key == "" || end == "" {
				c.AbortWithStatus(400)
				return
			}

			newVal, err := c.GetRawData()
			if err != nil {
				c.AbortWithStatus(400)
				return
			}
			err = configStore.SetConfigValueFor(key, string(newVal), end, transaction)
			if err != nil {
				c.AbortWithError(500, err)
				return
			}

		} else if c.Request.Method == "PUT" || c.Request.Method == "PATCH" {

			key := c.Param("key")
			end := c.Param("end")

			if key == "" || end == "" {
				c.AbortWithStatus(400)
				return
			}

			newVal, err := c.GetRawData()
			if err != nil {
				c.AbortWithStatus(400)
				return
			}
			err = configStore.SetConfigValueFor(key, string(newVal), end, transaction)
			if err != nil {
				c.AbortWithError(500, err)
				return
			}

		} else if c.Request.Method == "DELETE" {

			key := c.Param("key")
			end := c.Param("end")

			if key == "" || end == "" {
				c.AbortWithStatus(400)
				return
			}
			err := configStore.DeleteConfigValueFor(key, end, transaction)
			if err != nil {
				c.AbortWithError(500, err)
				return
			}

		}

	}
}
