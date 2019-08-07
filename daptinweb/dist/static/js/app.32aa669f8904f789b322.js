webpackJsonp([1],[
/* 0 */,
/* 1 */,
/* 2 */,
/* 3 */,
/* 4 */,
/* 5 */,
/* 6 */,
/* 7 */,
/* 8 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys__ = __webpack_require__(5);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_defineProperty__ = __webpack_require__(41);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_defineProperty___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_defineProperty__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_babel_runtime_core_js_promise__ = __webpack_require__(52);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_babel_runtime_core_js_promise___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_2_babel_runtime_core_js_promise__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3_axios__ = __webpack_require__(31);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3_axios___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_3_axios__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4__jsonapi__ = __webpack_require__(11);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_5__actionmanager__ = __webpack_require__(10);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_6__appconfig__ = __webpack_require__(51);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_7__utils_auth__ = __webpack_require__(18);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_8__store__ = __webpack_require__(125);












var WorldManager = function WorldManager() {
  var that = this;
  that.columnKeysCache = {};

  that.stateMachines = {};
  that.stateMachineEnabled = {};
  that.streams = {};

  that.getStateMachinesForType = function (typeName) {
    return new __WEBPACK_IMPORTED_MODULE_2_babel_runtime_core_js_promise___default.a(function (resolve, reject) {
      resolve(that.stateMachines[typeName]);
    });
  };

  that.startObjectTrack = function (objType, objRefId, stateMachineRefId) {

    return __WEBPACK_IMPORTED_MODULE_3_axios___default()({
      url: __WEBPACK_IMPORTED_MODULE_6__appconfig__["a" /* default */].apiRoot + "/track/start/" + stateMachineRefId,
      method: "POST",
      data: {
        typeName: objType,
        referenceId: objRefId
      },
      headers: {
        "Authorization": "Bearer " + __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_7__utils_auth__["d" /* getToken */])()
      }
    });
  };

  that.trackObjectEvent = function (typeName, stateMachineRefId, eventName) {
    var _axios;

    console.log("change object track", __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_7__utils_auth__["d" /* getToken */])());
    return __WEBPACK_IMPORTED_MODULE_3_axios___default()((_axios = {
      url: __WEBPACK_IMPORTED_MODULE_6__appconfig__["a" /* default */].apiRoot + "/track/event/" + stateMachineRefId + "/" + eventName
    }, __WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_defineProperty___default()(_axios, "url", __WEBPACK_IMPORTED_MODULE_6__appconfig__["a" /* default */].apiRoot + "/track/event/" + typeName + "/" + stateMachineRefId + "/" + eventName), __WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_defineProperty___default()(_axios, "method", "POST"), __WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_defineProperty___default()(_axios, "headers", {
      "Authorization": "Bearer " + __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_7__utils_auth__["d" /* getToken */])()
    }), _axios));
  };

  that.getColumnKeys = function (typeName, callback) {
    if (that.columnKeysCache[typeName]) {
      callback(that.columnKeysCache[typeName]);
      return;
    }

    __WEBPACK_IMPORTED_MODULE_3_axios___default()(__WEBPACK_IMPORTED_MODULE_6__appconfig__["a" /* default */].apiRoot + '/jsmodel/' + typeName + ".js", {
      headers: {
        "Authorization": "Bearer " + __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_7__utils_auth__["d" /* getToken */])()
      }
    }).then(function (r) {
      if (r.status == 200) {
        var r = r.data;
        console.log("Loaded Model inside :", typeName);
        if (r.Actions.length > 0) {
          console.log("Register actions", typeName, r.Actions);
          __WEBPACK_IMPORTED_MODULE_5__actionmanager__["a" /* default */].addAllActions(r.Actions);
        }
        that.stateMachines[typeName] = r.StateMachines;
        that.stateMachineEnabled[typeName] = r.IsStateMachineEnabled;
        that.columnKeysCache[typeName] = r;
        callback(r);
      } else {
        callback({}, r);
      }
    }, function (e) {
      callback(e);
    });
  };
  that.columnTypes = [];

  __WEBPACK_IMPORTED_MODULE_3_axios___default()(__WEBPACK_IMPORTED_MODULE_6__appconfig__["a" /* default */].apiRoot + "/meta?query=column_types", {
    headers: {
      "Authorization": "Bearer " + __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_7__utils_auth__["d" /* getToken */])()
    }
  }).then(function (r) {
    if (r.status == 200) {
      var r = r.data;
      that.columnTypes = r;
    } else {
      console.log("failed to get column types");
    }
  });

  that.getColumnFieldTypes = function () {
    console.log("Get column field types", that.columnTypes);
    return that.columnTypes;
  };

  that.isStateMachineEnabled = function (typeName) {
    return that.stateMachineEnabled[typeName] == true;
  };

  that.getColumnKeysWithErrorHandleWithThisBuilder = function () {
    return function (typeName, callback) {
      return that.getColumnKeys(typeName, function (a, e, s) {
        if (e === "error" && s === "Unauthorized") {
          that.logout();
        } else {
          callback(a, e, s);
        }
      });
    };
  };

  that.GetJsonApiModel = function (columnModel) {
    console.log('get json api model for ', columnModel);
    var model = {};
    if (!columnModel) {
      console.log("Column model is empty", columnModel);
      return model;
    }

    var keys = __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default()(columnModel);
    for (var i = 0; i < keys.length; i++) {
      var key = keys[i];

      var data = columnModel[key];

      if (data["jsonApi"]) {
        model[key] = data;
      } else {
        model[key] = data.ColumnType;
      }
    }

    return model;
  };

  var logoutHandler = ";";

  that.modelLoader = that.getColumnKeysWithErrorHandleWithThisBuilder(logoutHandler);

  that.worlds = [];

  that.getWorlds = function () {
    console.log("GET WORLDS", that.worlds);
    return that.worlds;
  };
  that.getWorldByName = function (name) {
    return that.worlds.filter(function (e) {
      return e.table_name == name;
    })[0];
  };

  that.systemActions = [];

  that.getSystemActions = function () {
    return that.systemActions;
  };

  that.reclineFieldTypeMap = {};

  __WEBPACK_IMPORTED_MODULE_3_axios___default()({
    url: __WEBPACK_IMPORTED_MODULE_6__appconfig__["a" /* default */].apiRoot + '/recline_model'
  }).then(function (res) {
    console.log("recline field type map", res);
    that.reclineFieldTypeMap = res.data;
  });

  that.getReclineModel = function (tableName, callback) {
    that.getColumnKeys(tableName, function (columnsModel) {
      var columns = columnsModel.ColumnModel;
      console.log("build recline model", columns);

      var colNames = __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default()(columns);
      var reclineModel = [];

      for (var i = 0; i < colNames.length; i++) {
        var colName = colNames[i];
        var colType = columns[colName];
        if (colType.ColumnType == "hidden") {
          continue;
        }

        var reclineType = that.reclineFieldTypeMap[colType.ColumnType];

        if (!reclineType) {

          if (colType.jsonApi == "hasOne") {} else if (colType.jsonApi == "hasMany") {}
        } else {

          reclineModel.push({
            id: colName,
            type: reclineType,
            label: window.titleCase(colType.ColumnName)
          });
        }
      }

      console.log("recline model", reclineModel);
      callback(reclineModel);
      return reclineModel;
    });
  };

  __WEBPACK_IMPORTED_MODULE_4__jsonapi__["a" /* default */].define("image.png|jpg|jpeg|gif|tiff", {
    "__type": "value",
    "contents": "value",
    "name": "value",
    "reference_id": "value",
    "src": "value",
    "type": "value"
  });

  __WEBPACK_IMPORTED_MODULE_4__jsonapi__["a" /* default */].define("image.png|jpg", {
    "__type": "value",
    "contents": "value",
    "name": "value",
    "reference_id": "value",
    "src": "value",
    "type": "value"
  });

  __WEBPACK_IMPORTED_MODULE_4__jsonapi__["a" /* default */].define("image.jpg|png", {
    "__type": "value",
    "contents": "value",
    "name": "value",
    "reference_id": "value",
    "src": "value",
    "type": "value"
  });

  __WEBPACK_IMPORTED_MODULE_4__jsonapi__["a" /* default */].define("image.png", {
    "__type": "value",
    "contents": "value",
    "name": "value",
    "reference_id": "value",
    "src": "value",
    "type": "value"
  });

  __WEBPACK_IMPORTED_MODULE_4__jsonapi__["a" /* default */].define("image.gif", {
    "__type": "value",
    "contents": "value",
    "name": "value",
    "reference_id": "value",
    "src": "value",
    "type": "value"
  });

  that.loadModel = function (modelName) {
    var promise = new __WEBPACK_IMPORTED_MODULE_2_babel_runtime_core_js_promise___default.a(function (resolve, reject) {

      that.modelLoader(modelName, function (columnKeys) {
        __WEBPACK_IMPORTED_MODULE_4__jsonapi__["a" /* default */].define(modelName, that.GetJsonApiModel(columnKeys.ColumnModel));
        resolve();
      });
    });

    return promise;
  };

  that.loadModels = function () {

    var promise = new __WEBPACK_IMPORTED_MODULE_2_babel_runtime_core_js_promise___default.a(function (resolve, reject) {
      that.modelLoader("user_account", function (columnKeys) {
        __WEBPACK_IMPORTED_MODULE_4__jsonapi__["a" /* default */].define("user_account", that.GetJsonApiModel(columnKeys.ColumnModel));
        that.modelLoader("usergroup", function (columnKeys) {
          __WEBPACK_IMPORTED_MODULE_4__jsonapi__["a" /* default */].define("usergroup", that.GetJsonApiModel(columnKeys.ColumnModel));

          that.modelLoader("world", function (columnKeys) {
            that.modelLoader("stream", function (streamKeys) {

              __WEBPACK_IMPORTED_MODULE_4__jsonapi__["a" /* default */].define("world", that.GetJsonApiModel(columnKeys.ColumnModel));
              __WEBPACK_IMPORTED_MODULE_4__jsonapi__["a" /* default */].define("stream", that.GetJsonApiModel(streamKeys.ColumnModel));

              console.log("Defined world", columnKeys.ColumnModel);
              that.systemActions = columnKeys.Actions;

              __WEBPACK_IMPORTED_MODULE_4__jsonapi__["a" /* default */].findAll('world', {
                page: { number: 1, size: 500 }
              }).then(function (res) {
                res = res.data;
                that.worlds = res;
                __WEBPACK_IMPORTED_MODULE_8__store__["a" /* default */].commit("SET_WORLDS", res);
                console.log("Get all worlds result", res);

                var total = res.length;

                for (var t = 0; t < res.length; t++) {
                  (function (typeName) {
                    that.modelLoader(typeName, function (model) {

                      total -= 1;

                      if (total < 1 && promise !== null) {
                        resolve("Stuff worked!");
                        promise = null;
                      }

                      __WEBPACK_IMPORTED_MODULE_4__jsonapi__["a" /* default */].define(typeName, that.GetJsonApiModel(model.ColumnModel));
                    });
                  })(res[t].table_name);
                }
              });

              __WEBPACK_IMPORTED_MODULE_4__jsonapi__["a" /* default */].findAll('stream', {
                page: { number: 1, size: 500 }
              }).then(function (res) {
                res = res.data;
                that.streams = res;
                __WEBPACK_IMPORTED_MODULE_8__store__["a" /* default */].commit("SET_STREAMS", res);
                console.log("Get all streams result", res);

                var total = res.length;
                for (var t = 0; t < total; t++) {
                  (function (typename) {
                    that.modelLoader(typename, function (model) {
                      console.log("Loaded stream model", typename, model);
                    });
                    __WEBPACK_IMPORTED_MODULE_4__jsonapi__["a" /* default */].define(typename, that.GetJsonApiModel(model.ColumnModel));
                  })(res[t].stream_name);
                }
              });
            });
          });
        });
      });
    });

    return promise;
  };
};

var worldmanager = new WorldManager();

/* harmony default export */ __webpack_exports__["a"] = (worldmanager);

/***/ }),
/* 9 */,
/* 10 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_json_stringify__ = __webpack_require__(21);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_json_stringify___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_json_stringify__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_promise__ = __webpack_require__(52);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_promise___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_promise__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_axios__ = __webpack_require__(31);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_axios___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_2_axios__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__appconfig__ = __webpack_require__(51);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4__utils_auth__ = __webpack_require__(18);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_5_element_ui__ = __webpack_require__(7);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_5_element_ui___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_5_element_ui__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_6_jwt_decode__ = __webpack_require__(174);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_6_jwt_decode___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_6_jwt_decode__);








var ActionManager = function ActionManager() {

  var that = this;
  that.actionMap = {};

  this.setActions = function (typeName, actions) {
    that.actionMap[typeName] = actions;
  };

  this.base64ToArrayBuffer = function (base64) {
    var binaryString = window.atob(base64);
    var binaryLen = binaryString.length;
    var bytes = new Uint8Array(binaryLen);
    for (var i = 0; i < binaryLen; i++) {
      var ascii = binaryString.charCodeAt(i);
      bytes[i] = ascii;
    }
    return bytes;
  };

  setTimeout(function () {
    that.a = document.createElement("a");
    document.body.appendChild(that.a);
    that.a.style = "display: none";
    return function (downloadData) {
      var blob = new Blob([atob(downloadData.content)], { type: downloadData.contentType }),
          url = window.URL.createObjectURL(blob);
      that.a.href = url;
      that.a.download = downloadData.name;
      that.a.click();
      window.URL.revokeObjectURL(url);
    };
  });

  this.saveByteArray = function (downloadData) {
    var blob = new Blob([atob(downloadData.content)], { type: downloadData.contentType }),
        url = window.URL.createObjectURL(blob);
    that.a.href = url;
    that.a.download = downloadData.name;
    that.a.click();
    window.URL.revokeObjectURL(url);
  };

  this.getGuestActions = function () {
    return new __WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_promise___default.a(function (resolve, reject) {
      __WEBPACK_IMPORTED_MODULE_2_axios___default()({
        url: __WEBPACK_IMPORTED_MODULE_3__appconfig__["a" /* default */].apiRoot + "/actions",
        method: "GET"
      }).then(function (respo) {
        console.log("Guest actions list: ", respo);
        resolve(respo.data);
      }, function (rs) {
        console.log("get actions list fetch failed", arguments);
        reject(rs);
      });
    });
  };

  this.doAction = function (type, actionName, data) {
    var that = this;
    return new __WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_promise___default.a(function (resolve, reject) {
      __WEBPACK_IMPORTED_MODULE_2_axios___default()({
        url: __WEBPACK_IMPORTED_MODULE_3__appconfig__["a" /* default */].apiRoot + "/action/" + type + "/" + actionName,
        method: "POST",
        headers: {
          "Authorization": "Bearer " + __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_4__utils_auth__["d" /* getToken */])()
        },
        data: {
          attributes: data
        }
      }).then(function (res) {
        resolve("completed");
        console.log("action response", res);
        var responses = res.data;
        if (responses && responses.length > 0) {
          for (var i = 0; i < responses.length; i++) {
            var responseType = responses[i].ResponseType;

            var data = responses[i].Attributes;
            switch (responseType) {
              case "client.notify":
                console.log("notify client", data);
                __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_5_element_ui__["Notification"])(data);
                break;
              case "client.store.set":
                console.log("notify client", data);
                window.localStorage.setItem(data.key, data.value);
                if (data.key == "token") {
                  window.localStorage.setItem('user', __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_json_stringify___default()(__WEBPACK_IMPORTED_MODULE_6_jwt_decode___default()(data.value)));
                }
                break;
              case "client.file.download":
                that.saveByteArray(data);
                break;
              case "client.redirect":
                (function (redirectAttrs) {

                  if (redirectAttrs.delay > 1500) {
                    __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_5_element_ui__["Notification"])({
                      title: "Redirecting",
                      type: 'success',
                      message: "In " + redirectAttrs.delay / 1000 + " seconds"
                    });
                  } else {
                    __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_5_element_ui__["Notification"])({
                      title: "Redirecting",
                      type: 'success',
                      message: "In a second"
                    });
                  }
                  setTimeout(function () {

                    var target = redirectAttrs["window"];

                    if (target == "self") {

                      if (redirectAttrs.location[0] == '/') {
                        window.location.replace(redirectAttrs.location);
                      } else {
                        window.location.replace(redirectAttrs.location);
                      }
                    } else {
                      window.open(redirectAttrs.location, "_target");
                    }
                  }, redirectAttrs.delay);
                })(data);
                break;

            }
          }
        } else {
          __WEBPACK_IMPORTED_MODULE_5_element_ui__["Notification"].success("Action " + actionName + " finished.");
        }
      }, function (res) {
        console.log("action failed", res);
        reject("Failed");
        if (res.response.data.Message) {
          __WEBPACK_IMPORTED_MODULE_5_element_ui__["Notification"].error(res.response.data.Message);
        } else {
          __WEBPACK_IMPORTED_MODULE_5_element_ui__["Notification"].error("I failed to " + window.titleCase(actionName));
        }
      });
    });
  };

  this.addAllActions = function (actions) {

    for (var i = 0; i < actions.length; i++) {
      var action = actions[i];
      var onType = action["OnType"];

      if (!that.actionMap[onType]) {
        that.actionMap[onType] = {};
      }

      that.actionMap[onType][action["Name"]] = action;
    }
  };

  this.getActions = function (typeName) {
    return that.actionMap[typeName];
  };

  this.getActionModel = function (typeName, actionName) {
    return that.actionMap[typeName][actionName];
  };

  return this;
};

var actionmanager = new ActionManager();
/* harmony default export */ __webpack_exports__["a"] = (actionmanager);

/***/ }),
/* 11 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_devour_client__ = __webpack_require__(384);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_devour_client___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_devour_client__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_element_ui__ = __webpack_require__(7);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_element_ui___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_element_ui__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__plugins_appconfig__ = __webpack_require__(51);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__utils_auth__ = __webpack_require__(18);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4_vuex__ = __webpack_require__(9);






var jsonapi = new __WEBPACK_IMPORTED_MODULE_0_devour_client___default.a({
  apiUrl: __WEBPACK_IMPORTED_MODULE_2__plugins_appconfig__["a" /* default */].apiRoot + '/api',
  pluralize: false,
  logger: false
});

jsonapi.replaceMiddleware('errors', {
  name: 'nothing-to-see-here',
  error: function error(response) {
    console.log("errors", response);
    response = response.response.data.errors[0];
    if (response.status === 401) {
      __WEBPACK_IMPORTED_MODULE_1_element_ui__["Notification"].error({
        "title": "Failed",
        "message": response.data
      });
      __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_3__utils_auth__["e" /* unsetToken */])();
      return;
    }

    if (response.status == 400) {
      __WEBPACK_IMPORTED_MODULE_1_element_ui__["Notification"].error({
        "title": "Failed",
        "message": response.title
      });
      return {};
    }

    if (response.data && !response.data.errors) {
      __WEBPACK_IMPORTED_MODULE_1_element_ui__["Notification"].error({
        "title": "Warn",
        "message": "Massive"
      });
      console.log("we dont know about this entity");
      return {};
    }

    for (var i = 0; i < response.data.errors.length; i++) {
      __WEBPACK_IMPORTED_MODULE_1_element_ui__["Notification"].error({
        "title": "Failed",
        "message": response.data.errors[i].title
      });
    }
    return { errors: [] };
  }
});

jsonapi.insertMiddlewareBefore("HEADER", {
  name: "Auth Header middleware",
  req: function req(_req) {
    jsonapi.headers['Authorization'] = 'Bearer ' + __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_3__utils_auth__["d" /* getToken */])();
    return _req;
  }
});

jsonapi.insertMiddlewareBefore('HEADER', {
  name: 'insert-query',
  req: function req(payload) {

    if (payload.req.method.toLowerCase() != "get") {
      return payload;
    }

    var query = $("#navbar-search-input").val();

    if (query && query.length > 2) {
      console.log("change payload for query", query);
      payload.req.params.filter = encodeURIComponent(query);
    }

    return payload;
  }
});
jsonapi.insertMiddlewareAfter('response', {
  name: 'track-request',
  req: function req(payload) {
    console.log("request initiate", payload);
    var requestMethod = payload.config.method.toUpperCase();
    if (requestMethod !== 'GET' && requestMethod !== 'OPTIONS') {
      if (parseInt(payload.status / 100) === 2) {
        var action = "Created ";

        if (requestMethod === "DELETE") {
          action = "Deleted ";
        } else if (requestMethod === "PUT" || requestMethod === "PATCH") {
          action = "Updated ";
        }

        __WEBPACK_IMPORTED_MODULE_1_element_ui__["Notification"].success({
          title: action + payload.config.model
        });
        console.log("return payload from response middleware");
      } else {
        __WEBPACK_IMPORTED_MODULE_1_element_ui__["Notification"].warn({
          "title": "Unidentified status"
        });
      }
    }
    return payload;
  },
  res: function res(r) {
    return r;
  }
});

jsonapi.insertMiddlewareAfter('response', {
  name: 'success-notification',
  res: function res(payload) {
    return payload;
  }
});

/* harmony default export */ __webpack_exports__["a"] = (jsonapi);

/***/ }),
/* 12 */,
/* 13 */,
/* 14 */,
/* 15 */,
/* 16 */,
/* 17 */,
/* 18 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* WEBPACK VAR INJECTION */(function(process) {/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "a", function() { return extractInfoFromHash; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "c", function() { return setToken; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "d", function() { return getToken; });
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "e", function() { return unsetToken; });
/* unused harmony export getUserFromCookie */
/* unused harmony export getUserFromLocalStorage */
/* unused harmony export setSecret */
/* harmony export (binding) */ __webpack_require__.d(__webpack_exports__, "b", function() { return checkSecret; });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_json_stringify__ = __webpack_require__(21);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_json_stringify___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_json_stringify__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_jwt_decode__ = __webpack_require__(174);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_jwt_decode___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_jwt_decode__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_js_cookie__ = __webpack_require__(459);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_js_cookie___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_2_js_cookie__);




var getQueryParams = function getQueryParams() {
  var params = {};
  window.location.href.replace(/([^(?|#)=&]+)(=([^&]*))?/g, function ($0, $1, $2, $3) {
    params[$1] = $3;
  });
  return params;
};

var extractInfoFromHash = function extractInfoFromHash() {
  if (process.SERVER_BUILD) return;

  var _getQueryParams = getQueryParams(),
      id_token = _getQueryParams.id_token,
      state = _getQueryParams.state,
      code = _getQueryParams.code;

  return {
    code: code,
    token: id_token,
    secret: state
  };
};

var setToken = function setToken(token) {
  window.localStorage.setItem('token', token);
  window.localStorage.setItem('user', __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_json_stringify___default()(__WEBPACK_IMPORTED_MODULE_1_jwt_decode___default()(token)));
  __WEBPACK_IMPORTED_MODULE_2_js_cookie___default.a.set('jwt', token);
};

var getToken = function getToken() {
  return window.localStorage.getItem('token');
};

var unsetToken = function unsetToken() {
  window.localStorage.removeItem('token');
  window.localStorage.removeItem('user');
  window.localStorage.removeItem('secret');
  __WEBPACK_IMPORTED_MODULE_2_js_cookie___default.a.remove('jwt');
  window.localStorage.setItem('logout', Date.now());
};

var getUserFromCookie = function getUserFromCookie(req) {
  if (!req.headers.cookie) return;
  var jwtCookie = req.headers.cookie.split(';').find(function (c) {
    return c.trim().startsWith('jwt=');
  });
  if (!jwtCookie) return;
  var jwt = jwtCookie.split('=')[1];
  return __WEBPACK_IMPORTED_MODULE_1_jwt_decode___default()(jwt);
};

var getUserFromLocalStorage = function getUserFromLocalStorage() {
  var json = window.localStorage.user;
  return json ? JSON.parse(json) : undefined;
};

var setSecret = function setSecret(secret) {
  console.log("set secret", secret);
  window.localStorage.setItem('secret', secret);
};

var checkSecret = function checkSecret(secret) {
  console.log("check ssecret", window.localStorage.secret, secret);
  return window.localStorage.secret === secret;
};
/* WEBPACK VAR INJECTION */}.call(__webpack_exports__, __webpack_require__(60)))

/***/ }),
/* 19 */,
/* 20 */,
/* 21 */,
/* 22 */,
/* 23 */,
/* 24 */,
/* 25 */,
/* 26 */,
/* 27 */,
/* 28 */,
/* 29 */,
/* 30 */,
/* 31 */,
/* 32 */,
/* 33 */,
/* 34 */,
/* 35 */,
/* 36 */,
/* 37 */,
/* 38 */,
/* 39 */,
/* 40 */,
/* 41 */,
/* 42 */,
/* 43 */,
/* 44 */,
/* 45 */,
/* 46 */,
/* 47 */,
/* 48 */,
/* 49 */,
/* 50 */,
/* 51 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
var AppConfig = function AppConfig() {

  var that = this;

  that.apiRoot = window.location.protocol + "//" + window.location.host;

  that.location = {
    protocol: window.location.protocol,
    host: window.location.host,
    hostname: window.location.hostname
  };

  if (that.location.hostname == "site.daptin.com") {
    that.apiRoot = that.location.protocol + "//api.daptin.com:6336";
  }

  var that1 = this;

  that1.data = {};

  that.localStorage = {
    getItem: function getItem(key) {
      return that1.data[key];
    },
    setItem: function setItem(key, item) {
      that1.data[key] = item;
    }
  };

  return that;
};

var appconfig = new AppConfig();

/* harmony default export */ __webpack_exports__["a"] = (appconfig);

/***/ }),
/* 52 */,
/* 53 */,
/* 54 */,
/* 55 */,
/* 56 */,
/* 57 */,
/* 58 */,
/* 59 */,
/* 60 */,
/* 61 */,
/* 62 */,
/* 63 */,
/* 64 */,
/* 65 */,
/* 66 */,
/* 67 */,
/* 68 */,
/* 69 */,
/* 70 */,
/* 71 */,
/* 72 */,
/* 73 */,
/* 74 */,
/* 75 */,
/* 76 */,
/* 77 */,
/* 78 */,
/* 79 */,
/* 80 */,
/* 81 */,
/* 82 */,
/* 83 */,
/* 84 */,
/* 85 */,
/* 86 */,
/* 87 */,
/* 88 */,
/* 89 */,
/* 90 */,
/* 91 */,
/* 92 */,
/* 93 */,
/* 94 */,
/* 95 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_promise__ = __webpack_require__(52);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_promise___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_promise__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__appconfig__ = __webpack_require__(51);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_axios__ = __webpack_require__(31);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_axios___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_2_axios__);






var ConfigManager = function ConfigManager() {

  this.getAllConfig = function () {
    var p = new __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_promise___default.a(function (resolve, reject) {
      __WEBPACK_IMPORTED_MODULE_2_axios___default()({
        url: __WEBPACK_IMPORTED_MODULE_1__appconfig__["a" /* default */].apiRoot + "/config",
        method: "GET"
      }).then(function (r) {
        resolve(r.data);
      }, function (r) {
        reject(r);
      });
    });
    return p;
  };

  return this;
};

/* unused harmony default export */ var _unused_webpack_default_export = (new ConfigManager());

/***/ }),
/* 96 */,
/* 97 */,
/* 98 */,
/* 99 */,
/* 100 */,
/* 101 */,
/* 102 */,
/* 103 */,
/* 104 */,
/* 105 */,
/* 106 */,
/* 107 */,
/* 108 */,
/* 109 */,
/* 110 */,
/* 111 */,
/* 112 */,
/* 113 */,
/* 114 */,
/* 115 */,
/* 116 */,
/* 117 */,
/* 118 */,
/* 119 */,
/* 120 */,
/* 121 */,
/* 122 */,
/* 123 */,
/* 124 */,
/* 125 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_vue__ = __webpack_require__(12);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_vuex__ = __webpack_require__(9);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__state__ = __webpack_require__(291);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__actions__ = __webpack_require__(288);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4__mutations__ = __webpack_require__(290);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_5__getters__ = __webpack_require__(289);







__WEBPACK_IMPORTED_MODULE_0_vue__["default"].use(__WEBPACK_IMPORTED_MODULE_1_vuex__["a" /* default */]);

/* harmony default export */ __webpack_exports__["a"] = (new __WEBPACK_IMPORTED_MODULE_1_vuex__["a" /* default */].Store({
  state: __WEBPACK_IMPORTED_MODULE_2__state__["a" /* default */],
  mutations: __WEBPACK_IMPORTED_MODULE_4__mutations__["a" /* default */],
  getters: __WEBPACK_IMPORTED_MODULE_5__getters__["a" /* default */],
  actions: __WEBPACK_IMPORTED_MODULE_3__actions__["a" /* default */]
}));

/***/ }),
/* 126 */,
/* 127 */,
/* 128 */,
/* 129 */,
/* 130 */,
/* 131 */,
/* 132 */,
/* 133 */,
/* 134 */,
/* 135 */,
/* 136 */,
/* 137 */,
/* 138 */,
/* 139 */,
/* 140 */,
/* 141 */,
/* 142 */,
/* 143 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys__ = __webpack_require__(5);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_axios__ = __webpack_require__(31);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_axios___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_axios__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__appconfig__ = __webpack_require__(51);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__utils_auth__ = __webpack_require__(18);





var StatsManager = function StatsManager() {
  var that = this;

  that.queryToParams = function (statsRequest) {

    var keys = __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default()(statsRequest);
    var list = [];

    for (var i = 0; i < keys.length; i++) {

      var key = keys[i];
      var values = statsRequest[key];

      if (!(values instanceof Array)) {
        values = [values];
      }

      for (var j = 0; j < values.length; j++) {
        list.push(encodeURIComponent(key) + "=" + encodeURIComponent(values));
      }
    }

    return "?" + list.join("&");
  };

  that.getStats = function (tableName, statsRequest) {

    console.log("create stats request", tableName, statsRequest);
    return __WEBPACK_IMPORTED_MODULE_1_axios___default()({
      url: __WEBPACK_IMPORTED_MODULE_2__appconfig__["a" /* default */].apiRoot + "/stats/" + tableName + that.queryToParams(statsRequest),
      headers: {
        "Authorization": "Bearer " + __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_3__utils_auth__["d" /* getToken */])()
      }
    });
  };
};

var statsManager = new StatsManager();

/* harmony default export */ __webpack_exports__["a"] = (statsManager);

/***/ }),
/* 144 */,
/* 145 */,
/* 146 */,
/* 147 */,
/* 148 */,
/* 149 */,
/* 150 */,
/* 151 */,
/* 152 */,
/* 153 */,
/* 154 */,
/* 155 */,
/* 156 */,
/* 157 */,
/* 158 */,
/* 159 */,
/* 160 */,
/* 161 */,
/* 162 */,
/* 163 */,
/* 164 */,
/* 165 */,
/* 166 */,
/* 167 */,
/* 168 */,
/* 169 */,
/* 170 */,
/* 171 */,
/* 172 */,
/* 173 */,
/* 174 */,
/* 175 */,
/* 176 */,
/* 177 */,
/* 178 */,
/* 179 */,
/* 180 */,
/* 181 */,
/* 182 */,
/* 183 */,
/* 184 */,
/* 185 */,
/* 186 */,
/* 187 */,
/* 188 */,
/* 189 */,
/* 190 */,
/* 191 */,
/* 192 */,
/* 193 */,
/* 194 */,
/* 195 */,
/* 196 */,
/* 197 */,
/* 198 */
/***/ (function(module, exports, __webpack_require__) {


/* styles */
__webpack_require__(442)

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(326),
  /* template */
  __webpack_require__(647),
  /* scopeId */
  "data-v-10530376",
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 199 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(328),
  /* template */
  __webpack_require__(644),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 200 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(332),
  /* template */
  null,
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 201 */,
/* 202 */,
/* 203 */,
/* 204 */,
/* 205 */
/***/ (function(module, exports) {

module.exports = {"AUTH0_CLIENT_ID":"edsjFX3nR9fqqpUi4kRXkaKJefzfRaf_","AUTH0_CLIENT_DOMAIN":"daptin.auth0.com"}

/***/ }),
/* 206 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_promise__ = __webpack_require__(52);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_promise___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_promise__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_axios__ = __webpack_require__(31);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_axios___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_axios__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_element_ui__ = __webpack_require__(7);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_element_ui___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_2_element_ui__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__utils_auth__ = __webpack_require__(18);







__WEBPACK_IMPORTED_MODULE_1_axios___default.a.interceptors.response.use(function (response) {
  return response;
}, function (error) {
  if (error.response && error.response.status == 403) {
    __WEBPACK_IMPORTED_MODULE_2_element_ui__["Notification"].error({
      "title": "Unauthorized",
      "message": error.message
    });
  } else if (error.response && error.response.status == 401) {
    __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_3__utils_auth__["e" /* unsetToken */])();
    __WEBPACK_IMPORTED_MODULE_2_element_ui__["Notification"].error({
      "title": "Unauthorized",
      "message": error.message
    });
  }
  return __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_promise___default.a.reject(error);
});

/***/ }),
/* 207 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_vue__ = __webpack_require__(12);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_element_ui__ = __webpack_require__(7);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_element_ui___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_element_ui__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__components_vuetable__ = __webpack_require__(286);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__components_daptable_DaptableView_vue__ = __webpack_require__(624);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__components_daptable_DaptableView_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_3__components_daptable_DaptableView_vue__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4__components_vuetable_components_Vuecard_vue__ = __webpack_require__(198);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4__components_vuetable_components_Vuecard_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_4__components_vuetable_components_Vuecard_vue__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_5__components_detailrow_DetailedRow_vue__ = __webpack_require__(626);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_5__components_detailrow_DetailedRow_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_5__components_detailrow_DetailedRow_vue__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_6__components_modelform_ModelForm_vue__ = __webpack_require__(633);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_6__components_modelform_ModelForm_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_6__components_modelform_ModelForm_vue__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_7__components_vuetable_components_VuetablePagination_vue__ = __webpack_require__(199);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_7__components_vuetable_components_VuetablePagination_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_7__components_vuetable_components_VuetablePagination_vue__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_8__components_detailrow_CustomActions_vue__ = __webpack_require__(625);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_8__components_detailrow_CustomActions_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_8__components_detailrow_CustomActions_vue__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_9__components_tableview_TableView_vue__ = __webpack_require__(636);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_9__components_tableview_TableView_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_9__components_tableview_TableView_vue__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_10__components_selectoneormore_SelectOneOrMore_vue__ = __webpack_require__(635);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_10__components_selectoneormore_SelectOneOrMore_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_10__components_selectoneormore_SelectOneOrMore_vue__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_11__components_listview_ListView_vue__ = __webpack_require__(632);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_11__components_listview_ListView_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_11__components_listview_ListView_vue__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_12__components_actionview_ActionView_vue__ = __webpack_require__(623);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_12__components_actionview_ActionView_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_12__components_actionview_ActionView_vue__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_13__components_reclineview_ReclineView_vue__ = __webpack_require__(634);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_13__components_reclineview_ReclineView_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_13__components_reclineview_ReclineView_vue__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_14_element_ui_lib_locale_lang_en__ = __webpack_require__(419);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_14_element_ui_lib_locale_lang_en___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_14_element_ui_lib_locale_lang_en__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_15_element_ui_lib_theme_default_index_css__ = __webpack_require__(433);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_15_element_ui_lib_theme_default_index_css___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_15_element_ui_lib_theme_default_index_css__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_16_tether_shepherd_dist_css_shepherd_theme_dark_css__ = __webpack_require__(439);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_16_tether_shepherd_dist_css_shepherd_theme_dark_css___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_16_tether_shepherd_dist_css_shepherd_theme_dark_css__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_17__components_vuetable_vuetable_css__ = __webpack_require__(440);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_17__components_vuetable_vuetable_css___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_17__components_vuetable_vuetable_css__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_18__components_fields_FileUpload_vue__ = __webpack_require__(630);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_18__components_fields_FileUpload_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_18__components_fields_FileUpload_vue__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_19__components_fields_PermissionField_vue__ = __webpack_require__(631);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_19__components_fields_PermissionField_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_19__components_fields_PermissionField_vue__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_20__components_fields_FileJsonEditor_vue__ = __webpack_require__(629);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_20__components_fields_FileJsonEditor_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_20__components_fields_FileJsonEditor_vue__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_21__components_fields_FancyCheckBox_vue__ = __webpack_require__(628);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_21__components_fields_FancyCheckBox_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_21__components_fields_FancyCheckBox_vue__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_22__components_fields_DateSelect_vue__ = __webpack_require__(627);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_22__components_fields_DateSelect_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_22__components_fields_DateSelect_vue__);




























__WEBPACK_IMPORTED_MODULE_0_vue__["default"].component("fieldFileUpload", __WEBPACK_IMPORTED_MODULE_18__components_fields_FileUpload_vue___default.a);
__WEBPACK_IMPORTED_MODULE_0_vue__["default"].component('fieldPermissionInput', __WEBPACK_IMPORTED_MODULE_19__components_fields_PermissionField_vue___default.a);
__WEBPACK_IMPORTED_MODULE_0_vue__["default"].component("fieldSelectOneOrMore", __WEBPACK_IMPORTED_MODULE_10__components_selectoneormore_SelectOneOrMore_vue___default.a);
__WEBPACK_IMPORTED_MODULE_0_vue__["default"].component("fieldDateSelect", __WEBPACK_IMPORTED_MODULE_22__components_fields_DateSelect_vue___default.a);
__WEBPACK_IMPORTED_MODULE_0_vue__["default"].component("fieldJsonEditor", __WEBPACK_IMPORTED_MODULE_20__components_fields_FileJsonEditor_vue___default.a);
__WEBPACK_IMPORTED_MODULE_0_vue__["default"].component("fieldFancyCheckBox", __WEBPACK_IMPORTED_MODULE_21__components_fields_FancyCheckBox_vue___default.a);

__WEBPACK_IMPORTED_MODULE_0_vue__["default"].use(__WEBPACK_IMPORTED_MODULE_1_element_ui___default.a, { locale: __WEBPACK_IMPORTED_MODULE_14_element_ui_lib_locale_lang_en___default.a });
__WEBPACK_IMPORTED_MODULE_0_vue__["default"].use(__WEBPACK_IMPORTED_MODULE_2__components_vuetable__["a" /* default */]);
__WEBPACK_IMPORTED_MODULE_0_vue__["default"].use(__WEBPACK_IMPORTED_MODULE_3__components_daptable_DaptableView_vue___default.a);
__WEBPACK_IMPORTED_MODULE_0_vue__["default"].use(__WEBPACK_IMPORTED_MODULE_4__components_vuetable_components_Vuecard_vue___default.a);
__WEBPACK_IMPORTED_MODULE_0_vue__["default"].use(__WEBPACK_IMPORTED_MODULE_7__components_vuetable_components_VuetablePagination_vue___default.a);
__WEBPACK_IMPORTED_MODULE_0_vue__["default"].use(__WEBPACK_IMPORTED_MODULE_5__components_detailrow_DetailedRow_vue___default.a);

__WEBPACK_IMPORTED_MODULE_0_vue__["default"].component('custom-actions', __WEBPACK_IMPORTED_MODULE_8__components_detailrow_CustomActions_vue___default.a);
__WEBPACK_IMPORTED_MODULE_0_vue__["default"].component('table-view', __WEBPACK_IMPORTED_MODULE_9__components_tableview_TableView_vue___default.a);

__WEBPACK_IMPORTED_MODULE_0_vue__["default"].component('recline-view', __WEBPACK_IMPORTED_MODULE_13__components_reclineview_ReclineView_vue___default.a);
__WEBPACK_IMPORTED_MODULE_0_vue__["default"].component('action-view', __WEBPACK_IMPORTED_MODULE_12__components_actionview_ActionView_vue___default.a);
__WEBPACK_IMPORTED_MODULE_0_vue__["default"].component('list-view', __WEBPACK_IMPORTED_MODULE_11__components_listview_ListView_vue___default.a);
__WEBPACK_IMPORTED_MODULE_0_vue__["default"].component('model-form', __WEBPACK_IMPORTED_MODULE_6__components_modelform_ModelForm_vue___default.a);
__WEBPACK_IMPORTED_MODULE_0_vue__["default"].component("vuetable", __WEBPACK_IMPORTED_MODULE_2__components_vuetable__["a" /* default */]);
__WEBPACK_IMPORTED_MODULE_0_vue__["default"].component("daptable", __WEBPACK_IMPORTED_MODULE_3__components_daptable_DaptableView_vue___default.a);
__WEBPACK_IMPORTED_MODULE_0_vue__["default"].component("vuecard", __WEBPACK_IMPORTED_MODULE_4__components_vuetable_components_Vuecard_vue___default.a);
__WEBPACK_IMPORTED_MODULE_0_vue__["default"].component("select-one-or-more", __WEBPACK_IMPORTED_MODULE_10__components_selectoneormore_SelectOneOrMore_vue___default.a);
__WEBPACK_IMPORTED_MODULE_0_vue__["default"].component("detailed-table-row", __WEBPACK_IMPORTED_MODULE_5__components_detailrow_DetailedRow_vue___default.a);
__WEBPACK_IMPORTED_MODULE_0_vue__["default"].component("vuetable-pagination", __WEBPACK_IMPORTED_MODULE_7__components_vuetable_components_VuetablePagination_vue___default.a);

/***/ }),
/* 208 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__components_Dash_vue__ = __webpack_require__(609);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__components_Dash_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0__components_Dash_vue__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__components_Login_vue__ = __webpack_require__(613);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__components_Login_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1__components_Login_vue__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__components_404_vue__ = __webpack_require__(606);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__components_404_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_2__components_404_vue__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__components_InstanceView__ = __webpack_require__(612);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__components_InstanceView___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_3__components_InstanceView__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4__components_EntityView__ = __webpack_require__(610);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4__components_EntityView___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_4__components_EntityView__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_5__components_NewItem__ = __webpack_require__(614);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_5__components_NewItem___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_5__components_NewItem__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_6__components_RelationView__ = __webpack_require__(616);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_6__components_RelationView___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_6__components_RelationView__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_7__components_Admin__ = __webpack_require__(608);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_7__components_Admin___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_7__components_Admin__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_8__components_SignIn__ = __webpack_require__(619);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_8__components_SignIn___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_8__components_SignIn__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_9__components_SignedIn__ = __webpack_require__(622);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_9__components_SignedIn___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_9__components_SignedIn__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_10__components_SignOut__ = __webpack_require__(620);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_10__components_SignOut___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_10__components_SignOut__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_11__components_OauthResponse__ = __webpack_require__(615);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_11__components_OauthResponse___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_11__components_OauthResponse__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_12__components_SignUp__ = __webpack_require__(621);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_12__components_SignUp___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_12__components_SignUp__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_13__components_Action__ = __webpack_require__(607);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_13__components_Action___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_13__components_Action__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_14__components_Home__ = __webpack_require__(611);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_14__components_Home___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_14__components_Home__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_15__components_views_Dashboard_vue__ = __webpack_require__(638);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_15__components_views_Dashboard_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_15__components_views_Dashboard_vue__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_16__components_views_AllInOne_vue__ = __webpack_require__(637);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_16__components_views_AllInOne_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_16__components_views_AllInOne_vue__);






















var routes = [{
  name: 'SignIn',
  path: '/auth/signin',
  component: __WEBPACK_IMPORTED_MODULE_8__components_SignIn___default.a
}, {
  name: 'SignedIn',
  path: '/auth/signedin',
  component: __WEBPACK_IMPORTED_MODULE_9__components_SignedIn___default.a
}, {
  name: 'SignUp',
  path: '/auth/signup',
  component: __WEBPACK_IMPORTED_MODULE_12__components_SignUp___default.a
}, {
  name: 'SignOut',
  path: '/auth/signout',
  component: __WEBPACK_IMPORTED_MODULE_10__components_SignOut___default.a
}, {
  name: "OauthResponse",
  path: '/oauth/response',
  component: __WEBPACK_IMPORTED_MODULE_11__components_OauthResponse___default.a
}, {
  path: '/',
  component: __WEBPACK_IMPORTED_MODULE_0__components_Dash_vue___default.a,
  children: [{
    path: '',
    name: 'Dashboard',
    component: __WEBPACK_IMPORTED_MODULE_15__components_views_Dashboard_vue___default.a
  }, {
    path: '/all',
    name: 'AllInOne',
    component: __WEBPACK_IMPORTED_MODULE_16__components_views_AllInOne_vue___default.a
  }, {
    path: '/act/:tablename/:actionname',
    name: 'Action',
    component: __WEBPACK_IMPORTED_MODULE_13__components_Action___default.a
  }, {
    path: '/',
    component: __WEBPACK_IMPORTED_MODULE_7__components_Admin___default.a,
    children: [{
      path: '/in/item/:tablename',
      name: 'Entity',
      component: __WEBPACK_IMPORTED_MODULE_4__components_EntityView___default.a
    }, {
      path: '/in/item/:tablename/new',
      name: 'NewEntity',
      component: __WEBPACK_IMPORTED_MODULE_4__components_EntityView___default.a
    }, {
      path: '/in/item/:tablename/:refId',
      name: 'Instance',
      component: __WEBPACK_IMPORTED_MODULE_3__components_InstanceView___default.a
    }, {
      path: '/in/meta/new',
      name: 'NewItem',
      component: __WEBPACK_IMPORTED_MODULE_5__components_NewItem___default.a
    }, {
      path: '/in/item/:tablename/:refId/:subTable',
      name: 'Relation',
      component: __WEBPACK_IMPORTED_MODULE_6__components_RelationView___default.a
    }]
  }]
}, {
  path: '*',
  component: __WEBPACK_IMPORTED_MODULE_2__components_404_vue___default.a
}];

/* harmony default export */ __webpack_exports__["a"] = (routes);

/***/ }),
/* 209 */,
/* 210 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(295),
  /* template */
  __webpack_require__(648),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 211 */,
/* 212 */,
/* 213 */,
/* 214 */,
/* 215 */,
/* 216 */,
/* 217 */,
/* 218 */,
/* 219 */,
/* 220 */,
/* 221 */,
/* 222 */,
/* 223 */,
/* 224 */,
/* 225 */,
/* 226 */,
/* 227 */,
/* 228 */,
/* 229 */,
/* 230 */,
/* 231 */,
/* 232 */,
/* 233 */,
/* 234 */,
/* 235 */,
/* 236 */,
/* 237 */,
/* 238 */,
/* 239 */,
/* 240 */,
/* 241 */,
/* 242 */,
/* 243 */,
/* 244 */,
/* 245 */,
/* 246 */,
/* 247 */,
/* 248 */,
/* 249 */,
/* 250 */,
/* 251 */,
/* 252 */,
/* 253 */,
/* 254 */,
/* 255 */,
/* 256 */,
/* 257 */,
/* 258 */,
/* 259 */,
/* 260 */,
/* 261 */,
/* 262 */,
/* 263 */,
/* 264 */,
/* 265 */,
/* 266 */,
/* 267 */,
/* 268 */,
/* 269 */,
/* 270 */,
/* 271 */,
/* 272 */,
/* 273 */,
/* 274 */,
/* 275 */,
/* 276 */,
/* 277 */,
/* 278 */,
/* 279 */,
/* 280 */,
/* 281 */,
/* 282 */,
/* 283 */,
/* 284 */,
/* 285 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_axios__ = __webpack_require__(31);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_axios___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_axios__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__config__ = __webpack_require__(205);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__config___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1__config__);



/* harmony default export */ __webpack_exports__["a"] = ({
  request: function request(method, uri) {
    var data = arguments.length > 2 && arguments[2] !== undefined ? arguments[2] : null;

    if (!method) {
      console.error('API function call requires method argument');
      return;
    }

    if (!uri) {
      console.error('API function call requires uri argument');
      return;
    }

    var url = __WEBPACK_IMPORTED_MODULE_1__config___default.a.serverURI + uri;
    return __WEBPACK_IMPORTED_MODULE_0_axios___default()({ method: method, url: url, data: data });
  }
});

/***/ }),
/* 286 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* unused harmony export install */
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__components_Vuetable_vue__ = __webpack_require__(639);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__components_Vuetable_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0__components_Vuetable_vue__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__components_Vuecard_vue__ = __webpack_require__(198);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__components_Vuecard_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1__components_Vuecard_vue__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__components_VuetablePagination_vue__ = __webpack_require__(199);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__components_VuetablePagination_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_2__components_VuetablePagination_vue__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__components_VuetablePaginationDropdown_vue__ = __webpack_require__(640);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__components_VuetablePaginationDropdown_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_3__components_VuetablePaginationDropdown_vue__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4__components_VuetablePaginationInfo_vue__ = __webpack_require__(641);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4__components_VuetablePaginationInfo_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_4__components_VuetablePaginationInfo_vue__);
/* unused harmony reexport Vuetable */
/* unused harmony reexport Vuecard */
/* unused harmony reexport VuetablePagination */
/* unused harmony reexport VuetablePaginationDropDown */
/* unused harmony reexport VuetablePaginationInfo */






function install(Vue) {
  Vue.component("vuetable", __WEBPACK_IMPORTED_MODULE_0__components_Vuetable_vue___default.a);
  Vue.component("vuecard", __WEBPACK_IMPORTED_MODULE_1__components_Vuecard_vue___default.a);
  Vue.component("vuetable-pagination", __WEBPACK_IMPORTED_MODULE_2__components_VuetablePagination_vue___default.a);
  Vue.component("vuetable-pagination-dropdown", __WEBPACK_IMPORTED_MODULE_3__components_VuetablePaginationDropdown_vue___default.a);
  Vue.component("vuetable-pagination-info", __WEBPACK_IMPORTED_MODULE_4__components_VuetablePaginationInfo_vue___default.a);
}


/* harmony default export */ __webpack_exports__["a"] = (__WEBPACK_IMPORTED_MODULE_0__components_Vuetable_vue___default.a);

/***/ }),
/* 287 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys__ = __webpack_require__(5);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_vue__ = __webpack_require__(12);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_vue_router__ = __webpack_require__(211);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3_vuex_router_sync__ = __webpack_require__(212);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3_vuex_router_sync___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_3_vuex_router_sync__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4__plugins_main__ = __webpack_require__(207);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_5__plugins_worldmanager__ = __webpack_require__(8);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_6__plugins_jsonapi__ = __webpack_require__(11);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_7__plugins_actionmanager__ = __webpack_require__(10);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_8__plugins_axios__ = __webpack_require__(206);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_9_vue_filter__ = __webpack_require__(209);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_9_vue_filter___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_9_vue_filter__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_10__routes__ = __webpack_require__(208);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_11__store__ = __webpack_require__(125);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_12__components_App_vue__ = __webpack_require__(210);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_12__components_App_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_12__components_App_vue__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_13_element_ui__ = __webpack_require__(7);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_13_element_ui___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_13_element_ui__);





















__WEBPACK_IMPORTED_MODULE_1_vue__["default"].use(__WEBPACK_IMPORTED_MODULE_13_element_ui___default.a);

window.stringToColor = function (str, prc) {
  var prc = typeof prc === 'number' ? prc : -10;

  var hash = function hash(word) {
    var h = 0;
    for (var i = 0; i < word.length; i++) {
      h = word.charCodeAt(i) + ((h << 5) - h);
    }
    return h;
  };

  var shade = function shade(color, prc) {
    var num = parseInt(color, 16),
        amt = Math.round(2.55 * prc),
        R = (num >> 16) + amt,
        G = (num >> 8 & 0x00FF) + amt,
        B = (num & 0x0000FF) + amt;
    return (0x1000000 + (R < 255 ? R < 1 ? 0 : R : 255) * 0x10000 + (G < 255 ? G < 1 ? 0 : G : 255) * 0x100 + (B < 255 ? B < 1 ? 0 : B : 255)).toString(16).slice(1);
  };

  var int_to_rgba = function int_to_rgba(i) {
    var color = (i >> 24 & 0xFF).toString(16) + (i >> 16 & 0xFF).toString(16) + (i >> 8 & 0xFF).toString(16) + (i & 0xFF).toString(16);
    return color;
  };

  return shade(int_to_rgba(hash(str)), prc);
};

window.chooseTitle = function (obj) {

  if (!obj) {
    return "_";
  }


  var candidates = ["name", "model", "title", "label"];

  var objType = obj["__type"];
  if (objType) {
    var objModel = __WEBPACK_IMPORTED_MODULE_6__plugins_jsonapi__["a" /* default */].modelFor(objType);
    var attrs = objModel.attributes;
    var attrKeys = __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default()(attrs);
    for (var i = 0; i < attrKeys.length; i++) {
      if (attrs[attrKeys[i]] == "label") {
        candidates.push(attrKeys[i]);
      }
    }
  }

  var keys = __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default()(obj);


  for (var i = 0; i < candidates.length; i++) {

    var found = keys.indexOf(candidates[i]);

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
        return obj[keys[i]];
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
      return "";
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
    return str;
  }

  if (!str || str.length < 2) {
    return str;
  }
  var s = str.replace(/[-_]+/g, " ").trim().split(' ').map(function (w) {
    return (w[0] ? w[0].toUpperCase() : "") + w.substr(1).toLowerCase();
  }).join(' ');

  return s;
};

__WEBPACK_IMPORTED_MODULE_1_vue__["default"].filter('chooseTitle', chooseTitle);
__WEBPACK_IMPORTED_MODULE_1_vue__["default"].filter('titleCase', titleCase);

__WEBPACK_IMPORTED_MODULE_1_vue__["default"].use(__WEBPACK_IMPORTED_MODULE_9_vue_filter___default.a);
__WEBPACK_IMPORTED_MODULE_1_vue__["default"].use(__WEBPACK_IMPORTED_MODULE_2_vue_router__["a" /* default */]);

var router = new __WEBPACK_IMPORTED_MODULE_2_vue_router__["a" /* default */]({
  routes: __WEBPACK_IMPORTED_MODULE_10__routes__["a" /* default */],
  mode: 'history',
  scrollBehavior: function scrollBehavior(to, from, savedPosition) {
    return { x: 0, y: 0 };
  }
});

router.beforeEach(function (to, from, next) {
  if (to.auth && to.router.app.$store.state.token === 'null') {
    window.console.log('Not authenticated');
    next({
      path: '/login',
      query: { redirect: to.fullPath }
    });
  } else {
    next();
  }
});

__webpack_require__.i(__WEBPACK_IMPORTED_MODULE_3_vuex_router_sync__["sync"])(__WEBPACK_IMPORTED_MODULE_11__store__["a" /* default */], router);

window.vueApp = new __WEBPACK_IMPORTED_MODULE_1_vue__["default"]({
  el: '#root',
  router: router,
  store: __WEBPACK_IMPORTED_MODULE_11__store__["a" /* default */],
  filter: __WEBPACK_IMPORTED_MODULE_9_vue_filter___default.a,
  render: function render(h) {
    return h(__WEBPACK_IMPORTED_MODULE_12__components_App_vue___default.a);
  }
});

if (window.localStorage) {
  var localUserString = window.localStorage.getItem('user') || 'null';
  var localUser = JSON.parse(localUserString);

  if (localUser && __WEBPACK_IMPORTED_MODULE_11__store__["a" /* default */].state.user !== localUser) {
    __WEBPACK_IMPORTED_MODULE_11__store__["a" /* default */].commit('SET_USER', localUser);
    __WEBPACK_IMPORTED_MODULE_11__store__["a" /* default */].commit('SET_TOKEN', window.localStorage.getItem('token'));
  }
}

/***/ }),
/* 288 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony default export */ __webpack_exports__["a"] = ({
  setQuery: function setQuery(_ref, query) {
    var commit = _ref.commit;

    commit("SET_QUERY", query);
  },
  setStreams: function setStreams(_ref2, streams) {
    var commit = _ref2.commit;

    commit("SET_STREAMS", streams);
  }
});

/***/ }),
/* 289 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty__ = __webpack_require__(41);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_object_keys__ = __webpack_require__(5);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_object_keys___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_object_keys__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__plugins_worldmanager__ = __webpack_require__(8);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__plugins_jsonapi__ = __webpack_require__(11);



var _subTableColumns$isAu;





/* harmony default export */ __webpack_exports__["a"] = (_subTableColumns$isAu = {
  subTableColumns: function subTableColumns(state) {
    return state.subTableColumns;
  },
  isAuthenticated: function isAuthenticated(state) {
    var x = JSON.parse(window.localStorage.getItem("user"));
    console.log("Auth check", x);
    if (!x || !x.exp || new Date(x.exp * 1000) < new Date()) {
      window.localStorage.removeItem("user");
      return false;
    }
    return !!window.localStorage.getItem("token");
  },
  systemActions: function systemActions(state) {
    return state.systemActions;
  },
  authToken: function authToken(state) {
    return window.localStorage.getItem("token");
  },
  selectedAction: function selectedAction(state) {
    return state.selectedAction;
  },
  selectedInstanceReferenceId: function selectedInstanceReferenceId(state) {
    return state.selectedInstanceReferenceId;
  },
  user: function user(state) {
    var user = JSON.parse(window.localStorage.getItem("user"));
    user = user || {};
    return user;
  },
  actions: function actions(state) {
    return state.actions;
  },
  selectedTable: function selectedTable(state) {
    console.log("get selected table", state.selectedTable);
    return state.selectedTable;
  },
  finder: function finder(state) {
    return state.finder;
  },
  selectedRow: function selectedRow(state) {
    return state.selectedRow;
  },
  selectedTableColumns: function selectedTableColumns(state) {
    return state.selectedTableColumns;
  }
}, __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_subTableColumns$isAu, "selectedInstanceReferenceId", function selectedInstanceReferenceId(state) {
  return state.selectedInstanceReferenceId;
}), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_subTableColumns$isAu, "selectedSubTable", function selectedSubTable(state) {
  return state.selectedSubTable;
}), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_subTableColumns$isAu, "showAddEdit", function showAddEdit(state) {
  return state.showAddEdit;
}), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_subTableColumns$isAu, "visibleWorlds", function visibleWorlds(state) {
  var filtered = state.worlds.filter(function (w, r) {
    if (!state.selectedInstanceReferenceId) {
      return w.is_top_level == 1 && w.is_hidden == 0;
    } else {
      var model = __WEBPACK_IMPORTED_MODULE_3__plugins_jsonapi__["a" /* default */].modelFor(w.table_name);
      var attrs = model["attributes"];
      var keys = __WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_object_keys___default()(attrs);
      if (keys.indexOf(state.selectedTable + "_id") > -1) {
        return w.is_top_level == 0 && w.is_join_table == 0;
      }
      return false;
    }
  });
  console.log("filtered worlds: ", filtered);

  return filtered;
}), _subTableColumns$isAu);

/***/ }),
/* 290 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_json_stringify__ = __webpack_require__(21);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_json_stringify___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_json_stringify__);

/* harmony default export */ __webpack_exports__["a"] = ({
  TOGGLE_LOADING: function TOGGLE_LOADING(state) {
    state.callingAPI = !state.callingAPI;
  },
  TOGGLE_SEARCHING: function TOGGLE_SEARCHING(state) {
    state.searching = state.searching === '' ? 'loading' : '';
  },
  SET_USER: function SET_USER(state, user) {
    state.user = user;
  },
  SET_LAST_URL: function SET_LAST_URL(state, route) {
    if (route) {
      window.localStorage.setItem("last_route", __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_json_stringify___default()(route));
    } else {
      window.localStorage.removeItem("last_route");
    }
  },
  SET_TOKEN: function SET_TOKEN(state, token) {
    window.localStorage.setItem("token", token);
  },
  SET_ACTIONS: function SET_ACTIONS(state, actions) {
    state.actions = actions;
  },
  SET_WORLDS: function SET_WORLDS(state, worlds) {
    console.log("\t\t\tSet worlds: ", worlds);
    state.worlds = worlds;
    state.visibleWorlds = worlds;
  },
  SET_WORLD_ACTIONS: function SET_WORLD_ACTIONS(state, actions) {
    state.systemActions = actions;
  },
  SET_SELECTED_TABLE: function SET_SELECTED_TABLE(state, selectedTable) {
    console.log("SET_SELECTED_TABLE", selectedTable);
    state.selectedTable = selectedTable;
  },
  SET_STREAMS: function SET_STREAMS(state, streams) {
    state.streams = streams;
  },
  SET_SELECTED_SUB_TABLE: function SET_SELECTED_SUB_TABLE(state, selectedSubTable) {
    state.selectedSubTable = selectedSubTable;
  },
  SET_QUERY: function SET_QUERY(state, query) {
    state.query = query;
  },
  SET_SELECTED_ROW: function SET_SELECTED_ROW(state, selectedRow) {
    state.selectedRow = selectedRow;
  },
  SET_SUBTABLE_COLUMNS: function SET_SUBTABLE_COLUMNS(state, columns) {
    state.subTableColumns = columns;
  },
  SET_SELECTED_TABLE_COLUMNS: function SET_SELECTED_TABLE_COLUMNS(state, columns) {
    state.selectedTableColumns = columns;
  },
  SET_SELECTED_ACTION: function SET_SELECTED_ACTION(state, action) {
    state.selectedAction = action;
  },
  SET_FINDER: function SET_FINDER(state, finder) {
    console.log("SET_FINDER", finder);
    state.finder = finder;
  },
  SET_SELECTED_INSTANCE_REFERENCE_ID: function SET_SELECTED_INSTANCE_REFERENCE_ID(state, refId) {
    state.selectedInstanceReferenceId = refId;
  },
  LOGOUT: function LOGOUT(state) {
    window.localStorage.clear("token");
    window.localStorage.clear("user");
  }
});

/***/ }),
/* 291 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony default export */ __webpack_exports__["a"] = ({
  callingAPI: false,
  searching: '',
  serverURI: 'http://10.110.1.136:8080',
  user: null,
  token: null,
  streams: [],
  query: null,
  userInfo: {
    messages: [{ 1: 'test', 2: 'test' }],
    notifications: [],
    tasks: []
  },

  selectedTable: null,
  authToken: null,
  route: null,
  authUser: null,
  selectedSubTable: null,
  selectedAction: null,

  viewMode: 'table',

  selectedTableColumns: [],
  showAddEdit: false,

  tableData: [],
  fileList: [],
  selectedRow: null,
  visibleWorlds: [],
  finder: [],
  systemActions: [],
  actionManager: null,
  selectedInstanceReferenceId: null,
  worlds: [],
  selectedInstanceTitle: null,
  subTableColumns: null,
  actions: null,
  selectedInstanceType: null
});

/***/ }),
/* 292 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });


/* harmony default export */ __webpack_exports__["default"] = ({
  name: 'NotFound'
});

/***/ }),
/* 293 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__plugins_worldmanager__ = __webpack_require__(8);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__plugins_actionmanager__ = __webpack_require__(10);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__plugins_jsonapi__ = __webpack_require__(11);






/* harmony default export */ __webpack_exports__["default"] = ({
  middleware: 'authenticated',
  data: function data() {
    return {
      action: null,
      jsonApi: __WEBPACK_IMPORTED_MODULE_2__plugins_jsonapi__["a" /* default */],
      tablename: null,
      model: {},
      actionname: null,
      actionManager: __WEBPACK_IMPORTED_MODULE_1__plugins_actionmanager__["a" /* default */]
    };
  },
  methods: {
    cancel: function cancel() {
      console.log("cancel action");
      window.history.back();
    },
    init: function init() {
      this.model = this.$route.query;
      console.log("action model", this.model);
      this.action = __WEBPACK_IMPORTED_MODULE_1__plugins_actionmanager__["a" /* default */].getActionModel(this.tablename, this.actionname);
    }
  },
  mounted: function mounted() {
    console.log("loaded action view", this.$route.params);
    this.tablename = this.$route.params.tablename;
    this.actionname = this.$route.params.actionname;
    this.init();
  },

  watch: {
    '$route.params.actionname': function $routeParamsActionname(newActionName) {
      console.log("New action name", newActionName);
      this.actionname = newActionName;
      this.init();
    },
    '$route.params.tablename': function $routeParamsTablename(newTableName) {
      console.log("New action name", newTableName);
      this.tablename = newTableName;
      this.init();
    }
  }
});

/***/ }),
/* 294 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_element_ui__ = __webpack_require__(7);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_element_ui___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_element_ui__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__plugins_worldmanager__ = __webpack_require__(8);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__plugins_jsonapi__ = __webpack_require__(11);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__plugins_actionmanager__ = __webpack_require__(10);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4_vuex__ = __webpack_require__(9);









/* harmony default export */ __webpack_exports__["default"] = ({
  name: 'AdminView',
  props: {},
  data: function data() {
    return {};
  }
});

/***/ }),
/* 295 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__utils_auth__ = __webpack_require__(18);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__plugins_worldmanager__ = __webpack_require__(8);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__plugins_actionmanager__ = __webpack_require__(10);






/* harmony default export */ __webpack_exports__["default"] = ({
  name: 'App',
  data: function data() {
    return {
      section: 'Head',
      loaded: false
    };
  },

  mounted: function mounted() {
    var that = this;
    if (!this.$store.getters.isAuthenticated) {
      var _extractInfoFromHash = __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_0__utils_auth__["a" /* extractInfoFromHash */])(),
          code = _extractInfoFromHash.code,
          token = _extractInfoFromHash.token,
          secret = _extractInfoFromHash.secret;

      console.log("check token", token, code, secret);
      if (token && __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_0__utils_auth__["b" /* checkSecret */])(secret)) {
        __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_0__utils_auth__["c" /* setToken */])(token);
        this.$router.go('/');
        window.location = "/";
        return;
      } else if (code && __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_0__utils_auth__["b" /* checkSecret */])(secret)) {
        console.log("got code in param", code);

        var query = this.$route.query;
        __WEBPACK_IMPORTED_MODULE_2__plugins_actionmanager__["a" /* default */].doAction("oauth_token", "oauth.login.response", this.$route.query).then(function () {
          console.log("oauth login response", arguments);
        }, function () {
          that.$notify.error({
            message: "Failed to validate connection"
          });
          that.$router.push({
            name: "Dashboard"
          });
        });
        return;
      } else {
        console.log(" is not authenticated ");
        if (this.$route.path == "/auth/signin" || this.$route.path == "/auth/signed") {} else {
          this.$store.commit('SET_LAST_URL', this.$route);
          this.$router.push({ name: 'SignIn' });
        }
      }
      that.loaded = true;
    } else {
      var that = this;
      console.log("begin load models");
      var promise = __WEBPACK_IMPORTED_MODULE_1__plugins_worldmanager__["a" /* default */].loadModels();
      promise.then(function () {
        console.log("World loaded, start view");

        if (window.localStorage) {
          var lastRoute = window.localStorage.getItem("last_route");
          if (lastRoute) {
            that.$store.commit('SET_LAST_URL', null);
            console.log("last route is present");
            that.$router.push(JSON.parse(lastRoute));
          } else {
            console.log("no last route present");
          }
        }

        that.loaded = true;
      });
    }
  },
  methods: {
    logout: function logout() {
      this.$store.commit('SET_USER', null);
      this.$store.commit('SET_TOKEN', null);

      if (window.localStorage) {
        window.localStorage.setItem('user', null);
        window.localStorage.setItem('token', null);
      }

      this.$router.push("/auth/signin");
    }
  }
});

/***/ }),
/* 296 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_extends__ = __webpack_require__(27);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_extends___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_extends__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_vuex__ = __webpack_require__(9);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__config__ = __webpack_require__(205);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__config___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_2__config__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__Sidebar__ = __webpack_require__(617);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__Sidebar___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_3__Sidebar__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4__utils_auth__ = __webpack_require__(18);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_5_element_ui__ = __webpack_require__(7);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_5_element_ui___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_5_element_ui__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_6__plugins_worldmanager__ = __webpack_require__(8);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_7_tether_shepherd__ = __webpack_require__(598);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_7_tether_shepherd___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_7_tether_shepherd__);















/* harmony default export */ __webpack_exports__["default"] = ({
  name: 'Dash',
  components: {
    Sidebar: __WEBPACK_IMPORTED_MODULE_3__Sidebar___default.a
  },
  data: function data() {
    return {
      query: "",

      year: new Date().getFullYear(),
      classes: {
        fixed_layout: __WEBPACK_IMPORTED_MODULE_2__config___default.a.fixedLayout,
        hide_logo: __WEBPACK_IMPORTED_MODULE_2__config___default.a.hideLogoOnMobile
      },
      error: ''
    };
  },
  computed: __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_extends___default()({}, __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_1_vuex__["c" /* mapGetters */])(["visibleWorlds", 'isAuthenticated', 'user']), {
    demo: function demo() {
      return {
        displayName: faker.name.findName(),
        avatar: faker.image.avatar(),
        email: faker.internet.email(),
        tour: null,
        randomCard: faker.helpers.createCard()
      };
    }
  }),
  mounted: function mounted() {
    var that = this;
    if (!this.isAuthenticated) {
      var _extractInfoFromHash = __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_4__utils_auth__["a" /* extractInfoFromHash */])(),
          token = _extractInfoFromHash.token,
          secret = _extractInfoFromHash.secret;

      if (!__webpack_require__.i(__WEBPACK_IMPORTED_MODULE_4__utils_auth__["b" /* checkSecret */])(secret) || !token) {
        console.info('Something happened with the Sign In request');

        this.$router.push("/auth/signin");
      } else {
        console.log("got token from url", token);
        __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_4__utils_auth__["c" /* setToken */])(token);
        window.location.hash = "";
        window.location.reload();
      }
    }

    document.body.className = document.body.className + " sidebar-collapse";
  },

  methods: __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_extends___default()({
    clearSearch: function clearSearch(e) {
      $("#navbar-search-input").val("");
      this.setQueryString(null);
    }
  }, __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_1_vuex__["d" /* mapActions */])(["setQuery"]), {
    setQueryString: function setQueryString(query) {
      console.log("set query", query);
      this.setQuery(query);
      return false;
    },
    changeloading: function changeloading() {
      this.$store.commit('TOGGLE_SEARCHING');
    }
  }),
  watch: {
    '$route': function $route() {
      setTimeout(function () {
        $(window).resize();
      }, 100);
    }
  }
});

/***/ }),
/* 297 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_extends__ = __webpack_require__(27);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_extends___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_extends__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_object_keys__ = __webpack_require__(5);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_object_keys___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_object_keys__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_element_ui__ = __webpack_require__(7);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_element_ui___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_2_element_ui__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__plugins_worldmanager__ = __webpack_require__(8);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__ = __webpack_require__(11);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_5__plugins_actionmanager__ = __webpack_require__(10);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_6_vuex__ = __webpack_require__(9);











/* harmony default export */ __webpack_exports__["default"] = ({
  name: 'EntityView',
  props: {
    tablename: {
      type: String,
      default: 'world'
    },
    refId: {
      type: String,
      default: null
    },
    subTable: {
      type: String,
      default: null
    },
    viewType: {
      type: String,
      default: 'table-view'
    }
  },
  data: function data() {
    return {
      jsonApi: __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__["a" /* default */],
      currentViewType: null,
      actionManager: __WEBPACK_IMPORTED_MODULE_5__plugins_actionmanager__["a" /* default */],
      showAddEdit: false,
      selectedWorldAction: {},
      addExchangeAction: null,
      viewMode: "card",
      rowBeingEdited: null,
      worldReferenceId: null
    };
  },

  methods: {
    hideModel: function hideModel() {
      console.log("Call to hide model");
      $('#uploadJson').modal('hide all');
    },
    doAction: function doAction(action) {
      console.log("set action", action);

      this.$store.commit("SET_SELECTED_ACTION", action);
      this.rowBeingEdited = true;
      this.showAddEdit = true;
    },
    uploadJsonSchemaFile: function uploadJsonSchemaFile() {
      console.log("this files list", this.$refs.upload);
    },
    handleCommand: function handleCommand(command) {
      if (command == "load-restart") {
        window.location.reload();
        return;
      }

      this.$router.push({
        name: 'Action',
        params: {
          tablename: "world",
          actionname: command
        }
      });
    },
    getCurrentTableType: function getCurrentTableType() {
      var that = this;
      return that.selectedTable;
    },
    deleteRow: function deleteRow(row) {
      var that = this;
      console.log("delete row", this.getCurrentTableType());

      __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__["a" /* default */].destroy(this.getCurrentTableType(), row["reference_id"]).then(function () {
        that.setTable();
      });
    },
    saveRow: function saveRow(row) {

      var that = this;
      var newRow = {};
      var keys = __WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_object_keys___default()(row);
      for (var i = 0; i < keys.length; i++) {
        if (row[keys[i]] != null) {
          newRow[keys[i]] = row[keys[i]];
        }
      }
      row = newRow;

      var currentTableType = this.getCurrentTableType();

      console.log("save row", row);
      if (row["id"]) {
        var that = this;
        __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__["a" /* default */].update(currentTableType, row).then(function () {
          that.setTable();
          that.showAddEdit = false;
        }, function (err) {
          console.log("failed to save row", err);
        });
      } else {
        var that = this;
        __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__["a" /* default */].create(currentTableType, row).then(function () {
          console.log("create complete", arguments);
          that.setTable();
          that.showAddEdit = false;
          that.$refs.tableview1.reloadData(currentTableType);
        }, function (r) {
          console.error("failed to save row", r);
        });
      }
    },

    reloadData: function reloadData() {
      var currentTableType = this.getCurrentTableType();
      var that = this;
      if (that.$refs.tableview1) {
        that.$refs.tableview1.reloadData(currentTableType);
      }
    },
    newRow: function newRow() {
      var that = this;
      console.log("new row", that.selectedTable);
      this.rowBeingEdited = {};
      this.showAddEdit = true;
    },
    editRow: function editRow(row) {
      var that = this;
      console.log("new row", that.selectedTable);
      this.rowBeingEdited = row;
      this.showAddEdit = true;
    },
    setTable: function setTable() {
      var that = this;
      var tableName;

      var world = __WEBPACK_IMPORTED_MODULE_3__plugins_worldmanager__["a" /* default */].getWorldByName(that.selectedTable);

      if (!world) {
        that.$notify({
          type: "error",
          title: "Error",
          message: "We dont yet know about anything like " + window.titleCase(that.selectedTable)
        });
        return;
      }

      this.worldReferenceId = world.id;

      var all = {};
      console.log("Admin set table -", that.visibleWorlds);
      console.log("Admin set table -", that.$store, that.selectedTable, that.selectedTable);

      all = __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__["a" /* default */].all(that.selectedTable);
      tableName = that.selectedTable;

      that.$route.meta.breadcrumb = [{
        label: tableName,
        to: {
          name: "Entity",
          params: {
            tablename: tableName
          }
        }
      }];

      if (that.selectedTable) {
        __WEBPACK_IMPORTED_MODULE_3__plugins_worldmanager__["a" /* default */].getColumnKeys(that.selectedTable, function (model) {
          console.log("Set selected world columns", model.ColumnModel);
          that.$store.commit("SET_SELECTED_TABLE_COLUMNS", model.ColumnModel);
        });
      }

      that.$store.commit("SET_FINDER", all.builderStack);
      console.log("Finder stack: ", that.finder);

      console.log("Selected table: ", that.selectedTable);

      that.$store.commit("SET_ACTIONS", __WEBPACK_IMPORTED_MODULE_5__plugins_actionmanager__["a" /* default */].getActions(that.selectedTable));

      all.builderStack = [];

      console.log("setTable for [tableview1]: ", tableName);
      if (that.$refs.tableview1) {
        console.log("tableview 1 is present");
        that.$refs.tableview1.reloadData(tableName);
      }
    },

    logout: function logout() {
      this.$parent.logout();
    }
  },
  mounted: function mounted() {

    var that = this;
    that.currentViewType = that.viewType;
    console.log("Entity view: ", that.$route);

    that.actionManager = __WEBPACK_IMPORTED_MODULE_5__plugins_actionmanager__["a" /* default */];
    var worldActions = __WEBPACK_IMPORTED_MODULE_5__plugins_actionmanager__["a" /* default */].getActions("world");
    console.log("world actions", worldActions);

    that.addExchangeAction = __WEBPACK_IMPORTED_MODULE_5__plugins_actionmanager__["a" /* default */].getActionModel("world", "add-exchange");

    if (that.$route.name == "NewEntity") {
      this.rowBeingEdited = {};
      that.showAddEdit = true;
    }

    var tableName = that.$route.params.tablename;
    var subTableName = that.$route.params.subTable;
    var selectedInstanceId = that.$route.params.refId;

    if (!tableName) {
      tableName = "user_account";
    }

    console.log("Set table 1", tableName);
    that.$store.commit("SET_SELECTED_TABLE", tableName);
    that.$store.commit("SET_ACTIONS", worldActions);

    if (selectedInstanceId) {
      that.$store.commit("SET_SELECTED_INSTANCE_REFERENCE_ID", selectedInstanceId);
      __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__["a" /* default */].one(tableName, selectedInstanceId).get(function (res) {
        console.log("got object", res);
        that.$store.commit("SET_SELECTED_ROW", res);
      });
    }

    if (selectedInstanceId && subTableName) {
      that.$store.commit("SET_SELECTED_TABLE", tableName);
    }

    that.setTable();
  },

  computed: __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_extends___default()({}, __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_6_vuex__["b" /* mapState */])(["selectedAction", "subTableColumns", "systemActions", "finder", "selectedTableColumns", "query", "selectedRow", "selectedTable", "selectedInstanceReferenceId"]), __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_6_vuex__["c" /* mapGetters */])(["visibleWorlds", "actions"])),
  watch: {
    '$route.params.tablename': function $routeParamsTablename(to, from) {
      console.log("tablename page, path changed: ", arguments);
      this.$store.commit("SET_SELECTED_TABLE", to);
      this.$store.commit("SET_SELECTED_SUB_TABLE", null);
      this.showAddEdit = false;
      this.setTable();
    },
    '$route.params.subTable': function $routeParamsSubTable(to, from) {
      this.showAddEdit = false;
      console.log("TableName SubTable changed", arguments);
      this.$store.commit("SET_SELECTED_SUB_TABLE", to);
      this.setTable();
    },
    '$route.name': function $routeName() {
      if (this.$route.name === "NewEntity") {
        this.showAddEdit = true;
        this.rowBeingEdited = {};
      } else {
        this.showAddEdit = false;
      }
    },
    'showAddEdit': function showAddEdit(newVal) {
      if (!newVal) {
        if (this.$route.name === "NewEntity") {
          console.log("triggr back");
          window.history.back();
        }
      }
    },
    'query': function query() {
      this.setTable();
    }
  }
});

/***/ }),
/* 298 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });


/* harmony default export */ __webpack_exports__["default"] = ({
  middleware: 'anonymous',
  mounted: function mounted() {
    console.log("sign in loaded");
  }
});

/***/ }),
/* 299 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_extends__ = __webpack_require__(27);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_extends___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_extends__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_object_keys__ = __webpack_require__(5);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_object_keys___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_object_keys__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_element_ui__ = __webpack_require__(7);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_element_ui___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_2_element_ui__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__plugins_worldmanager__ = __webpack_require__(8);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__ = __webpack_require__(11);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_5__plugins_actionmanager__ = __webpack_require__(10);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_6_vuex__ = __webpack_require__(9);










/* harmony default export */ __webpack_exports__["default"] = ({
  name: 'InstanceView',
  data: function data() {
    return {
      jsonApi: __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__["a" /* default */],
      actionManager: __WEBPACK_IMPORTED_MODULE_5__plugins_actionmanager__["a" /* default */],
      showAddEdit: false,
      stateMachines: [],
      selectedWorldAction: {},
      objectStates: [],
      rowBeingEdited: {},
      truefalse: []
    };
  },

  methods: {
    editRow: function editRow() {
      console.log("edit row");
      this.$store.commit("SET_SELECTED_ACTION", null);
      this.showAddEdit = true;
      this.rowBeingEdited = this.selectedRow;
    },
    refreshRow: function refreshRow() {
      var that = this;
      var tableName = that.$route.params.tablename;
      var selectedInstanceId = that.$route.params.refId;

      __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__["a" /* default */].find(tableName, selectedInstanceId).then(function (res) {
        console.log("got object", res);
        res = res.data;
        that.$store.commit("SET_SELECTED_ROW", res);
      }, function (err) {
        console.log("Errors", err);
      });
    },
    doEvent: function doEvent(action, event) {
      var that = this;
      console.log("do event", action, event);
      __WEBPACK_IMPORTED_MODULE_3__plugins_worldmanager__["a" /* default */].trackObjectEvent(this.selectedTable, action.id, event.name).then(function () {
        __WEBPACK_IMPORTED_MODULE_2_element_ui__["Notification"].success({
          title: "Updated",
          message: that.selectedTable + " status was updated for this track"
        });
        that.updateStates();
      }, function () {
        __WEBPACK_IMPORTED_MODULE_2_element_ui__["Notification"].error({
          title: "Failed",
          message: "Object status was not updated"
        });
      });
    },
    addStateMachine: function addStateMachine(machine) {
      console.log("Add state machine", machine);
      console.log("Selected row", this.selectedRow);
      var that = this;
      __WEBPACK_IMPORTED_MODULE_3__plugins_worldmanager__["a" /* default */].startObjectTrack(this.selectedTable, this.selectedRow["id"], machine["reference_id"]).then(function (res) {
        __WEBPACK_IMPORTED_MODULE_2_element_ui__["Notification"].success({
          title: "Done",
          message: "Started tracking status for " + that.selectedTable
        });
        that.updateStates();
      });
    },
    doAction: function doAction(action) {
      this.$store.commit("SET_SELECTED_ACTION", action);
      this.rowBeingEdited = null;
      this.showAddEdit = true;
    },
    saveRow: function saveRow(row) {
      var that = this;

      var currentTableType = this.selectedTable;

      if (that.selectedSubTable && that.selectedInstanceReferenceId) {
        row[that.selectedTable + "_id"] = {
          "id": that.selectedInstanceReferenceId
        };
      }

      var newRow = {};
      var keys = __WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_object_keys___default()(row);
      for (var i = 0; i < keys.length; i++) {
        if (row[keys[i]] != null) {
          newRow[keys[i]] = row[keys[i]];
        }
      }
      row = newRow;

      console.log("save row", row);
      if (row["id"]) {
        var that = this;
        __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__["a" /* default */].update(currentTableType, row).then(function () {
          that.setTable();
          that.showAddEdit = false;
        });
      } else {
        var that = this;
        __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__["a" /* default */].create(currentTableType, row).then(function () {
          console.log("create complete", arguments);
          that.setTable();
          that.showAddEdit = false;
          that.$refs.tableview1.reloadData(currentTableType);
          that.$refs.tableview2.reloadData(currentTableType);
        }, function (r) {
          console.error(r);
        });
      }
    },
    setTable: function setTable() {
      var that = this;

      console.log("Instance View: ", that.$route.params);

      that.actionManager = __WEBPACK_IMPORTED_MODULE_5__plugins_actionmanager__["a" /* default */];
      var worldActions = __WEBPACK_IMPORTED_MODULE_5__plugins_actionmanager__["a" /* default */].getActions("world");

      var tableName = that.$route.params.tablename;
      var selectedInstanceId = that.$route.params.refId;

      if (!tableName) {
        alert("no table name");
        return;
      }
      that.$route.meta.breadcrumb = [{
        label: tableName,
        to: {
          name: "Entity",
          params: {
            tablename: tableName
          }
        }
      }, {
        label: selectedInstanceId
      }];

      that.$store.commit("SET_SELECTED_TABLE", tableName);
      that.$store.commit("SET_ACTIONS", worldActions);

      that.$store.commit("SET_SELECTED_INSTANCE_REFERENCE_ID", selectedInstanceId);
      console.log("Get instance: ", tableName, selectedInstanceId);

      __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__["a" /* default */].find(tableName, selectedInstanceId).then(function (res) {
        console.log("got object", arguments);
        res = res.data;
        that.$store.commit("SET_SELECTED_ROW", res);
      }, function (err) {
        console.log("Errors", err);
      });

      that.$store.commit("SET_SELECTED_TABLE", tableName);

      var all = {};

      console.log("Admin set table -", that.$store, that.selectedTable, that.selectedTable);
      all = __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__["a" /* default */].all(that.selectedTable);
      tableName = that.selectedTable;

      if (that.selectedTable) {
        __WEBPACK_IMPORTED_MODULE_3__plugins_worldmanager__["a" /* default */].getColumnKeys(that.selectedTable, function (model) {
          console.log("Set selected world columns", model.ColumnModel);
          that.$store.commit("SET_SELECTED_TABLE_COLUMNS", model.ColumnModel);
        });
      }

      that.$store.commit("SET_FINDER", all.builderStack);
      console.log("Finder stack: ", that.finder);

      console.log("Selected sub table: ", that.selectedSubTable);
      console.log("Selected table: ", that.selectedTable);

      that.$store.commit("SET_ACTIONS", __WEBPACK_IMPORTED_MODULE_5__plugins_actionmanager__["a" /* default */].getActions(that.selectedTable));

      all.builderStack = [];

      __WEBPACK_IMPORTED_MODULE_3__plugins_worldmanager__["a" /* default */].getStateMachinesForType(that.selectedTable).then(function (machines) {
        console.log("state machines for ", that.selectedTable, machines);
        that.stateMachines = machines;
      });

      that.updateStates();

      if (that.$refs.tableview1) {
        console.log("setTable for [tableview1]: ", tableName);
        that.$refs.tableview1.reloadData(tableName);
      }
    },

    logout: function logout() {
      this.$parent.logout();
    },
    updateStates: function updateStates() {
      var that = this;

      var tableName = that.$route.params.tablename;
      var selectedInstanceId = that.$route.params.refId;

      console.log("Start get states for ", tableName, selectedInstanceId);
      var tableModel = __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__["a" /* default */].modelFor(tableName);
      console.log("json api model", tableModel);

      if (__WEBPACK_IMPORTED_MODULE_3__plugins_worldmanager__["a" /* default */].isStateMachineEnabled(tableName)) {
        __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__["a" /* default */].one(tableName, selectedInstanceId).all(tableName + "_has_state").get({
          page: {
            number: 1,
            size: 20
          }
        }).then(function (states) {
          states = states.data;
          console.log("states", states);
          states.map(function (e) {
            e.smd = e[tableName + "_smd"];
            e.smd.events = JSON.parse(e.smd.events);
            e.possibleActions = e.smd.events.filter(function (t) {
              return t.Src.indexOf(e.current_state) > -1;
            }).map(function (er) {
              return {
                name: er.Name,
                label: er.Label
              };
            });
            console.log(e);
          });

          that.objectStates = states;
        });
      }
    }
  },

  mounted: function mounted() {
    var that = this;
    that.setTable();
  },

  computed: __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_extends___default()({}, __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_6_vuex__["b" /* mapState */])(["selectedSubTable", "selectedAction", "subTableColumns", "systemActions", "finder", "selectedTableColumns", "selectedRow", "selectedTable", "selectedInstanceReferenceId"]), __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_6_vuex__["c" /* mapGetters */])(["visibleWorlds", "actions"])),
  watch: {
    '$route.params.tablename': function $routeParamsTablename(to, from) {
      var that = this;

      console.log("tablename, path changed: ", arguments, this.$route.params.refId);
      this.$store.commit("SET_SELECTED_TABLE", to);
      this.$store.commit("SET_SELECTED_SUB_TABLE", null);
      that.$store.commit("SET_SELECTED_INSTANCE_REFERENCE_ID", this.$route.params.refId);
      this.showAddEdit = false;

      __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__["a" /* default */].one(that.selectedTable, this.$route.params.refId).get().then(function (r) {
        console.log("TableName SET_SELECTED_ROW", r);
        that.$store.commit("SET_SELECTED_ROW", r);
        that.$store.commit("SET_SELECTED_INSTANCE_REFERENCE_ID", r["id"]);
      });
      this.setTable();
    },
    '$route.params.refId': function $routeParamsRefId(to, from) {
      var that = this;

      console.log("refId page, path changed: ", arguments, this.$route.params.refId);
      this.$store.commit("SET_SELECTED_TABLE", to);
      this.$store.commit("SET_SELECTED_SUB_TABLE", null);
      that.$store.commit("SET_SELECTED_INSTANCE_REFERENCE_ID", this.$route.params.refId);
      this.showAddEdit = false;

      __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__["a" /* default */].one(that.selectedTable, this.$route.params.refId).get().then(function (r) {
        console.log("TableName SET_SELECTED_ROW", r);
        that.$store.commit("SET_SELECTED_ROW", r);
        that.$store.commit("SET_SELECTED_INSTANCE_REFERENCE_ID", r["id"]);
      });
      this.setTable();
    }
  }
});

/***/ }),
/* 300 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_json_stringify__ = __webpack_require__(21);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_json_stringify___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_json_stringify__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__api__ = __webpack_require__(285);





/* harmony default export */ __webpack_exports__["default"] = ({
  name: 'Login',
  data: function data(router) {
    return {
      section: 'Login',
      loading: '',
      username: '',
      password: '',
      response: ''
    };
  },

  methods: {
    checkCreds: function checkCreds() {
      var _this = this;

      var username = this.username,
          password = this.password;


      this.toggleLoading();
      this.resetResponse();
      this.$store.commit('TOGGLE_LOADING');

      __WEBPACK_IMPORTED_MODULE_1__api__["a" /* default */].request('post', '/login', { username: username, password: password }).then(function (response) {
        _this.toggleLoading();

        var data = response.data;

        if (data.error) {
          var errorName = data.error.name;
          if (errorName) {
            _this.response = errorName === 'InvalidCredentialsError' ? 'Username/Password incorrect. Please try again.' : errorName;
          } else {
            _this.response = data.error;
          }

          return;
        }

        if (data.user) {
          var token = 'Bearer ' + data.token;

          _this.$store.commit('SET_USER', data.user);
          _this.$store.commit('SET_TOKEN', token);

          if (window.localStorage) {
            window.localStorage.setItem('user', __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_json_stringify___default()(data.user));
            window.localStorage.setItem('token', token);
          }

          _this.$router.push(data.redirect);
        }
      }).catch(function (error) {
        _this.$store.commit('TOGGLE_LOADING');
        console.log(error);

        _this.response = 'Server appears to be offline';
        _this.toggleLoading();
      });
    },
    toggleLoading: function toggleLoading() {
      this.loading = this.loading === '' ? 'loading' : '';
    },
    resetResponse: function resetResponse() {
      this.response = '';
    }
  }
});

/***/ }),
/* 301 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys__ = __webpack_require__(5);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_defineProperty__ = __webpack_require__(41);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_defineProperty___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_defineProperty__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_babel_runtime_core_js_json_stringify__ = __webpack_require__(21);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_babel_runtime_core_js_json_stringify___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_2_babel_runtime_core_js_json_stringify__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3_babel_runtime_helpers_extends__ = __webpack_require__(27);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3_babel_runtime_helpers_extends___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_3_babel_runtime_helpers_extends__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4__plugins_worldmanager__ = __webpack_require__(8);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_5_vuex__ = __webpack_require__(9);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_6__plugins_actionmanager__ = __webpack_require__(10);










var typeMeta = [{
  name: "entity",
  label: "Entity"
}];

/* harmony default export */ __webpack_exports__["default"] = ({
  computed: __WEBPACK_IMPORTED_MODULE_3_babel_runtime_helpers_extends___default()({}, __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_5_vuex__["b" /* mapState */])(['worlds']), {
    relatableWorlds: function relatableWorlds() {
      return this.worlds.filter(function (e) {
        return e.table_name.indexOf("_has_") == -1 && e.table_name.indexOf("_audit") == -1;
      });
    }
  }),
  methods: {
    removeColumn: function removeColumn(colData) {
      console.log("remove columne", colData);
      var index = this.data.Columns.indexOf(colData);
      if (index > -1) {
        this.data.Columns.splice(index, 1);
      }
    },
    removeRelation: function removeRelation(relation) {
      console.log("remove relation", relation);
      var index = this.data.Relations.indexOf(relation);

      if (index > -1) {
        this.data.Relations.splice(index, 1);
      }
    },
    setup: function setup() {
      console.log("query table name", this.$route.query);
    },

    createEntity: function createEntity() {
      var that = this;
      console.log(this.data);
      var fileContent = __WEBPACK_IMPORTED_MODULE_2_babel_runtime_core_js_json_stringify___default()({
        Tables: [{
          TableName: this.data.TableName,
          Columns: this.data.Columns.map(function (col) {
            if (!col.Name) {
              return null;
            }
            col.ColumnName = col.Name;
            col.DataType = that.columnTypes[col.ColumnType].DataTypes[0];
            return col;
          }).filter(function (e) {
            return !!e && !e.ReadOnly;
          })
        }],
        Relations: this.data.Relations.map(function (rel) {
          rel.Subject = that.data.TableName;
          return rel;
        }).filter(function (e) {
          return !!e && !e.ReadOnly;
        })
      });
      console.log("New table json", fileContent);

      var postData = {
        "schema_file": [{
          "name": this.data.TableName + ".json",
          "file": "data:application/json;base64," + btoa(fileContent),
          "type": "application/json"
        }]
      };
      __WEBPACK_IMPORTED_MODULE_6__plugins_actionmanager__["a" /* default */].doAction("world", "upload_system_schema", postData);
    }
  },
  data: function data() {
    return __WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_defineProperty___default()({
      columnTypes: [],
      data: {
        TableName: null,
        Columns: [{
          Name: 'name',
          ColumnType: "measurement"
        }],
        Relations: [{
          Relation: "belongs_to",
          Object: "user_account"
        }, {
          Relation: "has_many",
          Object: "usergroup"
        }]
      }
    }, 'columnTypes', [{
      name: "varchar",
      label: "Small text",
      description: "For names"
    }, {}]);
  },
  mounted: function mounted() {
    console.log("Loaded new meta page");
    var that = this;
    that.columnTypes = __WEBPACK_IMPORTED_MODULE_4__plugins_worldmanager__["a" /* default */].getColumnFieldTypes();
    var query = this.$route.query;
    if (query && query.table) {
      __WEBPACK_IMPORTED_MODULE_4__plugins_worldmanager__["a" /* default */].getColumnKeys(query.table, function (columns) {
        var columnModel = columns.ColumnModel;
        var columnNames = __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default()(columnModel);
        var finalColumns = [];
        var finalRelations = [];
        that.data.TableName = query.table;
        for (var i = 0; i < columnNames.length; i++) {
          var columnName = columnNames[i];
          if (columnName == "__type") {
            continue;
          }
          var model = columnModel[columnName];
          if (model.IsForeignKey || model.jsonApi) {

            if (model.type.indexOf("_audit") > -1) {
              continue;
            }

            var relationType = "has_many";
            switch (model.jsonApi) {
              case "hasMany":
                relationType = "has_many";
                break;
              case "belongsTo":
                relationType = "belongs_to";
                break;
              case "hasOne":
                relationType = "has_one";
                break;
            }

            console.log("add table relations", model);
            finalRelations.push({
              Relation: relationType,
              Subject: query.table,
              Object: model.type,
              ReadOnly: true
            });
          } else {
            console.log("add column", model);
            model.ReadOnly = true;
            finalColumns.push(model);
          }
        }
        finalColumns.forEach(function (e) {
          console.log("final column", e);
          e.ColumnType = e.ColumnType.split(".")[0];
        });
        that.data.Columns = finalColumns;
        that.data.Relations = finalRelations;
        console.log("selected world columns", columns);
      });
    }
    console.log("selected world", query);
    console.log("column types", that.columnTypes);
    that.setup();
  }
});

/***/ }),
/* 302 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__plugins_configmanager__ = __webpack_require__(95);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__plugins_actionmanager__ = __webpack_require__(10);





/* harmony default export */ __webpack_exports__["default"] = ({
  data: function data() {
    return {
      actionManager: __WEBPACK_IMPORTED_MODULE_1__plugins_actionmanager__["a" /* default */]
    };
  },

  methods: {
    init: function init() {
      var that = this;
      console.log("oauth response", this.$route);
      var query = this.$route.query;
      this.actionManager.doAction("oauth_token", "oauth.login.response", this.$route.query).then(function () {}, function () {
        that.$notify.error({
          message: "Failed to validate connection"
        });
        that.$router.push({
          name: "Dashboard"
        });
      });
    }
  },
  mounted: function mounted() {
    this.init();
  }
});

/***/ }),
/* 303 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_extends__ = __webpack_require__(27);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_extends___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_extends__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_object_keys__ = __webpack_require__(5);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_object_keys___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_object_keys__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_element_ui__ = __webpack_require__(7);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_element_ui___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_2_element_ui__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__plugins_worldmanager__ = __webpack_require__(8);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__ = __webpack_require__(11);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_5__plugins_actionmanager__ = __webpack_require__(10);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_6_vuex__ = __webpack_require__(9);











/* harmony default export */ __webpack_exports__["default"] = ({
  name: 'RelationView',
  props: {
    tablename: {
      type: String,
      default: 'world'
    },
    refId: {
      type: String,
      default: null
    },
    subTable: {
      type: String,
      default: null
    },
    viewType: {
      type: String,
      default: 'table-view'
    }
  },
  data: function data() {
    return {
      jsonApi: __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__["a" /* default */],
      currentViewType: null,
      worldReferenceId: null,
      actionManager: __WEBPACK_IMPORTED_MODULE_5__plugins_actionmanager__["a" /* default */],
      showAddEdit: false,
      selectedWorldAction: {}
    };
  },

  methods: {
    hideModel: function hideModel() {
      console.log("Call to hide model");
      $('#uploadJson').modal('hide all');
    },
    doAction: function doAction(action) {
      this.$store.commit("SET_SELECTED_ACTION", action);
      this.showAddEdit = true;
    },
    uploadJsonSchemaFile: function uploadJsonSchemaFile() {
      console.log("this files list", this.$refs.upload);
    },
    handleCommand: function handleCommand(command) {
      if (command == "load-restart") {
        window.location.reload();
        return;
      }

      this.$router.push({
        name: 'tablename-actionname',
        params: {
          tablename: "world",
          actionname: command
        }
      });
    },
    getCurrentTableType: function getCurrentTableType() {
      var that = this;
      return that.selectedSubTable;
    },
    deleteRow: function deleteRow(row) {
      var that = this;
      console.log("delete row", this.getCurrentTableType());

      __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__["a" /* default */].destroy(this.getCurrentTableType(), row["reference_id"]).then(function () {
        that.setTable();
      });
    },
    saveRow: function saveRow(row) {

      var that = this;

      var currentTableType = this.getCurrentTableType();

      if (that.selectedSubTable && that.selectedInstanceReferenceId) {
        row[that.selectedTable + "_id"] = {
          "id": that.selectedInstanceReferenceId
        };
      }
      var newRow = {};
      var keys = __WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_object_keys___default()(row);
      for (var i = 0; i < keys.length; i++) {
        if (row[keys[i]] != null) {
          newRow[keys[i]] = row[keys[i]];
        }
      }
      row = newRow;

      console.log("save row", row);
      if (row["id"]) {
        var that = this;
        __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__["a" /* default */].update(currentTableType, row).then(function () {
          that.setTable();
          that.showAddEdit = false;
        });
      } else {
        var that = this;
        __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__["a" /* default */].create(currentTableType, row).then(function () {
          console.log("create complete", arguments);
          that.setTable();
          that.showAddEdit = false;
          that.$refs.tableview2.reloadData(currentTableType);
        }, function (r) {
          console.error(r);
        });
      }
    },

    reloadData: function reloadData() {
      var currentTableType = this.getCurrentTableType();
      var that = this;

      that.$refs.tableview2.reloadData(currentTableType);
    },
    newRow: function newRow() {
      var that = this;
      console.log("new row", that.selectedSubTable);
      this.rowBeingEdited = {};
      this.showAddEdit = true;
    },
    editRow: function editRow(row) {
      var that = this;
      console.log("new row", that.selectedSubTable);
      this.rowBeingEdited = row;
      this.showAddEdit = true;
    },
    setTable: function setTable() {
      var that = this;
      var tableName;

      var all = {};
      console.log("Admin set table -", that.visibleWorlds);
      console.log("Admin set table -", that.$store, that.selectedTable, that.selectedTable);

      var world = __WEBPACK_IMPORTED_MODULE_3__plugins_worldmanager__["a" /* default */].getWorldByName(that.selectedSubTable);
      this.worldReferenceId = world.id;

      tableName = that.selectedSubTable;
      all = __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__["a" /* default */].one(that.selectedTable, that.selectedInstanceReferenceId).all(that.selectedSubTable + "_id");
      console.log("Set subtable columns: ", that.subTableColumns);

      __WEBPACK_IMPORTED_MODULE_3__plugins_worldmanager__["a" /* default */].getColumnKeys(that.selectedSubTable, function (model) {
        console.log("Set selected world columns", model.ColumnModel);
        that.$store.commit("SET_SUBTABLE_COLUMNS", model.ColumnModel);
      });

      that.$store.commit("SET_FINDER", all.builderStack);
      console.log("Finder stack: ", that.finder);

      console.log("Selected sub table: ", that.selectedSubTable);
      console.log("Selected table: ", that.selectedTable);

      that.$store.commit("SET_ACTIONS", __WEBPACK_IMPORTED_MODULE_5__plugins_actionmanager__["a" /* default */].getActions(that.selectedTable));

      all.builderStack = [];

      if (that.$refs.tableview2) {
        that.$refs.tableview2.reloadData(tableName);
      }
    },

    logout: function logout() {
      this.$parent.logout();
    }
  },

  mounted: function mounted() {
    var that = this;

    console.log("Enter tablename: ", that);
    that.currentViewType = that.viewType;

    that.actionManager = __WEBPACK_IMPORTED_MODULE_5__plugins_actionmanager__["a" /* default */];
    var worldActions = __WEBPACK_IMPORTED_MODULE_5__plugins_actionmanager__["a" /* default */].getActions("world");

    var tableName = that.$route.params.tablename;
    var subTableName = that.$route.params.subTable;
    var selectedInstanceId = that.$route.params.refId;

    if (!tableName) {
      tableName = "user_account";
    }
    console.log("Set table 1", tableName, subTableName);
    that.$store.commit("SET_SELECTED_TABLE", tableName);
    that.$store.commit("SET_ACTIONS", worldActions);

    that.$store.commit("SET_SELECTED_INSTANCE_REFERENCE_ID", selectedInstanceId);
    __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__["a" /* default */].one(tableName, selectedInstanceId).get(function (res) {
      console.log("got object", res);
      that.$store.commit("SET_SELECTED_ROW", res);
    });

    that.$store.commit("SET_SELECTED_SUB_TABLE", subTableName);

    that.setTable();
  },

  computed: __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_extends___default()({}, __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_6_vuex__["b" /* mapState */])(["selectedSubTable", "selectedAction", "subTableColumns", "systemActions", "finder", "selectedTableColumns", "selectedRow", "selectedTable", "selectedInstanceReferenceId"]), __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_6_vuex__["c" /* mapGetters */])(["visibleWorlds", "actions"])),
  watch: {
    '$route.params.tablename': function $routeParamsTablename(to, from) {
      console.log("tablename page, path changed: ", arguments);
      this.$store.commit("SET_SELECTED_TABLE", to);
      this.$store.commit("SET_SELECTED_SUB_TABLE", null);
      this.showAddEdit = false;
      this.setTable();
    },
    '$route.params.refId': function $routeParamsRefId(to, from) {
      var that = this;
      console.log("refId changed in tablename path", arguments);
      this.showAddEdit = false;

      if (!to) {
        this.$store.commit("SET_SELECTED_ROW", null);
        that.$store.commit("SET_SELECTED_INSTANCE_REFERENCE_ID", null);
      } else {
        __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__["a" /* default */].one(that.selectedTable, to).get().then(function (r) {
          console.log("TableName SET_SELECTED_ROW", r);
          that.$store.commit("SET_SELECTED_ROW", r);
          that.$store.commit("SET_SELECTED_INSTANCE_REFERENCE_ID", r["id"]);
        });
      }
      this.setTable();
    },
    '$route.params.subTable': function $routeParamsSubTable(to, from) {
      this.showAddEdit = false;
      console.log("TableName SubTable changed", arguments);
      this.$store.commit("SET_SELECTED_SUB_TABLE", to);
      this.setTable();
    }
  }
});

/***/ }),
/* 304 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__SidebarMenu__ = __webpack_require__(618);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__SidebarMenu___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0__SidebarMenu__);




/* harmony default export */ __webpack_exports__["default"] = ({
  name: 'Sidebar',
  props: ['user'],
  data: function data() {
    return {
      filter: ''
    };
  },
  components: { SidebarMenu: __WEBPACK_IMPORTED_MODULE_0__SidebarMenu___default.a }
});

/***/ }),
/* 305 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_extends__ = __webpack_require__(27);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_extends___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_extends__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_vuex__ = __webpack_require__(9);





/* harmony default export */ __webpack_exports__["default"] = ({
  name: 'SidebarName',
  methods: {
    toggleMenu: function toggleMenu(event) {
      var active = document.querySelector('li.pageLink.active');

      if (active) {
        active.classList.remove('active');
      }

      event.toElement.parentElement.className = 'pageLink active';
    }
  },
  props: {
    filter: {
      type: String,
      required: true,
      default: ''
    }
  },
  computed: __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_extends___default()({}, __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_1_vuex__["b" /* mapState */])(['worlds'])),
  data: function data() {
    return {
      topWorlds: []
    };
  },
  mounted: function mounted() {
    var that = this;
    console.log("sidebarmenu visible worlds: ", this.topWorlds);

    that.topWorlds = this.worlds.filter(function (w, r) {
      return w.is_top_level && !w.is_hidden;
    });

    setTimeout(function () {
      $(window).resize();
      console.log("this sidebar again", that.topWorlds);
    }, 300);
  },

  watch: {
    'worlds': function worlds() {
      console.log("got worlds");
      var that = this;
      that.topWorlds = that.worlds.filter(function (w, r) {
        return w.is_top_level && !w.is_hidden;
      });
    }
  }
});

/***/ }),
/* 306 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_json_stringify__ = __webpack_require__(21);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_json_stringify___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_json_stringify__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__plugins_configmanager__ = __webpack_require__(95);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__plugins_actionmanager__ = __webpack_require__(10);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__plugins_worldmanager__ = __webpack_require__(8);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__ = __webpack_require__(11);








/* harmony default export */ __webpack_exports__["default"] = ({
  data: function data() {
    return {
      response: null,
      signInAction: null,
      actionManager: __WEBPACK_IMPORTED_MODULE_2__plugins_actionmanager__["a" /* default */],
      oauthConnections: []
    };
  },

  methods: {
    oauthLogin: function oauthLogin(oauthConnect) {

      console.log("action initiate oauth login being for ", oauthConnect);
      __WEBPACK_IMPORTED_MODULE_2__plugins_actionmanager__["a" /* default */].doAction("oauth_connect", "oauth.login.begin", {
        "oauth_connect_id": oauthConnect.id
      }).then(function (actionResponse) {
        console.log("action response", actionResponse);
      });
    },
    init: function init() {
      var that = this;
      console.log("sign in loaded");

      __WEBPACK_IMPORTED_MODULE_2__plugins_actionmanager__["a" /* default */].getGuestActions().then(function (guestActions) {
        console.log("guest actions", guestActions, guestActions["user:signin"]);
        that.signInAction = guestActions["user:signin"];
      });

      __WEBPACK_IMPORTED_MODULE_3__plugins_worldmanager__["a" /* default */].loadModel("oauth_connect", {
        include: ""
      }).then(function () {

        __WEBPACK_IMPORTED_MODULE_4__plugins_jsonapi__["a" /* default */].findAll('oauth_connect', {
          page: { number: 1, size: 500 },
          query: btoa(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_json_stringify___default()([{
            "column": "allow_login",
            "operator": "is",
            "value": "1"
          }]))
        }).then(function (res) {
          res = res.data;
          console.log("visible oauth connections: ", res);
          that.oauthConnections = res;
        });
      });
    }
  },
  mounted: function mounted() {
    this.init();
  }
});

/***/ }),
/* 307 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__utils_auth__ = __webpack_require__(18);




/* harmony default export */ __webpack_exports__["default"] = ({
  mounted: function mounted() {
    __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_0__utils_auth__["e" /* unsetToken */])();
    console.log("logged out");
    this.$router.push({
      name: 'SignIn'
    });
  }
});

/***/ }),
/* 308 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__plugins_configmanager__ = __webpack_require__(95);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__plugins_actionmanager__ = __webpack_require__(10);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_element_ui__ = __webpack_require__(7);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_element_ui___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_2_element_ui__);







/* harmony default export */ __webpack_exports__["default"] = ({
  data: function data() {
    return {
      response: null,
      signInAction: null,
      actionManager: __WEBPACK_IMPORTED_MODULE_1__plugins_actionmanager__["a" /* default */],
      loading: ""
    };
  },

  methods: {
    signupComplete: function signupComplete() {},
    init: function init() {
      var that = this;
      console.log("sign in loaded");
      __WEBPACK_IMPORTED_MODULE_1__plugins_actionmanager__["a" /* default */].getGuestActions().then(function (guestActions) {
        console.log("guest actions", guestActions, guestActions["user:signup"]);
        that.signInAction = guestActions["user:signup"];
      });
    }
  },
  mounted: function mounted() {
    this.init();
  }
});

/***/ }),
/* 309 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__utils_auth__ = __webpack_require__(18);




/* harmony default export */ __webpack_exports__["default"] = ({
  mounted: function mounted() {
    console.log("signed in");

    var _extractInfoFromHash = __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_0__utils_auth__["a" /* extractInfoFromHash */])(),
        token = _extractInfoFromHash.token,
        secret = _extractInfoFromHash.secret;

    if (!__webpack_require__.i(__WEBPACK_IMPORTED_MODULE_0__utils_auth__["b" /* checkSecret */])(secret) || !token) {
      this.$router.replace('/auth/signin');
      console.error('Something happened with the Sign In request');
      return;
    }
    __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_0__utils_auth__["c" /* setToken */])(token);
    this.$router.replace('/');
  }
});

/***/ }),
/* 310 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys__ = __webpack_require__(5);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__plugins_actionmanager__ = __webpack_require__(10);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__plugins_jsonapi__ = __webpack_require__(11);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3_element_ui__ = __webpack_require__(7);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3_element_ui___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_3_element_ui__);







/* harmony default export */ __webpack_exports__["default"] = ({
  props: {
    hideTitle: {
      type: Boolean,
      required: false,
      default: false
    },
    hideCancel: {
      type: Boolean,
      required: false,
      default: false
    },
    action: {
      type: Object,
      required: true
    },
    model: {
      type: Object,
      required: false,
      default: function _default() {
        return null;
      }
    },
    actionManager: {
      type: Object,
      required: true
    },
    values: {
      type: Object,
      required: false
    }
  },
  data: function data() {
    return {
      meta: null,
      data: {},
      modelSchema: {},
      finalModel: null
    };
  },
  created: function created() {},

  computed: {},
  methods: {
    setModel: function setModel(m1) {
      console.log("set model", m1);
      this.finalModel = m1;
    },
    doAction: function doAction(actionData) {
      var that = this;

      if (!this.finalModel && !this.action.InstanceOptional) {
        __WEBPACK_IMPORTED_MODULE_3_element_ui__["Notification"].error({
          title: "Error",
          message: "Please select a " + this.action.OnType
        });
        return;
      }
      console.log("perform action", actionData, this.finalModel);
      if (this.finalModel && __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default()(this.finalModel).indexOf("id") > -1) {
        actionData[this.action.OnType + "_id"] = this.finalModel["id"];
      } else {}
      that.actionManager.doAction(that.action.OnType, that.action.Name, actionData).then(function () {
        that.$emit("action-complete", that.action);
      }, function () {
        console.log("not clearing out the form");
      });
    },
    cancel: function cancel() {
      this.$emit("cancel");
    },
    init: function init() {

      if (!this.action) {
        return;
      }

      var that = this;

      if (that.values) {
        console.log("values", that.values);
        var keys = __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default()(that.values);
        for (var i = 0; i < keys.length; i++) {
          var key = keys[i];
          that.model[key] = that.values[key];
        }
      }

      console.log("render action ", that.action, " on ", that.model);

      that.finalModel = that.model;
      var worldName = that.action.OnType;
      that.modelSchema = {
        inputType: worldName,
        value: null,
        multiple: false,
        name: that.action.OnType
      };

      var meta = {};

      for (var i = 0; this.action.InFields && i < this.action.InFields.length; i++) {
        meta[this.action.InFields[i].ColumnName] = that.action.InFields[i];
      }

      if (this.action.InFields && this.action.InFields.length == 0 && this.action.InstanceOptional) {

        var payload = this.model;
        if (!payload) {
          payload = {};
        }

        if (this.finalModel && this.finalModel["id"]) {
          payload[this.action.OnType + "_id"] = this.finalModel["id"];
        }

        this.actionManager.doAction(this.action.OnType, this.action.Name, payload).then(function () {}, function () {});
        setTimeout(function () {
          that.$emit("cancel");
        }, 400);
      }
      console.log("action meta", meta);
      this.meta = meta;
    }
  },
  mounted: function mounted() {
    console.log("Mounted action view");
    this.init();
  },
  watch: {
    'action': function action(newValue) {
      console.log("ActionView: action changed");
      this.init();
    }
  }
});

/***/ }),
/* 311 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys__ = __webpack_require__(5);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_typeof__ = __webpack_require__(71);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_typeof___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_typeof__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_babel_runtime_core_js_promise__ = __webpack_require__(52);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_babel_runtime_core_js_promise___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_2_babel_runtime_core_js_promise__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3_element_ui__ = __webpack_require__(7);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3_element_ui___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_3_element_ui__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4_kingtable__ = __webpack_require__(514);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4_kingtable___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_4_kingtable__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_5_kingtable_utils__ = __webpack_require__(528);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_5_kingtable_utils___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_5_kingtable_utils__);









var YAML = __webpack_require__(461);
__webpack_require__(437);
__webpack_require__(438);

function generateID() {
  var length = 5;
  var text = "";
  var possible = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";

  for (var i = 0; i < length; i++) {
    text += possible.charAt(Math.floor(Math.random() * possible.length));
  }

  return "a" + text;
}

/* harmony default export */ __webpack_exports__["default"] = ({
  name: 'table-view',
  props: {
    jsonApi: {
      type: Object,
      required: true
    },
    autoload: {
      type: Boolean,
      required: false,
      default: true
    },
    jsonApiModelName: {
      type: String,
      required: true
    }
  },
  data: function data() {
    return {
      world: [],
      selectedWorld: null,
      selectedWorldColumns: [],
      tableData: [],
      selectedRow: {},
      data: {},
      jsonModel: {},
      dataMap: {},
      tableId: generateID(),
      inputs: []
    };
  },

  methods: {
    loadTable: function loadTable() {
      var that = this;
      that.jsonApi.findAll(that.selectedWorld).then(function (data) {
        console.log("got all data", that.jsonModel, that.selectedWorldColumns);

        var attributes = that.jsonModel.attributes;
        var tableAttribtues = {};
        that.selectedWorldColumns.map(function (e) {
          console.log(e, attributes[e], "make table attribute");
          var attribute = attributes[e];

          if (attribute instanceof Object) {
            console.log(attribute, "is an object");
            return null;
          }

          var value = null;
          console.log("choose for ", attributes[e]);
          switch (attributes[e]) {
            case "hidden":
              break;
            case "json":
              value = {
                name: titleCase(e),
                html: function html(item, value) {
                  var val = YAML.stringify(JSON.parse(value)).replace(/\n/g, "<br>").replace(/\t/g, "  ").replace(/ /g, "&nbsp;");
                  return "<div class='input-cell'>" + value + "</div>";
                }
              };
              break;
            case "datetime":
              value = {
                name: titleCase(e),
                html: function html(item, value) {
                  if (value) {
                    return "<div class='input-cell'>" + value + "</div>";
                  }
                  return "<div class='input-cell'></div>";
                }
              };
              break;
            case "truefalse":
              value = {
                name: titleCase(e),
                html: function html(item, value) {
                  console.log("choose for truefaluse vaule", value);

                  value = value.toLowerCase();
                  if (value == "true" || value == "1") {
                    value = true;
                  } else if (value == "false" || value == "0") {
                    value = false;
                  }

                  if (value) {
                    return "<div class='input-cell'><input type=\"checkbox\" checked></div>";
                  }
                  return "<div class='input-cell'><input type=\"checkbox\"></div>";
                }
              };
              break;
            default:
              value = {
                name: titleCase(e),
                html: function html(item, value) {
                  if (value) {
                    return "<div class='input-cell'>" + value + "</div>";
                  }
                  return "<div class='input-cell'></div>";
                }
              };
          }
          if (value) {
            value.hidden = false, value.secret = false, tableAttribtues[e] = value;
          }
        });

        var keys = that.selectedWorldColumns;
        var attrs = {};
        for (var i = 0; i < keys.length; i++) {
          attrs[keys[i]] = {
            name: titleCase(keys[i])
          };
        }

        console.log("array data", attrs, tableAttribtues);
        var table = new __WEBPACK_IMPORTED_MODULE_4_kingtable___default.a({
          data: data.data,

          collectionName: titleCase(that.selectedWorld),

          id: "table-" + that.tableId,
          idProperty: 'id',
          onFetchDone: function onFetchDone(res) {
            console.log("on fetch done", res);
            res = res.data;
            return res;
          },
          element: document.getElementById("" + that.tableId),
          columns: tableAttribtues,
          columnDefault: {
            name: "",
            type: "text",
            sortable: true,
            allowSearch: true,
            hidden: true,
            secret: true,
            format: undefined },
          getTableData: function getTableData() {
            console.log("Get table data for ", arguments);
            return new __WEBPACK_IMPORTED_MODULE_2_babel_runtime_core_js_promise___default.a(function (resolve, reject) {
              that.jsonApi.findAll(that.selectedWorld).then(function (data) {
                console.log("resolving promise of table data", data);
                resolve(data.data);
              }, function (err) {
                reject(err);
              });
            });
          },
          fields: [],
          events: {
            "click .delete": function clickDelete(e, item) {},
            "click td": function clickTd(e, item) {
              console.log("item clicked", item);
            },
            "blur td input": function blurTdInput(e, item) {
              console.log("item blur", item);
            },
            "click .pagination-button.pagination-bar-refresh.oi": function clickPaginationButtonPaginationBarRefreshOi(e) {
              console.log("refresh clicked", arguments);
            }
          }
        });
        table.on("hard:refresh", function (filters) {
          console.log("call for hard refresh ");
          that.jsonApi.findAll(that.selectedWorld).then(function (data) {
            console.log("call for hard refresh completed");
            table.data = data.data;
          });
        });

        table.render();
      });
    },
    onAction: function onAction(action, data) {
      console.log("on action", action, data);
      var that = this;
      if (action === "view-item") {
        this.$refs.vuetable.toggleDetailRow(data.id);
      } else if (action === "edit-item") {
        this.$emit("editRow", data);
      } else if (action === "go-item") {

        this.$router.push({
          name: "Instance",
          params: {
            tablename: data["__type"],
            refId: data["id"]
          }
        });
      } else if (action === "delete-item") {
        this.jsonApi.destroy(this.selectedWorld, data.id).then(function () {
          that.setTable(that.selectedWorld);
        });
      }
    },

    titleCase: function titleCase(str) {
      return str.replace(/[-_]/g, " ").split(' ').map(function (w) {
        return w[0].toUpperCase() + w.substr(1).toLowerCase();
      }).join(' ');
    },
    onCellClicked: function onCellClicked(data, field, event) {
      console.log('cellClicked 1: ', data, this.selectedWorld);

      console.log("this router", data["id"]);
    },
    trueFalseView: function trueFalseView(value) {
      console.log("Render", value);
      return value === "1" ? '<span class="fa fa-check"></span>' : '<span class="fa fa-times"></span>';
    },
    onPaginationData: function onPaginationData(paginationData) {
      console.log("set pagifnation method", paginationData, this.$refs.pagination);
      this.$refs.pagination.setPaginationData(paginationData);
    },
    onChangePage: function onChangePage(page) {
      console.log("cnage pge", page, __WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_typeof___default()(this.$refs.vuetable));
      if (typeof this.$refs.vuetable !== "undefined") {
        this.$refs.vuetable.changePage(page);
      }
    },
    saveRow: function saveRow(row) {
      var that = void 0;
      console.log("save row", row);
      if (data.id) {
        that = this;
        that.jsonApi.update(this.selectedWorld, row).then(function () {
          that.setTable(that.selectedWorld);
          that.showAddEdit = false;
        });
      } else {
        that = this;
        that.jsonApi.create(this.selectedWorld, row).then(function () {
          that.setTable(that.selectedWorld);
          that.showAddEdit = false;
        });
      }
    },
    edit: function edit(row) {
      this.$parent.emit("editRow", row);
    },
    setTable: function setTable(tableName) {
      var that = this;
      that.selectedWorldColumns = {};
      that.tableData = [];
      that.showAddEdit = false;
      that.reloadData(tableName);
    },
    reloadData: function reloadData(tableName) {
      var that = this;

      if (!tableName) {
        tableName = that.selectedWorld;
      }

      if (!tableName) {
        alert("setting selected world to null");
      }

      that.selectedWorld = tableName;
      var jsonModel = that.jsonApi.modelFor(tableName);
      if (!jsonModel) {
        console.error("Failed to find json api model for ", tableName);
        that.$notify({
          type: "error",
          message: "This is out of reach.",
          title: "Unauthorized"
        });
        return;
      }
      console.log("selectedWorldColumns", that.selectedWorldColumns);
      that.selectedWorldColumns = jsonModel["attributes"];

      setTimeout(function () {
        try {
          that.$refs.vuetable.changePage(1);
          that.$refs.vuetable.reinit();
        } catch (e) {
          console.log("probably table doesnt exist yet", e);
        }
      }, 16);
    }
  },
  mounted: function mounted() {
    var that = this;
    that.selectedWorld = that.jsonApiModelName;
    var jsonModel = that.jsonApi.modelFor(that.jsonApiModelName);
    console.log("Mounted TableView for ", that.jsonApiModelName, jsonModel);
    that.jsonModel = jsonModel;
    if (!jsonModel) {
      console.error("Failed to find json api model for ", that.jsonApiModelName);
      return;
    }
    that.selectedWorldColumns = __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default()(jsonModel["attributes"]);
    that.loadTable();
  },

  watch: {}

});

/***/ }),
/* 312 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });


/* harmony default export */ __webpack_exports__["default"] = ({
  props: {
    rowData: {
      type: Object,
      required: true
    },
    rowIndex: {
      type: Number
    }
  },
  methods: {
    itemAction: function itemAction(action, data, index) {
      console.log('custom-actions: ' + action, data.name, index);
    }
  }
});

/***/ }),
/* 313 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty__ = __webpack_require__(41);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_json_stringify__ = __webpack_require__(21);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_json_stringify___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_json_stringify__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_babel_runtime_core_js_object_keys__ = __webpack_require__(5);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_babel_runtime_core_js_object_keys___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_2_babel_runtime_core_js_object_keys__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__plugins_worldmanager__ = __webpack_require__(8);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4_element_ui__ = __webpack_require__(7);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4_element_ui___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_4_element_ui__);




var _props$data$created$c;


var markdown_renderer = __webpack_require__(184)();


/* harmony default export */ __webpack_exports__["default"] = (_props$data$created$c = {
  props: {
    model: {
      type: Object,
      required: true
    },
    showAll: {
      type: Boolean,
      required: false,
      default: true
    },
    jsonApi: {
      type: Object,
      required: true
    },
    jsonApiModelName: {
      type: String,
      required: true
    },
    renderNextLevel: {
      type: Boolean,
      required: false,
      default: false
    }
  },
  data: function data() {
    return {
      meta: {},
      metaMap: {},
      activeTabName: "first",
      editData: null,
      attributes: null,
      visible2: false,
      normalFields: [],
      imageFields: [],
      relatedData: {},
      selectedTableColumns: null,
      rowBeingEdited: null,
      relations: [],
      showAddEdit: false,
      imageMap: {},
      relationFinder: {},
      truefalse: []
    };
  },
  created: function created() {},

  computed: {},
  methods: {
    saveRow: function saveRow(relatedRow) {
      var that = this;
      console.log("Save from row being edited", relatedRow);
      if (!this.showAll) {
        console.log("not the parent");
        this.$emit("saveRelatedRow", relatedRow);
      } else {
        console.log("start to save this row", that.jsonApiModelName, that.relations);

        var typeName = that.jsonApiModelName + "_" + that.jsonApiModelName + "_id_has_" + relatedRow["type"] + "_" + relatedRow["type"] + "_id",
            relatedRow;
        console.log("typename is", typeName);
        that.jsonApi.update(typeName, relatedRow).then(function (r) {
          that.$notify.success("Added " + relation.type);

          that.$refs[relation.name].reloadData();
        }, function (err) {
          that.$notify.error(err);
        });
      }
    },
    editPermission: function editPermission() {
      this.showAddEdit = true;
      this.selectedTableColumns = {
        "permission": {
          "Name": "permission",
          "ColumnName": "permission",
          "ColumnType": "value",
          "DataType": "int(11)"
        }
      };
      this.rowBeingEdited = this.model;
    },
    initiateDelete: function initiateDelete() {

      if (!this.showAll) {
        console.log("not the parent", this.model);
        this.$emit("deleteRow", this.model);
      } else {
        console.log("start to delete this row", this.model, this.showAll);
      }
    },
    loadFailed: function loadFailed(relation) {
      console.log("relation not loaded", relation);
      relation.failed = true;
    },
    getRelationByName: function getRelationByName(name) {
      for (var i = 0; i < this.relations.length; i++) {
        if (this.relations[i].name == name) {
          return this.relations[i];
        }
      }
      return null;
    },
    deleteRow: function deleteRow(colName, rowToDelete) {
      console.log("call to delete row", arguments);
    },
    addRow: function addRow(colName, newRow) {
      var relation = this.getRelationByName(colName);
      if (relation == null) {
        console.log("relation not found: ", colName);
        return;
      }

      var that = this;

      __WEBPACK_IMPORTED_MODULE_3__plugins_worldmanager__["a" /* default */].getColumnKeys(newRow.type, function (newRowTypeAttributes) {

        console.log("newRowTypeAttributes for ", newRow.type, newRowTypeAttributes, newRow);

        if (newRowTypeAttributes.ColumnModel[that.jsonApiModelName + "_id"] && newRowTypeAttributes.ColumnModel[that.jsonApiModelName + "_id"]["jsonApi"] === "hasOne") {
          newRow.data[that.jsonApiModelName + "_id"] = {
            type: that.jsonApiModelName,
            id: that.model["id"]
          };
        }

        if (!newRow.data["id"]) {
          that.jsonApi.create(newRow.type, newRow.data).then(function (newRowResult) {
            that.patchObjectAddRelation(colName, relation, newRowResult.id);
          });
        } else {
          that.patchObjectAddRelation(colName, relation, newRow.data.id);
        }
      });
    },

    patchObjectAddRelation: function patchObjectAddRelation(colName, relation, newRowId) {
      var that = this;
      console.log("add to existing object", newRowId);
      var patchObject = {};

      if (that.meta["attributes"][colName]["jsonApi"] == "hasMany") {
        patchObject[relation.name] = [{
          id: newRowId,
          type: relation.type
        }];
      } else {
        patchObject[relation.name] = {
          id: newRowId,
          type: relation.type
        };
      }

      patchObject["id"] = that.model["id"];

      console.log("patch object", patchObject);
      that.jsonApi.update(that.jsonApiModelName, patchObject).then(function (r) {
        that.$notify.success("Added " + relation.type);

        that.$refs[relation.name].reloadData();
      }, function (err) {
        that.$notify.error(err);
      });
    },
    titleCase: function titleCase(str) {
      return str.replace(/[-_]/g, " ").trim().split(' ').map(function (w) {
        return w[0].toUpperCase() + w.substr(1).toLowerCase();
      }).join(' ');
    },
    reloadData: function reloadData(relation) {},
    init: function init() {
      var that = this;
      console.log("data for detailed row ", this.model);

      this.meta = this.jsonApi.modelFor(this.jsonApiModelName);

      this.attributes = this.meta["attributes"];
      this.truefalse = [];
      this.imageFields = [];
      var attributes = this.meta["attributes"];

      var normalFields = [];
      that.relations = [];

      var columnKeys = __WEBPACK_IMPORTED_MODULE_2_babel_runtime_core_js_object_keys___default()(attributes);

      for (var i = 0; i < columnKeys.length; i++) {
        var colName = columnKeys[i];

        var item = {
          name: colName,
          value: this.model[colName]
        };

        var type = attributes[colName];
        if (typeof type == "string") {
          type = {
            type: type
          };
        }

        item.type = type.type;
        item.valueType = type.columnType;
        var columnNameTitleCase = this.titleCase(item.name);
        item.label = columnNameTitleCase;
        item.title = columnNameTitleCase;
        item.style = "";


        if (item.valueType == "entity") {

          (function (item) {

            var columnName = item.name;
            columnNameTitleCase = item.name;

            var builderStack = that.jsonApi.one(that.jsonApiModelName, that.model["id"]).all(item.name);
            var finder = builderStack.builderStack;
            builderStack.builderStack = [];


            try {
              var relationJsonApiModel = that.jsonApi.modelFor(item.type);

              if (item.type == "user_account" || item.type == "usergroup") {

                that.relations.push({
                  name: columnName,
                  title: item.title,
                  finder: finder,
                  label: item.label,
                  type: item.type,
                  failed: false,
                  jsonModelAttrs: relationJsonApiModel
                });
              } else {

                that.relations.unshift({
                  name: columnName,
                  title: item.title,
                  finder: finder,
                  label: item.label,
                  failed: false,
                  type: item.type,
                  jsonModelAttrs: relationJsonApiModel
                });
              }
            } catch (e) {
              console.log("Model for ", item.type, "not found");
            }
          })(item);

          continue;
        } else if (item.type == "truefalse") {
          this.truefalse.push(item);
          continue;
        }

        if (item.type.indexOf("image.") == 0) {
          this.imageFields.push(item);
          continue;
        }

        if (item.type == "datetime") {
          continue;
        }

        if (item.type == "hidden") {
          continue;
        }

        if (item.type == "json") {
          item.originalValue = item.value;
          item.value = "";
          item.style = "width: 100%; min-height: 20px;";
        }

        if (item.type == "markdown") {
          item.originalValue = item.value;
          item.value = markdown_renderer.render(item.originalValue);
        }

        if (item.name == "reference_id") {
          continue;
        }

        if (item.name == "password") {
          continue;
        }
        if (item.name == "created_at") {
          continue;
        }
        if (item.name == "updated_at") {
          continue;
        }

        if (item.name == "status") {
          continue;
        }

        if (item.type == "label") {
          normalFields.unshift(item);
        } else {
          normalFields.push(item);
        }
      }

      this.normalFields = normalFields;

      setTimeout(function () {
        $('.menu .item').tab();
      }, 600);
    }
  } }, __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_props$data$created$c, "created", function created() {

  this.init();

  var that = this;
  setTimeout(function () {
    for (var i = 0; i < that.normalFields.length; i++) {
      var field = that.normalFields[i];
      if (field.type == "json") {
        try {
          field.formattedValue = __WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_json_stringify___default()(JSON.parse(field.originalValue), null, 4);
        } catch (e) {
          console.log("Value is not proper json");
        }
      }
    }
  }, 400);
}), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_props$data$created$c, "watch", {
  "model": function model() {
    this.init();
  }
}), _props$data$created$c);

/***/ }),
/* 314 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_vue_form_generator__ = __webpack_require__(34);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_vue_form_generator___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_vue_form_generator__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_element_ui__ = __webpack_require__(7);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_element_ui___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_element_ui__);





/* harmony default export */ __webpack_exports__["default"] = ({
  components: { DatePicker: __WEBPACK_IMPORTED_MODULE_1_element_ui__["DatePicker"] },
  mixins: [__WEBPACK_IMPORTED_MODULE_0_vue_form_generator__["abstractField"]],
  data: function data() {
    return {
      editorOptions: {}
    };
  },
  mounted: function mounted() {},

  methods: {
    formatValueToModel: function formatValueToModel(d1111) {
      console.log("formatValueToModel", d1111);
      return d1111;
    }
  },
  watch: {
    'value': function value(newValue) {}
  }
});

/***/ }),
/* 315 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_vue_form_generator__ = __webpack_require__(34);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_vue_form_generator___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_vue_form_generator__);




/* harmony default export */ __webpack_exports__["default"] = ({
  mixins: [__WEBPACK_IMPORTED_MODULE_0_vue_form_generator__["abstractField"]],
  data: function data() {
    return {
      editorOptions: {}
    };
  },
  mounted: function mounted() {},

  methods: {
    formatValueToModel: function formatValueToModel(d1111) {
      console.log("formatValueToModel", d1111);
      return d1111;
    }
  },
  watch: {
    'value': function value(newValue) {}
  }
});

/***/ }),
/* 316 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_json_stringify__ = __webpack_require__(21);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_json_stringify___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_json_stringify__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_vue_form_generator__ = __webpack_require__(34);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_vue_form_generator___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_vue_form_generator__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_vue2_ace__ = __webpack_require__(684);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_vue2_ace___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_2_vue2_ace__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3_brace_theme_chrome__ = __webpack_require__(338);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3_brace_theme_chrome___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_3_brace_theme_chrome__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4_brace_mode_markdown__ = __webpack_require__(337);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4_brace_mode_markdown___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_4_brace_mode_markdown__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_5__plugins_jsonapi__ = __webpack_require__(11);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_6_jsoneditor__ = __webpack_require__(462);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_6_jsoneditor___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_6_jsoneditor__);










__webpack_require__(435);

/* harmony default export */ __webpack_exports__["default"] = ({
  mixins: [__WEBPACK_IMPORTED_MODULE_1_vue_form_generator__["abstractField"]],
  data: function data() {
    return {
      fileList: [],
      useAce: false,
      mode: 'none',
      initValue: null,
      options: {
        fontSize: 18,
        wrap: true
      }
    };
  },
  components: {
    editor: __WEBPACK_IMPORTED_MODULE_2_vue2_ace___default.a
  },
  updated: function updated() {},
  mounted: function mounted() {
    window.ace.require = function (mode) {
      return false;
    };
    var that = this;
    setTimeout(function () {
      var startVal = that.value;


      try {
        var t = JSON.parse(startVal);
        startVal = __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_json_stringify___default()(t, null, 2);
        that.value = startVal;
      } catch (e) {}

      var schema = void 0;

      console.log("field json schema", that.schema);

      __WEBPACK_IMPORTED_MODULE_5__plugins_jsonapi__["a" /* default */].findAll("json_schema", {
        filter: that.schema.inputType
      }).then(function (e) {
        e = e.data;
        if (e.length > 0) {
          var schema = {};
          try {
            schema = JSON.parse(e[0].json_schema);
          } catch (e) {
            console.log("Failed to parse json schema", e);
            return;
          }
          that.useAce = false;
          setTimeout(function () {
            var element = document.getElementById('jsonEditor');
            console.log("schema", schema, element);

            try {
              var startValNew = JSON.parse(startVal);
              startVal = startValNew;
            } catch (e) {}

            var editor = new JSONEditor(element, {
              startval: startVal,
              schema: schema,
              theme: 'bootstrap3'
            });
            editor.on('change', function () {
              console.log("Json data updated", editor.getValue());
              var val = editor.getValue();
              if (!val) {
                that.value = null;
              } else {
                that.value = __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_json_stringify___default()(editor.getValue());
              }
            });
          }, 1000);
        }
        console.log("got json schema", e);
      });

      console.log("this is new");
      if (false) {
        try {
          var json = JSON.parse(startVal);
          if (json instanceof Object) {
            var container = document.getElementById("jsonEditor");
            var editor = new Jsoneditor(container, {
              onChange: function onChange() {
                that.value = _JSON$stringify(editor.get());
              }
            });
            editor.set(json);
            return;
          }
        } catch (e) {
          console.log("Failed to init json editor", e);
        }
      }

      if (false) {} else {
        if (!that.value) {
          that.value = "";
        }
        schema = {};
        that.useAce = true;
        that.initValue = that.value;
        that.$on('editor-update', function (newValue) {
          console.log("Value  updated", newValue);
          that.value = newValue;
        });
      }
    }, 500);
  },

  methods: {
    updated: function updated() {
      console.log("editor adsflkj asdf", arguments);
    }
  },
  watch: {
    mode: function mode(newMode) {
      var that = this;
      var startVal = this.value;
      switch (newMode) {
        case "ace":
          break;
        case "je":
          var json = JSON.parse(startVal);
          if (json instanceof Object) {
            setTimeout(function () {
              var container = document.getElementById("jsonEditor");
              var editor = new __WEBPACK_IMPORTED_MODULE_6_jsoneditor___default.a(container, {
                onChange: function onChange() {
                  that.value = __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_json_stringify___default()(editor.get());
                }
              });
              editor.set(json);
            }, 500);
          }
          break;
      }
      console.log("mode changes", arguments);
    },
    initValue: function initValue(newValue) {
      this.value = newValue;
    }
  }
});

/***/ }),
/* 317 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_vue_form_generator__ = __webpack_require__(34);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_vue_form_generator___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_vue_form_generator__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_element_ui__ = __webpack_require__(7);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_element_ui___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_element_ui__);





/* harmony default export */ __webpack_exports__["default"] = ({
  components: { Upload: __WEBPACK_IMPORTED_MODULE_1_element_ui__["Upload"] },
  mixins: [__WEBPACK_IMPORTED_MODULE_0_vue_form_generator__["abstractField"]],
  data: function data() {
    return {
      fileList: []
    };
  },
  mounted: function mounted() {
    console.log("File upload initial value: ", this.value);
    setTimeout(function () {
      var $input = $("input[type=file]");
      if ($input && $input.length > 0) {
        $input.css("display", "none");
      }
    }, 100);
  },

  methods: {
    handlePreview: function handlePreview() {
      console.log("handle preview", arguments);
    },
    handleRemove: function handleRemove(file, filelist) {
      console.log("handle remove", file, filelist);
      var fileNameToRemove = file.name;
      var indexToRemove = -1;

      if (!this.value) {
        this.value = [];
      }

      for (var i = 0; i < this.value.length; i++) {
        if (this.value[i].name == fileNameToRemove) {
          var indexToRemove = i;
        }
      }
      if (indexToRemove > -1) {
        this.value.splice(indexToRemove, 1);
      }
    },
    processFile: function processFile(file, filelist) {
      console.log("provided schema", this.schema, file.raw);

      var expectedFileType = this.schema.inputType;
      if (expectedFileType !== "*") {
        var allTypes = expectedFileType.split("|");

        var fileName = file.raw.name;
        var fileNameParts = fileName.split(".");
        var fileExtension = "";
        if (fileNameParts.length > 1) {
          fileExtension = fileNameParts[fileNameParts.length - 1];
        }

        var isFileTypeOkay = allTypes.indexOf(fileExtension) > -1;

        if (!isFileTypeOkay) {

          for (var i = 0; i < filelist.length; i++) {
            if (filelist[i].uid == file.uid) {
              filelist.splice(i, 1);
              break;
            }
          }

          this.$message.error('Please select a ' + expectedFileType + ' file. You are uploading: ' + file.raw.type);
          return isFileTypeOkay;
        }
      }

      var that = this;
      console.log("process file arguments", arguments, file, filelist);
      that.value = [];
      for (var i = 0; i < filelist.length; i++) {
        var name = filelist[i].name;
        var type = filelist[i].raw.type;
        var reader = new FileReader();
        reader.onload = function (theFile, type) {
          return function (e) {
            that.value.push({
              name: theFile,
              file: e.target.result,
              type: type
            });
          };
        }(name, type);
        reader.readAsDataURL(filelist[i].raw);
      }
    }
  }
});

/***/ }),
/* 318 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys__ = __webpack_require__(5);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_vue_form_generator__ = __webpack_require__(34);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_vue_form_generator___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_vue_form_generator__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_element_ui__ = __webpack_require__(7);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_element_ui___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_2_element_ui__);






/* harmony default export */ __webpack_exports__["default"] = ({
  components: { DatePicker: __WEBPACK_IMPORTED_MODULE_2_element_ui__["DatePicker"] },
  mixins: [__WEBPACK_IMPORTED_MODULE_1_vue_form_generator__["abstractField"]],
  data: function data() {
    return {
      activeTabName: 'user',
      editorOptions: {},
      guestValue: {},
      ownerValue: {},
      groupValue: {},
      permissionStructure: {
        None: 0,
        Peek: 1 << 0,
        ReadStrict: 1 << 1,
        CreateStrict: 1 << 2,
        UpdateStrict: 1 << 3,
        DeleteStrict: 1 << 4,
        ExecuteStrict: 1 << 5,
        ReferStrict: 1 << 6,
        Read: 1 << 1 | 1 << 0,
        Refer: 1 << 6 | 1 << 1 | 1 << 0,
        Create: 1 << 2 | 1 << 1 | 1 << 0,
        Update: 1 << 3 | 1 << 1 | 1 << 0,
        Delete: 1 << 4 | 1 << 1 | 1 << 0,
        Execute: 1 << 5 | 1 << 0,
        CRUD: 1 << 0 | 1 << 1 | 1 << 2 | 1 << 3 | 1 << 4 | 1 << 6
      },
      parsedGuestPermission: {
        canPeek: false,
        canRead: false,
        canCreate: false,
        canUpdate: false,
        canDelete: false,
        canRefer: false,
        canReadStrict: false,
        canCreateStrict: false,
        canUpdateStrict: false,
        canDeleteStrict: false,
        canReferStrict: false,
        canCRUD: false,
        canExecute: false,
        canExecuteStrict: false
      },
      parsedOwnerPermission: {
        canPeek: false,
        canRead: false,
        canCreate: false,
        canUpdate: false,
        canDelete: false,
        canRefer: false,
        canReadStrict: false,
        canCreateStrict: false,
        canUpdateStrict: false,
        canDeleteStrict: false,
        canReferStrict: false,
        canCRUD: false,
        canExecute: false,
        canExecuteStrict: false
      },
      parsedGroupPermission: {
        canPeek: false,
        canRead: false,
        canCreate: false,
        canUpdate: false,
        canDelete: false,
        canRefer: false,
        canReadStrict: false,
        canCreateStrict: false,
        canUpdateStrict: false,
        canDeleteStrict: false,
        canReferStrict: false,
        canCRUD: false,
        canExecute: false,
        canExecuteStrict: false
      }
    };
  },
  mounted: function mounted() {
    var that = this;
    setTimeout(function () {
      console.log("permission value", that.value);
      var permissionValue = that.value;
      that.guestValue = permissionValue % 1000;
      permissionValue = parseInt(permissionValue / 1000);
      that.groupValue = permissionValue % 1000;
      permissionValue = parseInt(permissionValue / 1000);
      that.ownerValue = permissionValue % 1000;
      permissionValue = parseInt(permissionValue / 1000);
      console.log("Owner, group, guest", that.ownerValue, that.groupValue, that.guestValue);
      that.parsedGuestPermission = that.parsePermission(that.guestValue);
      that.parsedOwnerPermission = that.parsePermission(that.ownerValue);
      that.parsedGroupPermission = that.parsePermission(that.groupValue);
    }, 200);
  },

  methods: {
    setValue: function setValue(obj, newValue) {
      var keys = __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default()(obj);
      for (var i = 0; i < keys.length; i++) {
        if (newValue === undefined) {
          obj[keys[i]] = !obj[keys[i]];
        } else {
          obj[keys[i]] = newValue;
        }
      }
    },
    clearAll: function clearAll() {
      switch (this.activeTabName) {
        case "user":
          this.setValue(this.parsedOwnerPermission, false);
          break;
        case "group":
          this.setValue(this.parsedGroupPermission, false);
          break;
        case "guest":
          this.setValue(this.parsedGuestPermission, false);
          break;
      }
    },
    enableAll: function enableAll() {
      switch (this.activeTabName) {
        case "user":
          this.setValue(this.parsedOwnerPermission, true);
          break;
        case "group":
          this.setValue(this.parsedGroupPermission, true);
          break;
        case "guest":
          this.setValue(this.parsedGuestPermission, true);
          break;
      };
    },
    toggleSelectionAll: function toggleSelectionAll() {
      switch (this.activeTabName) {
        case "user":
          this.setValue(this.parsedOwnerPermission);
          break;
        case "group":
          this.setValue(this.parsedGroupPermission);
          break;
        case "guest":
          this.setValue(this.parsedGuestPermission);
          break;
      };
    },
    updatePermissionValue: function updatePermissionValue() {
      var ownerPermission = this.makePermission(this.parsedOwnerPermission);
      var guestPermission = this.makePermission(this.parsedGuestPermission);
      var groupPermission = this.makePermission(this.parsedGroupPermission);
      console.log("owner permission", ownerPermission);
      console.log("guest permission", guestPermission);
      console.log("group permission", groupPermission);

      this.value = ownerPermission * 1000 * 1000 + groupPermission * 1000 + guestPermission;
      console.log("updated permission value to ", this.value);
    },
    makePermission: function makePermission(permissionObject) {

      var value = 0;
      var perms = __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default()(this.permissionStructure);
      for (var i = 0; i < perms.length; i++) {
        var permissionName = perms[i];
        var permission = this.permissionStructure[permissionName];


        if (permissionObject["can" + permissionName]) {
          value = value | permission;
        }
      }

      return value;
    },
    parsePermission: function parsePermission(val) {
      var res = {
        canPeek: (val & this.permissionStructure.Peek) == this.permissionStructure.Peek,
        canRead: (val & this.permissionStructure.Read) == this.permissionStructure.Read,
        canCreate: (val & this.permissionStructure.Create) == this.permissionStructure.Create,
        canUpdate: (val & this.permissionStructure.Update) == this.permissionStructure.Update,
        canDelete: (val & this.permissionStructure.Delete) == this.permissionStructure.Delete,
        canRefer: (val & this.permissionStructure.Refer) == this.permissionStructure.Refer,
        canReadStrict: (val & this.permissionStructure.ReadStrict) == this.permissionStructure.ReadStrict,
        canCreateStrict: (val & this.permissionStructure.CreateStrict) == this.permissionStructure.CreateStrict,
        canUpdateStrict: (val & this.permissionStructure.UpdateStrict) == this.permissionStructure.UpdateStrict,
        canDeleteStrict: (val & this.permissionStructure.DeleteStrict) == this.permissionStructure.DeleteStrict,
        canReferStrict: (val & this.permissionStructure.ReferStrict) == this.permissionStructure.ReferStrict,
        canCRUD: (val & this.permissionStructure.CRUD) == this.permissionStructure.CRUD,
        canExecute: (val & this.permissionStructure.Execute) == this.permissionStructure.Execute,
        canExecuteStrict: (val & this.permissionStructure.ExecuteStrict) == this.permissionStructure.ExecuteStrict
      };
      console.log("parsed permission", res);
      return res;
    }
  },
  watch: {
    'parsedGuestPermission': {
      handler: function handler(newValue) {
        console.log("guest value updated", this.parsedGuestPermission);
        this.updatePermissionValue();
      },
      deep: true
    },
    'parsedOwnerPermission': {
      handler: function handler(newValue) {
        console.log("owner value updated", this.parsedGuestPermission);
        this.updatePermissionValue();
      },
      deep: true
    },
    'parsedGroupPermission': {
      handler: function handler(newValue) {
        console.log("group value updated", this.parsedGuestPermission);
        this.updatePermissionValue();
      },
      deep: true
    }
  }
});

/***/ }),
/* 319 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys__ = __webpack_require__(5);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_element_ui__ = __webpack_require__(7);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_element_ui___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_element_ui__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__plugins_worldmanager__ = __webpack_require__(8);






/* harmony default export */ __webpack_exports__["default"] = ({
  name: 'table-view',
  filters: {
    titleCase: function titleCase(str) {
      return str.replace(/[-_]/g, " ").split(' ').map(function (w) {
        return w[0].toUpperCase() + w.substr(1).toLowerCase();
      }).join(' ');
    }
  },
  props: {
    jsonApi: {
      type: Object,
      required: true
    },
    jsonApiRelationName: {
      type: String,
      required: false
    },
    autoload: {
      type: Boolean,
      required: false,
      default: false
    },
    jsonApiModelName: {
      type: String,
      required: true
    },
    finder: {
      type: Array,
      required: true
    },
    model: {
      type: Object,
      required: false
    }
  },
  data: function data() {
    return {
      selectedWorld: null,
      selectedWorldColumns: [],
      tableData: [],
      meta: null,
      showSelect: true,
      selectedRow: {},
      displayData: [],
      showAddEdit: false,
      css: {
        table: {
          tableClass: 'table table-striped table-bordered',
          ascendingIcon: 'fa fa-sort-alpha-desc',
          descendingIcon: 'fa fa-sort-alpha-asc',
          handleIcon: 'fa fa-wrench'
        },
        pagination: {
          wrapperClass: "pagination pull-right",
          activeClass: "btn-primary",
          disabledClass: "disabled",
          pageClass: "btn btn-border",
          linkClass: "btn btn-border",
          icons: {
            first: "fa fa-backward",
            prev: "fa fa-chevron-left",
            next: "fa fa-chevron-right",
            last: "fa fa-forward"
          }
        }
      }
    };
  },

  methods: {
    saveRelatedRow: function saveRelatedRow(relatedRow) {
      var that = this;
      console.log("save row from list view", relatedRow);

      that.$emit("saveRow", relatedRow);
    },
    deleteRow: function deleteRow(rowToDelete) {
      var that = this;
      console.log("Delete row from list view", rowToDelete, that.finder);

      that.jsonApi.builderStack = [];

      for (var i = 0; i < that.finder.length - 1; i++) {
        that.jsonApi.builderStack.push(that.finder[i]);
      }

      var top = that.finder[that.finder.length - 1];

      that.jsonApi.relationships().all(top.model).destroy([{
        "type": rowToDelete["__type"],
        "id": rowToDelete["id"]
      }]).then(function (e) {
        that.reloadData();
      }, function () {
        that.reloadData();
        that.failed();
      });
    },
    saveRow: function saveRow(obj) {
      var that = this;
      var res = { data: obj, type: this.jsonApiModelName };
      this.$emit("addRow", this.jsonApiRelationName, res);
      this.showAddEdit = false;
      setTimeout(function () {
        console.log("reload data");
        that.reloadData();
      }, 1000);
    },
    cancel: function cancel() {
      this.showAddEdit = false;
    },
    onPaginationData: function onPaginationData(paginationData) {
      console.log("set pagifnation method", paginationData, this.$refs.pagination);
      this.$refs.pagination.setPaginationData(paginationData);
    },
    onChangePage: function onChangePage(page) {
      var that = this;
      console.log("change pge", page);
      that.jsonApi.builderStack = that.finder;
      that.jsonApi.get({
        page: {
          number: page,
          size: 10
        }
      }).then(that.success, that.failed);
    },
    reloadData: function reloadData() {
      var that = this;
      console.log("reload data", that.selectedWorld, that.finder);

      that.jsonApi.builderStack = that.finder;
      that.jsonApi.get({
        page: {
          number: 1,
          size: 10
        }
      }).then(that.success, that.failed);
    },
    success: function success(data) {
      console.log("data loaded", data.links, data.data);
      this.onPaginationData(data.links);
      data = data.data;
      var that = this;
      that.tableData = data;
      that.$emit("onLoadSuccess", this.jsonApiRelationName, data);
    },
    failed: function failed() {
      this.tableData = [];
      console.log("data load failed", arguments);
      this.$emit("onLoadFailure");
    }
  },
  mounted: function mounted() {
    var that = this;

    __WEBPACK_IMPORTED_MODULE_2__plugins_worldmanager__["a" /* default */].getColumnKeys(that.jsonApiModelName, function (cols) {
      that.meta = cols.ColumnModel;
      var cols = __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default()(that.meta);

      that.selectedWorld = that.jsonApiModelName;
      that.selectedWorldColumns = __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default()(that.meta);

      if (that.autoload) {
        that.reloadData();
      }
    });
  }
});

/***/ }),
/* 320 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys__ = __webpack_require__(5);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_vue_form_generator__ = __webpack_require__(34);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_vue_form_generator___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_vue_form_generator__);






/* harmony default export */ __webpack_exports__["default"] = ({
  props: {
    model: {
      type: Object,
      required: false,
      default: function _default() {
        return {};
      }
    },
    title: {
      type: String,
      required: false,
      default: ""
    },
    hideTitle: {
      type: Boolean,
      required: false,
      default: false
    },
    hideCancel: {
      type: Boolean,
      required: false,
      default: false
    },
    meta: {
      type: Object,
      required: true
    }
  },
  components: {
    "vue-form-generator": __WEBPACK_IMPORTED_MODULE_1_vue_form_generator___default.a.component
  },
  data: function data() {
    return {
      formModel: null,
      formValue: {},
      loading: false,
      relations: [],
      hasPermissionField: false
    };
  },
  methods: {
    loadLast: function loadLast() {},
    setRelation: function setRelation(item) {
      console.log("save relation", item);

      var meta = this.meta[item.name];

      if (meta.jsonApi == "hasOne") {

        this.model[item.name] = {
          type: meta.ColumnType,
          id: item.id
        };
      } else {
        this.model[item.name] = [{
          type: meta.ColumnType,
          id: item.id
        }];
      }
    },
    getTextInputType: function getTextInputType(columnMeta) {
      var inputType = columnMeta.ColumnType;
      console.log("get text input type for ", columnMeta);
      if (inputType.indexOf(".") > 0) {
        var inputTypeParts = inputType.split(".");
        if (inputTypeParts[0] == "file") {
          inputTypeParts.shift();
          return inputTypeParts.join(".");
        } else if (inputTypeParts[0] == "audio") {
          inputTypeParts.shift();
          return inputTypeParts.join(".");
        } else if (inputTypeParts[0] == "video") {
          inputTypeParts.shift();
          return inputTypeParts.join(".");
        } else if (inputTypeParts[0] == "image") {
          inputTypeParts.shift();
          return inputTypeParts.join(".");
        } else if (inputTypeParts[0] == "json") {
          inputTypeParts.shift();
          return inputTypeParts.join(".");
        }
      }

      if (["json", "yaml"].indexOf(columnMeta.ColumnType) > -1) {
        console.log("get text input type for json ", this.model);
        return columnMeta.ColumnName;
      }

      switch (inputType) {
        case "hidden":
          inputType = "hidden";
          break;
        case "entity":
          inputType = columnMeta.type;
          break;
        case "password":
          inputType = "password";
          break;
        case "measurement":
          inputType = "number";
          break;
        case "date":
          inputType = "date";
          break;
        case "time":
          inputType = "time";
          break;
        case "datetime":
          inputType = "datetime";
          break;
        case "content":
          inputType = "";
          break;
        default:
          inputType = "text";
          break;
      }
      return inputType;
    },
    getInputType: function getInputType(columnMeta) {
      console.log("get input type for", columnMeta);
      var inputType = columnMeta.ColumnType;
      if (inputType.indexOf(".") > 0) {
        var inputTypeParts = inputType.split(".");
        if (inputTypeParts[0] == "file") {
          return "fileUpload";
        }
        if (inputTypeParts[0] == "video") {
          return "fileUpload";
        }
        if (inputTypeParts[0] == "audio") {
          return "fileUpload";
        }
        if (inputTypeParts[0] == "image") {
          return "fileUpload";
        }
      }

      if (columnMeta.ColumnName == "default_permission" || columnMeta.ColumnName == "permission") {
        return "permissionInput";
      }

      switch (inputType) {
        case "truefalse":
          inputType = "fancyCheckBox";
          break;
        case "entity":
          inputType = "selectOneOrMore";
          break;
        case "date":
          inputType = "dateSelect";
          break;
        case "measurement":
          inputType = "input";
          break;
        case "content":
          inputType = "textArea";
          break;
        case "json":
          inputType = "jsonEditor";
          break;
        case "yaml":
          inputType = "textArea";
          break;
        case "html":
          inputType = "textArea";
          break;
        case "markdown":
          inputType = "textArea";
          break;
        default:
          inputType = "input";
          break;
      }
      return inputType;
    },

    saveRow: function saveRow() {
      var that = this;
      console.log("save row", this.model);

      this.loading = true;
      setTimeout(function () {
        that.loading = false;
      }, 3000);
      this.$emit('save', this.model);
    },
    cancel: function cancel() {
      this.$emit('cancel');
    },
    titleCase: function titleCase(str) {
      if (!str) {
        return str;
      }
      return str.replace(/[-_]/g, " ").trim().split(' ').map(function (w) {
        return w[0].toUpperCase() + w.substr(1).toLowerCase();
      }).join(' ');
    },
    init: function init() {

      var that = this;
      var formFields = [];
      console.log("that mode", that.model);
      that.formValue = that.model;

      console.log("model form for ", this.meta);
      var columnsKeys = __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default()(this.meta);
      that.formModel = {};

      var skipColumns = ["reference_id", "id", "updated_at", "created_at", "deleted_at", "user_id", "usergroup_id"];

      var foreignKeys = [];

      formFields = columnsKeys.map(function (columnName) {
        if (skipColumns.indexOf(columnName) > -1) {
          return null;
        }

        var columnMeta = that.meta[columnName];
        columnMeta.ColumnName = columnName;
        var columnLabel = that.titleCase(columnMeta.Name);

        if (columnMeta.columnType && !columnMeta.ColumnType) {
          columnMeta.ColumnType = columnMeta.columnType;
        }

        if (columnMeta.ColumnType == "hidden") {
          return null;
        }

        if (columnMeta.ColumnName == "permission") {}

        if (!that.model["reference_id"]) {

          if (!that.model[columnMeta.ColumnName] && columnMeta.DefaultValue) {
            if (columnMeta.DefaultValue[0] == "'") {
              that.model[columnMeta.ColumnName] = columnMeta.DefaultValue.substring(1, columnMeta.DefaultValue.length - 1);
            } else {
              that.model[columnMeta.ColumnName] = columnMeta.DefaultValue;
            }
          }
        }

        if (columnMeta.ColumnType == "truefalse") {
          that.model[columnMeta.ColumnName] = that.model[columnMeta.ColumnName] === "1" || that.model[columnMeta.ColumnName] === 1;
        }

        if (columnMeta.ColumnType == "date") {
          var parseTime = Date.parse(that.model[columnMeta.ColumnName]);
          if (!isNaN(parseTime)) {
            console.log("parsed time is not nan", parseTime);
            that.model[columnMeta.ColumnName] = new Date(parseTime);
          }
        }

        var inputType = that.getInputType(columnMeta);
        var textInputType = that.getTextInputType(columnMeta);

        console.log("Add column model ", columnName, columnMeta, that.model[columnMeta.ColumnName]);

        var resVal = {
          type: inputType,
          inputType: textInputType,
          label: columnLabel,
          model: columnMeta.ColumnName,
          name: columnName,
          id: "id",
          readonly: false,
          value: columnMeta.DefaultValue,
          featured: true,
          disabled: false,
          required: !columnMeta.IsNullable,
          "default": columnMeta.DefaultValue,
          validator: null,
          onChanged: function onChanged(model, newVal, oldVal, field) {},
          onValidated: function onValidated(model, errors, field) {
            if (errors.length > 0) console.warn("Validation error in Name field! Errors:", errors);
          }
        };
        console.log("check column meta for entity", columnMeta);
        if (columnMeta.ColumnType == "entity") {
          if (columnMeta.jsonApi == "hasOne") {
            resVal.value = that.model[resVal.ColumnName];
            resVal.multiple = false;
            foreignKeys.push(resVal);
          } else {
            resVal.value = that.model[resVal.ColumnName];
            resVal.multiple = false;
          }
          return null;
        }

        return resVal;
      }).filter(function (e) {
        return !!e;
      });

      console.log("all form fields", formFields, foreignKeys);
      that.formModel.fields = formFields;
      that.relations = foreignKeys;

      if (formFields.length + foreignKeys.length == 0) {}
    }
  },
  mounted: function mounted() {
    this.init();
  },
  watch: {
    "model": function model() {
      var that = this;
      console.log("ModelForm: model changed", that.model);
      this.init();
    },
    "meta": function meta() {
      var that = this;
      console.log("ModelForm: meta changed", that.meta);
      this.init();
    }

  }
});

/***/ }),
/* 321 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys__ = __webpack_require__(5);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_underscore__ = __webpack_require__(603);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_underscore___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_underscore__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_axios__ = __webpack_require__(31);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_axios___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_2_axios__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__plugins_worldmanager__ = __webpack_require__(8);







window._ = __WEBPACK_IMPORTED_MODULE_1_underscore___default.a;

/* harmony default export */ __webpack_exports__["default"] = ({
  name: 'recline-view',
  props: {
    jsonApi: {
      type: Object,
      required: true
    },
    autoload: {
      type: Boolean,
      required: false,
      default: true
    },
    jsonApiModelName: {
      type: String,
      required: true
    },
    finder: {
      type: Array,
      required: true
    },
    viewMode: {
      type: String,
      required: false,
      default: "card"
    }
  },
  data: function data() {
    return {
      world: [],
      selectedWorld: null,
      selectedWorldColumns: [],
      tableData: [],
      selectedRow: {},
      multiView: null,
      explorerDiv: null
    };
  },

  methods: {
    onAction: function onAction(action, data) {
      console.log("on action", action, data);
      var that = this;
      if (action === "view-item") {
        this.$refs.vuetable.toggleDetailRow(data.id);
      } else if (action === "edit-item") {
        this.$emit("editRow", data);
      } else if (action === "go-item") {

        this.$router.push({
          name: "Instance",
          params: {
            tablename: data["__type"],
            refId: data["id"]
          }
        });
      } else if (action === "delete-item") {
        this.jsonApi.destroy(this.selectedWorld, data.id).then(function () {
          that.setTable(that.selectedWorld);
        });
      }
    },

    titleCase: function titleCase(str) {
      return str.replace(/[-_]/g, " ").split(' ').map(function (w) {
        return w[0].toUpperCase() + w.substr(1).toLowerCase();
      }).join(' ');
    },
    saveRow: function saveRow(data) {
      var that = void 0;
      console.log("save row", data);
      if (data.id) {
        that = this;
        that.jsonApi.update(this.selectedWorld, data).then(function () {
          that.setTable(that.selectedWorld);
          that.showAddEdit = false;
        });
      } else {
        that = this;
        that.jsonApi.create(this.selectedWorld, data).then(function () {
          that.setTable(that.selectedWorld);
          that.showAddEdit = false;
        });
      }
    },
    edit: function edit(row) {
      this.$parent.emit("editRow", row);
    },
    setTable: function setTable(tableName) {
      var that = this;
      console.log("Set table in tableview by [setTable] ", tableName, that.finder);
      that.selectedWorldColumns = {};
      that.tableData = [];
      that.showAddEdit = false;
      that.reloadData(tableName);
    },
    createMultiView: function createMultiView(dataset, state) {
      var that = this;
      console.log("that selected world columns", that.selectedWorldColumns);
      var dateColumns = [];
      var columnNames = __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default()(that.selectedWorldColumns);
      for (var i = 0; i < columnNames.length; i++) {
        var columnName = columnNames[i];

        if (columnName == "deleted_at") {
          continue;
        }

        if (columnName == "updated_at") {
          continue;
        }

        var columnType = that.selectedWorldColumns[columnName];
        if (columnType == "datetime" || columnType == "date" || columnType == "time" || columnType == "timestamp") {

          if (columnName == "created_at") {
            dateColumns.push(columnName);
          } else {
            dateColumns.unshift(columnName);
          }
        }
      }
      console.log('date column', dateColumns);

      var reload = false;
      if (that.multiView) {
        that.multiView.remove();
        that.multiView = null;
        reload = true;
      }

      var $el = $('<div />');
      $el.appendTo(that.explorerDiv);

      var timeline = new recline.View.Timeline({
        model: dataset,
        state: {
          startField: dateColumns[0],
          endField: dateColumns[1]
        }
      });

      timeline.convertRecord = function (record, fields) {
        var attrs = record.attributes;
        var objTitle = window.chooseTitle(attrs);
        console.log("convert 1record title", record, objTitle);

        return {
          "startDate": attrs[dateColumns[0]],
          "endDate": attrs[dateColumns[1]],
          "headline": objTitle,
          "text": attrs["description"],
          "tag": []
        };
        var out = this._convertRecord(record);
        if (out) {
          out.headline = record.get('height').toString();
        }
        console.log("out is ", out);
        return out;
      };

      var views = [{
        id: 'grid',
        label: 'Grid',
        view: new recline.View.SlickGrid({
          model: dataset,
          state: {
            gridOptions: {
              editable: true,

              enabledDelRow: true,

              enableReOrderRow: true,
              autoEdit: false,
              forceFitColumns: true,
              enableCellNavigation: true
            },
            columnsEditor: [{ column: 'date', editor: Slick.Editors.Date }, { column: 'date-time', editor: Slick.Editors.Date }, { column: 'title', editor: Slick.Editors.Text }]
          }
        })
      }, {
        id: 'graph',
        label: 'Graph',
        view: new recline.View.Graph({
          model: dataset

        })
      }, {
        id: 'map',
        label: 'Map',
        view: new recline.View.Map({
          model: dataset
        })
      }, {
        id: "timeline",
        label: "Timeline",
        view: timeline
      }];

      var multiView = new recline.View.MultiView({
        model: dataset,
        el: $el,
        state: state,
        views: views
      });
      return multiView;
    },
    reloadData: function reloadData(tableName) {
      var that = this;
      console.log("Reload data in tableview by [reloadData]", tableName, that.finder);

      if (!tableName) {
        tableName = that.selectedWorld;
      }

      if (!tableName) {
        alert("setting selected world to null");
      }

      that.selectedWorld = tableName;
      var jsonModel = that.jsonApi.modelFor(tableName);
      if (!jsonModel) {
        console.error("Failed to find json api model for ", tableName);
      }
      console.log("selectedWorldColumns", that.selectedWorldColumns);
      that.selectedWorldColumns = jsonModel["attributes"];


      that.explorerDiv = $('.data-explorer-here');
      that.explorerDiv.html("");

      var options = {
        enableColumnReorder: false
      };

      that.createDataset(function (dataset) {
        that.dataset = dataset;
        that.multiView = that.createMultiView(that.dataset);
        that.dataset.fetch();
        that.dataset.records.bind('all', function (name, obj) {
          console.log(name, obj);

          switch (name) {
            case "change":
              that.saveRow(obj.attributes);
              break;
            case "destroy":

              that.jsonApi.destroy(that.selectedWorld, obj.id).then(function () {});
              break;
          }
        });
      });
    },
    createDataset: function createDataset(callback) {
      var that = this;
      __WEBPACK_IMPORTED_MODULE_3__plugins_worldmanager__["a" /* default */].getReclineModel(that.jsonApiModelName, function (reclineModel) {
        console.log("columns", reclineModel);

        recline.Backend = recline.Backend || {};
        recline.Backend.JsonAPI = recline.Backend.JsonAPI || {};
        (function (my) {
          my.__type__ = 'jsonapi';
          var Deferred = typeof jQuery !== "undefined" && jQuery.Deferred || __WEBPACK_IMPORTED_MODULE_1_underscore___default.a.Deferred;

          my.fetch = function (config) {
            var dfd = new Deferred();
            console.log("backend fetch ", arguments);

            that.jsonApi.builderStack = that.finder;
            that.jsonApi.get({
              page: {
                number: 1,
                size: 100
              }
            }).then(function (result) {
              dfd.resolve([]);
            }, function () {
              that.$notify({
                type: "error",
                title: "Failed to fetch data",
                message: "Are you still logged in ?"
              });
              dfd.reject("Failed to fetch data: Are you still logged in ?");
            });

            return dfd.promise();
          };

          my.query = function (query) {
            var dfd = new Deferred();

            that.jsonApi.builderStack = that.finder;

            var sortOrder = query.sort;
            var sort = [];
            if (sortOrder && sortOrder.length > 0) {

              for (var y = 0; y < sortOrder.length; y++) {
                var field = sortOrder[y].field;
                var order = sortOrder[y].order;

                if (order == "desc") {
                  sort.push("-" + field);
                } else {
                  sort.push(field);
                }
              }
            }

            var _that$jsonApi$get$the = that.jsonApi.get({
              page: {
                number: query.from + 1,
                size: query.size
              },
              filter: query.q,
              sort: sort.length > 0 ? sort.join(",") : ""
            }).then(function (result) {
              console.log("here ");
              dfd.resolve({
                total: result.links.total,
                hits: result.data
              });
            }, function () {
              that.$notify({
                type: "error",
                title: "Failed to fetch data",
                message: "Are you still logged in ?"
              });
              dfd.reject("Failed to fetch data: Are you still logged in ?");
            }),
                data = _that$jsonApi$get$the.data,
                errors = _that$jsonApi$get$the.errors,
                meta = _that$jsonApi$get$the.meta,
                links = _that$jsonApi$get$the.links;

            console.log("backend query", arguments);
            return dfd.promise();
          };
        })(recline.Backend.JsonAPI);

        var dataset = new recline.Model.Dataset({
          fields: reclineModel,
          backend: 'jsonapi'
        });
        console.log("Dataset", dataset);
        callback(dataset);
      });
    }
  },
  mounted: function mounted() {
    var that = this;
    that.selectedWorld = that.jsonApiModelName;
    console.log("Mounted ReclineView for ", that.jsonApiModelName);
    var jsonModel = that.jsonApi.modelFor(that.jsonApiModelName);
    if (!jsonModel) {
      console.error("Failed to find json api model for ", that.jsonApiModelName);
      return;
    }
    that.selectedWorldColumns = __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default()(jsonModel["attributes"]);
    that.reloadData();
  },

  watch: {
    'finder': function finder(newFinder, oldFinder) {
      var that = this;
      console.log("finder updated in ", newFinder, oldFinder);
      setTimeout(function () {
        that.reloadData(that.selectedWorld);
      }, 100);
    }
  }
});

/***/ }),
/* 322 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_vue_form_generator__ = __webpack_require__(34);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_vue_form_generator___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_vue_form_generator__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__plugins_jsonapi__ = __webpack_require__(11);





/* harmony default export */ __webpack_exports__["default"] = ({
  mixins: [__WEBPACK_IMPORTED_MODULE_0_vue_form_generator__["abstractField"]],
  props: {
    model: {
      type: Object,
      required: false
    },
    schema: {
      type: Object,
      required: true
    }
  },
  data: function data() {
    return {
      formModel: null,
      loading: false,
      options: [],
      selectedItem: null
    };
  },
  methods: {
    formatValueToModel: function formatValueToModel(obj) {
      console.log("formatValueToModel", arguments);
      return {
        id: obj.id,
        type: obj.type
      };
    },

    addObject: function addObject() {
      var that = this;
      console.log("emit add object event", this.value);
      this.$emit("save", {
        name: that.schema.name,
        id: this.selectedItem.id
      });
    },
    remoteMethod: function remoteMethod(query) {
      console.log("remote method called", arguments);
      var that = this;
      this.loading = true;
      __WEBPACK_IMPORTED_MODULE_1__plugins_jsonapi__["a" /* default */].findAll(this.schema.inputType, {
        page: 1,
        size: 20,
        filter: query
      }).then(function (data) {
        data = data.data;
        console.log("remote method response", data);
        delete data["links"];
        for (var i = 0; i < data.length; i++) {
          data[i].label = window.chooseTitle(data[i]);
          data[i].value = data[i]["id"];
        }
        console.log("final result optiopsn", data);
        that.options = data;
        that.loading = false;
      });
    }
  },
  mounted: function mounted() {
    var that = this;
    that.selectedItem = that.model;
    console.log("select one or more value on mounted", that.value, that.schema.value);
    if (that.schema.multiple) {
      if (!(that.value instanceof Array)) {
        that.value = [that.value];
      }
    } else {}
    console.log("start select one or more", this.model, that.meta, that.value, this.schema);
  },
  watch: {
    'selectedItem': function selectedItem(to) {
      console.log("value change", to);
    }
  }
});

/***/ }),
/* 323 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys__ = __webpack_require__(5);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_json_stringify__ = __webpack_require__(21);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_json_stringify___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_json_stringify__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_babel_runtime_helpers_typeof__ = __webpack_require__(71);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_babel_runtime_helpers_typeof___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_2_babel_runtime_helpers_typeof__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3_element_ui__ = __webpack_require__(7);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3_element_ui___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_3_element_ui__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4_x_data_spreadsheet__ = __webpack_require__(728);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_5_jexcel__ = __webpack_require__(458);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_5_jexcel___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_5_jexcel__);









__webpack_require__(434);

/* harmony default export */ __webpack_exports__["default"] = ({
  name: 'table-view',
  props: {
    jsonApi: {
      type: Object,
      required: true
    },
    autoload: {
      type: Boolean,
      required: false,
      default: true
    },
    jsonApiModelName: {
      type: String,
      required: true
    },
    finder: {
      type: Array,
      required: true
    },
    viewMode: {
      type: String,
      required: false,
      default: "card"
    }
  },
  data: function data() {
    return {
      world: [],
      selectedWorld: null,
      selectedWorldColumns: [],
      tableData: [],
      sheet: null,
      selectedRow: {},
      css: {
        table: {
          tableClass: 'table table-striped table-bordered',
          ascendingIcon: 'fa fa-sort-alpha-desc',
          descendingIcon: 'fa fa-sort-alpha-asc',
          handleIcon: 'fa fa-wrench'
        },
        pagination: {
          wrapperClass: "pagination pull-right",
          activeClass: "btn-primary",
          disabledClass: "disabled",
          pageClass: "btn btn-border",
          linkClass: "btn btn-border",
          icons: {
            first: "fa fa-backward",
            prev: "fa fa-chevron-left",
            next: "fa fa-chevron-right",
            last: "fa fa-forward"
          }
        }
      }
    };
  },

  methods: {
    onAction: function onAction(action, data) {
      console.log("on action", action, data);
      var that = this;
      if (action === "view-item") {
        this.$refs.vuetable.toggleDetailRow(data.id);
      } else if (action === "edit-item") {
        this.$emit("editRow", data);
      } else if (action === "go-item") {

        this.$router.push({
          name: "Instance",
          params: {
            tablename: data["__type"],
            refId: data["id"]
          }
        });
      } else if (action === "delete-item") {
        this.jsonApi.destroy(this.selectedWorld, data.id).then(function () {
          that.setTable(that.selectedWorld);
        });
      }
    },

    titleCase: function titleCase(str) {
      return str.replace(/[-_]/g, " ").split(' ').map(function (w) {
        return w[0].toUpperCase() + w.substr(1).toLowerCase();
      }).join(' ');
    },
    onCellClicked: function onCellClicked(data, field, event) {
      console.log('cellClicked 1: ', data, this.selectedWorld);

      console.log("this router", data["id"]);
    },
    trueFalseView: function trueFalseView(value) {
      console.log("Render", value);
      return value === "1" ? '<span class="fa fa-check"></span>' : '<span class="fa fa-times"></span>';
    },
    onPaginationData: function onPaginationData(paginationData) {
      console.log("set pagifnation method", paginationData, this.$refs.pagination);
      this.$refs.pagination.setPaginationData(paginationData);
    },
    onChangePage: function onChangePage(page) {
      console.log("cnage pge", page, __WEBPACK_IMPORTED_MODULE_2_babel_runtime_helpers_typeof___default()(this.$refs.vuetable));
      if (typeof this.$refs.vuetable !== "undefined") {
        this.$refs.vuetable.changePage(page);
      }
    },
    saveRow: function saveRow(row) {
      var that = void 0;
      console.log("save row", row);
      if (data.created_at) {
        that = this;
        that.jsonApi.update(this.selectedWorld, row).then(function () {
          that.setTable(that.selectedWorld);
          that.showAddEdit = false;
        });
      } else {
        that = this;
        that.jsonApi.create(this.selectedWorld, row).then(function () {
          that.setTable(that.selectedWorld);
          that.showAddEdit = false;
        });
      }
    },
    edit: function edit(row) {
      this.$parent.emit("editRow", row);
    },
    setTable: function setTable(tableName) {
      var that = this;
      console.log("Set table in tableview by [setTable] ", tableName, that.finder);
      that.selectedWorldColumns = {};
      that.tableData = [];
      that.showAddEdit = false;
      that.reloadData(tableName);
    },
    reloadData: function reloadData(tableName) {
      var that = this;
      console.log("Reload data in tableview by [reloadData]", tableName, that.finder);

      if (!tableName) {
        tableName = that.selectedWorld;
      }

      if (!tableName) {
        alert("setting selected world to null");
      }

      that.selectedWorld = tableName;
      var jsonModel = that.jsonApi.modelFor(tableName);
      if (!jsonModel) {
        console.error("Failed to find json api model for ", tableName);
        that.$notify({
          type: "error",
          message: "This is out of reach.",
          title: "Unauthorized"
        });
        return;
      }
      console.log("selectedWorldColumns", that.selectedWorldColumns);
      that.selectedWorldColumns = jsonModel["attributes"];

      setTimeout(function () {
        try {
          if (that.viewMode == "card") {
            that.$refs.vuetable.reinit();
          } else {

            if (!that.sheet) {
              console.log("creating new spreadsheet");
            }

            console.log("load data for table");
            that.jsonApi.builderStack = that.finder;
            that.jsonApi.get({
              page: {
                number: 1,
                size: 1000
              }
            }).then(function (data) {

              var headers = [];
              var spreadSheetData = [];
              var rows = data.data;
              console.log("loaded data", data, spreadSheetData);

              for (var column in that.selectedWorldColumns) {
                if (column.endsWith("_id")) {
                  continue;
                }
                if (column.substring(0, 2) == "__") {
                  continue;
                }
                headers.push(column);
              }
              spreadSheetData.push(headers);
              var widths = [];
              var maxLength = [];
              for (var i = 0; i < rows.length; i++) {
                var row = [];
                for (var j in headers) {
                  var column = headers[j];

                  if (rows[i][column] instanceof Array) {
                    row.push(rows[i][column].join(","));
                  } else if (rows[i][column] instanceof Object) {
                    row.push(__WEBPACK_IMPORTED_MODULE_1_babel_runtime_core_js_json_stringify___default()(rows[i][column]));
                  } else {
                    row.push(rows[i][column]);
                  }
                  if (!maxLength[j] || maxLength[j] < new String(row[j]).length) {
                    maxLength[j] = row[j] ? new String(row[j]).length : 0;
                  }
                  j += 1;
                }
                spreadSheetData.push(row);
              }

              for (var i = 0; i < maxLength.length; i++) {
                if (maxLength[i] > 1000) {
                  maxLength[i] = 1000;
                }
                widths[i] = maxLength[i] * 3 + 100;
              }

              console.log("immediate load data", widths);

              var spreadsheet = __WEBPACK_IMPORTED_MODULE_5_jexcel___default()(that.$refs.tableViewDiv, {
                data: spreadSheetData,
                colWidths: widths
              });
            });
          }
        } catch (e) {
          console.log("probably table doesnt exist yet", e);
        }
      }, 36);
    }
  },
  mounted: function mounted() {
    var that = this;
    that.selectedWorld = that.jsonApiModelName;
    console.log("Mounted TableView for ", that.jsonApiModelName);
    var jsonModel = that.jsonApi.modelFor(that.jsonApiModelName);
    if (!jsonModel) {
      console.error("Failed to find json api model for ", that.jsonApiModelName);
      return;
    }
    that.selectedWorldColumns = __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default()(jsonModel["attributes"]);
    that.reloadData(that.selectedWorld);
  },

  watch: {
    'finder': function finder(newFinder, oldFinder) {
      var that = this;
      console.log("finder updated in ", newFinder, oldFinder);
      setTimeout(function () {
        that.reloadData(that.selectedWorld);
      }, 100);
    },
    'viewMode': function viewMode(newViewMode) {
      if (newViewMode == "table") {
        this.reloadData(this.selectedTable);
      }
    }
  }
});

/***/ }),
/* 324 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys__ = __webpack_require__(5);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_extends__ = __webpack_require__(27);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_extends___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_extends__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__plugins_jsonapi__ = __webpack_require__(11);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__plugins_actionmanager__ = __webpack_require__(10);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4__plugins_worldmanager__ = __webpack_require__(8);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_5__plugins_statsmanager__ = __webpack_require__(143);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_6_vuex__ = __webpack_require__(9);










/* harmony default export */ __webpack_exports__["default"] = ({
  data: function data() {
    return {
      worldActions: {},
      actionGroups: {},
      jsonApi: __WEBPACK_IMPORTED_MODULE_2__plugins_jsonapi__["a" /* default */],
      generateRandomNumbers: function generateRandomNumbers(numbers, max, min) {
        var a = [];
        for (var i = 0; i < numbers; i++) {
          a.push(Math.floor(Math.random() * (max - min + 1)) + max);
        }
        return a;
      },

      worlds: []
    };
  },

  computed: __WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_extends___default()({}, __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_6_vuex__["b" /* mapState */])(["query"]), {
    sortedWorldActions: function sortedWorldActions() {
      console.log("return sorted world actions", this.worldActions);
      var keys = __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default()(this.worldActions);

      keys.sort();

      var res = {};

      for (var key in keys) {
        res[key] = this.worldActions[key];
      }

      console.log("returning sorted worlds", res);
      return res;
    }
  }),
  methods: {
    stringToColor: function stringToColor(str) {
      return "#" + window.stringToColor(str);
    },
    reloadData: function reloadData() {
      var that = this;
      var newWorldActions = {};
      __WEBPACK_IMPORTED_MODULE_2__plugins_jsonapi__["a" /* default */].all("world").get({
        page: {
          number: 1,
          size: 200
        }
      }).then(function (worlds) {
        worlds = worlds.data;
        console.log("got worlds", worlds);
        that.worlds = worlds.map(function (e) {
          var parse = JSON.parse(e.world_schema_json);
          parse.Icon = e.icon;
          parse.Count = 0;
          return parse;
        }).filter(function (e) {
          console.log("filter ", e);
          return !e.IsHidden && !e.IsJoinTable && e.TableName.indexOf("_state") == -1;
        });
        that.worlds.forEach(function (w) {
          console.log("call stats", w);

          __WEBPACK_IMPORTED_MODULE_5__plugins_statsmanager__["a" /* default */].getStats(w.TableName, {
            column: ["count"]
          }).then(function (stats) {
            stats = stats.data;
            console.log("Stats received", stats);

            var rows = stats.data;
            var totalCount = rows[0]["count"];
            w.Count = totalCount;
          }, function (error) {
            console.log("Failed to query stats", error);
          });
        });

        var actionGroups = {
          System: [],
          User: []
        };
        console.log("worlds in dashboard", worlds);
        for (var i = 0; i < worlds.length; i++) {
          var tableName = worlds[i].table_name;
          var actions = __WEBPACK_IMPORTED_MODULE_3__plugins_actionmanager__["a" /* default */].getActions(tableName);

          if (!actions) {
            continue;
          }
          console.log("actions for ", tableName, actions);
          var actionKeys = __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default()(actions);
          for (var j = 0; j < actionKeys.length; j++) {
            var action = actions[actionKeys[j]];

            var onType = action.OnType;
            var onWorld = __WEBPACK_IMPORTED_MODULE_4__plugins_worldmanager__["a" /* default */].getWorldByName(onType);


            if (onWorld.is_hidden == "1") {
              actionGroups["System"].push(action);
            } else if (onWorld.table_name == "user_account") {
              actionGroups["User"].push(action);
            } else if (onWorld.table_name == "usergroup") {
              actionGroups["User"].push(action);
            } else {
              if (!newWorldActions[onWorld.table_name]) {
                newWorldActions[onWorld.table_name] = [];
              }
              newWorldActions[onWorld.table_name].push(action);
            }
          }
        }

        console.log("load world actions tabld");
        that.worldActions = newWorldActions;
        that.actionGroups = actionGroups;
      });
    }
  },
  updated: function updated() {
    document.getElementById("navbar-search-input").value = "";
  },

  watch: {
    query: function query(oldVal, newVal) {
      console.log("query change", arguments);
      this.reloadData();
    }
  },
  mounted: function mounted() {

    var that = this;
    that.$route.meta.breadcrumb = [{
      label: "Dashboard"
    }];
    this.reloadData();
  }
});

/***/ }),
/* 325 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys__ = __webpack_require__(5);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_extends__ = __webpack_require__(27);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_extends___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_extends__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__plugins_jsonapi__ = __webpack_require__(11);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__plugins_actionmanager__ = __webpack_require__(10);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4__plugins_worldmanager__ = __webpack_require__(8);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_5__plugins_statsmanager__ = __webpack_require__(143);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_6_vuex__ = __webpack_require__(9);










/* harmony default export */ __webpack_exports__["default"] = ({
  data: function data() {
    return {
      worldActions: {},
      actionGroups: {},
      jsonApi: __WEBPACK_IMPORTED_MODULE_2__plugins_jsonapi__["a" /* default */],
      generateRandomNumbers: function generateRandomNumbers(numbers, max, min) {
        var a = [];
        for (var i = 0; i < numbers; i++) {
          a.push(Math.floor(Math.random() * (max - min + 1)) + max);
        }
        return a;
      },

      worlds: []
    };
  },

  computed: __WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_extends___default()({}, __webpack_require__.i(__WEBPACK_IMPORTED_MODULE_6_vuex__["b" /* mapState */])(["query"]), {
    sortedWorldActions: function sortedWorldActions() {
      console.log("return sorted world actions", this.worldActions);
      var keys = __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default()(this.worldActions);

      keys.sort();

      var res = {};

      for (var key in keys) {
        res[key] = this.worldActions[key];
      }

      console.log("returning sorted worlds", res);
      return res;
    }
  }),
  methods: {
    stringToColor: function stringToColor(str) {

      return "#333";
    },
    reloadData: function reloadData() {
      var that = this;
      var newWorldActions = {};
      __WEBPACK_IMPORTED_MODULE_2__plugins_jsonapi__["a" /* default */].all("world").get({
        page: {
          number: 1,
          size: 200
        }
      }).then(function (worlds) {
        worlds = worlds.data;
        console.log("got worlds", worlds);
        that.worlds = worlds.map(function (e) {
          var parse = JSON.parse(e.world_schema_json);
          parse.Icon = parse.Icon == "" ? "fa-star" : parse.Icon;
          parse.Count = 0;
          return parse;
        }).filter(function (e) {
          console.log("filter ", e);
          return !e.IsHidden && !e.IsJoinTable && e.TableName.indexOf("_state") == -1;
        });
        that.worlds.forEach(function (w) {
          console.log("call stats", w);

          __WEBPACK_IMPORTED_MODULE_5__plugins_statsmanager__["a" /* default */].getStats(w.TableName, {
            column: ["count"]
          }).then(function (stats) {
            stats = stats.data;


            var rows = stats.data;
            var totalCount = rows[0]["count"];
            w.Count = totalCount;
          }, function (error) {
            console.log("Failed to query stats", error);
          });
        });

        that.worlds.sort(function (a, b) {

          var nameA = a.TableName;
          var nameB = b.TableName;
          if (nameA < nameB) return -1;
          if (nameA > nameB) return 1;
          return 0;
        });

        var actionGroups = {
          System: [],
          User: []
        };
        console.log("worlds in dashboard", worlds);
        for (var i = 0; i < worlds.length; i++) {
          var tableName = worlds[i].table_name;
          var actions = __WEBPACK_IMPORTED_MODULE_3__plugins_actionmanager__["a" /* default */].getActions(tableName);

          if (!actions) {
            continue;
          }
          console.log("actions for ", tableName, actions);
          var actionKeys = __WEBPACK_IMPORTED_MODULE_0_babel_runtime_core_js_object_keys___default()(actions);
          for (var j = 0; j < actionKeys.length; j++) {
            var action = actions[actionKeys[j]];

            var onType = action.OnType;
            var onWorld = __WEBPACK_IMPORTED_MODULE_4__plugins_worldmanager__["a" /* default */].getWorldByName(onType);


            if (onWorld.is_hidden == "1") {
              actionGroups["System"].push(action);
            } else if (onWorld.table_name == "user_account") {
              actionGroups["User"].push(action);
            } else if (onWorld.table_name == "usergroup") {
              actionGroups["User"].push(action);
            } else {
              if (!newWorldActions[onWorld.table_name]) {
                newWorldActions[onWorld.table_name] = [];
              }
              newWorldActions[onWorld.table_name].push(action);
            }
          }
        }

        console.log("load world actions tabld");
        that.worldActions = newWorldActions;
        that.actionGroups = actionGroups;
      });
    }
  },
  updated: function updated() {
    document.getElementById("navbar-search-input").value = "";
  },

  watch: {
    query: function query(oldVal, newVal) {
      console.log("query change", arguments);
      this.reloadData();
    }
  },
  mounted: function mounted() {

    var that = this;
    that.$route.meta.breadcrumb = [{
      label: "Dashboard"
    }];
    this.reloadData();
  }
});

/***/ }),
/* 326 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty__ = __webpack_require__(41);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_typeof__ = __webpack_require__(71);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_typeof___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_typeof__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_babel_runtime_core_js_object_keys__ = __webpack_require__(5);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_babel_runtime_core_js_object_keys___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_2_babel_runtime_core_js_object_keys__);




var _methods;

var markdown_renderer = __webpack_require__(184)();

/* harmony default export */ __webpack_exports__["default"] = ({
  props: {
    loadOnStart: {
      type: Boolean,
      default: true
    },
    apiUrl: {
      type: String,
      default: ''
    },
    apiMode: {
      type: Boolean,
      default: true
    },
    data: {
      type: Array,
      default: function _default() {
        return null;
      }
    },
    dataPath: {
      type: String,
      default: ''
    },
    paginationPath: {
      type: [String],
      default: 'links.pagination'
    },
    queryParams: {
      type: Object,
      default: function _default() {
        return {
          sort: 'sort',
          page: 'page',
          perPage: 'per_page'
        };
      }
    },
    appendParams: {
      type: Object,
      default: function _default() {
        return {};
      }
    },
    httpOptions: {
      type: Object,
      default: function _default() {
        return {};
      }
    },
    perPage: {
      type: Number,
      default: function _default() {
        return 10;
      }
    },
    sortOrder: {
      type: Array,
      default: function _default() {
        return [];
      }
    },
    multiSort: {
      type: Boolean,
      default: function _default() {
        return false;
      }
    },

    multiSortKey: {
      type: String,
      default: 'alt'
    },

    rowClassCallback: {
      type: [String, Function],
      default: ''
    },
    rowClass: {
      type: [String, Function],
      default: ''
    },
    detailRowComponent: {
      type: String,
      default: ''
    },
    detailRowTransition: {
      type: String,
      default: ''
    },
    trackBy: {
      type: String,
      default: 'id'
    },
    renderIcon: {
      type: Function,
      default: null
    },
    css: {
      type: Object,
      default: function _default() {
        return {
          tableClass: 'ui blue selectable celled stackable attached table',
          loadingClass: 'loading',
          ascendingIcon: 'blue chevron up icon',
          descendingIcon: 'blue chevron down icon',
          detailRowClass: 'vuecard-detail-row',
          handleIcon: 'grey sidebar icon'
        };
      }
    },
    minRows: {
      type: Number,
      default: 0
    },
    silent: {
      type: Boolean,
      default: false
    },
    jsonApi: {
      type: Object,
      default: null
    },
    finder: {
      type: Array,
      default: null
    },
    jsonApiModelName: {
      type: String,
      default: null
    }
  },
  data: function data() {
    return {
      eventPrefix: 'vuecard:',
      tableFields: [],
      tableData: null,
      tablePagination: null,
      actionSlot: null,
      currentPage: 1,
      selectedTo: [],
      visibleDetailRows: []
    };
  },
  created: function created() {
    this.normalizeFields();
    this.$nextTick(function () {
      this.emit1('initialized', this.tableFields);
    });

    if (this.apiMode && this.loadOnStart) {
      this.loadData();
    }
    if (this.apiMode == false && this.data.length > 0) {
      this.setData(this.data);
    }
  },

  computed: {
    useDetailRow: function useDetailRow() {
      if (this.tableData && this.tableData[0] && this.detailRowComponent !== '' && typeof this.tableData[0][this.trackBy] === 'undefined') {
        this.warn('You need to define unique row identifier in order for detail-row feature to work. Use `track-by` prop to define one!');
        return false;
      }

      return this.detailRowComponent !== '';
    },
    countVisibleFields: function countVisibleFields() {
      return this.tableFields.filter(function (field) {
        return field.visible;
      }).length;
    },

    lessThanMinRows: function lessThanMinRows() {
      if (this.tableData === null || this.tableData.length === 0) {
        return true;
      }
      return this.tableData.length < this.minRows;
    },
    blankRows: function blankRows() {
      if (this.tableData === null || this.tableData.length === 0) {
        return this.minRows;
      }
      if (this.tableData.length >= this.minRows) {
        return 0;
      }

      return this.minRows - this.tableData.length;
    }
  },
  methods: (_methods = {
    normalizeFields: function normalizeFields() {
      var that = this;

      var modelFor = this.jsonApi.modelFor(this.jsonApiModelName);


      if (!modelFor) {
        return;
      }
      this.fieldsData = modelFor["attributes"];
      this.fields = __WEBPACK_IMPORTED_MODULE_2_babel_runtime_core_js_object_keys___default()(this.fieldsData);


      this.tableFields = [];
      var self = this;
      var obj = void 0;
      this.fields.forEach(function (field, i) {
        var fieldType = that.fieldsData[field];

        field = {
          name: field,
          title: self.setTitle(field),
          callback: undefined,
          sortField: field
        };

        if (fieldType == "hidden") {
          field.visible = false;
        }

        if (fieldType == "encrypted") {
          field.visible = false;
        }
        if (fieldType.indexOf && fieldType.indexOf("image.") == 0) {
          field.callback = function (val, row) {
            console.log("render image on card", val);
            return "Image preview not available";
          };
        }

        if ((typeof fieldType === 'undefined' ? 'undefined' : __WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_typeof___default()(fieldType)) == "object") {
          field.visible = false;
        }

        if (fieldType === "truefalse") {
          field.callback = 'trueFalseView';
        }

        if (field.name == "updated_at") {
          field.visible = false;
        }

        if (field.name == "reference_id") {}

        if (field.name == "permission") {
          field.visible = false;
        }

        if (field.name == "status") {
          field.visible = false;
        }

        if (fieldType == "alias") {
          field.visible = false;
        }

        if (fieldType == "json") {
          field.visible = false;
        }

        if (fieldType == "truefalse") {
          field.visible = false;
        }

        if (fieldType == "content") {
          field.visible = false;
        }

        if (fieldType == "markdown") {
          field.callback = function (val, row) {
            return markdown_renderer.render(val);
          };
        }

        if (fieldType == "label") {
          field.callback = function (val, row) {
            return val;
          };
        }

        obj = {
          name: field.name,
          title: field.title === undefined ? self.setTitle(field.name) : field.title,
          sortField: field.sortField,
          titleClass: field.titleClass === undefined ? '' : field.titleClass,
          dataClass: field.dataClass === undefined ? '' : field.dataClass,
          callback: field.callback === undefined ? '' : field.callback,
          visible: field.visible === undefined ? true : field.visible
        };

        self.tableFields.push(obj);
      });
      self.actionSlot = {
        name: '__slot:actions',

        title: '',
        visible: true,
        titleClass: 'center aligned',
        dataClass: 'center aligned'
      };
    },
    setData: function setData(data) {
      this.apiMode = false;
      this.tableData = data;
    },
    titleCase: function titleCase(str) {
      return this.$parent.titleCase(str);
    },
    setTitle: function setTitle(str) {
      if (this.isSpecialField(str)) {
        return '';
      }

      return this.titleCase(str);
    },
    renderTitle: function renderTitle(field) {
      var title = typeof field.title === 'undefined' ? field.name.replace(/\.\_/g, ' ') : field.title;

      if (title.length > 0 && this.isInCurrentSortGroup(field)) {
        var style = 'opacity:' + this.sortIconOpacity(field) + ';position:relative;float:right';
        return title + ' ' + this.renderIconTag(['sort-icon', this.sortIcon(field)], 'style="' + style + '"');
      }

      return title;
    },
    isSpecialField: function isSpecialField(fieldName) {
      return fieldName.slice(0, 2) === '__';
    }
  }, __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'titleCase', function titleCase(str) {
    return str.replace(/[-_]/g, " ").split(' ').map(function (w) {
      return w[0].toUpperCase() + w.substr(1).toLowerCase();
    }).join(' ');
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'camelCase', function camelCase(str) {
    var delimiter = arguments.length > 1 && arguments[1] !== undefined ? arguments[1] : '_';

    var self = this;
    return str.split(delimiter).map(function (item) {
      return self.titleCase(item);
    }).join('');
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'notIn', function notIn(str, arr) {
    return arr.indexOf(str) === -1;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'loadData', function loadData() {
    var success = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : this.loadSuccess;
    var failed = arguments.length > 1 && arguments[1] !== undefined ? arguments[1] : this.loadFailed;

    var that = this;
    if (!this.apiMode) return;

    this.emit1('loading');

    this.httpOptions['params'] = this.getAllQueryParams();

    that.jsonApi.builderStack = this.finder;
    that.jsonApi.get(this.httpOptions["params"]).then(success, failed);
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'loadSuccess', function loadSuccess(response) {
    this.emit1('load-success', response);

    var body = this.transform(response);

    this.tableData = this.getObjectValue(body, this.dataPath, null);
    this.tablePagination = this.getObjectValue(body, this.paginationPath, null);

    if (this.tablePagination === null) {
      this.warn('vuecard: pagination-path "' + this.paginationPath + '" not found. ' + 'It looks like the data returned from the sever does not have pagination information ' + "or you may have set it incorrectly.\n" + 'You can explicitly suppress this warning by setting pagination-path="".');
    }

    var that = this;
    this.$nextTick(function () {
      that.emit1('pagination-data', this.tablePagination);
      that.emit1('loaded');
    });
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'loadFailed', function loadFailed(response) {
    console.error('load-error', response);
    this.emit1('load-error', response);
    this.emit1('loaded');
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'transform', function transform(data) {
    var func = 'transform';

    if (this.parentFunctionExists(func)) {
      return this.$parent[func].call(this.$parent, data);
    }

    return data;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'parentFunctionExists', function parentFunctionExists(func) {
    return func !== '' && typeof this.$parent[func] === 'function';
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'callParentFunction', function callParentFunction(func, args) {
    var defaultValue = arguments.length > 2 && arguments[2] !== undefined ? arguments[2] : null;

    if (this.parentFunctionExists(func)) {
      return this.$parent[func].call(this.$parent, args);
    }

    return defaultValue;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'emit1', function emit1(eventName, args) {
    this.$emit(eventName, args);
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'warn', function warn(msg) {
    if (!this.silent) {
      console.warn(msg);
    }
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'getAllQueryParams', function getAllQueryParams() {
    var params = {};
    params[this.queryParams.sort] = this.getSortParam();
    params[this.queryParams.page] = this.currentPage;
    params[this.queryParams.perPage] = this.perPage;

    for (var x in this.appendParams) {
      params[x] = this.appendParams[x];
    }

    return params;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'getSortParam', function getSortParam(sortOrder) {

    if (!this.sortOrder || this.sortOrder.field == '') {
      return '';
    }

    return this.sortOrder.map(function (sort) {
      return (sort.direction === 'desc' ? '' : '-') + sort.field;
    }).join(',');
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'getDefaultSortParam', function getDefaultSortParam() {
    var result = '';

    for (var i = 0; i < this.sortOrder.length; i++) {
      var fieldName = typeof this.sortOrder[i].sortField === 'undefined' ? this.sortOrder[i].field : this.sortOrder[i].sortField;

      result += fieldName + '|' + this.sortOrder[i].direction + (i + 1 < this.sortOrder.length ? ',' : '');
    }

    return result;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'extractName', function extractName(string) {
    return string.split(':')[0].trim();
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'extractArgs', function extractArgs(string) {
    return string.split(':')[1];
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'isSortable', function isSortable(field) {
    return !(typeof field.sortField === 'undefined');
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'isInCurrentSortGroup', function isInCurrentSortGroup(field) {
    return this.currentSortOrderPosition(field) !== false;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'currentSortOrderPosition', function currentSortOrderPosition(field) {
    if (!this.isSortable(field)) {
      return false;
    }

    for (var i = 0; i < this.sortOrder.length; i++) {
      if (this.fieldIsInSortOrderPosition(field, i)) {
        return i;
      }
    }

    return false;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'fieldIsInSortOrderPosition', function fieldIsInSortOrderPosition(field, i) {
    return this.sortOrder[i].field === field.name && this.sortOrder[i].sortField === field.sortField;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'orderBy', function orderBy(field, event) {
    if (!this.isSortable(field) || !this.apiMode) return;

    var key = this.multiSortKey.toLowerCase() + 'Key';

    if (this.multiSort && event[key]) {
      this.multiColumnSort(field);
    } else {
      this.singleColumnSort(field);
    }

    this.currentPage = 1;
    this.loadData();
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'multiColumnSort', function multiColumnSort(field) {
    var i = this.currentSortOrderPosition(field);

    if (i === false) {
      this.sortOrder.push({
        field: field.name,
        sortField: field.sortField,
        direction: 'asc'
      });
    } else {
      if (this.sortOrder[i].direction === 'asc') {
        this.sortOrder[i].direction = 'desc';
      } else {
        this.sortOrder.splice(i, 1);
      }
    }
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'singleColumnSort', function singleColumnSort(field) {
    if (this.sortOrder.length === 0) {
      this.clearSortOrder();
    }

    this.sortOrder.splice(1);

    if (this.fieldIsInSortOrderPosition(field, 0)) {
      this.sortOrder[0].direction = this.sortOrder[0].direction === 'asc' ? 'desc' : 'asc';
    } else {
      this.sortOrder[0].direction = 'asc';
    }
    this.sortOrder[0].field = field.name;
    this.sortOrder[0].sortField = field.sortField;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'clearSortOrder', function clearSortOrder() {
    this.sortOrder.push({
      field: '',
      sortField: '',
      direction: 'asc'
    });
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'sortIcon', function sortIcon(field) {
    var cls = '';
    var i = this.currentSortOrderPosition(field);

    if (i !== false) {
      cls = this.sortOrder[i].direction == 'asc' ? this.css.ascendingIcon : this.css.descendingIcon;
    }

    return cls;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'sortIconOpacity', function sortIconOpacity(field) {
    var max = 1.0,
        min = 0.3,
        step = 0.3;

    var count = this.sortOrder.length;
    var current = this.currentSortOrderPosition(field);

    if (max - count * step < min) {
      step = (max - min) / (count - 1);
    }

    var opacity = max - current * step;

    return opacity;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'hasCallback', function hasCallback(item) {
    return item.callback ? true : false;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'callCallback', function callCallback(field, item) {
    if (!this.hasCallback(field)) return;

    if (typeof field.callback == 'function') {
      return field.callback(this.getObjectValue(item, field.name));
    }

    var args = field.callback.split('|');
    var func = args.shift();

    if (typeof this.$parent[func] === 'function') {
      var value = this.getObjectValue(item, field.name);

      return args.length > 0 ? this.$parent[func].apply(this.$parent, [value].concat(args)) : this.$parent[func].call(this.$parent, value);
    }

    return null;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'getObjectValue', function getObjectValue(object, path, defaultValue) {
    defaultValue = typeof defaultValue === 'undefined' ? null : defaultValue;

    var obj = object;
    if (path.trim() != '') {
      var keys = path.split('.');
      keys.forEach(function (key) {
        if (obj !== null && typeof obj[key] !== 'undefined' && obj[key] !== null) {
          obj = obj[key];
        } else {
          obj = defaultValue;
        }
      });
    }
    return obj;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'toggleCheckbox', function toggleCheckbox(dataItem, fieldName, event) {
    var isChecked = event.target.checked;
    var idColumn = this.trackBy;

    if (dataItem[idColumn] === undefined) {
      this.warn('__checkbox field: The "' + this.trackBy + '" field does not exist! Make sure the field you specify in "track-by" prop does exist.');
      return;
    }

    var key = dataItem[idColumn];
    if (isChecked) {
      this.selectId(key);
    } else {
      this.unselectId(key);
    }
    this.emit1('vuecard:checkbox-toggled', isChecked, dataItem);
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'selectId', function selectId(key) {
    if (!this.isSelectedRow(key)) {
      this.selectedTo.push(key);
    }
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'unselectId', function unselectId(key) {
    this.selectedTo = this.selectedTo.filter(function (item) {
      return item !== key;
    });
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'isSelectedRow', function isSelectedRow(key) {
    return this.selectedTo.indexOf(key) >= 0;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'rowSelected', function rowSelected(dataItem, fieldName) {
    var idColumn = this.trackBy;
    var key = dataItem[idColumn];

    return this.isSelectedRow(key);
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'checkCheckboxesState', function checkCheckboxesState(fieldName) {
    if (!this.tableData) return;

    var self = this;
    var idColumn = this.trackBy;
    var selector = 'th.vuecard-th-checkbox-' + idColumn + ' input[type=checkbox]';
    var els = document.querySelectorAll(selector);

    if (els.forEach === undefined) els.forEach = function (cb) {
      [].forEach.call(els, cb);
    };

    var selected = this.tableData.filter(function (item) {
      return self.selectedTo.indexOf(item[idColumn]) >= 0;
    });

    if (selected.length <= 0) {
      els.forEach(function (el) {
        el.indeterminate = false;
      });
      return false;
    } else if (selected.length < this.perPage) {
        els.forEach(function (el) {
          el.indeterminate = true;
        });
        return true;
      } else {
          els.forEach(function (el) {
            el.indeterminate = false;
          });
          return true;
        }
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'toggleAllCheckboxes', function toggleAllCheckboxes(fieldName, event) {
    var self = this;
    var isChecked = event.target.checked;
    var idColumn = this.trackBy;

    if (isChecked) {
      this.tableData.forEach(function (dataItem) {
        self.selectId(dataItem[idColumn]);
      });
    } else {
      this.tableData.forEach(function (dataItem) {
        self.unselectId(dataItem[idColumn]);
      });
    }
    this.emit1('vuecard:checkbox-toggled-all', isChecked);
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'gotoPreviousPage', function gotoPreviousPage() {
    if (this.currentPage > 1) {
      this.currentPage--;
      this.loadData();
    }
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'gotoNextPage', function gotoNextPage() {
    if (this.currentPage < this.tablePagination.last_page) {
      this.currentPage++;
      this.loadData();
    }
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'gotoPage', function gotoPage(page) {
    if (page != this.currentPage && page > 0 && page <= this.tablePagination.last_page) {
      this.currentPage = page;
      this.loadData();
    }
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'isVisibleDetailRow', function isVisibleDetailRow(rowId) {
    return this.visibleDetailRows.indexOf(rowId) >= 0;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'showDetailRow', function showDetailRow(rowId) {
    if (!this.isVisibleDetailRow(rowId)) {
      this.visibleDetailRows.push(rowId);
    }
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'hideDetailRow', function hideDetailRow(rowId) {
    if (this.isVisibleDetailRow(rowId)) {
      this.visibleDetailRows.splice(this.visibleDetailRows.indexOf(rowId), 1);
    }
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'toggleDetailRow', function toggleDetailRow(rowId) {
    if (this.isVisibleDetailRow(rowId)) {
      this.hideDetailRow(rowId);
    } else {
      this.showDetailRow(rowId);
    }
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'showField', function showField(index) {
    if (index < 0 || index > this.tableFields.length) return;

    this.tableFields[index].visible = true;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'hideField', function hideField(index) {
    if (index < 0 || index > this.tableFields.length) return;

    this.tableFields[index].visible = false;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'toggleField', function toggleField(index) {
    if (index < 0 || index > this.tableFields.length) return;

    this.tableFields[index].visible = !this.tableFields[index].visible;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'renderIconTag', function renderIconTag(classes) {
    var options = arguments.length > 1 && arguments[1] !== undefined ? arguments[1] : '';

    return this.renderIcon === null ? '<i class="' + classes.join(' ') + '" ' + options + '></i>' : this.renderIcon(classes, options);
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'onRowClass', function onRowClass(dataItem, index) {
    if (this.rowClassCallback !== '') {
      this.warn('"row-class-callback" prop is deprecated, please use "row-class" prop instead.');
      return;
    }

    if (typeof this.rowClass === 'function') {
      return this.rowClass(dataItem, index);
    }

    return this.rowClass;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'onRowChanged', function onRowChanged(dataItem) {
    this.emit1('row-changed', dataItem);
    return true;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'onRowClicked', function onRowClicked(dataItem, event) {
    this.emit1(this.eventPrefix + 'row-clicked', dataItem, event);
    return true;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'onRowDoubleClicked', function onRowDoubleClicked(dataItem, event) {
    this.emit1(this.eventPrefix + 'row-dblclicked', dataItem, event);
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'onDetailRowClick', function onDetailRowClick(dataItem, event) {
    this.emit1(this.eventPrefix + 'detail-row-clicked', dataItem, event);
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'onCellClicked', function onCellClicked(dataItem, field, event) {
    this.emit1(this.eventPrefix + 'cell-clicked', dataItem, field, event);
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'onCellDoubleClicked', function onCellDoubleClicked(dataItem, field, event) {
    this.emit1(this.eventPrefix + 'cell-dblclicked', dataItem, field, event);
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'changePage', function changePage(page) {
    if (page === 'prev') {
      this.gotoPreviousPage();
    } else if (page === 'next') {
      this.gotoNextPage();
    } else {
      this.gotoPage(page);
    }
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'reload', function reload() {

    this.loadData();
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'refresh', function refresh() {
    this.currentPage = 1;
    this.loadData();
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'resetData', function resetData() {
    this.tableData = null;
    this.tablePagination = null;
    this.emit1('data-reset');
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'reinit', function reinit() {
    this.normalizeFields();
    this.$nextTick(function () {
      this.emit1('initialized', this.tableFields);
    });

    if (this.apiMode && this.loadOnStart) {
      this.loadData();
    }
    if (this.apiMode == false && this.data.length > 0) {
      this.setData(this.data);
    }
  }), _methods),
  watch: {
    'multiSort': function multiSort(newVal, oldVal) {
      if (newVal === false && this.sortOrder.length > 1) {
        this.sortOrder.splice(1);
        this.loadData();
      }
    },

    'apiUrl': function apiUrl(newVal, oldVal) {
      if (newVal !== oldVal) this.refresh();
    }
  }
});

/***/ }),
/* 327 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty__ = __webpack_require__(41);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_typeof__ = __webpack_require__(71);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_typeof___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_typeof__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_babel_runtime_core_js_object_keys__ = __webpack_require__(5);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2_babel_runtime_core_js_object_keys___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_2_babel_runtime_core_js_object_keys__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3_vue_virtual_scroll_list__ = __webpack_require__(683);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3_vue_virtual_scroll_list___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_3_vue_virtual_scroll_list__);




var _methods;



/* harmony default export */ __webpack_exports__["default"] = ({
  components: { 'virtual-list': __WEBPACK_IMPORTED_MODULE_3_vue_virtual_scroll_list___default.a },
  props: {
    loadOnStart: {
      type: Boolean,
      default: true
    },
    apiUrl: {
      type: String,
      default: ''
    },
    apiMode: {
      type: Boolean,
      default: true
    },
    data: {
      type: Array,
      default: function _default() {
        return null;
      }
    },
    dataPath: {
      type: String,
      default: ''
    },
    paginationPath: {
      type: [String],
      default: 'links.pagination'
    },
    queryParams: {
      type: Object,
      default: function _default() {
        return {
          sort: 'sort',
          page: 'page',
          perPage: 'per_page'
        };
      }
    },
    appendParams: {
      type: Object,
      default: function _default() {
        return {};
      }
    },
    httpOptions: {
      type: Object,
      default: function _default() {
        return {};
      }
    },
    perPage: {
      type: Number,
      default: function _default() {
        return 10;
      }
    },
    sortOrder: {
      type: Array,
      default: function _default() {
        return [];
      }
    },
    multiSort: {
      type: Boolean,
      default: function _default() {
        return false;
      }
    },

    multiSortKey: {
      type: String,
      default: 'alt'
    },

    rowClassCallback: {
      type: [String, Function],
      default: ''
    },
    rowClass: {
      type: [String, Function],
      default: ''
    },
    detailRowComponent: {
      type: String,
      default: ''
    },
    detailRowTransition: {
      type: String,
      default: ''
    },
    trackBy: {
      type: String,
      default: 'id'
    },
    renderIcon: {
      type: Function,
      default: null
    },
    css: {
      type: Object,
      default: function _default() {
        return {
          tableClass: 'ui blue selectable celled stackable attached table',
          loadingClass: 'loading',
          ascendingIcon: 'blue chevron up icon',
          descendingIcon: 'blue chevron down icon',
          detailRowClass: 'vuetable-detail-row',
          handleIcon: 'grey sidebar icon'
        };
      }
    },
    minRows: {
      type: Number,
      default: 0
    },
    silent: {
      type: Boolean,
      default: false
    },
    jsonApi: {
      type: Object,
      default: null
    },
    finder: {
      type: Array,
      default: null
    },
    jsonApiModelName: {
      type: String,
      default: null
    }
  },
  data: function data() {
    return {
      eventPrefix: 'vuetable:',
      tableFields: [],
      tableData: null,
      tablePagination: null,
      currentPage: 1,
      selectedTo: [],
      visibleDetailRows: []
    };
  },
  created: function created() {
    this.normalizeFields();
    this.$nextTick(function () {
      this.emit1('initialized', this.tableFields);
    });

    if (this.apiMode && this.loadOnStart) {
      this.loadData();
    }
    if (this.apiMode == false && this.data.length > 0) {
      this.setData(this.data);
    }
    var that = this;
  },

  computed: {
    useDetailRow: function useDetailRow() {
      if (this.tableData && this.tableData[0] && this.detailRowComponent !== '' && typeof this.tableData[0][this.trackBy] === 'undefined') {
        this.warn('You need to define unique row identifier in order for detail-row feature to work. Use `track-by` prop to define one!');
        return false;
      }

      return this.detailRowComponent !== '';
    },
    countVisibleFields: function countVisibleFields() {
      return this.tableFields.filter(function (field) {
        return field.visible;
      }).length;
    },

    lessThanMinRows: function lessThanMinRows() {
      if (this.tableData === null || this.tableData.length === 0) {
        return true;
      }
      return this.tableData.length < this.minRows;
    },
    blankRows: function blankRows() {
      if (this.tableData === null || this.tableData.length === 0) {
        return this.minRows;
      }
      if (this.tableData.length >= this.minRows) {
        return 0;
      }

      return this.minRows - this.tableData.length;
    }
  },
  methods: (_methods = {
    onScroll: function onScroll() {
      console.log("ddd");
    },
    normalizeFields: function normalizeFields() {
      var that = this;

      var modelFor = this.jsonApi.modelFor(this.jsonApiModelName);


      if (!modelFor) {
        return;
      }
      this.fieldsData = modelFor["attributes"];
      this.fields = __WEBPACK_IMPORTED_MODULE_2_babel_runtime_core_js_object_keys___default()(this.fieldsData);


      this.tableFields = [];
      var self = this;
      var obj = void 0;
      this.fields.forEach(function (field, i) {
        var fieldType = that.fieldsData[field];

        field = {
          name: field,
          title: self.setTitle(field),
          callback: undefined,
          sortField: field
        };

        if (fieldType == "hidden") {
          field.visible = false;
        }

        if (fieldType == "encrypted") {
          field.visible = false;
        }

        if ((typeof fieldType === 'undefined' ? 'undefined' : __WEBPACK_IMPORTED_MODULE_1_babel_runtime_helpers_typeof___default()(fieldType)) == "object") {
          field.visible = false;
        }

        if (fieldType === "truefalse") {
          field.callback = 'trueFalseView';
        }

        if (field.name == "updated_at") {
          field.visible = false;
        }

        if (field.name == "created_at") {
          field.visible = false;
        }

        if (field.name == "reference_id") {}

        if (field.name == "permission") {
          field.visible = false;
        }

        if (field.name == "status") {
          field.visible = false;
        }

        if (fieldType == "alias") {
          field.visible = false;
        }

        if (fieldType == "json") {
          field.visible = false;
        }

        if (fieldType == "truefalse") {
          field.visible = false;
        }

        if (fieldType == "content") {
          field.visible = false;
        }

        if (fieldType == "label") {
          field.callback = function (val, row) {
            return val;
          };
        }

        obj = {
          name: field.name,
          title: field.title === undefined ? self.setTitle(field.name) : field.title,
          sortField: field.sortField,
          titleClass: field.titleClass === undefined ? '' : field.titleClass,
          dataClass: field.dataClass === undefined ? '' : field.dataClass,
          callback: field.callback === undefined ? '' : field.callback,
          visible: field.visible === undefined ? true : field.visible
        };

        self.tableFields.push(obj);
      });
      self.tableFields.push({
        name: '__slot:actions',

        title: '',
        visible: true,
        titleClass: 'center aligned',
        dataClass: 'center aligned'
      });
    },
    setData: function setData(data) {
      this.apiMode = false;
      this.tableData = data;
    },
    titleCase: function titleCase(str) {
      return this.$parent.titleCase(str);
    },
    setTitle: function setTitle(str) {
      if (this.isSpecialField(str)) {
        return '';
      }

      return this.titleCase(str);
    },
    renderTitle: function renderTitle(field) {
      var title = typeof field.title === 'undefined' ? field.name.replace(/\.\_/g, ' ') : field.title;

      if (title.length > 0 && this.isInCurrentSortGroup(field)) {
        var style = 'opacity:' + this.sortIconOpacity(field) + ';position:relative;float:right';
        return title + ' ' + this.renderIconTag(['sort-icon', this.sortIcon(field)], 'style="' + style + '"');
      }

      return title;
    },
    isSpecialField: function isSpecialField(fieldName) {
      return fieldName.slice(0, 2) === '__';
    }
  }, __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'titleCase', function titleCase(str) {
    return str.replace(/[-_]/g, " ").split(' ').map(function (w) {
      return w[0].toUpperCase() + w.substr(1).toLowerCase();
    }).join(' ');
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'camelCase', function camelCase(str) {
    var delimiter = arguments.length > 1 && arguments[1] !== undefined ? arguments[1] : '_';

    var self = this;
    return str.split(delimiter).map(function (item) {
      return self.titleCase(item);
    }).join('');
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'notIn', function notIn(str, arr) {
    return arr.indexOf(str) === -1;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'loadData', function loadData() {
    var success = arguments.length > 0 && arguments[0] !== undefined ? arguments[0] : this.loadSuccess;
    var failed = arguments.length > 1 && arguments[1] !== undefined ? arguments[1] : this.loadFailed;

    var that = this;
    if (!this.apiMode) return;

    this.emit1('loading');

    this.httpOptions['params'] = this.getAllQueryParams();

    that.jsonApi.builderStack = this.finder;
    that.jsonApi.get(this.httpOptions["params"]).then(success, failed);
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'loadSuccess', function loadSuccess(response) {
    this.emit1('load-success', response);

    var body = this.transform(response);

    this.tableData = this.getObjectValue(body, this.dataPath, null);
    this.tablePagination = this.getObjectValue(body, this.paginationPath, null);

    if (this.tablePagination === null) {
      this.warn('vuetable: pagination-path "' + this.paginationPath + '" not found. ' + 'It looks like the data returned from the sever does not have pagination information ' + "or you may have set it incorrectly.\n" + 'You can explicitly suppress this warning by setting pagination-path="".');
    }

    var that = this;
    this.$nextTick(function () {
      that.emit1('pagination-data', this.tablePagination);
      that.emit1('loaded');
    });
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'loadFailed', function loadFailed(response) {
    console.error('load-error', response);
    this.emit1('load-error', response);
    this.emit1('loaded');
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'transform', function transform(data) {
    var func = 'transform';

    if (this.parentFunctionExists(func)) {
      return this.$parent[func].call(this.$parent, data);
    }

    return data;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'parentFunctionExists', function parentFunctionExists(func) {
    return func !== '' && typeof this.$parent[func] === 'function';
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'callParentFunction', function callParentFunction(func, args) {
    var defaultValue = arguments.length > 2 && arguments[2] !== undefined ? arguments[2] : null;

    if (this.parentFunctionExists(func)) {
      return this.$parent[func].call(this.$parent, args);
    }

    return defaultValue;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'emit1', function emit1(eventName, args) {
    this.$emit(eventName, args);
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'warn', function warn(msg) {
    if (!this.silent) {
      console.warn(msg);
    }
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'getAllQueryParams', function getAllQueryParams() {
    var params = {};
    params[this.queryParams.sort] = this.getSortParam();
    params[this.queryParams.page] = this.currentPage;
    params[this.queryParams.perPage] = this.perPage;

    for (var x in this.appendParams) {
      params[x] = this.appendParams[x];
    }

    return params;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'getSortParam', function getSortParam(sortOrder) {

    if (!this.sortOrder || this.sortOrder.field == '') {
      return '';
    }

    return this.sortOrder.map(function (sort) {
      return (sort.direction === 'desc' ? '' : '-') + sort.field;
    }).join(',');
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'getDefaultSortParam', function getDefaultSortParam() {
    var result = '';

    for (var i = 0; i < this.sortOrder.length; i++) {
      var fieldName = typeof this.sortOrder[i].sortField === 'undefined' ? this.sortOrder[i].field : this.sortOrder[i].sortField;

      result += fieldName + '|' + this.sortOrder[i].direction + (i + 1 < this.sortOrder.length ? ',' : '');
    }

    return result;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'extractName', function extractName(string) {
    return string.split(':')[0].trim();
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'extractArgs', function extractArgs(string) {
    return string.split(':')[1];
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'isSortable', function isSortable(field) {
    return !(typeof field.sortField === 'undefined');
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'isInCurrentSortGroup', function isInCurrentSortGroup(field) {
    return this.currentSortOrderPosition(field) !== false;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'currentSortOrderPosition', function currentSortOrderPosition(field) {
    if (!this.isSortable(field)) {
      return false;
    }

    for (var i = 0; i < this.sortOrder.length; i++) {
      if (this.fieldIsInSortOrderPosition(field, i)) {
        return i;
      }
    }

    return false;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'fieldIsInSortOrderPosition', function fieldIsInSortOrderPosition(field, i) {
    return this.sortOrder[i].field === field.name && this.sortOrder[i].sortField === field.sortField;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'orderBy', function orderBy(field, event) {
    if (!this.isSortable(field) || !this.apiMode) return;

    var key = this.multiSortKey.toLowerCase() + 'Key';

    if (this.multiSort && event[key]) {
      this.multiColumnSort(field);
    } else {
      this.singleColumnSort(field);
    }

    this.currentPage = 1;
    this.loadData();
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'multiColumnSort', function multiColumnSort(field) {
    var i = this.currentSortOrderPosition(field);

    if (i === false) {
      this.sortOrder.push({
        field: field.name,
        sortField: field.sortField,
        direction: 'asc'
      });
    } else {
      if (this.sortOrder[i].direction === 'asc') {
        this.sortOrder[i].direction = 'desc';
      } else {
        this.sortOrder.splice(i, 1);
      }
    }
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'singleColumnSort', function singleColumnSort(field) {
    if (this.sortOrder.length === 0) {
      this.clearSortOrder();
    }

    this.sortOrder.splice(1);

    if (this.fieldIsInSortOrderPosition(field, 0)) {
      this.sortOrder[0].direction = this.sortOrder[0].direction === 'asc' ? 'desc' : 'asc';
    } else {
      this.sortOrder[0].direction = 'asc';
    }
    this.sortOrder[0].field = field.name;
    this.sortOrder[0].sortField = field.sortField;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'clearSortOrder', function clearSortOrder() {
    this.sortOrder.push({
      field: '',
      sortField: '',
      direction: 'asc'
    });
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'sortIcon', function sortIcon(field) {
    var cls = '';
    var i = this.currentSortOrderPosition(field);

    if (i !== false) {
      cls = this.sortOrder[i].direction == 'asc' ? this.css.ascendingIcon : this.css.descendingIcon;
    }

    return cls;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'sortIconOpacity', function sortIconOpacity(field) {
    var max = 1.0,
        min = 0.3,
        step = 0.3;

    var count = this.sortOrder.length;
    var current = this.currentSortOrderPosition(field);

    if (max - count * step < min) {
      step = (max - min) / (count - 1);
    }

    var opacity = max - current * step;

    return opacity;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'hasCallback', function hasCallback(item) {
    return item.callback ? true : false;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'callCallback', function callCallback(field, item) {
    if (!this.hasCallback(field)) return;

    if (typeof field.callback == 'function') {
      return field.callback(this.getObjectValue(item, field.name));
    }

    var args = field.callback.split('|');
    var func = args.shift();

    if (typeof this.$parent[func] === 'function') {
      var value = this.getObjectValue(item, field.name);

      return args.length > 0 ? this.$parent[func].apply(this.$parent, [value].concat(args)) : this.$parent[func].call(this.$parent, value);
    }

    return null;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'getObjectValue', function getObjectValue(object, path, defaultValue) {
    defaultValue = typeof defaultValue === 'undefined' ? null : defaultValue;

    var obj = object;
    if (path.trim() != '') {
      var keys = path.split('.');
      keys.forEach(function (key) {
        if (obj !== null && typeof obj[key] !== 'undefined' && obj[key] !== null) {
          obj = obj[key];
        } else {
          obj = defaultValue;
        }
      });
    }
    return obj;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'toggleCheckbox', function toggleCheckbox(dataItem, fieldName, event) {
    var isChecked = event.target.checked;
    var idColumn = this.trackBy;

    if (dataItem[idColumn] === undefined) {
      this.warn('__checkbox field: The "' + this.trackBy + '" field does not exist! Make sure the field you specify in "track-by" prop does exist.');
      return;
    }

    var key = dataItem[idColumn];
    if (isChecked) {
      this.selectId(key);
    } else {
      this.unselectId(key);
    }
    this.emit1('vuetable:checkbox-toggled', isChecked, dataItem);
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'selectId', function selectId(key) {
    if (!this.isSelectedRow(key)) {
      this.selectedTo.push(key);
    }
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'unselectId', function unselectId(key) {
    this.selectedTo = this.selectedTo.filter(function (item) {
      return item !== key;
    });
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'isSelectedRow', function isSelectedRow(key) {
    return this.selectedTo.indexOf(key) >= 0;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'rowSelected', function rowSelected(dataItem, fieldName) {
    var idColumn = this.trackBy;
    var key = dataItem[idColumn];

    return this.isSelectedRow(key);
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'checkCheckboxesState', function checkCheckboxesState(fieldName) {
    if (!this.tableData) return;

    var self = this;
    var idColumn = this.trackBy;
    var selector = 'th.vuetable-th-checkbox-' + idColumn + ' input[type=checkbox]';
    var els = document.querySelectorAll(selector);

    if (els.forEach === undefined) els.forEach = function (cb) {
      [].forEach.call(els, cb);
    };

    var selected = this.tableData.filter(function (item) {
      return self.selectedTo.indexOf(item[idColumn]) >= 0;
    });

    if (selected.length <= 0) {
      els.forEach(function (el) {
        el.indeterminate = false;
      });
      return false;
    } else if (selected.length < this.perPage) {
        els.forEach(function (el) {
          el.indeterminate = true;
        });
        return true;
      } else {
          els.forEach(function (el) {
            el.indeterminate = false;
          });
          return true;
        }
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'toggleAllCheckboxes', function toggleAllCheckboxes(fieldName, event) {
    var self = this;
    var isChecked = event.target.checked;
    var idColumn = this.trackBy;

    if (isChecked) {
      this.tableData.forEach(function (dataItem) {
        self.selectId(dataItem[idColumn]);
      });
    } else {
      this.tableData.forEach(function (dataItem) {
        self.unselectId(dataItem[idColumn]);
      });
    }
    this.emit1('vuetable:checkbox-toggled-all', isChecked);
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'gotoPreviousPage', function gotoPreviousPage() {
    if (this.currentPage > 1) {
      this.currentPage--;
      this.loadData();
    }
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'gotoNextPage', function gotoNextPage() {
    if (this.currentPage < this.tablePagination.last_page) {
      this.currentPage++;
      this.loadData();
    }
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'gotoPage', function gotoPage(page) {
    if (page != this.currentPage && page > 0 && page <= this.tablePagination.last_page) {
      this.currentPage = page;
      this.loadData();
    }
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'isVisibleDetailRow', function isVisibleDetailRow(rowId) {
    return this.visibleDetailRows.indexOf(rowId) >= 0;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'showDetailRow', function showDetailRow(rowId) {
    if (!this.isVisibleDetailRow(rowId)) {
      this.visibleDetailRows.push(rowId);
    }
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'hideDetailRow', function hideDetailRow(rowId) {
    if (this.isVisibleDetailRow(rowId)) {
      this.visibleDetailRows.splice(this.visibleDetailRows.indexOf(rowId), 1);
    }
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'toggleDetailRow', function toggleDetailRow(rowId) {
    if (this.isVisibleDetailRow(rowId)) {
      this.hideDetailRow(rowId);
    } else {
      this.showDetailRow(rowId);
    }
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'showField', function showField(index) {
    if (index < 0 || index > this.tableFields.length) return;

    this.tableFields[index].visible = true;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'hideField', function hideField(index) {
    if (index < 0 || index > this.tableFields.length) return;

    this.tableFields[index].visible = false;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'toggleField', function toggleField(index) {
    if (index < 0 || index > this.tableFields.length) return;

    this.tableFields[index].visible = !this.tableFields[index].visible;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'renderIconTag', function renderIconTag(classes) {
    var options = arguments.length > 1 && arguments[1] !== undefined ? arguments[1] : '';

    return this.renderIcon === null ? '<i class="' + classes.join(' ') + '" ' + options + '></i>' : this.renderIcon(classes, options);
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'onRowClass', function onRowClass(dataItem, index) {
    if (this.rowClassCallback !== '') {
      this.warn('"row-class-callback" prop is deprecated, please use "row-class" prop instead.');
      return;
    }

    if (typeof this.rowClass === 'function') {
      return this.rowClass(dataItem, index);
    }

    return this.rowClass;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'onRowChanged', function onRowChanged(dataItem) {
    this.emit1('row-changed', dataItem);
    return true;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'onRowClicked', function onRowClicked(dataItem, event) {
    this.emit1(this.eventPrefix + 'row-clicked', dataItem, event);
    return true;
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'onRowDoubleClicked', function onRowDoubleClicked(dataItem, event) {
    this.emit1(this.eventPrefix + 'row-dblclicked', dataItem, event);
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'onDetailRowClick', function onDetailRowClick(dataItem, event) {
    this.emit1(this.eventPrefix + 'detail-row-clicked', dataItem, event);
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'onCellClicked', function onCellClicked(dataItem, field, event) {
    this.emit1(this.eventPrefix + 'cell-clicked', dataItem, field, event);
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'onCellDoubleClicked', function onCellDoubleClicked(dataItem, field, event) {
    this.emit1(this.eventPrefix + 'cell-dblclicked', dataItem, field, event);
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'changePage', function changePage(page) {
    if (page === 'prev') {
      this.gotoPreviousPage();
    } else if (page === 'next') {
      this.gotoNextPage();
    } else {
      this.gotoPage(page);
    }
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'reload', function reload() {
    this.loadData();
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'refresh', function refresh() {
    this.currentPage = 1;
    this.loadData();
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'resetData', function resetData() {
    this.tableData = null;
    this.tablePagination = null;
    this.emit1('data-reset');
  }), __WEBPACK_IMPORTED_MODULE_0_babel_runtime_helpers_defineProperty___default()(_methods, 'reinit', function reinit() {
    this.normalizeFields();
    this.$nextTick(function () {
      this.emit1('initialized', this.tableFields);
    });

    if (this.apiMode && this.loadOnStart) {
      this.loadData();
    }
    if (this.apiMode == false && this.data.length > 0) {
      this.setData(this.data);
    }
  }), _methods),
  watch: {
    'multiSort': function multiSort(newVal, oldVal) {
      if (newVal === false && this.sortOrder.length > 1) {
        this.sortOrder.splice(1);
        this.loadData();
      }
    },

    'apiUrl': function apiUrl(newVal, oldVal) {
      if (newVal !== oldVal) this.refresh();
    }
  }
});

/***/ }),
/* 328 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__VuetablePaginationMixin_vue__ = __webpack_require__(200);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__VuetablePaginationMixin_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0__VuetablePaginationMixin_vue__);




/* harmony default export */ __webpack_exports__["default"] = ({
  mixins: [__WEBPACK_IMPORTED_MODULE_0__VuetablePaginationMixin_vue___default.a]
});

/***/ }),
/* 329 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__VuetablePaginationMixin_vue__ = __webpack_require__(200);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__VuetablePaginationMixin_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0__VuetablePaginationMixin_vue__);




/* harmony default export */ __webpack_exports__["default"] = ({
  mixins: [__WEBPACK_IMPORTED_MODULE_0__VuetablePaginationMixin_vue___default.a],
  props: {
    pageText: {
      type: String,
      default: function _default() {
        return 'Page';
      }
    }
  },
  methods: {
    registerEvents: function registerEvents() {
      var self = this;

      this.$on('vuetable:pagination-data', function (tablePagination) {
        self.setPaginationData(tablePagination);
      });
    }
  },
  created: function created() {
    this.registerEvents();
  }
});

/***/ }),
/* 330 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__VuetablePaginationInfoMixin_vue__ = __webpack_require__(642);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__VuetablePaginationInfoMixin_vue___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0__VuetablePaginationInfoMixin_vue__);




/* harmony default export */ __webpack_exports__["default"] = ({
  mixins: [__WEBPACK_IMPORTED_MODULE_0__VuetablePaginationInfoMixin_vue___default.a]
});

/***/ }),
/* 331 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });

/* harmony default export */ __webpack_exports__["default"] = ({
  props: {
    css: {
      type: Object,
      default: function _default() {
        return {
          infoClass: 'left floated left aligned six wide column'
        };
      }
    },
    infoTemplate: {
      type: String,
      default: function _default() {
        return "Displaying {from} to {to} of {total} items";
      }
    },
    noDataTemplate: {
      type: String,
      default: function _default() {
        return 'No relevant data';
      }
    }
  },
  data: function data() {
    return {
      tablePagination: null
    };
  },
  computed: {
    paginationInfo: function paginationInfo() {
      if (this.tablePagination == null || this.tablePagination.total == 0) {
        return this.noDataTemplate;
      }

      return this.infoTemplate.replace('{from}', this.tablePagination.from || 0).replace('{to}', this.tablePagination.to || 0).replace('{total}', this.tablePagination.total || 0);
    }
  },
  methods: {
    setPaginationData: function setPaginationData(tablePagination) {
      this.tablePagination = tablePagination;
    },
    resetData: function resetData() {
      this.tablePagination = null;
    }
  }
});

/***/ }),
/* 332 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });

/* harmony default export */ __webpack_exports__["default"] = ({
  props: {
    css: {
      type: Object,
      default: function _default() {
        return {
          wrapperClass: 'ui right floated pagination menu',
          activeClass: 'active large',
          disabledClass: 'disabled',
          pageClass: 'item',
          linkClass: 'icon item',
          paginationClass: 'ui bottom attached segment grid',
          paginationInfoClass: 'left floated left aligned six wide column',
          dropdownClass: 'ui search dropdown',
          icons: {
            first: 'angle double left icon',
            prev: 'left chevron icon',
            next: 'right chevron icon',
            last: 'angle double right icon'
          }
        };
      }
    },
    onEachSide: {
      type: Number,
      default: function _default() {
        return 2;
      }
    }
  },
  data: function data() {
    return {
      eventPrefix: 'vuetable-pagination:',
      tablePagination: null
    };
  },
  computed: {
    totalPage: function totalPage() {
      return this.tablePagination === null ? 0 : this.tablePagination.last_page;
    },
    isOnFirstPage: function isOnFirstPage() {
      return this.tablePagination === null ? false : this.tablePagination.current_page === 1;
    },
    isOnLastPage: function isOnLastPage() {
      return this.tablePagination === null ? false : this.tablePagination.current_page === this.tablePagination.last_page;
    },
    notEnoughPages: function notEnoughPages() {
      return this.totalPage < this.onEachSide * 2 + 4;
    },
    windowSize: function windowSize() {
      return this.onEachSide * 2 + 1;
    },
    windowStart: function windowStart() {
      if (!this.tablePagination || this.tablePagination.current_page <= this.onEachSide) {
        return 1;
      } else if (this.tablePagination.current_page >= this.totalPage - this.onEachSide) {
        return this.totalPage - this.onEachSide * 2;
      }

      return this.tablePagination.current_page - this.onEachSide;
    }
  },
  methods: {
    loadPage: function loadPage(page) {
      this.$emit('change-page', page);
    },
    isCurrentPage: function isCurrentPage(page) {
      return page === this.tablePagination.current_page;
    },
    setPaginationData: function setPaginationData(tablePagination) {
      console.log("this set pagination", this.tablePagination);
      this.tablePagination = tablePagination;
    },
    resetData: function resetData() {
      this.tablePagination = null;
    }
  }
});

/***/ }),
/* 333 */,
/* 334 */,
/* 335 */,
/* 336 */,
/* 337 */,
/* 338 */,
/* 339 */,
/* 340 */,
/* 341 */,
/* 342 */,
/* 343 */,
/* 344 */,
/* 345 */,
/* 346 */,
/* 347 */,
/* 348 */,
/* 349 */,
/* 350 */,
/* 351 */,
/* 352 */,
/* 353 */,
/* 354 */,
/* 355 */,
/* 356 */,
/* 357 */,
/* 358 */,
/* 359 */,
/* 360 */,
/* 361 */,
/* 362 */,
/* 363 */,
/* 364 */,
/* 365 */,
/* 366 */,
/* 367 */,
/* 368 */,
/* 369 */,
/* 370 */,
/* 371 */,
/* 372 */,
/* 373 */,
/* 374 */,
/* 375 */,
/* 376 */,
/* 377 */,
/* 378 */,
/* 379 */,
/* 380 */,
/* 381 */,
/* 382 */,
/* 383 */,
/* 384 */,
/* 385 */,
/* 386 */,
/* 387 */,
/* 388 */,
/* 389 */,
/* 390 */,
/* 391 */,
/* 392 */,
/* 393 */,
/* 394 */,
/* 395 */,
/* 396 */,
/* 397 */,
/* 398 */,
/* 399 */,
/* 400 */,
/* 401 */,
/* 402 */,
/* 403 */,
/* 404 */,
/* 405 */,
/* 406 */,
/* 407 */,
/* 408 */,
/* 409 */,
/* 410 */,
/* 411 */,
/* 412 */,
/* 413 */,
/* 414 */,
/* 415 */,
/* 416 */,
/* 417 */,
/* 418 */,
/* 419 */,
/* 420 */,
/* 421 */,
/* 422 */,
/* 423 */,
/* 424 */,
/* 425 */,
/* 426 */,
/* 427 */,
/* 428 */,
/* 429 */,
/* 430 */,
/* 431 */
/***/ (function(module, exports) {

module.exports = {"Aacute":"Á","aacute":"á","Abreve":"Ă","abreve":"ă","ac":"∾","acd":"∿","acE":"∾̳","Acirc":"Â","acirc":"â","acute":"´","Acy":"А","acy":"а","AElig":"Æ","aelig":"æ","af":"⁡","Afr":"𝔄","afr":"𝔞","Agrave":"À","agrave":"à","alefsym":"ℵ","aleph":"ℵ","Alpha":"Α","alpha":"α","Amacr":"Ā","amacr":"ā","amalg":"⨿","amp":"&","AMP":"&","andand":"⩕","And":"⩓","and":"∧","andd":"⩜","andslope":"⩘","andv":"⩚","ang":"∠","ange":"⦤","angle":"∠","angmsdaa":"⦨","angmsdab":"⦩","angmsdac":"⦪","angmsdad":"⦫","angmsdae":"⦬","angmsdaf":"⦭","angmsdag":"⦮","angmsdah":"⦯","angmsd":"∡","angrt":"∟","angrtvb":"⊾","angrtvbd":"⦝","angsph":"∢","angst":"Å","angzarr":"⍼","Aogon":"Ą","aogon":"ą","Aopf":"𝔸","aopf":"𝕒","apacir":"⩯","ap":"≈","apE":"⩰","ape":"≊","apid":"≋","apos":"'","ApplyFunction":"⁡","approx":"≈","approxeq":"≊","Aring":"Å","aring":"å","Ascr":"𝒜","ascr":"𝒶","Assign":"≔","ast":"*","asymp":"≈","asympeq":"≍","Atilde":"Ã","atilde":"ã","Auml":"Ä","auml":"ä","awconint":"∳","awint":"⨑","backcong":"≌","backepsilon":"϶","backprime":"‵","backsim":"∽","backsimeq":"⋍","Backslash":"∖","Barv":"⫧","barvee":"⊽","barwed":"⌅","Barwed":"⌆","barwedge":"⌅","bbrk":"⎵","bbrktbrk":"⎶","bcong":"≌","Bcy":"Б","bcy":"б","bdquo":"„","becaus":"∵","because":"∵","Because":"∵","bemptyv":"⦰","bepsi":"϶","bernou":"ℬ","Bernoullis":"ℬ","Beta":"Β","beta":"β","beth":"ℶ","between":"≬","Bfr":"𝔅","bfr":"𝔟","bigcap":"⋂","bigcirc":"◯","bigcup":"⋃","bigodot":"⨀","bigoplus":"⨁","bigotimes":"⨂","bigsqcup":"⨆","bigstar":"★","bigtriangledown":"▽","bigtriangleup":"△","biguplus":"⨄","bigvee":"⋁","bigwedge":"⋀","bkarow":"⤍","blacklozenge":"⧫","blacksquare":"▪","blacktriangle":"▴","blacktriangledown":"▾","blacktriangleleft":"◂","blacktriangleright":"▸","blank":"␣","blk12":"▒","blk14":"░","blk34":"▓","block":"█","bne":"=⃥","bnequiv":"≡⃥","bNot":"⫭","bnot":"⌐","Bopf":"𝔹","bopf":"𝕓","bot":"⊥","bottom":"⊥","bowtie":"⋈","boxbox":"⧉","boxdl":"┐","boxdL":"╕","boxDl":"╖","boxDL":"╗","boxdr":"┌","boxdR":"╒","boxDr":"╓","boxDR":"╔","boxh":"─","boxH":"═","boxhd":"┬","boxHd":"╤","boxhD":"╥","boxHD":"╦","boxhu":"┴","boxHu":"╧","boxhU":"╨","boxHU":"╩","boxminus":"⊟","boxplus":"⊞","boxtimes":"⊠","boxul":"┘","boxuL":"╛","boxUl":"╜","boxUL":"╝","boxur":"└","boxuR":"╘","boxUr":"╙","boxUR":"╚","boxv":"│","boxV":"║","boxvh":"┼","boxvH":"╪","boxVh":"╫","boxVH":"╬","boxvl":"┤","boxvL":"╡","boxVl":"╢","boxVL":"╣","boxvr":"├","boxvR":"╞","boxVr":"╟","boxVR":"╠","bprime":"‵","breve":"˘","Breve":"˘","brvbar":"¦","bscr":"𝒷","Bscr":"ℬ","bsemi":"⁏","bsim":"∽","bsime":"⋍","bsolb":"⧅","bsol":"\\","bsolhsub":"⟈","bull":"•","bullet":"•","bump":"≎","bumpE":"⪮","bumpe":"≏","Bumpeq":"≎","bumpeq":"≏","Cacute":"Ć","cacute":"ć","capand":"⩄","capbrcup":"⩉","capcap":"⩋","cap":"∩","Cap":"⋒","capcup":"⩇","capdot":"⩀","CapitalDifferentialD":"ⅅ","caps":"∩︀","caret":"⁁","caron":"ˇ","Cayleys":"ℭ","ccaps":"⩍","Ccaron":"Č","ccaron":"č","Ccedil":"Ç","ccedil":"ç","Ccirc":"Ĉ","ccirc":"ĉ","Cconint":"∰","ccups":"⩌","ccupssm":"⩐","Cdot":"Ċ","cdot":"ċ","cedil":"¸","Cedilla":"¸","cemptyv":"⦲","cent":"¢","centerdot":"·","CenterDot":"·","cfr":"𝔠","Cfr":"ℭ","CHcy":"Ч","chcy":"ч","check":"✓","checkmark":"✓","Chi":"Χ","chi":"χ","circ":"ˆ","circeq":"≗","circlearrowleft":"↺","circlearrowright":"↻","circledast":"⊛","circledcirc":"⊚","circleddash":"⊝","CircleDot":"⊙","circledR":"®","circledS":"Ⓢ","CircleMinus":"⊖","CirclePlus":"⊕","CircleTimes":"⊗","cir":"○","cirE":"⧃","cire":"≗","cirfnint":"⨐","cirmid":"⫯","cirscir":"⧂","ClockwiseContourIntegral":"∲","CloseCurlyDoubleQuote":"”","CloseCurlyQuote":"’","clubs":"♣","clubsuit":"♣","colon":":","Colon":"∷","Colone":"⩴","colone":"≔","coloneq":"≔","comma":",","commat":"@","comp":"∁","compfn":"∘","complement":"∁","complexes":"ℂ","cong":"≅","congdot":"⩭","Congruent":"≡","conint":"∮","Conint":"∯","ContourIntegral":"∮","copf":"𝕔","Copf":"ℂ","coprod":"∐","Coproduct":"∐","copy":"©","COPY":"©","copysr":"℗","CounterClockwiseContourIntegral":"∳","crarr":"↵","cross":"✗","Cross":"⨯","Cscr":"𝒞","cscr":"𝒸","csub":"⫏","csube":"⫑","csup":"⫐","csupe":"⫒","ctdot":"⋯","cudarrl":"⤸","cudarrr":"⤵","cuepr":"⋞","cuesc":"⋟","cularr":"↶","cularrp":"⤽","cupbrcap":"⩈","cupcap":"⩆","CupCap":"≍","cup":"∪","Cup":"⋓","cupcup":"⩊","cupdot":"⊍","cupor":"⩅","cups":"∪︀","curarr":"↷","curarrm":"⤼","curlyeqprec":"⋞","curlyeqsucc":"⋟","curlyvee":"⋎","curlywedge":"⋏","curren":"¤","curvearrowleft":"↶","curvearrowright":"↷","cuvee":"⋎","cuwed":"⋏","cwconint":"∲","cwint":"∱","cylcty":"⌭","dagger":"†","Dagger":"‡","daleth":"ℸ","darr":"↓","Darr":"↡","dArr":"⇓","dash":"‐","Dashv":"⫤","dashv":"⊣","dbkarow":"⤏","dblac":"˝","Dcaron":"Ď","dcaron":"ď","Dcy":"Д","dcy":"д","ddagger":"‡","ddarr":"⇊","DD":"ⅅ","dd":"ⅆ","DDotrahd":"⤑","ddotseq":"⩷","deg":"°","Del":"∇","Delta":"Δ","delta":"δ","demptyv":"⦱","dfisht":"⥿","Dfr":"𝔇","dfr":"𝔡","dHar":"⥥","dharl":"⇃","dharr":"⇂","DiacriticalAcute":"´","DiacriticalDot":"˙","DiacriticalDoubleAcute":"˝","DiacriticalGrave":"`","DiacriticalTilde":"˜","diam":"⋄","diamond":"⋄","Diamond":"⋄","diamondsuit":"♦","diams":"♦","die":"¨","DifferentialD":"ⅆ","digamma":"ϝ","disin":"⋲","div":"÷","divide":"÷","divideontimes":"⋇","divonx":"⋇","DJcy":"Ђ","djcy":"ђ","dlcorn":"⌞","dlcrop":"⌍","dollar":"$","Dopf":"𝔻","dopf":"𝕕","Dot":"¨","dot":"˙","DotDot":"⃜","doteq":"≐","doteqdot":"≑","DotEqual":"≐","dotminus":"∸","dotplus":"∔","dotsquare":"⊡","doublebarwedge":"⌆","DoubleContourIntegral":"∯","DoubleDot":"¨","DoubleDownArrow":"⇓","DoubleLeftArrow":"⇐","DoubleLeftRightArrow":"⇔","DoubleLeftTee":"⫤","DoubleLongLeftArrow":"⟸","DoubleLongLeftRightArrow":"⟺","DoubleLongRightArrow":"⟹","DoubleRightArrow":"⇒","DoubleRightTee":"⊨","DoubleUpArrow":"⇑","DoubleUpDownArrow":"⇕","DoubleVerticalBar":"∥","DownArrowBar":"⤓","downarrow":"↓","DownArrow":"↓","Downarrow":"⇓","DownArrowUpArrow":"⇵","DownBreve":"̑","downdownarrows":"⇊","downharpoonleft":"⇃","downharpoonright":"⇂","DownLeftRightVector":"⥐","DownLeftTeeVector":"⥞","DownLeftVectorBar":"⥖","DownLeftVector":"↽","DownRightTeeVector":"⥟","DownRightVectorBar":"⥗","DownRightVector":"⇁","DownTeeArrow":"↧","DownTee":"⊤","drbkarow":"⤐","drcorn":"⌟","drcrop":"⌌","Dscr":"𝒟","dscr":"𝒹","DScy":"Ѕ","dscy":"ѕ","dsol":"⧶","Dstrok":"Đ","dstrok":"đ","dtdot":"⋱","dtri":"▿","dtrif":"▾","duarr":"⇵","duhar":"⥯","dwangle":"⦦","DZcy":"Џ","dzcy":"џ","dzigrarr":"⟿","Eacute":"É","eacute":"é","easter":"⩮","Ecaron":"Ě","ecaron":"ě","Ecirc":"Ê","ecirc":"ê","ecir":"≖","ecolon":"≕","Ecy":"Э","ecy":"э","eDDot":"⩷","Edot":"Ė","edot":"ė","eDot":"≑","ee":"ⅇ","efDot":"≒","Efr":"𝔈","efr":"𝔢","eg":"⪚","Egrave":"È","egrave":"è","egs":"⪖","egsdot":"⪘","el":"⪙","Element":"∈","elinters":"⏧","ell":"ℓ","els":"⪕","elsdot":"⪗","Emacr":"Ē","emacr":"ē","empty":"∅","emptyset":"∅","EmptySmallSquare":"◻","emptyv":"∅","EmptyVerySmallSquare":"▫","emsp13":" ","emsp14":" ","emsp":" ","ENG":"Ŋ","eng":"ŋ","ensp":" ","Eogon":"Ę","eogon":"ę","Eopf":"𝔼","eopf":"𝕖","epar":"⋕","eparsl":"⧣","eplus":"⩱","epsi":"ε","Epsilon":"Ε","epsilon":"ε","epsiv":"ϵ","eqcirc":"≖","eqcolon":"≕","eqsim":"≂","eqslantgtr":"⪖","eqslantless":"⪕","Equal":"⩵","equals":"=","EqualTilde":"≂","equest":"≟","Equilibrium":"⇌","equiv":"≡","equivDD":"⩸","eqvparsl":"⧥","erarr":"⥱","erDot":"≓","escr":"ℯ","Escr":"ℰ","esdot":"≐","Esim":"⩳","esim":"≂","Eta":"Η","eta":"η","ETH":"Ð","eth":"ð","Euml":"Ë","euml":"ë","euro":"€","excl":"!","exist":"∃","Exists":"∃","expectation":"ℰ","exponentiale":"ⅇ","ExponentialE":"ⅇ","fallingdotseq":"≒","Fcy":"Ф","fcy":"ф","female":"♀","ffilig":"ﬃ","fflig":"ﬀ","ffllig":"ﬄ","Ffr":"𝔉","ffr":"𝔣","filig":"ﬁ","FilledSmallSquare":"◼","FilledVerySmallSquare":"▪","fjlig":"fj","flat":"♭","fllig":"ﬂ","fltns":"▱","fnof":"ƒ","Fopf":"𝔽","fopf":"𝕗","forall":"∀","ForAll":"∀","fork":"⋔","forkv":"⫙","Fouriertrf":"ℱ","fpartint":"⨍","frac12":"½","frac13":"⅓","frac14":"¼","frac15":"⅕","frac16":"⅙","frac18":"⅛","frac23":"⅔","frac25":"⅖","frac34":"¾","frac35":"⅗","frac38":"⅜","frac45":"⅘","frac56":"⅚","frac58":"⅝","frac78":"⅞","frasl":"⁄","frown":"⌢","fscr":"𝒻","Fscr":"ℱ","gacute":"ǵ","Gamma":"Γ","gamma":"γ","Gammad":"Ϝ","gammad":"ϝ","gap":"⪆","Gbreve":"Ğ","gbreve":"ğ","Gcedil":"Ģ","Gcirc":"Ĝ","gcirc":"ĝ","Gcy":"Г","gcy":"г","Gdot":"Ġ","gdot":"ġ","ge":"≥","gE":"≧","gEl":"⪌","gel":"⋛","geq":"≥","geqq":"≧","geqslant":"⩾","gescc":"⪩","ges":"⩾","gesdot":"⪀","gesdoto":"⪂","gesdotol":"⪄","gesl":"⋛︀","gesles":"⪔","Gfr":"𝔊","gfr":"𝔤","gg":"≫","Gg":"⋙","ggg":"⋙","gimel":"ℷ","GJcy":"Ѓ","gjcy":"ѓ","gla":"⪥","gl":"≷","glE":"⪒","glj":"⪤","gnap":"⪊","gnapprox":"⪊","gne":"⪈","gnE":"≩","gneq":"⪈","gneqq":"≩","gnsim":"⋧","Gopf":"𝔾","gopf":"𝕘","grave":"`","GreaterEqual":"≥","GreaterEqualLess":"⋛","GreaterFullEqual":"≧","GreaterGreater":"⪢","GreaterLess":"≷","GreaterSlantEqual":"⩾","GreaterTilde":"≳","Gscr":"𝒢","gscr":"ℊ","gsim":"≳","gsime":"⪎","gsiml":"⪐","gtcc":"⪧","gtcir":"⩺","gt":">","GT":">","Gt":"≫","gtdot":"⋗","gtlPar":"⦕","gtquest":"⩼","gtrapprox":"⪆","gtrarr":"⥸","gtrdot":"⋗","gtreqless":"⋛","gtreqqless":"⪌","gtrless":"≷","gtrsim":"≳","gvertneqq":"≩︀","gvnE":"≩︀","Hacek":"ˇ","hairsp":" ","half":"½","hamilt":"ℋ","HARDcy":"Ъ","hardcy":"ъ","harrcir":"⥈","harr":"↔","hArr":"⇔","harrw":"↭","Hat":"^","hbar":"ℏ","Hcirc":"Ĥ","hcirc":"ĥ","hearts":"♥","heartsuit":"♥","hellip":"…","hercon":"⊹","hfr":"𝔥","Hfr":"ℌ","HilbertSpace":"ℋ","hksearow":"⤥","hkswarow":"⤦","hoarr":"⇿","homtht":"∻","hookleftarrow":"↩","hookrightarrow":"↪","hopf":"𝕙","Hopf":"ℍ","horbar":"―","HorizontalLine":"─","hscr":"𝒽","Hscr":"ℋ","hslash":"ℏ","Hstrok":"Ħ","hstrok":"ħ","HumpDownHump":"≎","HumpEqual":"≏","hybull":"⁃","hyphen":"‐","Iacute":"Í","iacute":"í","ic":"⁣","Icirc":"Î","icirc":"î","Icy":"И","icy":"и","Idot":"İ","IEcy":"Е","iecy":"е","iexcl":"¡","iff":"⇔","ifr":"𝔦","Ifr":"ℑ","Igrave":"Ì","igrave":"ì","ii":"ⅈ","iiiint":"⨌","iiint":"∭","iinfin":"⧜","iiota":"℩","IJlig":"Ĳ","ijlig":"ĳ","Imacr":"Ī","imacr":"ī","image":"ℑ","ImaginaryI":"ⅈ","imagline":"ℐ","imagpart":"ℑ","imath":"ı","Im":"ℑ","imof":"⊷","imped":"Ƶ","Implies":"⇒","incare":"℅","in":"∈","infin":"∞","infintie":"⧝","inodot":"ı","intcal":"⊺","int":"∫","Int":"∬","integers":"ℤ","Integral":"∫","intercal":"⊺","Intersection":"⋂","intlarhk":"⨗","intprod":"⨼","InvisibleComma":"⁣","InvisibleTimes":"⁢","IOcy":"Ё","iocy":"ё","Iogon":"Į","iogon":"į","Iopf":"𝕀","iopf":"𝕚","Iota":"Ι","iota":"ι","iprod":"⨼","iquest":"¿","iscr":"𝒾","Iscr":"ℐ","isin":"∈","isindot":"⋵","isinE":"⋹","isins":"⋴","isinsv":"⋳","isinv":"∈","it":"⁢","Itilde":"Ĩ","itilde":"ĩ","Iukcy":"І","iukcy":"і","Iuml":"Ï","iuml":"ï","Jcirc":"Ĵ","jcirc":"ĵ","Jcy":"Й","jcy":"й","Jfr":"𝔍","jfr":"𝔧","jmath":"ȷ","Jopf":"𝕁","jopf":"𝕛","Jscr":"𝒥","jscr":"𝒿","Jsercy":"Ј","jsercy":"ј","Jukcy":"Є","jukcy":"є","Kappa":"Κ","kappa":"κ","kappav":"ϰ","Kcedil":"Ķ","kcedil":"ķ","Kcy":"К","kcy":"к","Kfr":"𝔎","kfr":"𝔨","kgreen":"ĸ","KHcy":"Х","khcy":"х","KJcy":"Ќ","kjcy":"ќ","Kopf":"𝕂","kopf":"𝕜","Kscr":"𝒦","kscr":"𝓀","lAarr":"⇚","Lacute":"Ĺ","lacute":"ĺ","laemptyv":"⦴","lagran":"ℒ","Lambda":"Λ","lambda":"λ","lang":"⟨","Lang":"⟪","langd":"⦑","langle":"⟨","lap":"⪅","Laplacetrf":"ℒ","laquo":"«","larrb":"⇤","larrbfs":"⤟","larr":"←","Larr":"↞","lArr":"⇐","larrfs":"⤝","larrhk":"↩","larrlp":"↫","larrpl":"⤹","larrsim":"⥳","larrtl":"↢","latail":"⤙","lAtail":"⤛","lat":"⪫","late":"⪭","lates":"⪭︀","lbarr":"⤌","lBarr":"⤎","lbbrk":"❲","lbrace":"{","lbrack":"[","lbrke":"⦋","lbrksld":"⦏","lbrkslu":"⦍","Lcaron":"Ľ","lcaron":"ľ","Lcedil":"Ļ","lcedil":"ļ","lceil":"⌈","lcub":"{","Lcy":"Л","lcy":"л","ldca":"⤶","ldquo":"“","ldquor":"„","ldrdhar":"⥧","ldrushar":"⥋","ldsh":"↲","le":"≤","lE":"≦","LeftAngleBracket":"⟨","LeftArrowBar":"⇤","leftarrow":"←","LeftArrow":"←","Leftarrow":"⇐","LeftArrowRightArrow":"⇆","leftarrowtail":"↢","LeftCeiling":"⌈","LeftDoubleBracket":"⟦","LeftDownTeeVector":"⥡","LeftDownVectorBar":"⥙","LeftDownVector":"⇃","LeftFloor":"⌊","leftharpoondown":"↽","leftharpoonup":"↼","leftleftarrows":"⇇","leftrightarrow":"↔","LeftRightArrow":"↔","Leftrightarrow":"⇔","leftrightarrows":"⇆","leftrightharpoons":"⇋","leftrightsquigarrow":"↭","LeftRightVector":"⥎","LeftTeeArrow":"↤","LeftTee":"⊣","LeftTeeVector":"⥚","leftthreetimes":"⋋","LeftTriangleBar":"⧏","LeftTriangle":"⊲","LeftTriangleEqual":"⊴","LeftUpDownVector":"⥑","LeftUpTeeVector":"⥠","LeftUpVectorBar":"⥘","LeftUpVector":"↿","LeftVectorBar":"⥒","LeftVector":"↼","lEg":"⪋","leg":"⋚","leq":"≤","leqq":"≦","leqslant":"⩽","lescc":"⪨","les":"⩽","lesdot":"⩿","lesdoto":"⪁","lesdotor":"⪃","lesg":"⋚︀","lesges":"⪓","lessapprox":"⪅","lessdot":"⋖","lesseqgtr":"⋚","lesseqqgtr":"⪋","LessEqualGreater":"⋚","LessFullEqual":"≦","LessGreater":"≶","lessgtr":"≶","LessLess":"⪡","lesssim":"≲","LessSlantEqual":"⩽","LessTilde":"≲","lfisht":"⥼","lfloor":"⌊","Lfr":"𝔏","lfr":"𝔩","lg":"≶","lgE":"⪑","lHar":"⥢","lhard":"↽","lharu":"↼","lharul":"⥪","lhblk":"▄","LJcy":"Љ","ljcy":"љ","llarr":"⇇","ll":"≪","Ll":"⋘","llcorner":"⌞","Lleftarrow":"⇚","llhard":"⥫","lltri":"◺","Lmidot":"Ŀ","lmidot":"ŀ","lmoustache":"⎰","lmoust":"⎰","lnap":"⪉","lnapprox":"⪉","lne":"⪇","lnE":"≨","lneq":"⪇","lneqq":"≨","lnsim":"⋦","loang":"⟬","loarr":"⇽","lobrk":"⟦","longleftarrow":"⟵","LongLeftArrow":"⟵","Longleftarrow":"⟸","longleftrightarrow":"⟷","LongLeftRightArrow":"⟷","Longleftrightarrow":"⟺","longmapsto":"⟼","longrightarrow":"⟶","LongRightArrow":"⟶","Longrightarrow":"⟹","looparrowleft":"↫","looparrowright":"↬","lopar":"⦅","Lopf":"𝕃","lopf":"𝕝","loplus":"⨭","lotimes":"⨴","lowast":"∗","lowbar":"_","LowerLeftArrow":"↙","LowerRightArrow":"↘","loz":"◊","lozenge":"◊","lozf":"⧫","lpar":"(","lparlt":"⦓","lrarr":"⇆","lrcorner":"⌟","lrhar":"⇋","lrhard":"⥭","lrm":"‎","lrtri":"⊿","lsaquo":"‹","lscr":"𝓁","Lscr":"ℒ","lsh":"↰","Lsh":"↰","lsim":"≲","lsime":"⪍","lsimg":"⪏","lsqb":"[","lsquo":"‘","lsquor":"‚","Lstrok":"Ł","lstrok":"ł","ltcc":"⪦","ltcir":"⩹","lt":"<","LT":"<","Lt":"≪","ltdot":"⋖","lthree":"⋋","ltimes":"⋉","ltlarr":"⥶","ltquest":"⩻","ltri":"◃","ltrie":"⊴","ltrif":"◂","ltrPar":"⦖","lurdshar":"⥊","luruhar":"⥦","lvertneqq":"≨︀","lvnE":"≨︀","macr":"¯","male":"♂","malt":"✠","maltese":"✠","Map":"⤅","map":"↦","mapsto":"↦","mapstodown":"↧","mapstoleft":"↤","mapstoup":"↥","marker":"▮","mcomma":"⨩","Mcy":"М","mcy":"м","mdash":"—","mDDot":"∺","measuredangle":"∡","MediumSpace":" ","Mellintrf":"ℳ","Mfr":"𝔐","mfr":"𝔪","mho":"℧","micro":"µ","midast":"*","midcir":"⫰","mid":"∣","middot":"·","minusb":"⊟","minus":"−","minusd":"∸","minusdu":"⨪","MinusPlus":"∓","mlcp":"⫛","mldr":"…","mnplus":"∓","models":"⊧","Mopf":"𝕄","mopf":"𝕞","mp":"∓","mscr":"𝓂","Mscr":"ℳ","mstpos":"∾","Mu":"Μ","mu":"μ","multimap":"⊸","mumap":"⊸","nabla":"∇","Nacute":"Ń","nacute":"ń","nang":"∠⃒","nap":"≉","napE":"⩰̸","napid":"≋̸","napos":"ŉ","napprox":"≉","natural":"♮","naturals":"ℕ","natur":"♮","nbsp":" ","nbump":"≎̸","nbumpe":"≏̸","ncap":"⩃","Ncaron":"Ň","ncaron":"ň","Ncedil":"Ņ","ncedil":"ņ","ncong":"≇","ncongdot":"⩭̸","ncup":"⩂","Ncy":"Н","ncy":"н","ndash":"–","nearhk":"⤤","nearr":"↗","neArr":"⇗","nearrow":"↗","ne":"≠","nedot":"≐̸","NegativeMediumSpace":"​","NegativeThickSpace":"​","NegativeThinSpace":"​","NegativeVeryThinSpace":"​","nequiv":"≢","nesear":"⤨","nesim":"≂̸","NestedGreaterGreater":"≫","NestedLessLess":"≪","NewLine":"\n","nexist":"∄","nexists":"∄","Nfr":"𝔑","nfr":"𝔫","ngE":"≧̸","nge":"≱","ngeq":"≱","ngeqq":"≧̸","ngeqslant":"⩾̸","nges":"⩾̸","nGg":"⋙̸","ngsim":"≵","nGt":"≫⃒","ngt":"≯","ngtr":"≯","nGtv":"≫̸","nharr":"↮","nhArr":"⇎","nhpar":"⫲","ni":"∋","nis":"⋼","nisd":"⋺","niv":"∋","NJcy":"Њ","njcy":"њ","nlarr":"↚","nlArr":"⇍","nldr":"‥","nlE":"≦̸","nle":"≰","nleftarrow":"↚","nLeftarrow":"⇍","nleftrightarrow":"↮","nLeftrightarrow":"⇎","nleq":"≰","nleqq":"≦̸","nleqslant":"⩽̸","nles":"⩽̸","nless":"≮","nLl":"⋘̸","nlsim":"≴","nLt":"≪⃒","nlt":"≮","nltri":"⋪","nltrie":"⋬","nLtv":"≪̸","nmid":"∤","NoBreak":"⁠","NonBreakingSpace":" ","nopf":"𝕟","Nopf":"ℕ","Not":"⫬","not":"¬","NotCongruent":"≢","NotCupCap":"≭","NotDoubleVerticalBar":"∦","NotElement":"∉","NotEqual":"≠","NotEqualTilde":"≂̸","NotExists":"∄","NotGreater":"≯","NotGreaterEqual":"≱","NotGreaterFullEqual":"≧̸","NotGreaterGreater":"≫̸","NotGreaterLess":"≹","NotGreaterSlantEqual":"⩾̸","NotGreaterTilde":"≵","NotHumpDownHump":"≎̸","NotHumpEqual":"≏̸","notin":"∉","notindot":"⋵̸","notinE":"⋹̸","notinva":"∉","notinvb":"⋷","notinvc":"⋶","NotLeftTriangleBar":"⧏̸","NotLeftTriangle":"⋪","NotLeftTriangleEqual":"⋬","NotLess":"≮","NotLessEqual":"≰","NotLessGreater":"≸","NotLessLess":"≪̸","NotLessSlantEqual":"⩽̸","NotLessTilde":"≴","NotNestedGreaterGreater":"⪢̸","NotNestedLessLess":"⪡̸","notni":"∌","notniva":"∌","notnivb":"⋾","notnivc":"⋽","NotPrecedes":"⊀","NotPrecedesEqual":"⪯̸","NotPrecedesSlantEqual":"⋠","NotReverseElement":"∌","NotRightTriangleBar":"⧐̸","NotRightTriangle":"⋫","NotRightTriangleEqual":"⋭","NotSquareSubset":"⊏̸","NotSquareSubsetEqual":"⋢","NotSquareSuperset":"⊐̸","NotSquareSupersetEqual":"⋣","NotSubset":"⊂⃒","NotSubsetEqual":"⊈","NotSucceeds":"⊁","NotSucceedsEqual":"⪰̸","NotSucceedsSlantEqual":"⋡","NotSucceedsTilde":"≿̸","NotSuperset":"⊃⃒","NotSupersetEqual":"⊉","NotTilde":"≁","NotTildeEqual":"≄","NotTildeFullEqual":"≇","NotTildeTilde":"≉","NotVerticalBar":"∤","nparallel":"∦","npar":"∦","nparsl":"⫽⃥","npart":"∂̸","npolint":"⨔","npr":"⊀","nprcue":"⋠","nprec":"⊀","npreceq":"⪯̸","npre":"⪯̸","nrarrc":"⤳̸","nrarr":"↛","nrArr":"⇏","nrarrw":"↝̸","nrightarrow":"↛","nRightarrow":"⇏","nrtri":"⋫","nrtrie":"⋭","nsc":"⊁","nsccue":"⋡","nsce":"⪰̸","Nscr":"𝒩","nscr":"𝓃","nshortmid":"∤","nshortparallel":"∦","nsim":"≁","nsime":"≄","nsimeq":"≄","nsmid":"∤","nspar":"∦","nsqsube":"⋢","nsqsupe":"⋣","nsub":"⊄","nsubE":"⫅̸","nsube":"⊈","nsubset":"⊂⃒","nsubseteq":"⊈","nsubseteqq":"⫅̸","nsucc":"⊁","nsucceq":"⪰̸","nsup":"⊅","nsupE":"⫆̸","nsupe":"⊉","nsupset":"⊃⃒","nsupseteq":"⊉","nsupseteqq":"⫆̸","ntgl":"≹","Ntilde":"Ñ","ntilde":"ñ","ntlg":"≸","ntriangleleft":"⋪","ntrianglelefteq":"⋬","ntriangleright":"⋫","ntrianglerighteq":"⋭","Nu":"Ν","nu":"ν","num":"#","numero":"№","numsp":" ","nvap":"≍⃒","nvdash":"⊬","nvDash":"⊭","nVdash":"⊮","nVDash":"⊯","nvge":"≥⃒","nvgt":">⃒","nvHarr":"⤄","nvinfin":"⧞","nvlArr":"⤂","nvle":"≤⃒","nvlt":"<⃒","nvltrie":"⊴⃒","nvrArr":"⤃","nvrtrie":"⊵⃒","nvsim":"∼⃒","nwarhk":"⤣","nwarr":"↖","nwArr":"⇖","nwarrow":"↖","nwnear":"⤧","Oacute":"Ó","oacute":"ó","oast":"⊛","Ocirc":"Ô","ocirc":"ô","ocir":"⊚","Ocy":"О","ocy":"о","odash":"⊝","Odblac":"Ő","odblac":"ő","odiv":"⨸","odot":"⊙","odsold":"⦼","OElig":"Œ","oelig":"œ","ofcir":"⦿","Ofr":"𝔒","ofr":"𝔬","ogon":"˛","Ograve":"Ò","ograve":"ò","ogt":"⧁","ohbar":"⦵","ohm":"Ω","oint":"∮","olarr":"↺","olcir":"⦾","olcross":"⦻","oline":"‾","olt":"⧀","Omacr":"Ō","omacr":"ō","Omega":"Ω","omega":"ω","Omicron":"Ο","omicron":"ο","omid":"⦶","ominus":"⊖","Oopf":"𝕆","oopf":"𝕠","opar":"⦷","OpenCurlyDoubleQuote":"“","OpenCurlyQuote":"‘","operp":"⦹","oplus":"⊕","orarr":"↻","Or":"⩔","or":"∨","ord":"⩝","order":"ℴ","orderof":"ℴ","ordf":"ª","ordm":"º","origof":"⊶","oror":"⩖","orslope":"⩗","orv":"⩛","oS":"Ⓢ","Oscr":"𝒪","oscr":"ℴ","Oslash":"Ø","oslash":"ø","osol":"⊘","Otilde":"Õ","otilde":"õ","otimesas":"⨶","Otimes":"⨷","otimes":"⊗","Ouml":"Ö","ouml":"ö","ovbar":"⌽","OverBar":"‾","OverBrace":"⏞","OverBracket":"⎴","OverParenthesis":"⏜","para":"¶","parallel":"∥","par":"∥","parsim":"⫳","parsl":"⫽","part":"∂","PartialD":"∂","Pcy":"П","pcy":"п","percnt":"%","period":".","permil":"‰","perp":"⊥","pertenk":"‱","Pfr":"𝔓","pfr":"𝔭","Phi":"Φ","phi":"φ","phiv":"ϕ","phmmat":"ℳ","phone":"☎","Pi":"Π","pi":"π","pitchfork":"⋔","piv":"ϖ","planck":"ℏ","planckh":"ℎ","plankv":"ℏ","plusacir":"⨣","plusb":"⊞","pluscir":"⨢","plus":"+","plusdo":"∔","plusdu":"⨥","pluse":"⩲","PlusMinus":"±","plusmn":"±","plussim":"⨦","plustwo":"⨧","pm":"±","Poincareplane":"ℌ","pointint":"⨕","popf":"𝕡","Popf":"ℙ","pound":"£","prap":"⪷","Pr":"⪻","pr":"≺","prcue":"≼","precapprox":"⪷","prec":"≺","preccurlyeq":"≼","Precedes":"≺","PrecedesEqual":"⪯","PrecedesSlantEqual":"≼","PrecedesTilde":"≾","preceq":"⪯","precnapprox":"⪹","precneqq":"⪵","precnsim":"⋨","pre":"⪯","prE":"⪳","precsim":"≾","prime":"′","Prime":"″","primes":"ℙ","prnap":"⪹","prnE":"⪵","prnsim":"⋨","prod":"∏","Product":"∏","profalar":"⌮","profline":"⌒","profsurf":"⌓","prop":"∝","Proportional":"∝","Proportion":"∷","propto":"∝","prsim":"≾","prurel":"⊰","Pscr":"𝒫","pscr":"𝓅","Psi":"Ψ","psi":"ψ","puncsp":" ","Qfr":"𝔔","qfr":"𝔮","qint":"⨌","qopf":"𝕢","Qopf":"ℚ","qprime":"⁗","Qscr":"𝒬","qscr":"𝓆","quaternions":"ℍ","quatint":"⨖","quest":"?","questeq":"≟","quot":"\"","QUOT":"\"","rAarr":"⇛","race":"∽̱","Racute":"Ŕ","racute":"ŕ","radic":"√","raemptyv":"⦳","rang":"⟩","Rang":"⟫","rangd":"⦒","range":"⦥","rangle":"⟩","raquo":"»","rarrap":"⥵","rarrb":"⇥","rarrbfs":"⤠","rarrc":"⤳","rarr":"→","Rarr":"↠","rArr":"⇒","rarrfs":"⤞","rarrhk":"↪","rarrlp":"↬","rarrpl":"⥅","rarrsim":"⥴","Rarrtl":"⤖","rarrtl":"↣","rarrw":"↝","ratail":"⤚","rAtail":"⤜","ratio":"∶","rationals":"ℚ","rbarr":"⤍","rBarr":"⤏","RBarr":"⤐","rbbrk":"❳","rbrace":"}","rbrack":"]","rbrke":"⦌","rbrksld":"⦎","rbrkslu":"⦐","Rcaron":"Ř","rcaron":"ř","Rcedil":"Ŗ","rcedil":"ŗ","rceil":"⌉","rcub":"}","Rcy":"Р","rcy":"р","rdca":"⤷","rdldhar":"⥩","rdquo":"”","rdquor":"”","rdsh":"↳","real":"ℜ","realine":"ℛ","realpart":"ℜ","reals":"ℝ","Re":"ℜ","rect":"▭","reg":"®","REG":"®","ReverseElement":"∋","ReverseEquilibrium":"⇋","ReverseUpEquilibrium":"⥯","rfisht":"⥽","rfloor":"⌋","rfr":"𝔯","Rfr":"ℜ","rHar":"⥤","rhard":"⇁","rharu":"⇀","rharul":"⥬","Rho":"Ρ","rho":"ρ","rhov":"ϱ","RightAngleBracket":"⟩","RightArrowBar":"⇥","rightarrow":"→","RightArrow":"→","Rightarrow":"⇒","RightArrowLeftArrow":"⇄","rightarrowtail":"↣","RightCeiling":"⌉","RightDoubleBracket":"⟧","RightDownTeeVector":"⥝","RightDownVectorBar":"⥕","RightDownVector":"⇂","RightFloor":"⌋","rightharpoondown":"⇁","rightharpoonup":"⇀","rightleftarrows":"⇄","rightleftharpoons":"⇌","rightrightarrows":"⇉","rightsquigarrow":"↝","RightTeeArrow":"↦","RightTee":"⊢","RightTeeVector":"⥛","rightthreetimes":"⋌","RightTriangleBar":"⧐","RightTriangle":"⊳","RightTriangleEqual":"⊵","RightUpDownVector":"⥏","RightUpTeeVector":"⥜","RightUpVectorBar":"⥔","RightUpVector":"↾","RightVectorBar":"⥓","RightVector":"⇀","ring":"˚","risingdotseq":"≓","rlarr":"⇄","rlhar":"⇌","rlm":"‏","rmoustache":"⎱","rmoust":"⎱","rnmid":"⫮","roang":"⟭","roarr":"⇾","robrk":"⟧","ropar":"⦆","ropf":"𝕣","Ropf":"ℝ","roplus":"⨮","rotimes":"⨵","RoundImplies":"⥰","rpar":")","rpargt":"⦔","rppolint":"⨒","rrarr":"⇉","Rrightarrow":"⇛","rsaquo":"›","rscr":"𝓇","Rscr":"ℛ","rsh":"↱","Rsh":"↱","rsqb":"]","rsquo":"’","rsquor":"’","rthree":"⋌","rtimes":"⋊","rtri":"▹","rtrie":"⊵","rtrif":"▸","rtriltri":"⧎","RuleDelayed":"⧴","ruluhar":"⥨","rx":"℞","Sacute":"Ś","sacute":"ś","sbquo":"‚","scap":"⪸","Scaron":"Š","scaron":"š","Sc":"⪼","sc":"≻","sccue":"≽","sce":"⪰","scE":"⪴","Scedil":"Ş","scedil":"ş","Scirc":"Ŝ","scirc":"ŝ","scnap":"⪺","scnE":"⪶","scnsim":"⋩","scpolint":"⨓","scsim":"≿","Scy":"С","scy":"с","sdotb":"⊡","sdot":"⋅","sdote":"⩦","searhk":"⤥","searr":"↘","seArr":"⇘","searrow":"↘","sect":"§","semi":";","seswar":"⤩","setminus":"∖","setmn":"∖","sext":"✶","Sfr":"𝔖","sfr":"𝔰","sfrown":"⌢","sharp":"♯","SHCHcy":"Щ","shchcy":"щ","SHcy":"Ш","shcy":"ш","ShortDownArrow":"↓","ShortLeftArrow":"←","shortmid":"∣","shortparallel":"∥","ShortRightArrow":"→","ShortUpArrow":"↑","shy":"­","Sigma":"Σ","sigma":"σ","sigmaf":"ς","sigmav":"ς","sim":"∼","simdot":"⩪","sime":"≃","simeq":"≃","simg":"⪞","simgE":"⪠","siml":"⪝","simlE":"⪟","simne":"≆","simplus":"⨤","simrarr":"⥲","slarr":"←","SmallCircle":"∘","smallsetminus":"∖","smashp":"⨳","smeparsl":"⧤","smid":"∣","smile":"⌣","smt":"⪪","smte":"⪬","smtes":"⪬︀","SOFTcy":"Ь","softcy":"ь","solbar":"⌿","solb":"⧄","sol":"/","Sopf":"𝕊","sopf":"𝕤","spades":"♠","spadesuit":"♠","spar":"∥","sqcap":"⊓","sqcaps":"⊓︀","sqcup":"⊔","sqcups":"⊔︀","Sqrt":"√","sqsub":"⊏","sqsube":"⊑","sqsubset":"⊏","sqsubseteq":"⊑","sqsup":"⊐","sqsupe":"⊒","sqsupset":"⊐","sqsupseteq":"⊒","square":"□","Square":"□","SquareIntersection":"⊓","SquareSubset":"⊏","SquareSubsetEqual":"⊑","SquareSuperset":"⊐","SquareSupersetEqual":"⊒","SquareUnion":"⊔","squarf":"▪","squ":"□","squf":"▪","srarr":"→","Sscr":"𝒮","sscr":"𝓈","ssetmn":"∖","ssmile":"⌣","sstarf":"⋆","Star":"⋆","star":"☆","starf":"★","straightepsilon":"ϵ","straightphi":"ϕ","strns":"¯","sub":"⊂","Sub":"⋐","subdot":"⪽","subE":"⫅","sube":"⊆","subedot":"⫃","submult":"⫁","subnE":"⫋","subne":"⊊","subplus":"⪿","subrarr":"⥹","subset":"⊂","Subset":"⋐","subseteq":"⊆","subseteqq":"⫅","SubsetEqual":"⊆","subsetneq":"⊊","subsetneqq":"⫋","subsim":"⫇","subsub":"⫕","subsup":"⫓","succapprox":"⪸","succ":"≻","succcurlyeq":"≽","Succeeds":"≻","SucceedsEqual":"⪰","SucceedsSlantEqual":"≽","SucceedsTilde":"≿","succeq":"⪰","succnapprox":"⪺","succneqq":"⪶","succnsim":"⋩","succsim":"≿","SuchThat":"∋","sum":"∑","Sum":"∑","sung":"♪","sup1":"¹","sup2":"²","sup3":"³","sup":"⊃","Sup":"⋑","supdot":"⪾","supdsub":"⫘","supE":"⫆","supe":"⊇","supedot":"⫄","Superset":"⊃","SupersetEqual":"⊇","suphsol":"⟉","suphsub":"⫗","suplarr":"⥻","supmult":"⫂","supnE":"⫌","supne":"⊋","supplus":"⫀","supset":"⊃","Supset":"⋑","supseteq":"⊇","supseteqq":"⫆","supsetneq":"⊋","supsetneqq":"⫌","supsim":"⫈","supsub":"⫔","supsup":"⫖","swarhk":"⤦","swarr":"↙","swArr":"⇙","swarrow":"↙","swnwar":"⤪","szlig":"ß","Tab":"\t","target":"⌖","Tau":"Τ","tau":"τ","tbrk":"⎴","Tcaron":"Ť","tcaron":"ť","Tcedil":"Ţ","tcedil":"ţ","Tcy":"Т","tcy":"т","tdot":"⃛","telrec":"⌕","Tfr":"𝔗","tfr":"𝔱","there4":"∴","therefore":"∴","Therefore":"∴","Theta":"Θ","theta":"θ","thetasym":"ϑ","thetav":"ϑ","thickapprox":"≈","thicksim":"∼","ThickSpace":"  ","ThinSpace":" ","thinsp":" ","thkap":"≈","thksim":"∼","THORN":"Þ","thorn":"þ","tilde":"˜","Tilde":"∼","TildeEqual":"≃","TildeFullEqual":"≅","TildeTilde":"≈","timesbar":"⨱","timesb":"⊠","times":"×","timesd":"⨰","tint":"∭","toea":"⤨","topbot":"⌶","topcir":"⫱","top":"⊤","Topf":"𝕋","topf":"𝕥","topfork":"⫚","tosa":"⤩","tprime":"‴","trade":"™","TRADE":"™","triangle":"▵","triangledown":"▿","triangleleft":"◃","trianglelefteq":"⊴","triangleq":"≜","triangleright":"▹","trianglerighteq":"⊵","tridot":"◬","trie":"≜","triminus":"⨺","TripleDot":"⃛","triplus":"⨹","trisb":"⧍","tritime":"⨻","trpezium":"⏢","Tscr":"𝒯","tscr":"𝓉","TScy":"Ц","tscy":"ц","TSHcy":"Ћ","tshcy":"ћ","Tstrok":"Ŧ","tstrok":"ŧ","twixt":"≬","twoheadleftarrow":"↞","twoheadrightarrow":"↠","Uacute":"Ú","uacute":"ú","uarr":"↑","Uarr":"↟","uArr":"⇑","Uarrocir":"⥉","Ubrcy":"Ў","ubrcy":"ў","Ubreve":"Ŭ","ubreve":"ŭ","Ucirc":"Û","ucirc":"û","Ucy":"У","ucy":"у","udarr":"⇅","Udblac":"Ű","udblac":"ű","udhar":"⥮","ufisht":"⥾","Ufr":"𝔘","ufr":"𝔲","Ugrave":"Ù","ugrave":"ù","uHar":"⥣","uharl":"↿","uharr":"↾","uhblk":"▀","ulcorn":"⌜","ulcorner":"⌜","ulcrop":"⌏","ultri":"◸","Umacr":"Ū","umacr":"ū","uml":"¨","UnderBar":"_","UnderBrace":"⏟","UnderBracket":"⎵","UnderParenthesis":"⏝","Union":"⋃","UnionPlus":"⊎","Uogon":"Ų","uogon":"ų","Uopf":"𝕌","uopf":"𝕦","UpArrowBar":"⤒","uparrow":"↑","UpArrow":"↑","Uparrow":"⇑","UpArrowDownArrow":"⇅","updownarrow":"↕","UpDownArrow":"↕","Updownarrow":"⇕","UpEquilibrium":"⥮","upharpoonleft":"↿","upharpoonright":"↾","uplus":"⊎","UpperLeftArrow":"↖","UpperRightArrow":"↗","upsi":"υ","Upsi":"ϒ","upsih":"ϒ","Upsilon":"Υ","upsilon":"υ","UpTeeArrow":"↥","UpTee":"⊥","upuparrows":"⇈","urcorn":"⌝","urcorner":"⌝","urcrop":"⌎","Uring":"Ů","uring":"ů","urtri":"◹","Uscr":"𝒰","uscr":"𝓊","utdot":"⋰","Utilde":"Ũ","utilde":"ũ","utri":"▵","utrif":"▴","uuarr":"⇈","Uuml":"Ü","uuml":"ü","uwangle":"⦧","vangrt":"⦜","varepsilon":"ϵ","varkappa":"ϰ","varnothing":"∅","varphi":"ϕ","varpi":"ϖ","varpropto":"∝","varr":"↕","vArr":"⇕","varrho":"ϱ","varsigma":"ς","varsubsetneq":"⊊︀","varsubsetneqq":"⫋︀","varsupsetneq":"⊋︀","varsupsetneqq":"⫌︀","vartheta":"ϑ","vartriangleleft":"⊲","vartriangleright":"⊳","vBar":"⫨","Vbar":"⫫","vBarv":"⫩","Vcy":"В","vcy":"в","vdash":"⊢","vDash":"⊨","Vdash":"⊩","VDash":"⊫","Vdashl":"⫦","veebar":"⊻","vee":"∨","Vee":"⋁","veeeq":"≚","vellip":"⋮","verbar":"|","Verbar":"‖","vert":"|","Vert":"‖","VerticalBar":"∣","VerticalLine":"|","VerticalSeparator":"❘","VerticalTilde":"≀","VeryThinSpace":" ","Vfr":"𝔙","vfr":"𝔳","vltri":"⊲","vnsub":"⊂⃒","vnsup":"⊃⃒","Vopf":"𝕍","vopf":"𝕧","vprop":"∝","vrtri":"⊳","Vscr":"𝒱","vscr":"𝓋","vsubnE":"⫋︀","vsubne":"⊊︀","vsupnE":"⫌︀","vsupne":"⊋︀","Vvdash":"⊪","vzigzag":"⦚","Wcirc":"Ŵ","wcirc":"ŵ","wedbar":"⩟","wedge":"∧","Wedge":"⋀","wedgeq":"≙","weierp":"℘","Wfr":"𝔚","wfr":"𝔴","Wopf":"𝕎","wopf":"𝕨","wp":"℘","wr":"≀","wreath":"≀","Wscr":"𝒲","wscr":"𝓌","xcap":"⋂","xcirc":"◯","xcup":"⋃","xdtri":"▽","Xfr":"𝔛","xfr":"𝔵","xharr":"⟷","xhArr":"⟺","Xi":"Ξ","xi":"ξ","xlarr":"⟵","xlArr":"⟸","xmap":"⟼","xnis":"⋻","xodot":"⨀","Xopf":"𝕏","xopf":"𝕩","xoplus":"⨁","xotime":"⨂","xrarr":"⟶","xrArr":"⟹","Xscr":"𝒳","xscr":"𝓍","xsqcup":"⨆","xuplus":"⨄","xutri":"△","xvee":"⋁","xwedge":"⋀","Yacute":"Ý","yacute":"ý","YAcy":"Я","yacy":"я","Ycirc":"Ŷ","ycirc":"ŷ","Ycy":"Ы","ycy":"ы","yen":"¥","Yfr":"𝔜","yfr":"𝔶","YIcy":"Ї","yicy":"ї","Yopf":"𝕐","yopf":"𝕪","Yscr":"𝒴","yscr":"𝓎","YUcy":"Ю","yucy":"ю","yuml":"ÿ","Yuml":"Ÿ","Zacute":"Ź","zacute":"ź","Zcaron":"Ž","zcaron":"ž","Zcy":"З","zcy":"з","Zdot":"Ż","zdot":"ż","zeetrf":"ℨ","ZeroWidthSpace":"​","Zeta":"Ζ","zeta":"ζ","zfr":"𝔷","Zfr":"ℨ","ZHcy":"Ж","zhcy":"ж","zigrarr":"⇝","zopf":"𝕫","Zopf":"ℤ","Zscr":"𝒵","zscr":"𝓏","zwj":"‍","zwnj":"‌"}

/***/ }),
/* 432 */,
/* 433 */
/***/ (function(module, exports) {

// removed by extract-text-webpack-plugin

/***/ }),
/* 434 */
/***/ (function(module, exports) {

// removed by extract-text-webpack-plugin

/***/ }),
/* 435 */
/***/ (function(module, exports) {

// removed by extract-text-webpack-plugin

/***/ }),
/* 436 */
/***/ (function(module, exports) {

// removed by extract-text-webpack-plugin

/***/ }),
/* 437 */
/***/ (function(module, exports) {

// removed by extract-text-webpack-plugin

/***/ }),
/* 438 */
/***/ (function(module, exports) {

// removed by extract-text-webpack-plugin

/***/ }),
/* 439 */
/***/ (function(module, exports) {

// removed by extract-text-webpack-plugin

/***/ }),
/* 440 */
/***/ (function(module, exports) {

// removed by extract-text-webpack-plugin

/***/ }),
/* 441 */
/***/ (function(module, exports) {

// removed by extract-text-webpack-plugin

/***/ }),
/* 442 */
/***/ (function(module, exports) {

// removed by extract-text-webpack-plugin

/***/ }),
/* 443 */
/***/ (function(module, exports) {

// removed by extract-text-webpack-plugin

/***/ }),
/* 444 */
/***/ (function(module, exports) {

// removed by extract-text-webpack-plugin

/***/ }),
/* 445 */
/***/ (function(module, exports) {

// removed by extract-text-webpack-plugin

/***/ }),
/* 446 */
/***/ (function(module, exports) {

// removed by extract-text-webpack-plugin

/***/ }),
/* 447 */
/***/ (function(module, exports) {

// removed by extract-text-webpack-plugin

/***/ }),
/* 448 */
/***/ (function(module, exports) {

// removed by extract-text-webpack-plugin

/***/ }),
/* 449 */
/***/ (function(module, exports) {

// removed by extract-text-webpack-plugin

/***/ }),
/* 450 */
/***/ (function(module, exports) {

// removed by extract-text-webpack-plugin

/***/ }),
/* 451 */
/***/ (function(module, exports) {

// removed by extract-text-webpack-plugin

/***/ }),
/* 452 */
/***/ (function(module, exports) {

// removed by extract-text-webpack-plugin

/***/ }),
/* 453 */
/***/ (function(module, exports) {

// removed by extract-text-webpack-plugin

/***/ }),
/* 454 */
/***/ (function(module, exports) {

// removed by extract-text-webpack-plugin

/***/ }),
/* 455 */
/***/ (function(module, exports) {

// removed by extract-text-webpack-plugin

/***/ }),
/* 456 */,
/* 457 */,
/* 458 */,
/* 459 */,
/* 460 */,
/* 461 */,
/* 462 */,
/* 463 */,
/* 464 */,
/* 465 */,
/* 466 */,
/* 467 */,
/* 468 */,
/* 469 */,
/* 470 */,
/* 471 */,
/* 472 */,
/* 473 */,
/* 474 */,
/* 475 */,
/* 476 */,
/* 477 */,
/* 478 */,
/* 479 */,
/* 480 */,
/* 481 */,
/* 482 */,
/* 483 */,
/* 484 */,
/* 485 */,
/* 486 */,
/* 487 */,
/* 488 */,
/* 489 */,
/* 490 */,
/* 491 */,
/* 492 */
/***/ (function(module, exports) {

module.exports = {"$schema":"http://json-schema.org/draft-06/schema#","$id":"https://raw.githubusercontent.com/epoberezkin/ajv/master/lib/refs/$data.json#","description":"Meta-schema for $data reference (JSON-schema extension proposal)","type":"object","required":["$data"],"properties":{"$data":{"type":"string","anyOf":[{"format":"relative-json-pointer"},{"format":"json-pointer"}]}},"additionalProperties":false}

/***/ }),
/* 493 */
/***/ (function(module, exports) {

module.exports = {"$schema":"http://json-schema.org/draft-06/schema#","$id":"http://json-schema.org/draft-06/schema#","title":"Core schema meta-schema","definitions":{"schemaArray":{"type":"array","minItems":1,"items":{"$ref":"#"}},"nonNegativeInteger":{"type":"integer","minimum":0},"nonNegativeIntegerDefault0":{"allOf":[{"$ref":"#/definitions/nonNegativeInteger"},{"default":0}]},"simpleTypes":{"enum":["array","boolean","integer","null","number","object","string"]},"stringArray":{"type":"array","items":{"type":"string"},"uniqueItems":true,"default":[]}},"type":["object","boolean"],"properties":{"$id":{"type":"string","format":"uri-reference"},"$schema":{"type":"string","format":"uri"},"$ref":{"type":"string","format":"uri-reference"},"title":{"type":"string"},"description":{"type":"string"},"default":{},"examples":{"type":"array","items":{}},"multipleOf":{"type":"number","exclusiveMinimum":0},"maximum":{"type":"number"},"exclusiveMaximum":{"type":"number"},"minimum":{"type":"number"},"exclusiveMinimum":{"type":"number"},"maxLength":{"$ref":"#/definitions/nonNegativeInteger"},"minLength":{"$ref":"#/definitions/nonNegativeIntegerDefault0"},"pattern":{"type":"string","format":"regex"},"additionalItems":{"$ref":"#"},"items":{"anyOf":[{"$ref":"#"},{"$ref":"#/definitions/schemaArray"}],"default":{}},"maxItems":{"$ref":"#/definitions/nonNegativeInteger"},"minItems":{"$ref":"#/definitions/nonNegativeIntegerDefault0"},"uniqueItems":{"type":"boolean","default":false},"contains":{"$ref":"#"},"maxProperties":{"$ref":"#/definitions/nonNegativeInteger"},"minProperties":{"$ref":"#/definitions/nonNegativeIntegerDefault0"},"required":{"$ref":"#/definitions/stringArray"},"additionalProperties":{"$ref":"#"},"definitions":{"type":"object","additionalProperties":{"$ref":"#"},"default":{}},"properties":{"type":"object","additionalProperties":{"$ref":"#"},"default":{}},"patternProperties":{"type":"object","additionalProperties":{"$ref":"#"},"default":{}},"dependencies":{"type":"object","additionalProperties":{"anyOf":[{"$ref":"#"},{"$ref":"#/definitions/stringArray"}]}},"propertyNames":{"$ref":"#"},"const":{},"enum":{"type":"array","minItems":1,"uniqueItems":true},"type":{"anyOf":[{"$ref":"#/definitions/simpleTypes"},{"type":"array","items":{"$ref":"#/definitions/simpleTypes"},"minItems":1,"uniqueItems":true}]},"format":{"type":"string"},"allOf":{"$ref":"#/definitions/schemaArray"},"anyOf":{"$ref":"#/definitions/schemaArray"},"oneOf":{"$ref":"#/definitions/schemaArray"},"not":{"$ref":"#"}},"default":{}}

/***/ }),
/* 494 */,
/* 495 */,
/* 496 */,
/* 497 */,
/* 498 */,
/* 499 */,
/* 500 */,
/* 501 */,
/* 502 */,
/* 503 */,
/* 504 */,
/* 505 */,
/* 506 */,
/* 507 */,
/* 508 */,
/* 509 */,
/* 510 */,
/* 511 */,
/* 512 */,
/* 513 */,
/* 514 */,
/* 515 */,
/* 516 */,
/* 517 */,
/* 518 */,
/* 519 */,
/* 520 */,
/* 521 */,
/* 522 */,
/* 523 */,
/* 524 */,
/* 525 */,
/* 526 */,
/* 527 */,
/* 528 */,
/* 529 */,
/* 530 */,
/* 531 */,
/* 532 */,
/* 533 */,
/* 534 */,
/* 535 */,
/* 536 */,
/* 537 */,
/* 538 */,
/* 539 */,
/* 540 */,
/* 541 */,
/* 542 */,
/* 543 */,
/* 544 */,
/* 545 */,
/* 546 */,
/* 547 */,
/* 548 */,
/* 549 */,
/* 550 */,
/* 551 */,
/* 552 */,
/* 553 */,
/* 554 */,
/* 555 */,
/* 556 */,
/* 557 */,
/* 558 */,
/* 559 */,
/* 560 */,
/* 561 */,
/* 562 */,
/* 563 */,
/* 564 */,
/* 565 */,
/* 566 */,
/* 567 */,
/* 568 */,
/* 569 */,
/* 570 */,
/* 571 */,
/* 572 */,
/* 573 */,
/* 574 */,
/* 575 */,
/* 576 */,
/* 577 */,
/* 578 */,
/* 579 */,
/* 580 */,
/* 581 */,
/* 582 */,
/* 583 */,
/* 584 */,
/* 585 */,
/* 586 */,
/* 587 */,
/* 588 */,
/* 589 */,
/* 590 */,
/* 591 */,
/* 592 */,
/* 593 */,
/* 594 */,
/* 595 */,
/* 596 */,
/* 597 */,
/* 598 */,
/* 599 */,
/* 600 */,
/* 601 */,
/* 602 */,
/* 603 */,
/* 604 */,
/* 605 */,
/* 606 */
/***/ (function(module, exports, __webpack_require__) {


/* styles */
__webpack_require__(444)

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(292),
  /* template */
  __webpack_require__(650),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 607 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(293),
  /* template */
  __webpack_require__(669),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 608 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(294),
  /* template */
  __webpack_require__(646),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 609 */
/***/ (function(module, exports, __webpack_require__) {


/* styles */
__webpack_require__(450)

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(296),
  /* template */
  __webpack_require__(664),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 610 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(297),
  /* template */
  __webpack_require__(674),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 611 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(298),
  /* template */
  __webpack_require__(659),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 612 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(299),
  /* template */
  __webpack_require__(645),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 613 */
/***/ (function(module, exports, __webpack_require__) {


/* styles */
__webpack_require__(454)

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(300),
  /* template */
  __webpack_require__(672),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 614 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(301),
  /* template */
  __webpack_require__(652),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 615 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(302),
  /* template */
  __webpack_require__(680),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 616 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(303),
  /* template */
  __webpack_require__(656),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 617 */
/***/ (function(module, exports, __webpack_require__) {


/* styles */
__webpack_require__(446)

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(304),
  /* template */
  __webpack_require__(657),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 618 */
/***/ (function(module, exports, __webpack_require__) {


/* styles */
__webpack_require__(443)

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(305),
  /* template */
  __webpack_require__(649),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 619 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(306),
  /* template */
  __webpack_require__(673),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 620 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(307),
  /* template */
  __webpack_require__(660),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 621 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(308),
  /* template */
  __webpack_require__(671),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 622 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(309),
  /* template */
  __webpack_require__(666),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 623 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(310),
  /* template */
  __webpack_require__(667),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 624 */
/***/ (function(module, exports, __webpack_require__) {


/* styles */
__webpack_require__(445)

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(311),
  /* template */
  __webpack_require__(653),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 625 */
/***/ (function(module, exports, __webpack_require__) {


/* styles */
__webpack_require__(451)

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(312),
  /* template */
  __webpack_require__(665),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 626 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(313),
  /* template */
  __webpack_require__(676),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 627 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(314),
  /* template */
  __webpack_require__(678),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 628 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(315),
  /* template */
  __webpack_require__(643),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 629 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(316),
  /* template */
  __webpack_require__(679),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 630 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(317),
  /* template */
  __webpack_require__(655),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 631 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(318),
  /* template */
  __webpack_require__(651),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 632 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(319),
  /* template */
  __webpack_require__(670),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 633 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(320),
  /* template */
  __webpack_require__(662),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 634 */
/***/ (function(module, exports, __webpack_require__) {


/* styles */
__webpack_require__(447)

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(321),
  /* template */
  __webpack_require__(658),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 635 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(322),
  /* template */
  __webpack_require__(654),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 636 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(323),
  /* template */
  __webpack_require__(663),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 637 */
/***/ (function(module, exports, __webpack_require__) {


/* styles */
__webpack_require__(448)
__webpack_require__(449)

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(324),
  /* template */
  __webpack_require__(661),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 638 */
/***/ (function(module, exports, __webpack_require__) {


/* styles */
__webpack_require__(452)
__webpack_require__(453)

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(325),
  /* template */
  __webpack_require__(668),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 639 */
/***/ (function(module, exports, __webpack_require__) {


/* styles */
__webpack_require__(455)

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(327),
  /* template */
  __webpack_require__(675),
  /* scopeId */
  "data-v-c4a3db2e",
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 640 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(329),
  /* template */
  __webpack_require__(681),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 641 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(330),
  /* template */
  __webpack_require__(677),
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 642 */
/***/ (function(module, exports, __webpack_require__) {

var Component = __webpack_require__(0)(
  /* script */
  __webpack_require__(331),
  /* template */
  null,
  /* scopeId */
  null,
  /* cssModules */
  null
)

module.exports = Component.exports


/***/ }),
/* 643 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "checkbox"
  }, [_c('label', [_c('input', {
    directives: [{
      name: "model",
      rawName: "v-model",
      value: (_vm.value),
      expression: "value"
    }],
    attrs: {
      "type": "checkbox"
    },
    domProps: {
      "checked": Array.isArray(_vm.value) ? _vm._i(_vm.value, null) > -1 : (_vm.value)
    },
    on: {
      "change": function($event) {
        var $$a = _vm.value,
          $$el = $event.target,
          $$c = $$el.checked ? (true) : (false);
        if (Array.isArray($$a)) {
          var $$v = null,
            $$i = _vm._i($$a, $$v);
          if ($$el.checked) {
            $$i < 0 && (_vm.value = $$a.concat([$$v]))
          } else {
            $$i > -1 && (_vm.value = $$a.slice(0, $$i).concat($$a.slice($$i + 1)))
          }
        } else {
          _vm.value = $$c
        }
      }
    }
  })])])
},staticRenderFns: []}

/***/ }),
/* 644 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return (_vm.tablePagination && _vm.tablePagination.last_page > 1) ? _c('div', {
    class: _vm.css.wrapperClass
  }, [_c('a', {
    class: ['btn-nav', _vm.css.linkClass, _vm.isOnFirstPage ? _vm.css.disabledClass : ''],
    on: {
      "click": function($event) {
        _vm.loadPage(1)
      }
    }
  }, [(_vm.css.icons.first != '') ? _c('i', {
    class: [_vm.css.icons.first]
  }) : _c('span', [_vm._v("«")])]), _vm._v(" "), _c('a', {
    class: ['btn-nav', _vm.css.linkClass, _vm.isOnFirstPage ? _vm.css.disabledClass : ''],
    on: {
      "click": function($event) {
        _vm.loadPage('prev')
      }
    }
  }, [(_vm.css.icons.next != '') ? _c('i', {
    class: [_vm.css.icons.prev]
  }) : _c('span', [_vm._v(" ‹")])]), _vm._v(" "), (_vm.notEnoughPages) ? [_vm._l((_vm.totalPage), function(n) {
    return [_c('a', {
      class: [_vm.css.pageClass, _vm.isCurrentPage(n) ? _vm.css.activeClass : ''],
      domProps: {
        "innerHTML": _vm._s(n)
      },
      on: {
        "click": function($event) {
          _vm.loadPage(n)
        }
      }
    })]
  })] : [_vm._l((_vm.windowSize), function(n) {
    return [_c('a', {
      class: [_vm.css.pageClass, _vm.isCurrentPage(_vm.windowStart + n - 1) ? _vm.css.activeClass : ''],
      domProps: {
        "innerHTML": _vm._s(_vm.windowStart + n - 1)
      },
      on: {
        "click": function($event) {
          _vm.loadPage(_vm.windowStart + n - 1)
        }
      }
    })]
  })], _vm._v(" "), _c('a', {
    class: ['btn-nav', _vm.css.linkClass, _vm.isOnLastPage ? _vm.css.disabledClass : ''],
    on: {
      "click": function($event) {
        _vm.loadPage('next')
      }
    }
  }, [(_vm.css.icons.next != '') ? _c('i', {
    class: [_vm.css.icons.next]
  }) : _c('span', [_vm._v("› ")])]), _vm._v(" "), _c('a', {
    class: ['btn-nav', _vm.css.linkClass, _vm.isOnLastPage ? _vm.css.disabledClass : ''],
    on: {
      "click": function($event) {
        _vm.loadPage(_vm.totalPage)
      }
    }
  }, [(_vm.css.icons.last != '') ? _c('i', {
    class: [_vm.css.icons.last]
  }) : _c('span', [_vm._v("»")])])], 2) : _vm._e()
},staticRenderFns: []}

/***/ }),
/* 645 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "content-wrapper"
  }, [_c('section', {
    staticClass: "content-header"
  }, [_c('h1', [_vm._v("\n      " + _vm._s(_vm._f("titleCase")(_vm.selectedTable)) + " - "), _c('b', [_vm._v(_vm._s(_vm._f("titleCase")(_vm._f("chooseTitle")(_vm.selectedRow))))]), _vm._v(" "), _c('small', [_vm._v(_vm._s(_vm.$route.meta.description))])]), _vm._v(" "), _c('ol', {
    staticClass: "breadcrumb"
  }, [_vm._m(0), _vm._v(" "), _vm._l((_vm.$route.meta.breadcrumb), function(crumb) {
    return _c('li', [(crumb.to) ? [_c('router-link', {
      attrs: {
        "to": crumb.to
      }
    }, [_vm._v(_vm._s(crumb.label))])] : [_vm._v("\n          " + _vm._s(crumb.label) + "\n        ")]], 2)
  })], 2), _vm._v(" "), _c('div', {
    staticClass: "pull-right"
  }, [_c('div', {
    staticClass: "ui icon buttons"
  }, [_c('button', {
    staticClass: "btn btn-box-tool",
    on: {
      "click": function($event) {
        $event.preventDefault();
        _vm.editRow()
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-edit fa-3x "
  })]), _vm._v(" "), _c('button', {
    staticClass: "btn btn-box-tool",
    on: {
      "click": function($event) {
        $event.preventDefault();
        _vm.refreshRow()
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-sync fa-3x "
  })])])])]), _vm._v(" "), _c('section', {
    staticClass: "content"
  }, [(_vm.showAddEdit) ? _c('div', {
    staticClass: "col-md-12"
  }, [(_vm.selectedAction != null) ? _c('div', {
    staticClass: "row"
  }, [_c('action-view', {
    attrs: {
      "action-manager": _vm.actionManager,
      "action": _vm.selectedAction,
      "json-api": _vm.jsonApi,
      "model": _vm.selectedRow
    },
    on: {
      "cancel": function($event) {
        _vm.showAddEdit = false
      },
      "action-complete": function($event) {
        _vm.showAddEdit = false
      }
    }
  })], 1) : _vm._e(), _vm._v(" "), (_vm.rowBeingEdited != null) ? _c('div', {
    staticClass: "row"
  }, [_c('model-form', {
    ref: "modelform",
    attrs: {
      "json-api": _vm.jsonApi,
      "model": _vm.rowBeingEdited,
      "meta": _vm.selectedTableColumns
    },
    on: {
      "save": function($event) {
        _vm.saveRow(_vm.rowBeingEdited)
      },
      "cancel": function($event) {
        _vm.showAddEdit = false
      }
    }
  })], 1) : _vm._e()]) : _vm._e(), _vm._v(" "), _c('div', {
    staticClass: "col-md-9"
  }, [(_vm.selectedRow) ? _c('detailed-table-row', {
    attrs: {
      "model": _vm.selectedRow,
      "json-api": _vm.jsonApi,
      "json-api-model-name": _vm.selectedTable
    }
  }) : _vm._e()], 1), _vm._v(" "), _c('div', {
    staticClass: "col-md-3"
  }, [(_vm.stateMachines != null && _vm.stateMachines.length > 0) ? _c('div', {
    staticClass: "row"
  }, [_vm._m(1), _vm._v(" "), _vm._l((_vm.stateMachines), function(a, k) {
    return _c('div', {
      staticClass: "col-md-12"
    }, [_c('button', {
      staticClass: "btn btn-default",
      staticStyle: {
        "width": "100%"
      },
      on: {
        "click": function($event) {
          _vm.addStateMachine(a)
        }
      }
    }, [_vm._v(_vm._s(a.label))])])
  })], 2) : _vm._e(), _vm._v(" "), (_vm.actions != null) ? _c('div', {
    staticClass: "row"
  }, [_vm._m(2), _vm._v(" "), _vm._l((_vm.actions), function(a, k) {
    return (!a.InstanceOptional) ? _c('div', {
      staticClass: "col-md-12"
    }, [_c('button', {
      staticClass: "btn btn-default",
      staticStyle: {
        "width": "100%"
      },
      on: {
        "click": function($event) {
          _vm.doAction(a)
        }
      }
    }, [_vm._v(_vm._s(a.Label))])]) : _vm._e()
  })], 2) : _vm._e(), _vm._v(" "), (_vm.visibleWorlds.length > 0) ? _c('div', {
    staticClass: "row"
  }, [_vm._m(3), _vm._v(" "), _vm._l((_vm.visibleWorlds), function(world) {
    return _c('div', {
      staticClass: "col-md-12"
    }, [(_vm.selectedInstanceReferenceId) ? _c('router-link', {
      staticClass: "btn btn-default",
      staticStyle: {
        "width": "100%"
      },
      attrs: {
        "to": {
          name: 'Relation',
          params: {
            tablename: _vm.selectedTable,
            refId: _vm.selectedInstanceReferenceId,
            subTable: world.table_name
          }
        }
      }
    }, [_vm._v("\n            " + _vm._s(_vm._f("titleCase")(world.table_name)) + "\n          ")]) : _vm._e()], 1)
  })], 2) : _vm._e()]), _vm._v(" "), (_vm.objectStates.length > 0) ? _c('div', {
    staticClass: "col-md-12"
  }, [_c('h3', [_vm._v("Status tracks")]), _vm._v(" "), _c('div', {
    staticClass: "row"
  }, _vm._l((_vm.objectStates), function(state, k) {
    return _c('div', {
      staticClass: "col-md-3"
    }, [_c('div', {
      staticClass: "box"
    }, [_c('div', {
      staticClass: "box-header"
    }, [_c('div', {
      staticClass: "box-title"
    }, [_c('small', [_vm._v(_vm._s(state.smd.label))])]), _vm._v(" "), _c('div', {
      staticClass: "box-title pull-right"
    }, [_vm._v("\n                " + _vm._s(_vm._f("titleCase")(state.current_state)) + "\n              ")])]), _vm._v(" "), _c('div', {
      staticClass: "box-body"
    }, _vm._l((state.possibleActions), function(action) {
      return _c('div', {
        staticClass: "col-md-12"
      }, [_c('button', {
        staticClass: "btn btn-primary btn-xs btn-flat",
        staticStyle: {
          "width": "100%",
          "border-radius": "5px",
          "margin": "5px"
        },
        on: {
          "click": function($event) {
            _vm.doEvent(state, action)
          }
        }
      }, [_vm._v(_vm._s(action.label) + "\n                ")])])
    }))])])
  }))]) : _vm._e()])])
},staticRenderFns: [function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('li', [_c('a', {
    attrs: {
      "href": "javascript:;"
    }
  }, [_c('i', {
    staticClass: "fa fa-home"
  }), _vm._v("Home")])])
},function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "col-md-12"
  }, [_c('h2', [_vm._v("Start Tracking")])])
},function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "col-md-12"
  }, [_c('h2', [_vm._v("Actions")])])
},function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "col-md-12"
  }, [_c('h2', [_vm._v("Related")])])
}]}

/***/ }),
/* 646 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('router-view')
},staticRenderFns: []}

/***/ }),
/* 647 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    class: ['vuecard', 'row', _vm.css.tableClass]
  }, [_c('div', {
    staticClass: "vuecard-body"
  }, [_vm._l((_vm.tableData), function(item, index) {
    return _c('div', {
      staticClass: "col-md-4"
    }, [_c('div', {
      class: [_vm.onRowClass(item, index), 'box'],
      staticStyle: {
        "min-height": "250px"
      },
      attrs: {
        "item-index": index,
        "render": _vm.onRowChanged(item)
      },
      on: {
        "dblclick": function($event) {
          _vm.onRowDoubleClicked(item, $event)
        },
        "click": function($event) {
          _vm.onRowClicked(item, $event)
        }
      }
    }, [_c('div', {
      staticClass: "box-header"
    }, [_c('div', {
      staticClass: "box-title"
    }, [_c('span', {
      staticClass: "bold"
    }, [_vm._v(_vm._s(_vm._f("titleCase")(_vm._f("chooseTitle")(item))))])]), _vm._v(" "), _c('div', {
      staticClass: "box-tools pull-right"
    }, [_vm._t("actions", null, {
      rowData: item,
      rowIndex: index
    })], 2)]), _vm._v(" "), _c('div', {
      staticClass: "box-body"
    }, [_vm._l((_vm.tableFields), function(field) {
      return [(field.visible) ? _c('dl', [(!_vm.isSpecialField(field.name)) ? _c('dt', [_vm._v(_vm._s(_vm._f("titleCase")(field.name)))]) : _vm._e(), _vm._v(" "), (_vm.isSpecialField(field.name)) ? [(_vm.apiMode && _vm.extractName(field.name) == '__sequence') ? _c('dd', {
        class: ['vuecard-sequence', field.dataClass],
        domProps: {
          "innerHTML": _vm._s(_vm.tablePagination.from + index)
        }
      }) : _vm._e(), _vm._v(" "), (_vm.extractName(field.name) == '__handle') ? _c('dd', {
        class: ['vuecard-handle', field.dataClass],
        domProps: {
          "innerHTML": _vm._s(_vm.renderIconTag(['handle-icon', _vm.css.handleIcon]))
        }
      }) : _vm._e(), _vm._v(" "), (_vm.extractName(field.name) == '__checkbox') ? _c('dd', {
        class: ['vuecard-checkboxes', field.dataClass]
      }, [_c('input', {
        attrs: {
          "type": "checkbox"
        },
        domProps: {
          "checked": _vm.rowSelected(item, field.name)
        },
        on: {
          "change": function($event) {
            _vm.toggleCheckbox(item, field.name, $event)
          }
        }
      })]) : _vm._e(), _vm._v(" "), (_vm.extractName(field.name) === '__component') ? _c('dd', {
        class: ['vuecard-component', field.dataClass]
      }, [_c(_vm.extractArgs(field.name), {
        tag: "component",
        attrs: {
          "row-data": item,
          "row-index": index,
          "row-field": field.sortField
        }
      })], 1) : _vm._e(), _vm._v(" "), (_vm.extractName(field.name) === '__slot') ? _c('dd', {
        class: ['vuecard-slot', field.dataClass]
      }, [_vm._t(_vm.extractArgs(field.name), null, {
        rowData: item,
        rowIndex: index,
        rowField: field.sortField
      })], 2) : _vm._e()] : [(_vm.hasCallback(field)) ? _c('dd', {
        class: field.dataClass,
        domProps: {
          "innerHTML": _vm._s(_vm.callCallback(field, item))
        },
        on: {
          "click": function($event) {
            _vm.onCellClicked(item, field, $event)
          },
          "dblclick": function($event) {
            _vm.onCellDoubleClicked(item, field, $event)
          }
        }
      }) : _c('dd', {
        class: field.dataClass,
        domProps: {
          "innerHTML": _vm._s(_vm.getObjectValue(item, field.name, ''))
        },
        on: {
          "click": function($event) {
            _vm.onCellClicked(item, field, $event)
          },
          "dblclick": function($event) {
            _vm.onCellDoubleClicked(item, field, $event)
          }
        }
      })]], 2) : _vm._e()]
    })], 2)]), _vm._v(" "), (_vm.useDetailRow) ? [(_vm.isVisibleDetailRow(item[_vm.trackBy])) ? _c('tr', {
      class: [_vm.css.detailRowClass],
      on: {
        "click": function($event) {
          _vm.onDetailRowClick(item, $event)
        }
      }
    }, [_c('transition', {
      attrs: {
        "name": _vm.detailRowTransition
      }
    }, [_c('td', {
      attrs: {
        "colspan": _vm.countVisibleFields
      }
    }, [_c(_vm.detailRowComponent, {
      tag: "component",
      attrs: {
        "model": item,
        "json-api": _vm.jsonApi,
        "json-api-model-name": _vm.jsonApiModelName,
        "row-index": index
      }
    })], 1)])], 1) : _vm._e()] : _vm._e()], 2)
  }), _vm._v(" "), (_vm.lessThanMinRows) ? _vm._l((_vm.blankRows), function(i) {
    return _c('tr', {
      staticClass: "blank-row"
    }, [_vm._l((_vm.tableFields), function(field) {
      return [(field.visible) ? _c('td', [_vm._v(" ")]) : _vm._e()]
    })], 2)
  }) : _vm._e()], 2)])
},staticRenderFns: []}

/***/ }),
/* 648 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    attrs: {
      "id": "app"
    }
  }, [(_vm.loaded) ? [_c('router-view')] : _vm._e()], 2)
},staticRenderFns: []}

/***/ }),
/* 649 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('ul', {
    staticClass: "sidebar-menu"
  }, [_c('li', {
    staticClass: "pageLink",
    on: {
      "click": _vm.toggleMenu
    }
  }, [_c('router-link', {
    attrs: {
      "to": {
        name: 'Dashboard',
        params: {}
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-tv"
  }), _vm._v(" "), _c('span', {
    staticClass: "page"
  }, [_vm._v("Dashboard")])])], 1), _vm._v(" "), _c('li', {
    staticClass: "treeview"
  }, [_vm._m(0), _vm._v(" "), _c('ul', {
    staticClass: "treeview-menu"
  }, _vm._l((_vm.topWorlds), function(w) {
    return (w.table_name != 'user_account' && w.table_name != 'usergroup') ? _c('li', {
      staticClass: "pageLink",
      on: {
        "click": _vm.toggleMenu
      }
    }, [_c('router-link', {
      class: w.table_name + '-link',
      attrs: {
        "to": {
          name: 'Entity',
          params: {
            tablename: w.table_name
          }
        }
      }
    }, [_c('span', {
      staticClass: "page"
    }, [_vm._v(_vm._s(_vm._f("titleCase")(w.table_name)))])])], 1) : _vm._e()
  }))]), _vm._v(" "), _c('li', {
    staticClass: "treeview"
  }, [_vm._m(1), _vm._v(" "), _c('ul', {
    staticClass: "treeview-menu"
  }, [_c('li', {
    staticClass: "pageLink",
    on: {
      "click": _vm.toggleMenu
    }
  }, [_c('router-link', {
    class: 'user-link',
    attrs: {
      "to": {
        name: 'Entity',
        params: {
          tablename: 'user_account'
        }
      }
    }
  }, [_c('i', {
    staticClass: "fa fa-user"
  }), _vm._v(" "), _c('span', {
    staticClass: "page"
  }, [_vm._v("User account")])])], 1), _vm._v(" "), _c('li', {
    staticClass: "pageLink",
    on: {
      "click": _vm.toggleMenu
    }
  }, [_c('router-link', {
    class: 'user-link',
    attrs: {
      "to": {
        name: 'Entity',
        params: {
          tablename: 'usergroup'
        }
      }
    }
  }, [_c('i', {
    staticClass: "fa fa-users"
  }), _vm._v(" "), _c('span', {
    staticClass: "page"
  }, [_vm._v("User Group")])])], 1)])]), _vm._v(" "), _c('li', {
    staticClass: "treeview help-support"
  }, [_vm._m(2), _vm._v(" "), _c('ul', {
    staticClass: "treeview-menu"
  }, [_c('li', {
    staticClass: "pageLink",
    on: {
      "click": _vm.toggleMenu
    }
  }, [_c('router-link', {
    attrs: {
      "to": {
        name: 'Entity',
        params: {
          tablename: 'world'
        }
      }
    }
  }, [_c('i', {
    staticClass: "fa fa-th"
  }), _vm._v(" "), _c('span', {
    staticClass: "page"
  }, [_vm._v("All tables")])])], 1)])]), _vm._v(" "), _vm._m(3)])
},staticRenderFns: [function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('a', {
    attrs: {
      "href": "#"
    }
  }, [_c('i', {
    staticClass: "fas fa-book"
  }), _vm._v(" "), _c('span', [_vm._v("Items")]), _vm._v(" "), _c('span', {
    staticClass: "pull-right-container"
  }, [_c('i', {
    staticClass: "fa fa-angle-left fa-fw pull-right"
  })])])
},function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('a', {
    attrs: {
      "href": "#"
    }
  }, [_c('i', {
    staticClass: "fa fa-users"
  }), _vm._v(" "), _c('span', [_vm._v("People")]), _vm._v(" "), _c('span', {
    staticClass: "pull-right-container"
  }, [_c('i', {
    staticClass: "fa fa-angle-left fa-fw pull-right"
  })])])
},function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('a', {
    attrs: {
      "href": "#"
    }
  }, [_c('i', {
    staticClass: "fas fa-keyboard"
  }), _vm._v(" "), _c('span', [_vm._v("Administration")]), _vm._v(" "), _c('span', {
    staticClass: "pull-right-container"
  }, [_c('i', {
    staticClass: "fas fa-angle-left fa-fw pull-right"
  })])])
},function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('li', {
    staticClass: "treeview help-support"
  }, [_c('a', {
    attrs: {
      "href": "#"
    }
  }, [_c('i', {
    staticClass: "fa fa-comment"
  }), _vm._v(" "), _c('span', [_vm._v("Support")]), _vm._v(" "), _c('span', {
    staticClass: "pull-right-container"
  }, [_c('i', {
    staticClass: "fa fa-angle-left fa-fw pull-right"
  })])]), _vm._v(" "), _c('ul', {
    staticClass: "treeview-menu"
  }, [_c('li', [_c('a', {
    attrs: {
      "href": "https://github.com/artpar/daptin/wiki",
      "target": "_blank"
    }
  }, [_c('span', {
    staticClass: "fa fa-files-o"
  }), _vm._v("\n        Dev help")])]), _vm._v(" "), _c('li', [_c('a', {
    attrs: {
      "href": "https://github.com/artpar/daptin/issues/new",
      "target": "_blank"
    }
  }, [_c('span', {
    staticClass: "fa fa-cogs"
  }), _vm._v("\n        File an issue/bug")])]), _vm._v(" "), _c('li', [_c('a', {
    attrs: {
      "href": "mailto:artpar@gmail.com?subject=Daptin&body=Hi Parth,\\n"
    }
  }, [_c('span', {
    staticClass: "fa fa-envelope-o"
  }), _vm._v("\n        Email support")])])])])
}]}

/***/ }),
/* 650 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "container container-table"
  }, [_c('div', {
    staticClass: "row vertical-10p"
  }, [_c('div', {
    staticClass: "container"
  }, [_c('img', {
    staticClass: "center-block logo",
    attrs: {
      "src": "/static/img/logo.png"
    }
  }), _vm._v(" "), _c('div', {
    staticClass: "text-center col-sm-6 col-sm-offset-3"
  }, [_c('h1', [_vm._v("You are lost.")]), _vm._v(" "), _c('h4', [_vm._v("This page doesn't exist.")]), _vm._v(" "), _c('router-link', {
    attrs: {
      "to": "/"
    }
  }, [_vm._v("Take me home.")])], 1)])])])
},staticRenderFns: []}

/***/ }),
/* 651 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', [_c('el-row', [_c('el-tabs', {
    model: {
      value: (_vm.activeTabName),
      callback: function($$v) {
        _vm.activeTabName = $$v
      },
      expression: "activeTabName"
    }
  }, [_c('el-tab-pane', {
    attrs: {
      "label": "User",
      "name": "user"
    }
  }, [_c('div', [_c('el-row', [_c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedOwnerPermission.canPeek),
      callback: function($$v) {
        _vm.$set(_vm.parsedOwnerPermission, "canPeek", $$v)
      },
      expression: "parsedOwnerPermission.canPeek"
    }
  }, [_vm._v("Peek")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedOwnerPermission.canCRUD),
      callback: function($$v) {
        _vm.$set(_vm.parsedOwnerPermission, "canCRUD", $$v)
      },
      expression: "parsedOwnerPermission.canCRUD"
    }
  }, [_vm._v("CRUD")])], 1)], 1), _vm._v(" "), _c('el-row', [_c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedOwnerPermission.canRead),
      callback: function($$v) {
        _vm.$set(_vm.parsedOwnerPermission, "canRead", $$v)
      },
      expression: "parsedOwnerPermission.canRead"
    }
  }, [_vm._v("Read")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedOwnerPermission.canCreate),
      callback: function($$v) {
        _vm.$set(_vm.parsedOwnerPermission, "canCreate", $$v)
      },
      expression: "parsedOwnerPermission.canCreate"
    }
  }, [_vm._v("Create")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedOwnerPermission.canUpdate),
      callback: function($$v) {
        _vm.$set(_vm.parsedOwnerPermission, "canUpdate", $$v)
      },
      expression: "parsedOwnerPermission.canUpdate"
    }
  }, [_vm._v("Update")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedOwnerPermission.canDelete),
      callback: function($$v) {
        _vm.$set(_vm.parsedOwnerPermission, "canDelete", $$v)
      },
      expression: "parsedOwnerPermission.canDelete"
    }
  }, [_vm._v("Delete")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedOwnerPermission.canExecute),
      callback: function($$v) {
        _vm.$set(_vm.parsedOwnerPermission, "canExecute", $$v)
      },
      expression: "parsedOwnerPermission.canExecute"
    }
  }, [_vm._v("Execute")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedOwnerPermission.canRefer),
      callback: function($$v) {
        _vm.$set(_vm.parsedOwnerPermission, "canRefer", $$v)
      },
      expression: "parsedOwnerPermission.canRefer"
    }
  }, [_vm._v("Refer")])], 1)], 1), _vm._v(" "), _c('el-row', [_c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedOwnerPermission.canReadStrict),
      callback: function($$v) {
        _vm.$set(_vm.parsedOwnerPermission, "canReadStrict", $$v)
      },
      expression: "parsedOwnerPermission.canReadStrict"
    }
  }, [_vm._v("Read Strict")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedOwnerPermission.canCreateStrict),
      callback: function($$v) {
        _vm.$set(_vm.parsedOwnerPermission, "canCreateStrict", $$v)
      },
      expression: "parsedOwnerPermission.canCreateStrict"
    }
  }, [_vm._v("Create Strict")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedOwnerPermission.canUpdateStrict),
      callback: function($$v) {
        _vm.$set(_vm.parsedOwnerPermission, "canUpdateStrict", $$v)
      },
      expression: "parsedOwnerPermission.canUpdateStrict"
    }
  }, [_vm._v("Update Strict")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedOwnerPermission.canDeleteStrict),
      callback: function($$v) {
        _vm.$set(_vm.parsedOwnerPermission, "canDeleteStrict", $$v)
      },
      expression: "parsedOwnerPermission.canDeleteStrict"
    }
  }, [_vm._v("Delete Strict")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedOwnerPermission.canExecuteStrict),
      callback: function($$v) {
        _vm.$set(_vm.parsedOwnerPermission, "canExecuteStrict", $$v)
      },
      expression: "parsedOwnerPermission.canExecuteStrict"
    }
  }, [_vm._v("Execute Strict")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedOwnerPermission.canReferStrict),
      callback: function($$v) {
        _vm.$set(_vm.parsedOwnerPermission, "canReferStrict", $$v)
      },
      expression: "parsedOwnerPermission.canReferStrict"
    }
  }, [_vm._v("Refer Strict")])], 1)], 1)], 1)]), _vm._v(" "), _c('el-tab-pane', {
    attrs: {
      "label": "Group",
      "name": "group"
    }
  }, [_c('div', [_c('el-row', [_c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGroupPermission.canPeek),
      callback: function($$v) {
        _vm.$set(_vm.parsedGroupPermission, "canPeek", $$v)
      },
      expression: "parsedGroupPermission.canPeek"
    }
  }, [_vm._v("Peek")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGroupPermission.canCRUD),
      callback: function($$v) {
        _vm.$set(_vm.parsedGroupPermission, "canCRUD", $$v)
      },
      expression: "parsedGroupPermission.canCRUD"
    }
  }, [_vm._v("CRUD")])], 1)], 1), _vm._v(" "), _c('el-row', [_c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGroupPermission.canRead),
      callback: function($$v) {
        _vm.$set(_vm.parsedGroupPermission, "canRead", $$v)
      },
      expression: "parsedGroupPermission.canRead"
    }
  }, [_vm._v("Read")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGroupPermission.canCreate),
      callback: function($$v) {
        _vm.$set(_vm.parsedGroupPermission, "canCreate", $$v)
      },
      expression: "parsedGroupPermission.canCreate"
    }
  }, [_vm._v("Create")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGroupPermission.canUpdate),
      callback: function($$v) {
        _vm.$set(_vm.parsedGroupPermission, "canUpdate", $$v)
      },
      expression: "parsedGroupPermission.canUpdate"
    }
  }, [_vm._v("Update")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGroupPermission.canDelete),
      callback: function($$v) {
        _vm.$set(_vm.parsedGroupPermission, "canDelete", $$v)
      },
      expression: "parsedGroupPermission.canDelete"
    }
  }, [_vm._v("Delete")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGroupPermission.canExecute),
      callback: function($$v) {
        _vm.$set(_vm.parsedGroupPermission, "canExecute", $$v)
      },
      expression: "parsedGroupPermission.canExecute"
    }
  }, [_vm._v("Execute")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGroupPermission.canRefer),
      callback: function($$v) {
        _vm.$set(_vm.parsedGroupPermission, "canRefer", $$v)
      },
      expression: "parsedGroupPermission.canRefer"
    }
  }, [_vm._v("Refer")])], 1)], 1), _vm._v(" "), _c('el-row', [_c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGroupPermission.canReadStrict),
      callback: function($$v) {
        _vm.$set(_vm.parsedGroupPermission, "canReadStrict", $$v)
      },
      expression: "parsedGroupPermission.canReadStrict"
    }
  }, [_vm._v("Read Strict")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGroupPermission.canCreateStrict),
      callback: function($$v) {
        _vm.$set(_vm.parsedGroupPermission, "canCreateStrict", $$v)
      },
      expression: "parsedGroupPermission.canCreateStrict"
    }
  }, [_vm._v("Create Strict")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGroupPermission.canUpdateStrict),
      callback: function($$v) {
        _vm.$set(_vm.parsedGroupPermission, "canUpdateStrict", $$v)
      },
      expression: "parsedGroupPermission.canUpdateStrict"
    }
  }, [_vm._v("Update Strict")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGroupPermission.canDeleteStrict),
      callback: function($$v) {
        _vm.$set(_vm.parsedGroupPermission, "canDeleteStrict", $$v)
      },
      expression: "parsedGroupPermission.canDeleteStrict"
    }
  }, [_vm._v("Delete Strict")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGroupPermission.canExecuteStrict),
      callback: function($$v) {
        _vm.$set(_vm.parsedGroupPermission, "canExecuteStrict", $$v)
      },
      expression: "parsedGroupPermission.canExecuteStrict"
    }
  }, [_vm._v("Execute Strict")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGroupPermission.canReferStrict),
      callback: function($$v) {
        _vm.$set(_vm.parsedGroupPermission, "canReferStrict", $$v)
      },
      expression: "parsedGroupPermission.canReferStrict"
    }
  }, [_vm._v("Refer Strict")])], 1)], 1)], 1)]), _vm._v(" "), _c('el-tab-pane', {
    attrs: {
      "label": "Guest",
      "name": "guest"
    }
  }, [_c('div', {}, [_c('el-row', [_c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGuestPermission.canPeek),
      callback: function($$v) {
        _vm.$set(_vm.parsedGuestPermission, "canPeek", $$v)
      },
      expression: "parsedGuestPermission.canPeek"
    }
  }, [_vm._v("Peek")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGuestPermission.canCRUD),
      callback: function($$v) {
        _vm.$set(_vm.parsedGuestPermission, "canCRUD", $$v)
      },
      expression: "parsedGuestPermission.canCRUD"
    }
  }, [_vm._v("CRUD")])], 1)], 1), _vm._v(" "), _c('el-row', [_c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGuestPermission.canRead),
      callback: function($$v) {
        _vm.$set(_vm.parsedGuestPermission, "canRead", $$v)
      },
      expression: "parsedGuestPermission.canRead"
    }
  }, [_vm._v("Read")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGuestPermission.canCreate),
      callback: function($$v) {
        _vm.$set(_vm.parsedGuestPermission, "canCreate", $$v)
      },
      expression: "parsedGuestPermission.canCreate"
    }
  }, [_vm._v("Create")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGuestPermission.canUpdate),
      callback: function($$v) {
        _vm.$set(_vm.parsedGuestPermission, "canUpdate", $$v)
      },
      expression: "parsedGuestPermission.canUpdate"
    }
  }, [_vm._v("Update")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGuestPermission.canDelete),
      callback: function($$v) {
        _vm.$set(_vm.parsedGuestPermission, "canDelete", $$v)
      },
      expression: "parsedGuestPermission.canDelete"
    }
  }, [_vm._v("Delete")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGuestPermission.canExecute),
      callback: function($$v) {
        _vm.$set(_vm.parsedGuestPermission, "canExecute", $$v)
      },
      expression: "parsedGuestPermission.canExecute"
    }
  }, [_vm._v("Execute")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGuestPermission.canRefer),
      callback: function($$v) {
        _vm.$set(_vm.parsedGuestPermission, "canRefer", $$v)
      },
      expression: "parsedGuestPermission.canRefer"
    }
  }, [_vm._v("Refer")])], 1)], 1), _vm._v(" "), _c('el-row', [_c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGuestPermission.canReadStrict),
      callback: function($$v) {
        _vm.$set(_vm.parsedGuestPermission, "canReadStrict", $$v)
      },
      expression: "parsedGuestPermission.canReadStrict"
    }
  }, [_vm._v("Read Strict")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGuestPermission.canCreateStrict),
      callback: function($$v) {
        _vm.$set(_vm.parsedGuestPermission, "canCreateStrict", $$v)
      },
      expression: "parsedGuestPermission.canCreateStrict"
    }
  }, [_vm._v("Create Strict")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGuestPermission.canUpdateStrict),
      callback: function($$v) {
        _vm.$set(_vm.parsedGuestPermission, "canUpdateStrict", $$v)
      },
      expression: "parsedGuestPermission.canUpdateStrict"
    }
  }, [_vm._v("Update Strict")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGuestPermission.canDeleteStrict),
      callback: function($$v) {
        _vm.$set(_vm.parsedGuestPermission, "canDeleteStrict", $$v)
      },
      expression: "parsedGuestPermission.canDeleteStrict"
    }
  }, [_vm._v("Delete Strict")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGuestPermission.canExecuteStrict),
      callback: function($$v) {
        _vm.$set(_vm.parsedGuestPermission, "canExecuteStrict", $$v)
      },
      expression: "parsedGuestPermission.canExecuteStrict"
    }
  }, [_vm._v("Execute Strict")])], 1), _vm._v(" "), _c('el-col', {
    attrs: {
      "span": 8
    }
  }, [_c('el-checkbox', {
    model: {
      value: (_vm.parsedGuestPermission.canReferStrict),
      callback: function($$v) {
        _vm.$set(_vm.parsedGuestPermission, "canReferStrict", $$v)
      },
      expression: "parsedGuestPermission.canReferStrict"
    }
  }, [_vm._v("Refer Strict")])], 1)], 1)], 1)])], 1)], 1), _vm._v(" "), _c('el-row', [_c('el-button', {
    on: {
      "click": _vm.clearAll
    }
  }, [_vm._v("Clear all")]), _vm._v(" "), _c('el-button', {
    on: {
      "click": _vm.enableAll
    }
  }, [_vm._v("Enable all")]), _vm._v(" "), _c('el-button', {
    on: {
      "click": _vm.toggleSelectionAll
    }
  }, [_vm._v("Toggle")])], 1)], 1)
},staticRenderFns: []}

/***/ }),
/* 652 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "content-wrapper"
  }, [_vm._m(0), _vm._v(" "), _c('section', {
    staticClass: "content"
  }, [_c('div', {
    staticClass: "box"
  }, [_c('div', {
    staticClass: "box-header"
  }, [(!_vm.data.TableName) ? _c('h1', [_vm._v("New Table")]) : _c('h1', [_vm._v(_vm._s(_vm._f("titleCase")(_vm.data.TableName)))])]), _vm._v(" "), _c('div', {
    staticClass: "box-body"
  }, [_c('form', {
    attrs: {
      "onsubmit": "return false",
      "role": "form"
    }
  }, [_c('div', {
    staticClass: "row"
  }, [_c('div', {
    staticClass: "col-md-6"
  }, [_c('div', {
    staticClass: "form-group"
  }, [_c('h3', [_vm._v("Name")]), _vm._v(" "), _c('input', {
    directives: [{
      name: "model",
      rawName: "v-model",
      value: (_vm.data.TableName),
      expression: "data.TableName"
    }],
    staticClass: "form-control",
    attrs: {
      "type": "text",
      "name": "name",
      "placeholder": "sale_record"
    },
    domProps: {
      "value": (_vm.data.TableName)
    },
    on: {
      "input": function($event) {
        if ($event.target.composing) { return; }
        _vm.$set(_vm.data, "TableName", $event.target.value)
      }
    }
  })])])]), _vm._v(" "), _c('div', {
    staticClass: "row"
  }, [_c('div', {
    staticClass: "col-md-6"
  }, [_c('div', {
    staticClass: "box"
  }, [_c('div', {
    staticClass: "box-header"
  }, [_c('h3', {
    staticClass: "box-title"
  }, [_vm._v("Columns")]), _vm._v(" "), _c('div', {
    staticClass: "box-tools pull-right"
  }, [_c('button', {
    staticClass: "btn btn-primary",
    on: {
      "click": function($event) {
        _vm.data.Columns.push({})
      }
    }
  }, [_c('i', {
    staticClass: "fa fa-plus"
  })])])]), _vm._v(" "), _c('div', {
    staticClass: "box-body"
  }, _vm._l((_vm.data.Columns), function(col) {
    return _c('div', {
      staticClass: "form-group"
    }, [_c('div', {
      staticClass: "row"
    }, [_c('div', {
      staticClass: "col-md-6"
    }, [_c('input', {
      directives: [{
        name: "model",
        rawName: "v-model",
        value: (col.Name),
        expression: "col.Name"
      }],
      staticClass: "form-control",
      attrs: {
        "type": "text",
        "placeholder": "name",
        "disabled": col.ReadOnly
      },
      domProps: {
        "value": (col.Name)
      },
      on: {
        "input": function($event) {
          if ($event.target.composing) { return; }
          _vm.$set(col, "Name", $event.target.value)
        }
      }
    })]), _vm._v(" "), _c('div', {
      staticClass: "col-md-5"
    }, [_c('select', {
      directives: [{
        name: "model",
        rawName: "v-model",
        value: (col.ColumnType),
        expression: "col.ColumnType"
      }],
      staticClass: "form-control",
      attrs: {
        "disabled": col.ReadOnly
      },
      on: {
        "change": function($event) {
          var $$selectedVal = Array.prototype.filter.call($event.target.options, function(o) {
            return o.selected
          }).map(function(o) {
            var val = "_value" in o ? o._value : o.value;
            return val
          });
          _vm.$set(col, "ColumnType", $event.target.multiple ? $$selectedVal : $$selectedVal[0])
        }
      }
    }, _vm._l((_vm.columnTypes), function(colData, colTypeName) {
      return _c('option', {
        domProps: {
          "value": colData.Name
        }
      }, [_vm._v("\n                            " + _vm._s(_vm._f("titleCase")(colTypeName)) + "\n                          ")])
    }))]), _vm._v(" "), (!col.ReadOnly) ? _c('div', {
      staticClass: "col-md-1",
      staticStyle: {
        "padding-left": "0px"
      }
    }, [_c('button', {
      staticClass: "btn btn-danger btn-sm",
      on: {
        "click": function($event) {
          _vm.removeColumn(col)
        }
      }
    }, [_c('i', {
      staticClass: "fa fa-minus"
    })])]) : _vm._e()])])
  }))])]), _vm._v(" "), _c('div', {
    staticClass: "col-md-6"
  }, [_c('div', {
    staticClass: "box"
  }, [_c('div', {
    staticClass: "box-header"
  }, [_c('h2', {
    staticClass: "box-title"
  }, [_vm._v("Relations")]), _vm._v(" "), _c('div', {
    staticClass: "box-tools pull-right"
  }, [_c('button', {
    staticClass: "btn btn-primary",
    on: {
      "click": function($event) {
        _vm.data.Relations.push({})
      }
    }
  }, [_c('i', {
    staticClass: "fa fa-plus"
  })])])]), _vm._v(" "), _c('div', {
    staticClass: "box-body"
  }, _vm._l((_vm.data.Relations), function(relation) {
    return _c('div', {
      staticClass: "form-group"
    }, [_c('div', {
      staticClass: "row"
    }, [_c('div', {
      staticClass: "col-md-6"
    }, [_c('select', {
      directives: [{
        name: "model",
        rawName: "v-model",
        value: (relation.Relation),
        expression: "relation.Relation"
      }],
      staticClass: "form-control",
      attrs: {
        "disabled": relation.ReadOnly
      },
      on: {
        "change": function($event) {
          var $$selectedVal = Array.prototype.filter.call($event.target.options, function(o) {
            return o.selected
          }).map(function(o) {
            var val = "_value" in o ? o._value : o.value;
            return val
          });
          _vm.$set(relation, "Relation", $event.target.multiple ? $$selectedVal : $$selectedVal[0])
        }
      }
    }, [_c('option', {
      attrs: {
        "value": "has_one"
      }
    }, [_vm._v("Has one")]), _vm._v(" "), _c('option', {
      attrs: {
        "value": "belongs_to"
      }
    }, [_vm._v("Belongs to")]), _vm._v(" "), _c('option', {
      attrs: {
        "value": "has_many"
      }
    }, [_vm._v("Has many")]), _vm._v(" "), _c('option', {
      attrs: {
        "value": "has_many_and_belongs_to_many"
      }
    }, [_vm._v("Has many and belongs to many")])])]), _vm._v(" "), _c('div', {
      staticClass: "col-md-5"
    }, [_c('select', {
      directives: [{
        name: "model",
        rawName: "v-model",
        value: (relation.Object),
        expression: "relation.Object"
      }],
      staticClass: "form-control",
      attrs: {
        "disabled": relation.ReadOnly
      },
      on: {
        "change": function($event) {
          var $$selectedVal = Array.prototype.filter.call($event.target.options, function(o) {
            return o.selected
          }).map(function(o) {
            var val = "_value" in o ? o._value : o.value;
            return val
          });
          _vm.$set(relation, "Object", $event.target.multiple ? $$selectedVal : $$selectedVal[0])
        }
      }
    }, _vm._l((_vm.relatableWorlds), function(world) {
      return _c('option', {
        domProps: {
          "value": world.table_name
        }
      }, [_vm._v("\n                            " + _vm._s(_vm._f("titleCase")(world.table_name)) + "\n                          ")])
    }))]), _vm._v(" "), (!relation.ReadOnly) ? _c('div', {
      staticClass: "col-md-1",
      staticStyle: {
        "padding-left": "0px"
      }
    }, [_c('button', {
      staticClass: "btn btn-danger btn-sm",
      on: {
        "click": function($event) {
          _vm.removeRelation(relation)
        }
      }
    }, [_c('i', {
      staticClass: "fa fa-minus"
    })])]) : _vm._e()])])
  }))])])])])]), _vm._v(" "), _c('div', {
    staticClass: "box-footer"
  }, [_c('div', {
    staticClass: "form-group"
  }, [_c('button', {
    staticClass: "btn btn-primary btn-lg",
    on: {
      "click": _vm.createEntity
    }
  }, [_vm._v("Create")])])])])])])
},staticRenderFns: [function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('section', {
    staticClass: "content-header"
  }, [_c('ol', {
    staticClass: "breadcrumb"
  }, [_c('li', [_c('a', {
    attrs: {
      "href": "javascript:"
    }
  }, [_c('i', {
    staticClass: "fa fa-home"
  }), _vm._v("New item")])])])])
}]}

/***/ }),
/* 653 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "container-fluid flat-white"
  }, [_c('div', {
    ref: "tabl",
    attrs: {
      "id": _vm.tableId
    }
  })])
},staticRenderFns: []}

/***/ }),
/* 654 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "box"
  }, [_c('div', {
    staticClass: "box-title"
  }, [_c('div', {
    staticClass: "box-header"
  }, [_c('span', {
    staticClass: "font-size: 20px; font-weight: 400"
  }, [_vm._v(_vm._s(_vm._f("titleCase")(_vm.schema.name)))])])]), _vm._v(" "), _c('div', {
    staticClass: "box-body"
  }, [_c('div', {
    staticClass: "ui section"
  }, [_c('el-select', {
    attrs: {
      "filterable": "",
      "remote": "",
      "multiple": _vm.schema.multiple,
      "placeholder": 'Search and add ' + _vm.schema.inputType,
      "remote-method": _vm.remoteMethod,
      "loading": _vm.loading
    },
    model: {
      value: (_vm.selectedItem),
      callback: function($$v) {
        _vm.selectedItem = $$v
      },
      expression: "selectedItem"
    }
  }, _vm._l((_vm.options), function(item) {
    return _c('el-option', {
      key: item.value,
      attrs: {
        "label": item.label,
        "value": item
      }
    })
  }))], 1), _vm._v(" "), (_vm.selectedItem) ? _c('div', {
    staticClass: "ui section"
  }, [_c('p', [_vm._v(" Selected: " + _vm._s(_vm._f("titleCase")(_vm._f("chooseTitle")(_vm.selectedItem))))])]) : _vm._e()]), _vm._v(" "), _c('div', {
    staticClass: "box-footer"
  }, [(_vm.selectedItem != null) ? _c('button', {
    staticClass: "btn btn-primary",
    on: {
      "click": function($event) {
        $event.preventDefault();
        _vm.addObject($event)
      }
    }
  }, [_vm._v(" Add " + _vm._s(_vm._f("titleCase")(_vm.schema.name)) + "\n    ")]) : _vm._e()])])
},staticRenderFns: []}

/***/ }),
/* 655 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('el-upload', {
    attrs: {
      "action": "https://jsonplaceholder.typicode.com/posts/",
      "on-preview": _vm.handlePreview,
      "on-remove": _vm.handleRemove,
      "auto-upload": false,
      "on-change": _vm.processFile,
      "before-upload": _vm.handlePreview,
      "file-list": _vm.fileList
    }
  }, [_c('el-button', {
    attrs: {
      "size": "small",
      "type": "primary"
    }
  }, [_vm._v("Add file")]), _vm._v(" "), _c('div', {
    staticClass: "el-upload__tip",
    attrs: {
      "slot": "tip"
    },
    slot: "tip"
  }, [_vm._v("File type: " + _vm._s(_vm.schema.inputType.split("|").join(" or ")))])], 1)
},staticRenderFns: []}

/***/ }),
/* 656 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "content-wrapper"
  }, [_c('section', {
    staticClass: "content-header"
  }, [_c('h1', [_vm._v("\n      " + _vm._s(_vm._f("titleCase")(_vm.selectedSubTable)) + "\n      "), _c('small', [_vm._v(_vm._s(_vm.$route.meta.description))])]), _vm._v(" "), _c('ol', {
    staticClass: "breadcrumb"
  }, [_vm._m(0), _vm._v(" "), _c('li', [_c('router-link', {
    attrs: {
      "to": {
        name: 'Entity',
        params: {
          tablename: _vm.selectedTable
        }
      }
    }
  }, [_vm._v("\n          " + _vm._s(_vm._f("titleCase")(_vm.selectedTable)) + "\n        ")])], 1), _vm._v(" "), _c('li', {
    staticClass: "active"
  }, [_c('router-link', {
    attrs: {
      "to": {
        name: 'Instance',
        params: {
          tablename: _vm.selectedTable,
          refId: _vm.$route.params.refId
        }
      }
    }
  }, [_vm._v("\n          " + _vm._s(_vm._f("titleCase")(_vm._f("chooseTitle")(_vm.selectedRow))) + "\n        ")])], 1)]), _vm._v(" "), _c('div', {
    staticClass: "box-tools pull-right"
  }, [_c('div', {
    staticClass: "ui icon buttons"
  }, [_c('button', {
    staticClass: "btn btn-box-tool",
    on: {
      "click": function($event) {
        $event.preventDefault();
        _vm.viewMode = 'table';
        _vm.currentViewType = 'table-view';
      }
    }
  }, [_c('i', {
    staticClass: "fa  fa-2x fa-table grey "
  })]), _vm._v(" "), _c('button', {
    staticClass: "btn btn-box-tool",
    on: {
      "click": function($event) {
        $event.preventDefault();
        _vm.viewMode = 'items';
        _vm.currentViewType = 'table-view';
      }
    }
  }, [_c('i', {
    staticClass: "fa  fa-2x fa-th-large grey"
  })]), _vm._v(" "), _c('button', {
    staticClass: "btn btn-box-tool",
    on: {
      "click": function($event) {
        $event.preventDefault();
        _vm.currentViewType = 'recline-view'
      }
    }
  }, [_c('i', {
    staticClass: "fa  fa-2x fa-area-chart grey"
  })]), _vm._v(" "), _c('button', {
    staticClass: "btn btn-box-tool",
    on: {
      "click": function($event) {
        $event.preventDefault();
        _vm.newRow()
      }
    }
  }, [_c('i', {
    staticClass: "fa fa-2x fa-plus green "
  })]), _vm._v(" "), _c('button', {
    staticClass: "btn btn-box-tool",
    on: {
      "click": function($event) {
        $event.preventDefault();
        _vm.reloadData()
      }
    }
  }, [_c('i', {
    staticClass: "fa fa-2x fa-refresh grey"
  })]), _vm._v(" "), _c('router-link', {
    staticClass: "btn btn-box-tool",
    attrs: {
      "to": {
        name: 'Action',
        params: {
          actionname: 'add_exchange',
          tablename: 'world'
        },
        query: {
          world_id: _vm.worldReferenceId
        }
      }
    }
  }, [_c('i', {
    staticClass: "fa fa-2x fa-link grey"
  })]), _vm._v(" "), _c('router-link', {
    staticClass: "btn btn-box-tool",
    attrs: {
      "to": {
        name: 'Action',
        params: {
          actionname: 'export_data',
          tablename: 'world'
        },
        query: {
          world_id: _vm.worldReferenceId
        }
      }
    }
  }, [_c('i', {
    staticClass: "fa fa-2x fa-cloud-download grey"
  })])], 1)])]), _vm._v(" "), _c('section', {
    staticClass: "content"
  }, [(_vm.showAddEdit && _vm.rowBeingEdited != null) ? _c('div', {
    staticClass: "col-md-12"
  }, [(_vm.showAddEdit) ? _c('model-form', {
    ref: "modelform",
    attrs: {
      "json-api": _vm.jsonApi,
      "model": _vm.rowBeingEdited,
      "meta": _vm.subTableColumns
    },
    on: {
      "save": function($event) {
        _vm.saveRow(_vm.rowBeingEdited)
      },
      "cancel": function($event) {
        _vm.showAddEdit = false
      }
    }
  }) : _vm._e()], 1) : _vm._e(), _vm._v(" "), _c('div', {
    staticClass: "col-md-12"
  }, [(_vm.currentViewType == 'table-view') ? [(_vm.selectedSubTable) ? _c('table-view', {
    ref: "tableview2",
    attrs: {
      "finder": _vm.finder,
      "json-api": _vm.jsonApi,
      "json-api-model-name": _vm.selectedSubTable
    },
    on: {
      "newRow": function($event) {
        _vm.newRow()
      },
      "editRow": _vm.editRow
    }
  }) : _vm._e()] : (_vm.currentViewType == 'recline-view') ? [(_vm.selectedSubTable && !_vm.showAddEdit) ? _c('recline-view', {
    ref: "tableview1",
    attrs: {
      "finder": _vm.finder,
      "json-api": _vm.jsonApi,
      "json-api-model-name": _vm.selectedSubTable
    },
    on: {
      "newRow": function($event) {
        _vm.newRow()
      },
      "editRow": _vm.editRow
    }
  }) : _vm._e()] : _vm._e()], 2)])])
},staticRenderFns: [function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('li', [_c('a', {
    attrs: {
      "href": "javascript:;"
    }
  }, [_c('i', {
    staticClass: "fa fa-home"
  }), _vm._v("Home")])])
}]}

/***/ }),
/* 657 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('aside', {
    staticClass: "main-sidebar"
  }, [_c('section', {
    staticClass: "sidebar"
  }, [(_vm.user) ? _c('div', {
    staticClass: "user-panel"
  }, [_c('div', {
    staticClass: "pull-left image"
  }, [_c('img', {
    attrs: {
      "src": _vm.user.picture
    }
  })]), _vm._v(" "), _c('div', {
    staticClass: "pull-left info"
  }, [_c('div', [_c('p', {
    staticClass: "black"
  }, [_vm._v(_vm._s(_vm.user.name))])]), _vm._v(" "), _vm._m(0)])]) : _vm._e(), _vm._v(" "), (!_vm.user) ? _c('div', {
    staticClass: "user-panel"
  }, [_vm._m(1)]) : _vm._e(), _vm._v(" "), _c('sidebar-menu', {
    attrs: {
      "filter": _vm.filter
    }
  })], 1)])
},staticRenderFns: [function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('a', {
    attrs: {
      "href": "javascript:;"
    }
  }, [_c('i', {
    staticClass: "fas fa-circle text-success"
  }), _vm._v(" Online\n        ")])
},function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "pull-left"
  }, [_c('a', {
    attrs: {
      "href": "/auth/signin"
    }
  }, [_c('i', {
    staticClass: "fas fa-circle text-success"
  }), _vm._v(" Login\n        ")])])
}]}

/***/ }),
/* 658 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _vm._m(0)
},staticRenderFns: [function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "col-md-12",
    staticStyle: {
      "height": "500px"
    }
  }, [_c('div', {
    staticClass: "data-explorer-here"
  }, [_vm._v("\n    data explorer\n  ")]), _vm._v(" "), _c('div', {
    staticStyle: {
      "clear": "both"
    }
  })])
}]}

/***/ }),
/* 659 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    attrs: {
      "id": "lock"
    }
  })
},staticRenderFns: []}

/***/ }),
/* 660 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('p', [_vm._v("Signing off...")])
},staticRenderFns: []}

/***/ }),
/* 661 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "content-wrapper"
  }, [_c('section', {
    staticClass: "content-header"
  }, [_c('ol', {
    staticClass: "breadcrumb"
  }, [_vm._m(0), _vm._v(" "), _vm._l((_vm.$route.meta.breadcrumb), function(crumb) {
    return _c('li', {
      key: crumb.label
    }, [(crumb.to) ? void 0 : [_vm._v("\n          " + _vm._s(crumb.label) + "\n        ")]], 2)
  })], 2)]), _vm._v(" "), _c('section', {
    staticClass: "content"
  }, [_c('el-tabs', {
    attrs: {
      "type": "card"
    }
  }, _vm._l((_vm.worlds), function(world) {
    return _c('el-tab-pane', {
      key: world.TableName,
      attrs: {
        "label": _vm._f("titleCase")(world.TableName)
      }
    }, [_c('daptable', {
      attrs: {
        "json-api": _vm.jsonApi,
        "data-path": "data",
        "json-api-model-name": world.TableName
      }
    })], 1)
  }))], 1)])
},staticRenderFns: [function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('li', [_c('a', {
    attrs: {
      "href": "javascript:"
    }
  }, [_c('i', {
    staticClass: "fa fa-home"
  }), _vm._v("Home ")])])
}]}

/***/ }),
/* 662 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "box"
  }, [(!_vm.hideTitle) ? _c('div', {
    staticClass: "box-header"
  }, [_c('div', {
    staticClass: "box-title"
  }, [_vm._v("\n      " + _vm._s(_vm.title) + "\n    ")])]) : _vm._e(), _vm._v(" "), _c('div', {
    staticClass: "box-body"
  }, [_c('div', {
    class: {
      'col-md-12': _vm.relations.length == 0 && !_vm.hasPermissionField, 'col-md-6': _vm.relations.length > 0 || _vm.hasPermissionField
    }
  }, [_c('vue-form-generator', {
    attrs: {
      "schema": _vm.formModel,
      "model": _vm.model
    }
  })], 1), _vm._v(" "), (_vm.relations.length > 0) ? _c('div', {
    staticClass: "col-md-3"
  }, [_c('div', {
    staticClass: "row"
  }, _vm._l((_vm.relations), function(item) {
    return _c('div', {
      key: item.value,
      staticClass: "col-md-12"
    }, [_c('select-one-or-more', {
      attrs: {
        "value": item.value,
        "schema": item
      },
      on: {
        "save": _vm.setRelation
      }
    })], 1)
  })), _vm._v(" "), (_vm.hasPermissionField) ? _c('div', {
    staticClass: "col-md-6"
  }, [_c('fieldPermissionInput', {
    attrs: {
      "value": _vm.model.permission
    }
  })], 1) : _vm._e()]) : _vm._e()]), _vm._v(" "), _c('div', {
    staticClass: "box-footer"
  }, [_c('el-button', {
    directives: [{
      name: "loading",
      rawName: "v-loading.body",
      value: (_vm.loading),
      expression: "loading",
      modifiers: {
        "body": true
      }
    }],
    staticClass: "bg-yellow",
    attrs: {
      "type": "submit"
    },
    on: {
      "click": function($event) {
        $event.preventDefault();
        _vm.saveRow()
      }
    }
  }, [_vm._v(" Submit\n    ")]), _vm._v(" "), (!_vm.hideCancel) ? _c('el-button', {
    staticClass: "bg-red",
    on: {
      "click": function($event) {
        _vm.cancel()
      }
    }
  }, [_vm._v("Cancel")]) : _vm._e()], 1)])
},staticRenderFns: []}

/***/ }),
/* 663 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "row"
  }, [(_vm.viewMode != 'table') ? _c('div', {
    staticClass: "col-md-12"
  }, [_c('vuetable-pagination', {
    ref: "pagination",
    attrs: {
      "css": _vm.css.pagination
    },
    on: {
      "change-page": _vm.onChangePage
    }
  })], 1) : _vm._e(), _vm._v(" "), _c('div', {
    staticClass: "col-md-12",
    staticStyle: {
      "position": "relative",
      "height": "700px",
      "overflow-y": "scroll"
    }
  }, [(_vm.viewMode == 'table') ? [_c('div', {
    ref: "tableViewDiv",
    attrs: {
      "id": "tableView"
    }
  })] : _vm._e(), _vm._v(" "), (_vm.viewMode == 'card') ? _c('vuecard', {
    ref: "vuetable",
    attrs: {
      "json-api": _vm.jsonApi,
      "finder": _vm.finder,
      "track-by": "id",
      "detail-row-component": "detailed-table-row",
      "pagination-path": "links",
      "data-path": "data",
      "css": _vm.css.table,
      "json-api-model-name": _vm.jsonApiModelName,
      "api-mode": true,
      "query-params": {
        sort: 'sort',
        page: 'page[number]',
        perPage: 'page[size]'
      },
      "load-on-start": _vm.autoload
    },
    on: {
      "vuetable:cell-clicked": _vm.onCellClicked,
      "pagination-data": _vm.onPaginationData
    },
    scopedSlots: _vm._u([{
      key: "actions",
      fn: function(props) {
        return [_c('div', {
          staticClass: "custom-actions"
        }, [_c('button', {
          staticClass: "btn btn-box-tool",
          on: {
            "click": function($event) {
              _vm.onAction('go-item', props.rowData, props.rowIndex)
            }
          }
        }, [_c('i', {
          staticClass: "fa fa-2x fa-expand-arrows-alt"
        })]), _vm._v(" "), _c('button', {
          staticClass: "btn btn-box-tool",
          on: {
            "click": function($event) {
              _vm.onAction('edit-item', props.rowData, props.rowIndex)
            }
          }
        }, [_c('i', {
          staticClass: "fas fa-pencil-alt  fa-2x"
        })]), _vm._v(" "), _c('el-popover', {
          attrs: {
            "placement": "top",
            "trigger": "click",
            "width": "160"
          }
        }, [_c('p', [_vm._v("Are you sure to delete this?")]), _vm._v(" "), _c('div', {
          staticStyle: {
            "text-align": "right",
            "margin": "0"
          }
        }, [_c('el-button', {
          attrs: {
            "type": "primary",
            "size": "mini"
          },
          on: {
            "click": function($event) {
              _vm.onAction('delete-item', props.rowData, props.rowIndex)
            }
          }
        }, [_vm._v("\n                confirm\n              ")])], 1), _vm._v(" "), _c('button', {
          staticClass: "btn btn-box-tool",
          attrs: {
            "slot": "reference"
          },
          slot: "reference"
        }, [_c('i', {
          staticClass: "fa fa-2x fa-times red"
        })])])], 1)]
      }
    }])
  }) : _vm._e()], 2)])
},staticRenderFns: []}

/***/ }),
/* 664 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    class: ['wrapper', _vm.classes]
  }, [_c('header', {
    staticClass: "main-header"
  }, [_vm._m(0), _vm._v(" "), _c('nav', {
    staticClass: "navbar navbar-static-top",
    attrs: {
      "role": "navigation"
    }
  }, [_vm._m(1), _vm._v(" "), _vm._m(2), _vm._v(" "), _c('div', {
    staticClass: "navbar-collapse collapse",
    staticStyle: {
      "height": "1px"
    },
    attrs: {
      "id": "navbar-collapse-1"
    }
  }, [_c('div', {
    staticClass: "col-sm-3 col-md-3"
  }, [_c('form', {
    staticClass: "navbar-form",
    attrs: {
      "role": "search"
    },
    on: {
      "submit": function($event) {
        $event.preventDefault();
        _vm.setQueryString($event)
      }
    }
  }, [_c('div', {
    staticClass: "input-group"
  }, [_c('input', {
    staticClass: "form-control",
    attrs: {
      "id": "navbar-search-input",
      "type": "text",
      "placeholder": "Search",
      "name": "q"
    }
  }), _vm._v(" "), _c('div', {
    staticClass: "input-group-btn"
  }, [_vm._m(3), _vm._v(" "), _c('button', {
    staticClass: "btn btn-default",
    attrs: {
      "type": "clear"
    },
    on: {
      "click": function($event) {
        $event.preventDefault();
        _vm.clearSearch($event)
      }
    }
  }, [_c('i', {
    staticClass: "fa fa-times"
  })])])])])]), _vm._v(" "), _c('div', {
    staticClass: "navbar-custom-menu"
  }, [_c('ul', {
    staticClass: "nav navbar-nav"
  }, [_c('li', {
    staticClass: "dropdown user user-menu"
  }, [_c('a', {
    staticClass: "dropdown-toggle",
    attrs: {
      "href": "#",
      "data-toggle": "dropdown",
      "aria-expanded": "false"
    }
  }, [_c('img', {
    staticClass: "user-image",
    attrs: {
      "src": _vm.user.picture,
      "alt": "User Image"
    }
  }), _vm._v(" "), _c('span', {
    staticClass: "hidden-xs"
  }, [_vm._v(_vm._s(_vm.user.name))])]), _vm._v(" "), _c('ul', {
    staticClass: "dropdown-menu"
  }, [_c('li', {
    staticClass: "user-header"
  }, [_c('img', {
    staticClass: "img-circle",
    attrs: {
      "src": _vm.user.picture,
      "alt": "User Image"
    }
  }), _vm._v(" "), _c('p', [_vm._v("\n                      " + _vm._s(_vm.user.name) + "\n                      "), _c('small')])]), _vm._v(" "), _c('li', {
    staticClass: "user-footer"
  }, [_c('div', {
    staticClass: "pull-right"
  }, [_c('router-link', {
    staticClass: "btn btn-default btn-flat",
    attrs: {
      "to": {
        name: 'SignOut'
      }
    }
  }, [_vm._v("Sign out")])], 1)])])])])])])])]), _vm._v(" "), _c('sidebar', {
    attrs: {
      "user": _vm.user
    }
  }), _vm._v(" "), _c('router-view')], 1)
},staticRenderFns: [function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('span', {
    staticClass: "logo-mini"
  }, [_c('a', {
    attrs: {
      "href": "/"
    }
  }, [_c('img', {
    staticClass: "img-responsive center-block logo",
    attrs: {
      "src": "/static/img/copilot-logo-white.svg",
      "alt": "Logo"
    }
  })])])
},function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('a', {
    staticClass: "sidebar-toggle fa",
    attrs: {
      "href": "javascript:",
      "data-toggle": "offcanvas",
      "role": "button"
    }
  }, [_c('span', {
    staticClass: "sr-only"
  }, [_vm._v(" Toggle navigation")])])
},function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('button', {
    staticClass: "navbar-toggle collapsed",
    attrs: {
      "type": "button",
      "data-toggle": "collapse",
      "data-target": "#navbar-collapse-1"
    }
  }, [_c('span', {
    staticClass: "sr-only"
  }, [_vm._v("Toggle navigation")]), _vm._v(" "), _c('span', {
    staticClass: "icon-bar"
  }), _vm._v(" "), _c('span', {
    staticClass: "icon-bar"
  }), _vm._v(" "), _c('span', {
    staticClass: "icon-bar"
  })])
},function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('button', {
    staticClass: "btn btn-default",
    attrs: {
      "type": "submit"
    }
  }, [_c('i', {
    staticClass: "fa fa-search"
  })])
}]}

/***/ }),
/* 665 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "custom-actions"
  }, [_c('button', {
    staticClass: "ui basic button",
    on: {
      "click": function($event) {
        _vm.itemAction('view-item', _vm.rowData, _vm.rowIndex)
      }
    }
  }, [_c('i', {
    staticClass: "zoom icon"
  })]), _vm._v(" "), _c('button', {
    staticClass: "ui basic button",
    on: {
      "click": function($event) {
        _vm.itemAction('edit-item', _vm.rowData, _vm.rowIndex)
      }
    }
  }, [_c('i', {
    staticClass: "edit icon"
  })]), _vm._v(" "), _c('button', {
    staticClass: "ui basic button",
    on: {
      "click": function($event) {
        _vm.itemAction('delete-item', _vm.rowData, _vm.rowIndex)
      }
    }
  }, [_c('i', {
    staticClass: "delete icon"
  })])])
},staticRenderFns: []}

/***/ }),
/* 666 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('p', [_vm._v("Signing in...")])
},staticRenderFns: []}

/***/ }),
/* 667 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return (_vm.action) ? _c('div', {
    staticClass: "box"
  }, [(!_vm.hideTitle) ? _c('div', {
    staticClass: "box-header"
  }, [_c('div', {
    staticClass: "box-title"
  }, [_c('h1', [_vm._v(" " + _vm._s(_vm.action.Label))])])]) : _vm._e(), _vm._v(" "), _c('div', {
    staticClass: "box-body"
  }, [(!_vm.finalModel && !_vm.action.InstanceOptional) ? _c('div', {
    staticClass: "col-md-12"
  }, [_c('select-one-or-more', {
    attrs: {
      "value": _vm.finalModel,
      "schema": _vm.modelSchema
    },
    on: {
      "save": _vm.setModel
    }
  })], 1) : _vm._e(), _vm._v(" "), _c('div', {
    staticClass: "col-md-12"
  }, [(_vm.meta != null) ? _c('model-form', {
    attrs: {
      "hide-title": true,
      "hide-cancel": _vm.hideCancel,
      "meta": _vm.meta,
      "model": _vm.data
    },
    on: {
      "save": function($event) {
        _vm.doAction(_vm.data)
      },
      "cancel": function($event) {
        _vm.cancel()
      },
      "update:model": function($event) {
        _vm.data = $event
      }
    }
  }) : _vm._e()], 1)])]) : _vm._e()
},staticRenderFns: []}

/***/ }),
/* 668 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "content-wrapper"
  }, [_c('section', {
    staticClass: "content-header"
  }, [_c('ol', {
    staticClass: "breadcrumb"
  }, [_vm._m(0), _vm._v(" "), _vm._l((_vm.$route.meta.breadcrumb), function(crumb) {
    return _c('li', {
      key: crumb.label
    }, [(crumb.to) ? void 0 : [_vm._v("\n          " + _vm._s(crumb.label) + "\n        ")]], 2)
  })], 2)]), _vm._v(" "), _c('section', {
    staticClass: "content"
  }, [_c('div', {
    staticClass: "row"
  }, [_c('div', {
    staticClass: "col-md-9"
  }, [_c('div', {
    staticClass: "row"
  }, _vm._l((_vm.worlds), function(world) {
    return _c('div', {
      key: world.id,
      staticClass: "col-lg-4 col-xs-6"
    }, [_c('div', {
      staticClass: "small-box",
      style: ({
        backgroundColor: _vm.stringToColor(world.TableName),
        color: 'white'
      })
    }, [_c('div', {
      staticClass: "inner"
    }, [_c('h3', [_vm._v(_vm._s(world.Count))]), _vm._v(" "), _c('p', [_vm._v(_vm._s(_vm._f("titleCase")(world.TableName)) + "s ")])]), _vm._v(" "), _c('div', {
      staticClass: "icon"
    }, [_c('i', {
      class: 'fa ' + world.Icon,
      staticStyle: {
        "color": "#bbb"
      }
    })]), _vm._v(" "), _c('router-link', {
      staticClass: "small-box-footer",
      attrs: {
        "to": {
          name: 'Entity',
          params: {
            tablename: world.TableName
          }
        }
      }
    }, [_c('i', {
      staticClass: "fa fa-arrow-circle-right"
    })])], 1)])
  }))]), _vm._v(" "), _c('div', {
    staticClass: "col-md-3"
  }, [_c('div', {
    staticClass: "row"
  }, _vm._l((_vm.worldActions), function(worlds, tableName) {
    return (worlds.length > 0) ? _c('div', {
      key: tableName,
      staticClass: "col-md-12"
    }, [(worlds.filter(function(e) {
      return e.InstanceOptional
    }).length > 0) ? _c('div', {
      staticClass: "box box-solid"
    }, [_c('div', {
      staticClass: "box-header with-border"
    }, [_c('h3', {
      staticClass: "box-title"
    }, [_vm._v(_vm._s(_vm._f("titleCase")(tableName)))]), _vm._v(" "), _vm._m(1, true)]), _vm._v(" "), _c('div', {
      staticClass: "box-body no-padding"
    }, [_c('ul', {
      staticClass: "nav nav-pills nav-stacked"
    }, _vm._l((worlds), function(action) {
      return (action.InstanceOptional) ? _c('li', {
        key: action.Name
      }, [_c('router-link', {
        attrs: {
          "to": {
            name: 'Action',
            params: {
              tablename: action.OnType,
              actionname: action.Name
            }
          }
        }
      }, [_vm._v("\n                      " + _vm._s(action.Label) + "\n                    ")])], 1) : _vm._e()
    }))])]) : _vm._e()]) : _vm._e()
  }))])]), _vm._v(" "), _c('div', {
    staticClass: "row"
  }, [_c('div', {
    staticClass: "col-md-12"
  }, [_c('div', {
    staticClass: "row"
  }, [_c('div', {
    staticClass: "col-md-12"
  }, [_c('div', {
    staticClass: "row"
  }, [_c('div', {
    staticClass: "col-sm-12"
  }, [_c('router-link', {
    staticClass: "btn btn-lg btn-app dashboard_button",
    attrs: {
      "to": {
        name: 'Entity',
        params: {
          tablename: 'marketplace'
        }
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-shopping-cart"
  }), _c('br'), _vm._v("\n                  Market places\n                ")]), _vm._v(" "), _c('router-link', {
    staticClass: "btn btn-lg btn-app dashboard_button",
    attrs: {
      "to": {
        name: 'Entity',
        params: {
          tablename: 'data_exchange'
        }
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-exchange-alt"
  }), _c('br'), _vm._v("\n                  Data Exchange\n                ")]), _vm._v(" "), _c('router-link', {
    staticClass: "btn btn-lg btn-app dashboard_button",
    attrs: {
      "to": {
        name: 'Entity',
        params: {
          tablename: 'oauth_token'
        }
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-key"
  }), _c('br'), _vm._v("\n                  Oauth Tokens\n                ")]), _vm._v(" "), _c('router-link', {
    staticClass: "btn btn-lg btn-app dashboard_button",
    attrs: {
      "to": {
        name: 'Entity',
        params: {
          tablename: 'oauth_connect'
        }
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-plug"
  }), _vm._v(" "), _c('br'), _vm._v("\n                  Oauth Connections\n                ")]), _vm._v(" "), _c('router-link', {
    staticClass: "btn btn-lg btn-app dashboard_button",
    attrs: {
      "to": {
        name: 'Entity',
        params: {
          tablename: 'cloud_store'
        }
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-cloud"
  }), _c('br'), _vm._v("Storage\n                ")]), _vm._v(" "), _c('router-link', {
    staticClass: "btn btn-lg btn-app dashboard_button",
    attrs: {
      "to": {
        name: 'Entity',
        params: {
          tablename: 'site'
        }
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-cubes "
  }), _c('br'), _vm._v("Sub sites\n                ")]), _vm._v(" "), _c('router-link', {
    staticClass: "btn btn-lg btn-app dashboard_button",
    attrs: {
      "to": {
        name: 'Entity',
        params: {
          tablename: 'stream'
        }
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-film "
  }), _c('br'), _vm._v("Data views\n                ")]), _vm._v(" "), _c('router-link', {
    staticClass: "btn btn-lg btn-app dashboard_button",
    attrs: {
      "to": {
        name: 'Entity',
        params: {
          tablename: 'json_schema'
        }
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-puzzle-piece "
  }), _c('br'), _vm._v("Json Schemas\n                ")])], 1)]), _vm._v(" "), _c('h3', [_vm._v("People")]), _vm._v(" "), _c('div', {
    staticClass: "row"
  }, [_c('div', {
    staticClass: "col-sm-12"
  }, [_c('router-link', {
    staticClass: "btn btn-lg btn-app dashboard_button",
    attrs: {
      "to": {
        name: 'Action',
        params: {
          tablename: 'user_account',
          actionname: 'signup'
        }
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-user-plus"
  }), _c('br'), _vm._v("Create new user\n                ")]), _vm._v(" "), _c('router-link', {
    staticClass: "btn btn-lg btn-app dashboard_button",
    attrs: {
      "to": {
        name: 'NewEntity',
        params: {
          tablename: 'usergroup'
        }
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-users"
  }), _c('br'), _vm._v("Create new user group\n                ")]), _vm._v(" "), _c('router-link', {
    staticClass: "btn btn-lg btn-app dashboard_button",
    attrs: {
      "to": {
        name: 'Action',
        params: {
          tablename: 'world',
          actionname: 'become_an_administrator'
        }
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-lock"
  }), _c('br'), _vm._v("Become admin\n                ")]), _vm._v(" "), _c('router-link', {
    staticClass: "btn btn-lg btn-app dashboard_button",
    attrs: {
      "to": {
        name: 'Action',
        params: {
          tablename: 'world',
          actionname: 'restart_daptin'
        }
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-retweet"
  }), _c('br'), _vm._v("Restart\n                ")])], 1)]), _vm._v(" "), _c('h3', [_vm._v("Create")]), _vm._v(" "), _c('div', {
    staticClass: "row"
  }, [_c('div', {
    staticClass: "col-sm-12"
  }, [_c('router-link', {
    staticClass: "btn btn-lg btn-app dashboard_button",
    attrs: {
      "to": {
        name: 'Action',
        params: {
          tablename: 'world',
          actionname: 'upload_system_schema'
        }
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-plus "
  }), _c('br'), _vm._v("Upload Schema JSON\n                ")]), _vm._v(" "), _c('router-link', {
    staticClass: "btn btn-lg btn-app dashboard_button",
    attrs: {
      "to": {
        name: 'Action',
        params: {
          tablename: 'world',
          actionname: 'upload_xls_to_system_schema'
        }
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-file-excel"
  }), _c('br'), _vm._v("Upload XLSX\n                ")]), _vm._v(" "), _c('router-link', {
    staticClass: "btn btn-lg btn-app dashboard_button",
    attrs: {
      "to": {
        name: 'Action',
        params: {
          tablename: 'world',
          actionname: 'upload_csv_to_system_schema'
        }
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-file-alt"
  }), _c('br'), _vm._v("Upload CSV\n                ")]), _vm._v(" "), _c('router-link', {
    staticClass: "btn btn-lg btn-app dashboard_button",
    attrs: {
      "to": {
        name: 'Action',
        params: {
          tablename: 'world',
          actionname: 'import_data'
        }
      }
    }
  }, [_c('i', {
    staticClass: "fab fa-js"
  }), _c('br'), _vm._v("Upload Data JSON\n                ")]), _vm._v(" "), _c('router-link', {
    staticClass: "btn btn-lg btn-app dashboard_button",
    attrs: {
      "to": {
        name: 'NewItem'
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-pencil-alt"
  }), _c('br'), _vm._v("Online designer\n                ")])], 1)]), _vm._v(" "), _c('h3', [_vm._v("Backup")]), _vm._v(" "), _c('div', {
    staticClass: "row"
  }, [_c('div', {
    staticClass: "col-sm-12"
  }, [_c('router-link', {
    staticClass: "btn btn-lg btn-app dashboard_button",
    attrs: {
      "to": {
        name: 'Action',
        params: {
          tablename: 'world',
          actionname: 'download_system_schema'
        }
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-object-group"
  }), _c('br'), _vm._v("Download JSON schema\n                ")]), _vm._v(" "), _c('router-link', {
    staticClass: "btn btn-lg btn-app dashboard_button",
    attrs: {
      "to": {
        name: 'Action',
        params: {
          tablename: 'world',
          actionname: 'export_data'
        }
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-database"
  }), _c('br'), _vm._v("Download JSON dump\n                ")])], 1)])])])])]), _vm._v(" "), _c('div', {
    staticClass: "row"
  })])])
},staticRenderFns: [function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('li', [_c('a', {
    attrs: {
      "href": "javascript:"
    }
  }, [_c('i', {
    staticClass: "fa fa-home"
  }), _vm._v("Home ")])])
},function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "box-tools"
  }, [_c('button', {
    staticClass: "btn btn-box-tool",
    attrs: {
      "type": "button",
      "data-widget": "collapse"
    }
  }, [_c('i', {
    staticClass: "fa fa-minus"
  })])])
}]}

/***/ }),
/* 669 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "content-wrapper"
  }, [_c('section', {
    staticClass: "content-header"
  }, [_c('h1', [_c('small', [_vm._v(_vm._s(_vm.$route.actionname))])]), _vm._v(" "), _c('ol', {
    staticClass: "breadcrumb"
  }, [_vm._m(0), _vm._v(" "), _c('li', {
    staticClass: "active"
  }, [_vm._v(_vm._s(_vm.$route.name.toUpperCase()))])])]), _vm._v(" "), _c('section', {
    staticClass: "content"
  }, [_c('div', {
    staticClass: "col-md-12"
  }, [(_vm.action) ? _c('action-view', {
    ref: "systemActionView",
    attrs: {
      "hide-title": false,
      "action-manager": _vm.actionManager,
      "action": _vm.action,
      "model": _vm.model,
      "json-api": _vm.jsonApi
    },
    on: {
      "cancel": _vm.cancel
    }
  }) : _vm._e()], 1), _vm._v(" "), (!_vm.action) ? _c('h3', [_vm._v("404, Action not found")]) : _vm._e()])])
},staticRenderFns: [function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('li', [_c('a', {
    attrs: {
      "href": "javascript:;"
    }
  }, [_c('i', {
    staticClass: "fa fa-home"
  }), _vm._v("Home")])])
}]}

/***/ }),
/* 670 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "box"
  }, [_c('div', {
    staticClass: "box-header"
  }, [_c('div', {
    staticClass: "box-title"
  }), _vm._v(" "), _c('div', {
    staticClass: "box-tools"
  }, [_c('div', {
    staticClass: "ui icon buttons"
  }, [_c('vuetable-pagination', {
    ref: "pagination",
    staticStyle: {
      "margin": "0px"
    },
    attrs: {
      "css": _vm.css.pagination
    },
    on: {
      "change-page": _vm.onChangePage
    }
  }), _vm._v(" "), _c('button', {
    staticClass: "btn btn-box-tool",
    attrs: {
      "type": "button"
    },
    on: {
      "click": function($event) {
        _vm.reloadData()
      }
    }
  }, [_vm._m(0)]), _vm._v(" "), _c('button', {
    staticClass: "btn btn-box-tool",
    attrs: {
      "type": "button"
    },
    on: {
      "click": function($event) {
        _vm.showAddEdit = true
      }
    }
  }, [_vm._m(1)])], 1)])]), _vm._v(" "), _c('div', {
    staticClass: "box-body"
  }, [(_vm.showAddEdit) ? _c('div', {
    staticClass: "col-md-12"
  }, [(_vm.showSelect) ? _c('button', {
    staticClass: "btn btn-success",
    on: {
      "click": function($event) {
        _vm.showSelect = false
      }
    }
  }, [_vm._v("\n        Create new " + _vm._s(_vm._f("titleCase")(_vm.jsonApiModelName)) + "\n      ")]) : _vm._e(), _vm._v(" "), (!_vm.showSelect) ? _c('button', {
    staticClass: "btn btn-primary",
    on: {
      "click": function($event) {
        _vm.showSelect = true
      }
    }
  }, [_vm._v("\n        Search and add " + _vm._s(_vm._f("titleCase")(_vm.jsonApiModelName)) + "\n      ")]) : _vm._e()]) : _vm._e(), _vm._v(" "), (_vm.showAddEdit && _vm.meta) ? [(_vm.showSelect) ? _c('div', {
    staticClass: "col-md-6 pull-right"
  }, [_c('select-one-or-more', {
    attrs: {
      "schema": {
        inputType: _vm.jsonApiModelName
      }
    },
    on: {
      "save": _vm.saveRow
    }
  })], 1) : _vm._e(), _vm._v(" "), (!_vm.showSelect) ? _c('div', {
    staticClass: "col-md-12"
  }, [_c('model-form', {
    attrs: {
      "json-api": _vm.jsonApi,
      "meta": _vm.meta
    },
    on: {
      "save": _vm.saveRow,
      "cancel": function($event) {
        _vm.cancel()
      }
    }
  })], 1) : _vm._e()] : _vm._e(), _vm._v(" "), _vm._l((_vm.tableData), function(item) {
    return _c('div', {
      staticClass: "col-md-12"
    }, [_c('detailed-table-row', {
      key: item.id,
      ref: "vuetable",
      refInFor: true,
      attrs: {
        "show-all": false,
        "model": item,
        "json-api": _vm.jsonApi,
        "json-api-model-name": _vm.jsonApiModelName
      },
      on: {
        "saveRelatedRow": _vm.saveRelatedRow,
        "deleteRow": _vm.deleteRow
      }
    })], 1)
  })], 2)])
},staticRenderFns: [function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('span', [_c('i', {
    staticClass: "fas fa-sync fa-2x  yellow"
  })])
},function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('span', [_c('i', {
    staticClass: "fas fa-plus fa-2x green"
  })])
}]}

/***/ }),
/* 671 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "container"
  }, [_c('div', {
    staticClass: "row vertical-10p"
  }, [_c('div', {
    staticClass: "container"
  }, [_vm._m(0), _vm._v(" "), _c('div', {
    staticClass: "col-md-4 col-sm-offset-4"
  }, [(_vm.signInAction) ? _c('action-view', {
    attrs: {
      "model": {},
      "hide-cancel": true,
      "actionManager": _vm.actionManager,
      "action": _vm.signInAction
    },
    on: {
      "action-complete": _vm.signupComplete
    }
  }) : _vm._e(), _vm._v(" "), (_vm.response) ? _c('div', {
    staticClass: "text-red"
  }, [_c('p', [_vm._v(_vm._s(_vm.response))])]) : _vm._e()], 1), _vm._v(" "), _c('div', {
    staticClass: "col-md-4 col-sm-offset-4"
  }, [_c('div', {
    staticClass: "box"
  }, [_c('div', {
    staticClass: "box-body"
  }, [_c('router-link', {
    class: 'btn bg-blue',
    attrs: {
      "to": {
        name: 'SignIn'
      }
    }
  }, [_vm._v("Sign In")])], 1)])])])])])
},staticRenderFns: [function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "register-logo"
  }, [_c('a', {
    attrs: {
      "href": "javascript:;"
    }
  }, [_c('b', [_vm._v("Daptin")])])])
}]}

/***/ }),
/* 672 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "container container-table login"
  }, [_c('div', {
    staticClass: "row vertical-5p"
  }, [_c('div', {
    staticClass: "container"
  }, [_c('img', {
    staticClass: "center-block logo",
    attrs: {
      "src": "/static/img/logo.png"
    }
  }), _vm._v(" "), _c('div', {
    staticClass: "text-center col-md-4 col-sm-offset-4"
  }, [_c('form', {
    staticClass: "ui form loginForm",
    on: {
      "submit": function($event) {
        $event.preventDefault();
        _vm.checkCreds($event)
      }
    }
  }, [_c('div', {
    staticClass: "input-group"
  }, [_vm._m(0), _vm._v(" "), _c('input', {
    directives: [{
      name: "model",
      rawName: "v-model",
      value: (_vm.username),
      expression: "username"
    }],
    staticClass: "form-control",
    attrs: {
      "name": "username",
      "placeholder": "Username",
      "type": "text"
    },
    domProps: {
      "value": (_vm.username)
    },
    on: {
      "input": function($event) {
        if ($event.target.composing) { return; }
        _vm.username = $event.target.value
      }
    }
  })]), _vm._v(" "), _c('div', {
    staticClass: "input-group"
  }, [_vm._m(1), _vm._v(" "), _c('input', {
    directives: [{
      name: "model",
      rawName: "v-model",
      value: (_vm.password),
      expression: "password"
    }],
    staticClass: "form-control",
    attrs: {
      "name": "password",
      "placeholder": "Password",
      "type": "password"
    },
    domProps: {
      "value": (_vm.password)
    },
    on: {
      "input": function($event) {
        if ($event.target.composing) { return; }
        _vm.password = $event.target.value
      }
    }
  })]), _vm._v(" "), _c('button', {
    class: 'btn btn-primary btn-lg ' + _vm.loading,
    attrs: {
      "type": "submit"
    }
  }, [_vm._v("Submit")])]), _vm._v(" "), (_vm.response) ? _c('div', {
    staticClass: "text-red"
  }, [_c('p', [_vm._v(_vm._s(_vm.response))])]) : _vm._e()])])])])
},staticRenderFns: [function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('span', {
    staticClass: "input-group-addon"
  }, [_c('i', {
    staticClass: "fa fa-envelope"
  })])
},function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('span', {
    staticClass: "input-group-addon"
  }, [_c('i', {
    staticClass: "fa fa-lock"
  })])
}]}

/***/ }),
/* 673 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "container"
  }, [_c('div', {
    staticClass: "row vertical-10p"
  }, [_c('div', {
    staticClass: "container"
  }, [_vm._m(0), _vm._v(" "), _c('div', {
    staticClass: "col-md-4 col-sm-offset-4"
  }, [(_vm.signInAction) ? _c('action-view', {
    attrs: {
      "model": {},
      "hide-cancel": true,
      "actionManager": _vm.actionManager,
      "action": _vm.signInAction
    }
  }) : _vm._e(), _vm._v(" "), (_vm.response) ? _c('div', {
    staticClass: "text-red"
  }, [_c('p', [_vm._v(_vm._s(_vm.response))])]) : _vm._e()], 1), _vm._v(" "), _c('div', {
    staticClass: "col-md-3"
  }, _vm._l((_vm.oauthConnections), function(connect) {
    return _c('div', {
      staticClass: "row"
    }, [_c('div', {
      staticClass: "col-md-12"
    }, [_c('el-button', {
      staticStyle: {
        "margin": "5px"
      },
      on: {
        "click": function($event) {
          _vm.oauthLogin(connect)
        }
      }
    }, [_vm._v("Login via " + _vm._s(_vm._f("chooseTitle")(connect)))])], 1)])
  })), _vm._v(" "), _c('div', {
    staticClass: "col-md-4 col-sm-offset-4"
  }, [_c('div', {
    staticClass: "box"
  }, [_c('div', {
    staticClass: "box-body"
  }, [_c('router-link', {
    staticClass: "btn bg-blue",
    attrs: {
      "to": {
        name: 'SignUp'
      }
    }
  }, [_vm._v("Sign Up")])], 1)])])])])])
},staticRenderFns: [function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "register-logo"
  }, [_c('a', {
    attrs: {
      "href": "javascript:;"
    }
  }, [_c('b', [_vm._v("Daptin")])])])
}]}

/***/ }),
/* 674 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "content-wrapper"
  }, [_c('section', {
    staticClass: "content-header"
  }, [_c('h1', [_vm._v("\n\n      " + _vm._s(_vm._f("titleCase")(_vm.selectedTable)) + "\n      "), _c('small', [_vm._v(_vm._s(_vm.$route.meta.description))])]), _vm._v(" "), _c('ol', {
    staticClass: "breadcrumb"
  }, [_vm._m(0), _vm._v(" "), _vm._l((_vm.$route.meta.breadcrumb), function(crumb) {
    return _c('li', [(crumb.to) ? void 0 : [_vm._v("\n          " + _vm._s(crumb.label) + "\n        ")]], 2)
  })], 2), _vm._v(" "), _c('div', {
    staticClass: "box-tools pull-right"
  }, [_c('div', {
    staticClass: "ui icon buttons"
  }, [_c('button', {
    staticClass: "btn btn-box-tool",
    on: {
      "click": function($event) {
        $event.preventDefault();
        _vm.viewMode = 'table';
        _vm.currentViewType = 'table-view';
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-table fa-2x"
  })]), _vm._v(" "), _c('button', {
    staticClass: "btn btn-box-tool",
    on: {
      "click": function($event) {
        $event.preventDefault();
        _vm.viewMode = 'card';
        _vm.currentViewType = 'table-view';
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-th-large fa-2x  grey"
  })]), _vm._v(" "), _c('button', {
    staticClass: "btn btn-box-tool",
    on: {
      "click": function($event) {
        $event.preventDefault();
        _vm.currentViewType = 'recline-view'
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-chart-bar fa-2x grey"
  })]), _vm._v(" "), (_vm.selectedTable) ? _c('router-link', {
    staticClass: "btn btn-box-tool",
    attrs: {
      "to": {
        name: 'NewEntity',
        params: {
          tablename: _vm.selectedTable
        }
      }
    },
    on: {
      "click": function($event) {
        $event.preventDefault();
        _vm.newRow()
      }
    }
  }, [_c('i', {
    staticClass: "fa fa-2x fa-plus green "
  })]) : _vm._e(), _vm._v(" "), _c('button', {
    staticClass: "btn btn-box-tool",
    on: {
      "click": function($event) {
        $event.preventDefault();
        _vm.reloadData()
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-sync fa-2x grey"
  })]), _vm._v(" "), _c('router-link', {
    staticClass: "btn btn-box-tool",
    attrs: {
      "to": {
        name: 'NewItem',
        query: {
          table: _vm.selectedTable
        }
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-edit fa-2x grey"
  })]), _vm._v(" "), _c('router-link', {
    staticClass: "btn btn-box-tool",
    attrs: {
      "to": {
        name: 'Action',
        params: {
          actionname: 'add_exchange',
          tablename: 'world'
        },
        query: {
          id: _vm.worldReferenceId
        }
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-link fa-2x  grey"
  })]), _vm._v(" "), _c('router-link', {
    staticClass: "btn btn-box-tool",
    attrs: {
      "to": {
        name: 'Action',
        params: {
          actionname: 'export_data',
          tablename: 'world'
        },
        query: {
          world_id: _vm.worldReferenceId
        }
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-download fa-2x  grey"
  })]), _vm._v(" "), _c('router-link', {
    staticClass: "btn btn-box-tool",
    attrs: {
      "to": {
        name: 'Action',
        params: {
          actionname: 'export_csv_data',
          tablename: 'world'
        },
        query: {
          world_id: _vm.worldReferenceId
        }
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-bars fa-2x  grey"
  })])], 1)])]), _vm._v(" "), _c('section', {
    staticClass: "content"
  }, [(_vm.showAddEdit && _vm.rowBeingEdited != null) ? _c('div', {
    staticClass: "row"
  }, [_c('div', {
    staticClass: "col-md-12"
  }, [_c('model-form', {
    ref: "modelform",
    attrs: {
      "hideTitle": true,
      "json-api": _vm.jsonApi,
      "model": _vm.rowBeingEdited,
      "meta": _vm.selectedTableColumns
    },
    on: {
      "save": function($event) {
        _vm.saveRow(_vm.rowBeingEdited)
      },
      "cancel": function($event) {
        _vm.showAddEdit = false
      }
    }
  })], 1)]) : _vm._e(), _vm._v(" "), (_vm.currentViewType == 'table-view') ? [(_vm.selectedTable && !_vm.showAddEdit) ? _c('table-view', {
    ref: "tableview1",
    attrs: {
      "finder": _vm.finder,
      "view-mode": _vm.viewMode,
      "json-api": _vm.jsonApi,
      "json-api-model-name": _vm.selectedTable
    },
    on: {
      "newRow": function($event) {
        _vm.newRow()
      },
      "editRow": _vm.editRow
    }
  }) : _vm._e()] : (_vm.currentViewType == 'recline-view') ? [(_vm.selectedTable && !_vm.showAddEdit) ? _c('recline-view', {
    ref: "tableview1",
    attrs: {
      "finder": _vm.finder,
      "view-mode": _vm.viewMode,
      "json-api": _vm.jsonApi,
      "json-api-model-name": _vm.selectedTable
    },
    on: {
      "newRow": function($event) {
        _vm.newRow()
      },
      "editRow": _vm.editRow
    }
  }) : _vm._e()] : (_vm.currentViewType == 'voyager-view') ? [(_vm.selectedTable && !_vm.showAddEdit) ? _c('voyager-view', {
    ref: "tableview1",
    attrs: {
      "finder": _vm.finder,
      "view-mode": _vm.viewMode,
      "json-api": _vm.jsonApi,
      "json-api-model-name": _vm.selectedTable
    },
    on: {
      "newRow": function($event) {
        _vm.newRow()
      },
      "editRow": _vm.editRow
    }
  }) : _vm._e()] : _vm._e()], 2)])
},staticRenderFns: [function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('li', [_c('a', {
    attrs: {
      "href": "javascript:"
    }
  }, [_c('i', {
    staticClass: "fa fa-home"
  }), _vm._v("Home")])])
}]}

/***/ }),
/* 675 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticStyle: {
      "position": "relative",
      "overflow": "scroll",
      "height": "700px"
    }
  }, [_c('div', {
    staticClass: "table-header"
  }, [_c('table', {
    class: ['vuetable', 'fixed'],
    staticStyle: {
      "position": "relative"
    }
  }, [_c('thead', {
    staticClass: "vuetable-header"
  }, [_c('tr', [_vm._l((_vm.tableFields), function(field) {
    return [(field.visible) ? [(_vm.isSpecialField(field.name)) ? [(_vm.extractName(field.name) == '__checkbox') ? _c('th', {
      class: ['vuetable-th-checkbox-' + _vm.trackBy, field.titleClass]
    }, [_c('input', {
      attrs: {
        "type": "checkbox"
      },
      domProps: {
        "checked": _vm.checkCheckboxesState(field.name)
      },
      on: {
        "change": function($event) {
          _vm.toggleAllCheckboxes(field.name, $event)
        }
      }
    })]) : _vm._e(), _vm._v(" "), (_vm.extractName(field.name) == '__component') ? _c('th', {
      class: ['vuetable-th-component-' + _vm.trackBy, field.titleClass, {
        'sortable': _vm.isSortable(field)
      }],
      domProps: {
        "innerHTML": _vm._s(_vm.renderTitle(field))
      },
      on: {
        "click": function($event) {
          _vm.orderBy(field, $event)
        }
      }
    }) : _vm._e(), _vm._v(" "), (_vm.extractName(field.name) == '__slot') ? _c('th', {
      class: ['vuetable-th-slot-' + _vm.extractArgs(field.name), field.titleClass, {
        'sortable': _vm.isSortable(field)
      }],
      on: {
        "click": function($event) {
          _vm.orderBy(field, $event)
        }
      }
    }, [_c('div', {
      staticClass: "header-cell",
      domProps: {
        "innerHTML": _vm._s(_vm.renderTitle(field))
      }
    })]) : _vm._e(), _vm._v(" "), (_vm.apiMode && _vm.extractName(field.name) == '__sequence') ? _c('th', {
      class: ['vuetable-th-sequence', field.titleClass || '']
    }, [_c('div', {
      staticClass: "header-cell",
      domProps: {
        "innerHTML": _vm._s(_vm.renderTitle(field))
      }
    })]) : _vm._e(), _vm._v(" "), (_vm.notIn(_vm.extractName(field.name), ['__sequence', '__checkbox', '__component', '__slot'])) ? _c('th', {
      class: ['vuetable-th-' + field.name, field.titleClass || '']
    }, [_c('div', {
      staticClass: "header-cell",
      domProps: {
        "innerHTML": _vm._s(_vm.renderTitle(field))
      }
    })]) : _vm._e()] : [_c('th', {
      class: ['vuetable-th-' + field.name, field.titleClass, {
        'sortable': _vm.isSortable(field)
      }],
      attrs: {
        "id": '_' + field.name
      },
      on: {
        "click": function($event) {
          _vm.orderBy(field, $event)
        }
      }
    }, [_c('div', {
      staticClass: "header-cell",
      domProps: {
        "innerHTML": _vm._s(_vm.renderTitle(field))
      }
    })])]] : _vm._e()]
  })], 2)])])]), _vm._v(" "), _c('div', {
    staticClass: "table-body"
  }, [_c('virtual-list', {
    staticClass: "vuetable",
    attrs: {
      "rtag": "table",
      "wtag": "tbody",
      "bench": 20,
      "size": 40,
      "remain": 40
    }
  }, _vm._l((_vm.tableData), function(item, index) {
    return _c('tr', [
      [_vm._l((_vm.tableFields), function(field) {
        return [(field.visible) ? [(_vm.isSpecialField(field.name)) ? [(_vm.apiMode && _vm.extractName(field.name) == '__sequence') ? _c('td', {
          class: ['vuetable-sequence', field.dataClass]
        }, [_c('div', {
          staticClass: "table-cell",
          domProps: {
            "innerHTML": _vm._s(_vm.tablePagination.from + index)
          }
        })]) : _vm._e(), _vm._v(" "), (_vm.extractName(field.name) == '__handle') ? _c('td', {
          class: ['vuetable-handle', field.dataClass],
          domProps: {
            "innerHTML": _vm._s(_vm.renderIconTag(['handle-icon', _vm.css.handleIcon]))
          }
        }) : _vm._e(), _vm._v(" "), (_vm.extractName(field.name) == '__checkbox') ? _c('td', {
          class: ['vuetable-checkboxes', field.dataClass]
        }, [_c('input', {
          attrs: {
            "type": "checkbox"
          },
          domProps: {
            "checked": _vm.rowSelected(item, field.name)
          },
          on: {
            "change": function($event) {
              _vm.toggleCheckbox(item, field.name, $event)
            }
          }
        })]) : _vm._e(), _vm._v(" "), (_vm.extractName(field.name) === '__component') ? _c('td', {
          class: ['vuetable-component', field.dataClass]
        }, [_c(_vm.extractArgs(field.name), {
          tag: "component",
          attrs: {
            "row-data": item,
            "row-index": index,
            "row-field": field.sortField
          }
        })], 1) : _vm._e(), _vm._v(" "), (_vm.extractName(field.name) === '__slot') ? _c('td', {
          class: ['vuetable-slot', field.dataClass]
        }, [_vm._t(_vm.extractArgs(field.name), null, {
          rowData: item,
          rowIndex: index,
          rowField: field.sortField
        })], 2) : _vm._e()] : [(_vm.hasCallback(field)) ? _c('td', {
          class: field.dataClass,
          on: {
            "click": function($event) {
              _vm.onCellClicked(item, field, $event)
            },
            "dblclick": function($event) {
              _vm.onCellDoubleClicked(item, field, $event)
            }
          }
        }, [_c('div', {
          staticClass: "table-cell",
          domProps: {
            "innerHTML": _vm._s(_vm.callCallback(field, item))
          }
        })]) : _c('td', {
          class: field.dataClass,
          on: {
            "click": function($event) {
              _vm.onCellClicked(item, field, $event)
            },
            "dblclick": function($event) {
              _vm.onCellDoubleClicked(item, field, $event)
            }
          }
        }, [_c('div', {
          staticClass: "table-cell",
          domProps: {
            "innerHTML": _vm._s(_vm.getObjectValue(item, field.name, ''))
          }
        })])]] : _vm._e()]
      })]
    ], 2)
  }))], 1)])
},staticRenderFns: []}

/***/ }),
/* 676 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "row"
  }, [(!_vm.showAll) ? _c('div', {
    staticClass: "col-md-12"
  }, [_c('div', {
    staticClass: "box"
  }, [_c('div', {
    staticClass: "box-header"
  }, [_c('div', {
    staticClass: "box-title"
  }, [_vm._v("\n          " + _vm._s(_vm._f("titleCase")(_vm._f("chooseTitle")(_vm.model))) + "\n        ")]), _vm._v(" "), _c('div', {
    staticClass: "box-tools pull-right"
  }, [_c('div', {
    staticClass: "ui icon buttons"
  }, [_c('button', {
    staticClass: "btn btn-box-tool",
    attrs: {
      "type": "button"
    },
    on: {
      "click": _vm.initiateDelete
    }
  }, [_c('span', {
    staticClass: "fa fa-2x fa-times red"
  })]), _vm._v(" "), (_vm.jsonApiModelName == 'usergroup') ? _c('button', {
    staticClass: "btn btn-box-tool",
    attrs: {
      "type": "button"
    },
    on: {
      "click": _vm.editPermission
    }
  }, [_c('span', {
    staticClass: "fas fa-edit fa-2x grey"
  })]) : _vm._e(), _vm._v(" "), _c('router-link', {
    staticClass: "btn btn-box-tool",
    attrs: {
      "type": "button",
      "to": {
        name: 'Instance',
        params: {
          tablename: _vm.jsonApiModelName,
          refId: _vm.model.reference_id
        }
      }
    }
  }, [_c('span', {
    staticClass: "fa fa-2x fa-expand"
  })])], 1)])]), _vm._v(" "), _c('div', {
    staticClass: "box-body"
  }, [_vm._l((_vm.truefalse), function(tf) {
    return _c('div', {
      staticClass: "col-md-4"
    }, [_c('input', {
      attrs: {
        "disabled": "",
        "type": "checkbox",
        "name": "tf.name"
      },
      domProps: {
        "checked": tf.value
      }
    }), _vm._v(" "), _c('label', [_vm._v(_vm._s(tf.label))])])
  }), _vm._v(" "), _c('div', {
    staticClass: "col-md-6"
  }, [_c('table', {
    staticClass: "table"
  }, [_c('tbody', _vm._l((_vm.normalFields), function(col) {
    return (col.value != '') ? _c('tr', {
      attrs: {
        "id": col.name
      }
    }, [_c('td', {
      staticStyle: {
        "width": "50%"
      }
    }, [_c('b', [_vm._v(" " + _vm._s(col.label) + " ")])]), _vm._v(" "), _c('td', {
      style: (col.style)
    }, [_vm._v(" " + _vm._s(col.value))])]) : _vm._e()
  }))])]), _vm._v(" "), (_vm.rowBeingEdited && _vm.showAddEdit) ? _c('div', {
    staticClass: "col-md-6"
  }, [_c('model-form', {
    ref: "modelform",
    attrs: {
      "hideTitle": true,
      "json-api": _vm.jsonApi,
      "model": _vm.rowBeingEdited,
      "meta": _vm.selectedTableColumns
    },
    on: {
      "save": function($event) {
        _vm.saveRow(_vm.rowBeingEdited)
      },
      "cancel": function($event) {
        _vm.showAddEdit = false
      }
    }
  })], 1) : _vm._e()], 2)])]) : _vm._e(), _vm._v(" "), (_vm.showAll) ? [_c('div', {
    staticClass: "col-md-12"
  }, [_c('el-tabs', [_c('el-tab-pane', {
    attrs: {
      "label": "Overview"
    }
  }, [_c('div', {
    staticClass: "col-md-6"
  }, [_c('div', {
    staticClass: "box-invisible"
  }, [_c('div', {
    staticClass: "box-header"
  }, [_c('div', {
    staticClass: "box-title"
  }, [_vm._v("\n                  Details\n                ")])]), _vm._v(" "), _c('div', {
    staticClass: "box-body"
  }, [_c('table', {
    staticClass: "table"
  }, [_c('tbody', _vm._l((_vm.normalFields), function(col) {
    return _c('tr', {
      attrs: {
        "id": col.name
      }
    }, [_c('td', [_c('b', [_vm._v(" " + _vm._s(col.label) + " ")])]), _vm._v(" "), _c('td', {
      style: (col.style),
      domProps: {
        "innerHTML": _vm._s(col.value)
      }
    })])
  }))])])])]), _vm._v(" "), _c('div', {
    staticClass: "col-md-6"
  }, _vm._l((_vm.imageFields), function(imageField) {
    return _c('div', {
      staticClass: "row"
    }, [_c('h3', [_vm._v(_vm._s(_vm._f("titleCase")(imageField.name)))]), _vm._v(" "), _vm._l((imageField.value), function(image) {
      return _c('div', {
        staticClass: "col-md-6"
      }, [_c('img', {
        staticStyle: {
          "height": "200px",
          "width": "100%"
        },
        attrs: {
          "src": 'data:image/jpeg;base64,' + image.contents
        }
      })])
    })], 2)
  })), _vm._v(" "), (_vm.truefalse != null && _vm.truefalse.length > 0) ? _c('div', {
    staticClass: "col-md-6"
  }, [_c('div', {
    staticClass: "box"
  }, [_c('div', {
    staticClass: "box-header"
  }, [_c('div', {
    staticClass: "box-title"
  }, [_vm._v("\n                  Options\n                ")])]), _vm._v(" "), _c('div', {
    staticClass: "box-body"
  }, [_c('table', {
    staticClass: "table"
  }, [_c('tbody', _vm._l((_vm.truefalse), function(tf) {
    return _c('tr', [_c('td', [_c('input', {
      attrs: {
        "disabled": "",
        "type": "checkbox",
        "name": "tf.name"
      },
      domProps: {
        "checked": tf.value
      }
    })]), _vm._v(" "), _c('td', [_c('label', [_vm._v(_vm._s(tf.label))])])])
  }))])])])]) : _vm._e()]), _vm._v(" "), _vm._l((_vm.relations), function(relation) {
    return (!relation.failed) ? _c('el-tab-pane', {
      key: relation.name,
      attrs: {
        "label": relation.label
      }
    }, [_c('list-view', {
      ref: relation.name,
      refInFor: true,
      staticClass: "tab",
      attrs: {
        "json-api": _vm.jsonApi,
        "data-tab": relation.name,
        "json-api-model-name": relation.type,
        "json-api-relation-name": relation.name,
        "autoload": true,
        "finder": relation.finder
      },
      on: {
        "onDeleteRow": _vm.initiateDelete,
        "saveRow": _vm.saveRow,
        "addRow": _vm.addRow,
        "onLoadFailure": function($event) {
          _vm.loadFailed(relation)
        }
      }
    })], 1) : _vm._e()
  })], 2)], 1)] : _vm._e()], 2)
},staticRenderFns: []}

/***/ }),
/* 677 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    class: ['vuetable-pagination-info', _vm.css.infoClass],
    domProps: {
      "innerHTML": _vm._s(_vm.paginationInfo)
    }
  })
},staticRenderFns: []}

/***/ }),
/* 678 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('el-date-picker', {
    attrs: {
      "type": "date",
      "placeholder": "Select a date"
    },
    model: {
      value: (_vm.value),
      callback: function($$v) {
        _vm.value = $$v
      },
      expression: "value"
    }
  })
},staticRenderFns: []}

/***/ }),
/* 679 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "col-md-12"
  }, [_c('div', {
    staticClass: "ui icon buttons"
  }, [_c('button', {
    staticClass: "btn btn-box-tool",
    on: {
      "click": function($event) {
        _vm.mode = 'ace'
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-align-justify fa-2x grey"
  })]), _vm._v(" "), _c('button', {
    staticClass: "btn btn-box-tool",
    on: {
      "click": function($event) {
        _vm.mode = 'je'
      }
    }
  }, [_c('i', {
    staticClass: "fas fa-edit fa-2x grey"
  })])]), _vm._v(" "), (_vm.mode == 'je') ? _c('div', {
    staticStyle: {
      "width": "100%",
      "height": "600px"
    },
    attrs: {
      "id": "jsonEditor"
    }
  }) : _vm._e(), _vm._v(" "), (_vm.mode == 'ace') ? _c('editor', {
    ref: "aceEditor",
    attrs: {
      "options": _vm.options,
      "content": _vm.initValue,
      "lang": 'markdown',
      "sync": true
    }
  }) : _vm._e()], 1)
},staticRenderFns: []}

/***/ }),
/* 680 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    staticClass: "container"
  }, [_vm._v("\n  Oauth response handler\n")])
},staticRenderFns: []}

/***/ }),
/* 681 */
/***/ (function(module, exports) {

module.exports={render:function (){var _vm=this;var _h=_vm.$createElement;var _c=_vm._self._c||_h;
  return _c('div', {
    class: [_vm.css.wrapperClass]
  }, [_c('a', {
    class: [_vm.css.linkClass, ( _obj = {}, _obj[_vm.css.disabledClass] = _vm.isOnFirstPage, _obj )],
    on: {
      "click": function($event) {
        _vm.loadPage('prev')
      }
    }
  }, [_c('i', {
    class: _vm.css.icons.prev
  })]), _vm._v(" "), _c('select', {
    class: ['vuetable-pagination-dropdown', _vm.css.dropdownClass],
    on: {
      "change": function($event) {
        _vm.loadPage($event.target.selectedIndex + 1)
      }
    }
  }, _vm._l((_vm.totalPage), function(n) {
    return _c('option', {
      class: [_vm.css.pageClass],
      domProps: {
        "value": n,
        "selected": _vm.isCurrentPage(n)
      }
    }, [_vm._v("\n      " + _vm._s(_vm.pageText) + " " + _vm._s(n) + "\n    ")])
  })), _vm._v(" "), _c('a', {
    class: [_vm.css.linkClass, ( _obj$1 = {}, _obj$1[_vm.css.disabledClass] = _vm.isOnLastPage, _obj$1 )],
    on: {
      "click": function($event) {
        _vm.loadPage('next')
      }
    }
  }, [_c('i', {
    class: _vm.css.icons.next
  })])])
  var _obj;
  var _obj$1;
},staticRenderFns: []}

/***/ }),
/* 682 */,
/* 683 */,
/* 684 */,
/* 685 */,
/* 686 */,
/* 687 */,
/* 688 */,
/* 689 */,
/* 690 */,
/* 691 */,
/* 692 */,
/* 693 */,
/* 694 */,
/* 695 */,
/* 696 */,
/* 697 */,
/* 698 */,
/* 699 */,
/* 700 */,
/* 701 */,
/* 702 */,
/* 703 */,
/* 704 */,
/* 705 */,
/* 706 */,
/* 707 */,
/* 708 */,
/* 709 */,
/* 710 */,
/* 711 */,
/* 712 */,
/* 713 */,
/* 714 */,
/* 715 */,
/* 716 */,
/* 717 */,
/* 718 */,
/* 719 */,
/* 720 */,
/* 721 */,
/* 722 */,
/* 723 */,
/* 724 */,
/* 725 */,
/* 726 */,
/* 727 */,
/* 728 */,
/* 729 */,
/* 730 */
/***/ (function(module, exports) {

/* (ignored) */

/***/ })
],[287]);
//# sourceMappingURL=app.32aa669f8904f789b322.js.map