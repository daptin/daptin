package resource

import (
	"github.com/daptin/daptin/server/subsite"
	validator "gopkg.in/go-playground/validator.v9"
)

func init() {
	if err := ValidatorInstance.RegisterValidation("site_headers", func(field validator.FieldLevel) bool {
		_, err := subsite.ParseResponseHeaders(field.Field().Interface())
		return err == nil
	}, true); err != nil {
		panic(err)
	}
}
