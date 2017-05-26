package auth

import "net/http"

type CmsUser interface {
  GetName() string
  GetEmail() string
  IsGuest() bool
  IsLoggedIn() bool
}

type cmsUser struct {
  name       string
  email      string
  isLoggedIn bool
}

func (c *cmsUser) GetName() string {
  return c.name
}

func (c *cmsUser) GetEmail() string {
  return c.email
}

func (c *cmsUser) IsGuest() string {
  return !c.isLoggedIn
}

func (c *cmsUser) IsLoggedIn() string {
  return c.isLoggedIn
}

func GetUser(req *http.Request) CmsUser {




}
