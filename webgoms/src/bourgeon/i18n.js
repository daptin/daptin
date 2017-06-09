/* eslint-disable no-unused-vars */
import Vue from 'vue'
import VueI18n from 'vue-i18n'

export default {
  install: function (Vue, options) {
    var locales = options

    Vue.use(VueI18n)

    // vue-i18n configuration
    Vue.config.fallbackLang = locales[0]
    Vue.config.lang = options.indexOf(navigator.language > -1)
      ? navigator.language
      : Vue.config.fallbackLang

    options.forEach(function (lang) {
      Vue.locale(lang, require(`locales/${lang}.yml`))
    })

    Vue.mixin({
      computed: {
        lang: {
          get () {
            return Vue.config.lang
          },
          set (lang) {
            Vue.config.lang = lang
          }
        },
        locales: {
          get () {
            return locales
          }
        }
      },
      methods: {
        setLang (lang) {
          this.lang = lang
        },
        isLang (lang) {
          return this.lang === lang
        }
      }
    })
  }
}
