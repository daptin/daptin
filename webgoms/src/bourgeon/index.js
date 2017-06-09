/* eslint-disable no-unused-vars */
import Icon from 'bourgeon/Icon.vue'
import Router from 'bourgeon/router'
import I18n from 'bourgeon/i18n'

export default {
  install (Vue, options) {
    Vue.use(Router)
    Vue.use(I18n, options.locales)

    Vue.component('icon', Icon)
  }
}
