// The Vue build version to load with the `import` command
// (runtime-only or standalone) has been set in webpack.base.conf with an alias.
import Vue from 'vue'
import App from './App'
import router from './router'
import ElementUI from 'element-ui'
import Vuetable from './components/vuetable'
import DetailedRow from './components/detailrow/DetailedRow.vue'
import ModelForm from './components/modelform/ModelForm.vue'
import VuetablePagination from './components/vuetable/components/VuetablePagination.vue'
import CustomActions from './components/detailrow/CustomActions.vue'
import TableView from './components/tableview/TableView.vue'
import SelectOneOrMore from './components/selectoneormore/SelectOneOrMore.vue'
import ListView from './components/listview/ListView.vue'
import EventView from './components/eventview/EventView.vue'
import {Notification} from 'element-ui';

global.jQuery = require('jquery');

Vue.config.productionTip = false;

Vue.use(ElementUI);
Vue.use(Vuetable);
Vue.use(VuetablePagination);
Vue.use(DetailedRow);

import 'element-ui/lib/theme-default/index.css'
import './components/vuetable/vuetable.css'
import JsonApi from 'devour-client'
import axios from 'axios'

Vue.component('custom-actions', CustomActions);
Vue.component('table-view', TableView);
Vue.component('event-view', EventView);
Vue.component('list-view', ListView);
Vue.component('model-form', ModelForm);
Vue.component("vuetable", Vuetable);
Vue.component("select-one-or-more", SelectOneOrMore);
Vue.component("detailed-table-row", DetailedRow);
Vue.component("vuetable-pagination", VuetablePagination);

// Vue.component("vuetable-pagination-dropdown", Vuetable.VueTablePaginationDropDown);
// Vue.component("vuetable-pagination-info", Vuetable.VueTablePaginationInfo);


window.apiRoot = "http://localhost:6336/api";
window.actionRoot = "http://localhost:6336/action";

window.jsonApi = new JsonApi({
  apiUrl: window.apiRoot,
  pluralize: false,
});


// Add a response interceptor
axios.interceptors.response.use(function (response) {
  // Do something with response data
  return response;
}, function (error) {
  // Do something with response error
  if (error.response && error.response.status == 403) {
    Notification.error({
      "title": "Unauthorized",
      "message": error.message
    })
  }
  console.log("error", error)
  return Promise.reject(error);
});

var ActionManager = function () {

  var that = this;
  that.actionMap = {};

  this.setActions = function (typeName, actions) {
    actionMap[typeName] = actions;
  };

  this.doAction = function (type, actionName, data) {
    console.log("invoke action", type, actionName, data);
    return axios({
      url: window.actionRoot + "/" + actionName,
      method: "POST",
      headers: {
        "Authorization": "Bearer " + localStorage.getItem("id_token")
      },
      data: {
        type: type,
        action: actionName,
        attributes: data
      }
    })


  };

  this.addAllActions = function (actions) {

    for (var i = 0; i < actions.length; i++) {
      var action = actions[i];
      var onType = action["onType"];

      if (!that.actionMap[onType]) {
        that.actionMap[onType] = {};
      }

      that.actionMap[onType][action["name"]] = action;
    }
  };

  this.getActions = function (typeName) {
    console.log("actions for ", typeName, that.actionMap[typeName])
    return that.actionMap[typeName];
  };

  this.getActionModel = function (typeName, actionName) {
    return that.actionMap[typeName].filter(function (i, r) {
      return r.ActionName == actionName;
    })[0];
  };

  return this;
};

window.actionManager = new ActionManager();

jsonApi.replaceMiddleware('errors', {
  name: 'nothing-to-see-here',
  error: function (payload) {
    console.log("errors", payload);

    if (payload.status == 401) {
      Notification.error({
        "title": "Failed",
        "message": payload.data
      });
      window.location = window.location;
      return;
    }


    for (var i = 0; i < payload.data.errors.length; i++) {
      Notification.error({
        "title": "Failed",
        "message": payload.data.errors[i].title
      })
    }
    return {errors: []}
  }
});


var requests = {};

jsonApi.insertMiddlewareBefore('response', {
  name: 'track-request',
  req: function (payload) {
    console.log("request initiate", payload);
    if (payload.config.method !== 'GET' && payload.config.method !== 'OPTIONS') {


      console.log("Create request complete: ", payload, payload.status / 100);
      if (parseInt(payload.status / 100) == 2) {
        var action = "Created ";

        if (payload.config.method == "DELETE") {
          action = "Deleted "
        } else if (payload.config.method == "PUT" || payload.config.method == "PATCH") {
          action = "Updated "
        }

        Notification.success({
          title: action + payload.config.model
        })
      } else {
        Notification.warn({
          "title": "Unidentified status"
        })
      }
    }
    return payload
  }
});


jsonApi.insertMiddlewareAfter('response', {
  name: 'success-notification',
  res: function (payload) {
    console.log("request complete", arguments);
    return payload
  }
});


window.jsonApi.headers['Authorization'] = 'Bearer ' + localStorage.getItem('id_token');


window.columnKeysCache = {};

window.getColumnKeys = function (typeName, callback) {

  if (window.columnKeysCache[typeName]) {
    callback(window.columnKeysCache[typeName])
    return
  }


  jQuery.ajax({
    url: 'http://localhost:6336/jsmodel/' + typeName + ".js",
    headers: {
      "Authorization": "Bearer " + localStorage.getItem("id_token")
    },
    success: function (r, e, s) {
      if (r.Actions.length > 0) {
        console.log("register actions", r.Actions)
        actionManager.addAllActions(r.Actions);
      }
      window.columnKeysCache[typeName] = r;
      callback(r, e, s);
    },
    error: function (r, e, s) {
      callback(r, e, s)
    },
  })
};

window.getColumnKeysWithErrorHandleWithThisBuilder = function (that) {
//    console.log("builder column model getter")
  return function (typeName, callback) {
    return getColumnKeys(typeName, function (a, e, s) {
//        console.log("get column kets respone: ", arguments)
      if (e == "error" && s == "Unauthorized") {
        that.logout();
      } else {
        callback(a, e, s)
      }
    })
  }
};


var logoutHandler = {
  logout: function () {
    localStorage.clear("id_token");
    console.log("logout");
    startApp();
  }
};


window.lock = {};

let v1 = typeof Auth0Lock;
let v2 = typeof v1;
console.log("type of", v1, v2);

if (v1 != "undefined") {
  console.log("it is not undefined");
  lock = new Auth0Lock('edsjFX3nR9fqqpUi4kRXkaKJefzfRaf_', 'gocms.auth0.com', {
    auth: {
      redirectUrl: 'http://localhost:8080/#/',
      responseType: 'token',
      params: {
        scope: 'openid email' // Learn about scopes: https://auth0.com/docs/scopes
      }
    }
  });
} else {
  lock = {
    checkAuth: function () {
      return !!localStorage.getItem("id_token");
    },
    on: function (vev) {
      console.log("nobody is listening to ", vev);
    }
  }
}

lock.checkAuth = function () {
  return !!localStorage.getItem('id_token');
}


var authenticated = lock.checkAuth();

lock.on('authenticated', (authResult) => {
  console.log('authenticated');
  localStorage.setItem('id_token', authResult.idToken);
  window.jsonApi.headers['Authorization'] = 'Bearer ' + authResult.idToken;

  loadModels();
  lock.getProfile(authResult.idToken, (error, profile) => {
    if (error) {
      // Handle error
      return;
    }
    // Set the token and user profile in local storage
    localStorage.setItem('profile', JSON.stringify(profile));

    this.authenticated = true;
    // window.location = window.location;
  });
})
;

lock.on('authorization_error', (error) => {
      // handle error when authorizaton fails
    }
);


function startApp() {
  console.log("Start app")

  /* eslint-disable no-new */
  new Vue({
    el: '#app',
    router,
    template: '<App/>',
    components: {App},
  });

}

window.GetJsonApiModel = function (columnModel) {

  var model = {};

  var keys = Object.keys(columnModel);
  for (var i = 0; i < keys.length; i++) {
    var key = keys[i];

    var data = columnModel[key];

    if (data["jsonApi"]) {
      model[key] = data;
    } else {
      model[key] = data.ColumnType;
    }
  }

  // console.log("returning model", model)
  return model;

}


var modelLoader = getColumnKeysWithErrorHandleWithThisBuilder(logoutHandler);


function loadModels() {
  modelLoader("user", function (columnKeys) {
    jsonApi.define("user", GetJsonApiModel(columnKeys.ColumnModel));
    modelLoader("usergroup", function (columnKeys) {
      jsonApi.define("usergroup", GetJsonApiModel(columnKeys.ColumnModel));

      modelLoader("world", function (columnKeys) {
        jsonApi.define("world", GetJsonApiModel(columnKeys.ColumnModel));

        jsonApi.findAll('world', {
          page: {number: 1, size: 50},
          include: ['world_column']
        }).then(function (res) {
          var total = res.length;


          for (var t = 0; t < res.length; t++) {


            (function (typeName) {
              modelLoader(typeName, function (model) {
                console.log("Loaded model", typeName, model);

                total -= 1;
                if (total < 1) {
                  startApp();
                }
                jsonApi.define(typeName, GetJsonApiModel(model.ColumnModel));
              })
            })(res[t].table_name)

          }
        });

      })
    });
  })
}

if (authenticated) {

  loadModels();

} else {
  setTimeout(function () {
    if (!lock.checkAuth()) {
      startApp()
    }
  }, 1000);
}
