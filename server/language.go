package server

import (
	"context"
	"strings"

	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"golang.org/x/text/language"
)

// CorsMiddleware provides a configurable CORS implementation.
type LanguageMiddleware struct {
	configStore     *resource.ConfigStore
	defaultLanguage string
}

// Bound input size and range count before the parser allocates and sorts tags.
// Also cap '-' plus '_' separators: '_' is normalized internally and can
// otherwise drive quadratic work even for a single language range.
const (
	maxAcceptLanguageBytes      = 8 * 1024
	maxAcceptLanguageRanges     = 32
	maxAcceptLanguageSeparators = 32
)

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

	pref := GetLanguagePreference(c.GetHeader("Accept-Language"), lm.defaultLanguage)

	//c.Request.Context("language_preference", pref)
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "language_preference", pref))

}

func GetLanguagePreference(header string, defaultLanguage string) []string {
	preferredLanguage := header

	if preferredLanguage == "" || preferredLanguage == "undefined" || preferredLanguage == "null" || acceptLanguageTooComplex(preferredLanguage) {
		preferredLanguage = defaultLanguage
		if acceptLanguageTooComplex(preferredLanguage) {
			return []string{}
		}
	}

	languageTags, _, err := language.ParseAcceptLanguage(preferredLanguage)
	if err != nil {
		log.Debug("Ignoring invalid Accept-Language header")
	}
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
	// Check bytes first so oversized input is rejected without scanning its contents.
	return len(header) > maxAcceptLanguageBytes ||
		strings.Count(header, ",") >= maxAcceptLanguageRanges ||
		strings.Count(header, "-")+strings.Count(header, "_") > maxAcceptLanguageSeparators
}
