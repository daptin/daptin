// Import System requirements
import Vue from 'vue'
import VueRouter from 'vue-router'

import {sync} from 'vuex-router-sync'

import systemInit from "./plugins/main";
import worldManager from "./plugins/worldmanager";
import jsonApi from "./plugins/jsonapi";
import actionManager from "./plugins/actionmanager";
import axios from "./plugins/axios";

import vueFilter from 'vue-filter';

import routes from './routes'
import store from './store'

// Import Helpers for filters
import {domain, count, prettyDate, pluralize} from './filters'

// Import Views - Top level
import AppView from './components/App.vue'

// Import Install and register helper items
Vue.filter('count', count);
Vue.filter('domain', domain);
Vue.filter('prettyDate', prettyDate);
Vue.filter('pluralize', pluralize);
Vue.filter('chooseTitle', function (obj) {

    if (!obj) {
      return "_"
    }

    var keys = Object.keys(obj);
    console.log("choose title for ", obj);
    for (var i = 0; i < keys.length; i++) {
      if (keys[i].indexOf("name") > -1 && typeof obj[keys[i]] == "string" && obj[keys[i]].length > 0) {
        return obj[keys[i]];
      }
    }


    for (var i = 0; i < keys.length; i++) {
      if (keys[i].indexOf("title") > -1 && typeof obj[keys[i]] == "string" && obj[keys[i]].length > 0) {
        return obj[keys[i]];
      }
    }


    for (var i = 0; i < keys.length; i++) {
      if (keys[i].indexOf("label") > -1 && typeof obj[keys[i]] == "string" && obj[keys[i]].length > 0) {
        return obj[keys[i]];
      }
    }
    return obj["id"].toUpperCase();

  }
);
Vue.filter('titleCase', function (str) {
//        console.log("TitleCase  : ", str)
  if (!str || str.length < 2) {
    return str;
  }
  return str.replace(/[-_]+/g, " ").trim().split(' ')
    .map(w => (w[0] ? w[0].toUpperCase() : "") + w.substr(1).toLowerCase()).join(' ')
});

Vue.use(VueRouter);

// Routing logic
var router = new VueRouter({
  routes: routes,
  scrollBehavior: function (to, from, savedPosition) {
    return savedPosition || {x: 0, y: 0}
  }
});

// Some middleware to help us ensure the user is authenticated.
router.beforeEach((to, from, next) => {
  // window.console.log('Transition', transition)
  if (to.auth && (to.router.app.$store.state.token === 'null')) {
    window.console.log('Not authenticated');
    next({
      path: '/login',
      query: {redirect: to.fullPath}
    })
  } else {
    next()
  }
});

sync(store, router);

// Start out app!
// eslint-disable-next-line no-new
new Vue({
  el: '#root',
  router: router,
  store: store,
  render: h => h(AppView)
});

// Check local storage to handle refreshes
if (window.localStorage) {
  var localUserString = window.localStorage.getItem('user') || 'null';
  var localUser = JSON.parse(localUserString);

  if (localUser && store.state.user !== localUser) {
    store.commit('SET_USER', localUser);
    store.commit('SET_TOKEN', window.localStorage.getItem('token'))
  }
}
