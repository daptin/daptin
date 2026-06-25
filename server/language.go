package server

import (
	"context"
	"strings"

	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"golang.org/x/text/language"
)

// CorsMiddleware provides a configurable CORS implementation.
type LanguageMiddleware struct {
	configStore     *resource.ConfigStore
	defaultLanguage string
}

// maxAcceptLanguageSeparators caps the number of '-' plus '_' separators we
// pass to language.ParseAcceptLanguage. The parser guards large '-' lists, but
// '_' is normalized internally and can otherwise drive quadratic parse work.
const maxAcceptLanguageSeparators = 32

func NewLanguageMiddleware(configStore *resource.ConfigStore, transaction *sqlx.Tx) *LanguageMiddleware {

	defaultLanguage, err := configStore.GetConfigValueFor("language.default", "backend", transaction)
	if err != nil {
		defaultLanguage = "en"
		err = configStore.SetConfigValueFor("language.default", "en", "backend", transaction)
		resource.CheckErr(err, "Failed to store default value for default language")
	}

	return &LanguageMiddleware{
		configStore:     configStore,
		defaultLanguage: defaultLanguage,
	}
}

func (lm *LanguageMiddleware) LanguageMiddlewareFunc(c *gin.Context) {
	//log.Printf("middleware ")

	header := c.GetHeader("Accept-Language")
	if acceptLanguageTooComplex(header) {
		header = ""
	}
	pref := GetLanguagePreference(header, lm.defaultLanguage)

	//c.Request.Context("language_preference", pref)
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "language_preference", pref))

}

func GetLanguagePreference(header string, defaultLanguage string) []string {
	preferredLanguage := header

	if preferredLanguage == "" || preferredLanguage == "undefined" || preferredLanguage == "null" {
		preferredLanguage = defaultLanguage
	}
	if acceptLanguageTooComplex(preferredLanguage) {
		preferredLanguage = defaultLanguage
	}

	languageTags, _, err := language.ParseAcceptLanguage(preferredLanguage)
	resource.CheckErr(err, "Failed to parse Accept-Language header [%v]", preferredLanguage)
	pref := make([]string, 0)
	prefMap := make(map[string]bool)

	if len(languageTags) == 1 && languageTags[0].String() == defaultLanguage {

	} else {

		for _, tag := range languageTags {
			base, conf := tag.Base()
			if conf == 0 {
				continue
			}
			if prefMap[base.String()] == true {
				continue
			}
			prefMap[base.String()] = true
			pref = append(pref, base.String())
		}

	}
	return pref
}

func acceptLanguageTooComplex(header string) bool {
	return strings.Count(header, "-")+strings.Count(header, "_") > maxAcceptLanguageSeparators
}
