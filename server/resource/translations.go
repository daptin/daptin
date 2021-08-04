package resource

import (
	english "github.com/go-playground/locales/en"
	"github.com/go-playground/universal-translator"
	en2 "gopkg.in/go-playground/validator.v9/translations/en"
)

func RegisterTranslations() {

	eng := english.New()
	uni := ut.New(eng, eng)
	trans, _ := uni.GetTranslator("en")

	err := en2.RegisterDefaultTranslations(ValidatorInstance, trans)
	CheckErr(err, "Failed to register translactions")
}
