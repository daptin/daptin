package server

import (
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
	"golang.org/x/text/language"
)

// CorsMiddleware provides a configurable CORS implementation.
type LanguageMiddleware struct {
	configStore     *resource.ConfigStore
	defaultLanguage string
}

func NewLanguageMiddleware(configStore *resource.ConfigStore) *LanguageMiddleware {

	defaultLanguage, err := configStore.GetConfigValueFor("langauge.default", "backend")
	if err != nil {
		defaultLanguage = "en"
		err = configStore.SetConfigValueFor("language.default", "en", "backend")
		resource.CheckErr(err, "Failed to store default value for default language")
	}

	return &LanguageMiddleware{
		configStore:     configStore,
		defaultLanguage: defaultLanguage,
	}
}

func (lm *LanguageMiddleware) LanguageMiddlewareFunc(c *gin.Context) {
	//log.Infof("middleware ")

	preferredLanguage := c.GetHeader("Accept-Language")

	if preferredLanguage == "" {
		preferredLanguage = lm.defaultLanguage
	}

	languageTags, _, err := language.ParseAcceptLanguage(preferredLanguage)
	resource.CheckErr(err, "Failed to parse Accept-Language header [%v]", preferredLanguage)
	pref := make([]string, 0)

	if len(languageTags) == 1 && languageTags[0].String() == lm.defaultLanguage {

	} else {

		for _, tag := range languageTags {
			base, conf := tag.Base()
			if conf == 0 {
				continue
			}
			pref = append(pref, base.String())
		}

	}

	c.Set("language_preference", pref)

}
