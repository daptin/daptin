// Import System requirements
import Vue from 'vue'
import VueRouter from 'vue-router'

import {sync} from 'vuex-router-sync'

import systemInit from "./plugins/main";
import worldManager from "./plugins/worldmanager";
import jsonApi from "./plugins/jsonapi";
import actionManager from "./plugins/actionmanager";
import axios from "./plugins/axios";

import VueFilter from 'vue-filter';

import routes from './routes'
import store from './store'

// Import Views - Top level
import AppView from './components/App.vue'
import Element from 'element-ui'

Vue.use(Element)

// Import Install and register helper items

window.stringToColor = function (str, prc) {
  // Check for optional lightness/darkness
  var prc = typeof prc === 'number' ? prc : -10;

  // Generate a Hash for the String
  var hash = function (word) {
    var h = 0;
    for (var i = 0; i < word.length; i++) {
      h = word.charCodeAt(i) + ((h << 5) - h);
    }
    return h;
  };

  // Change the darkness or lightness
  var shade = function (color, prc) {
    var num = parseInt(color, 16),
      amt = Math.round(2.55 * prc),
      R = (num >> 16) + amt,
      G = (num >> 8 & 0x00FF) + amt,
      B = (num & 0x0000FF) + amt;
    return (0x1000000 + (R < 255 ? R < 1 ? 0 : R : 255) * 0x10000 +
      (G < 255 ? G < 1 ? 0 : G : 255) * 0x100 +
      (B < 255 ? B < 1 ? 0 : B : 255))
      .toString(16)
      .slice(1);
  };

  // Convert init to an RGBA
  var int_to_rgba = function (i) {
    var color = ((i >> 24) & 0xFF).toString(16) +
      ((i >> 16) & 0xFF).toString(16) +
      ((i >> 8) & 0xFF).toString(16) +
      (i & 0xFF).toString(16);
    return color;
  };

  return shade(int_to_rgba(hash(str)), prc);

};


window.chooseTitle = function (obj) {

  if (!obj) {
    return "_"
  }
  // console.log("choose title for ", obj)

  var candidates = ["name", "model", "title", "label"];

  var objType = obj["__type"];
  if (objType) {
    var objModel = jsonApi.modelFor(objType);
    var attrs = objModel.attributes;
    var attrKeys = Object.keys(attrs);
    for (var i = 0; i < attrKeys.length; i++) {
      if (attrs[attrKeys[i]] == "label") {
        candidates.push(attrKeys[i])
      }
    }
  }

  var keys = Object.keys(obj);
  // console.log("choose title for ", obj);


  for (var i = 0; i < candidates.length; i++) {

    var found = keys.indexOf(candidates[i])

    var found = keys.filter(function (k) {
      return k.indexOf(candidates[i]) > -1;
    });


    if (found.length > 0) {
      for (var u = 0; u < found.length; u++) {
        var val = obj[found[u]];
        if (typeof val == "string" && val.length > 0) {
          if (isNaN(parseInt(val))) {
            return val;
          }
        }
      }
    }
  }

  for (var i = 0; i < keys.length; i++) {
    if (keys[i].indexOf("description") > -1 && typeof obj[keys[i]] == "string" && obj[keys[i]].length > 0) {
      if (obj[keys[i]].length > 30) {
        return obj[keys[i]].substring(0, 30) + " ...";
      } else {
        return obj[keys[i]]
      }
    }
  }


  for (var i = 0; i < keys.length; i++) {

    if (!obj[keys[i]]) {
      continue;
    }
    if (obj[keys[i]] instanceof Array) {
      continue;
    }
    if (!(obj[keys[i]] instanceof Object)) {
      continue;
    }

    if (!obj[keys[i]]) {
      return ""
    }

    var childTitle = chooseTitle(obj[keys[i]]);
    return titleCase(obj["type"]) + " for " + childTitle;


    return obj[keys[i]];
  }

  if (obj["id"]) {
    return obj["id"].toUpperCase();
  }


  return "#un-named";
};

window.titleCase = function (str) {
  if (!str || !str.replace) {
    return str
  }
  // console.log("TitleCase  : [" + str + "]", str)
  if (!str || str.length < 2) {
    return str;
  }
  let s = str.replace(/[-_]+/g, " ").trim().split(' ')
    .map(w => (w[0] ? w[0].toUpperCase() : "") + w.substr(1).toLowerCase()).join(' ');
  // console.log("titled: ", s);
  return s
};

Vue.filter('chooseTitle', chooseTitle);
Vue.filter('titleCase', titleCase);

Vue.use(VueFilter);
Vue.use(VueRouter);

// Routing logic
var router = new VueRouter({
  routes: routes,
  mode: 'history',
  scrollBehavior: function (to, from, savedPosition) {
    return {x: 0, y: 0}
    // return savedPosition || {x: 0, y: 0}
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
    });
  } else {
    next();
  }
});

sync(store, router);

// Start out app!
// eslint-disable-next-line no-new
window.vueApp = new Vue({
  el: '#root',
  router: router,
  store: store,
  filter: VueFilter,
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
