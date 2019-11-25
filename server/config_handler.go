package server

import (
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
)

func CreateConfigHandler(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource, configStore *resource.ConfigStore) func(*gin.Context) {
	return func(c *gin.Context) {

		user := c.Request.Context().Value("user")
		sessionUser := &auth.SessionUser{}

		if user != nil {
			sessionUser = user.(*auth.SessionUser)
		}

		if sessionUser.UserReferenceId != cruds[resource.USER_ACCOUNT_TABLE_NAME].GetAdminReferenceId() {
			c.AbortWithStatus(403)
			return
		}

		if c.Request.Method == "GET" {

			key := c.Param("key")

			if key == "" {
				c.AbortWithStatusJSON(200, configStore.GetAllConfig())
			} else {
				end := c.Param("end")
				val, err := configStore.GetConfigValueFor(key, end)
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
			err = configStore.SetConfigValueFor(key, string(newVal), end)
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
			err = configStore.SetConfigValueFor(key, string(newVal), end)
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
			err := configStore.DeleteConfigValueFor(key, end)
			if err != nil {
				c.AbortWithError(500, err)
				return
			}

		}

	}
}
