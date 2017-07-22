/**
 * Created by artpar on 6/7/17.
 */


import axios from "axios"
import jsonApi from "./jsonapi"
import actionManager from "./actionmanager"
import appconfig from "./appconfig"
import {getToken} from '../utils/auth'
import store from '../store'


const WorldManager = function () {
  const that = this;
  that.columnKeysCache = {};


  that.stateMachines = {};
  that.stateMachineEnabled = {};


  that.getStateMachinesForType = function (typeName) {
    return new Promise(function (resolve, reject) {
      resolve(that.stateMachines[typeName]);
    });
  };

  that.startObjectTrack = function (objType, objRefId, stateMachineRefId) {

    return axios({
      url: appconfig.apiRoot + "/track/start/" + stateMachineRefId,
      method: "POST",
      data: {
        typeName: objType,
        referenceId: objRefId
      },
      headers: {
        "Authorization": "Bearer " + getToken()
      }
    })
  };

  that.trackObjectEvent = function (typeName, stateMachineRefId, eventName) {
    console.log("change object track", getToken())
    return axios({
      url: appconfig.apiRoot + "/track/event/" + stateMachineRefId + "/" + eventName,
      url: appconfig.apiRoot + "/track/event/" + typeName + "/" + stateMachineRefId + "/" + eventName,
      method: "POST",
      headers: {
        "Authorization": "Bearer " + getToken()
      }
    })
  };

  that.getColumnKeys = function (typeName, callback) {
    // console.log("get column keys for ", typeName);
    if (that.columnKeysCache[typeName]) {
      callback(that.columnKeysCache[typeName]);
      return
    }

    axios(appconfig.apiRoot + '/jsmodel/' + typeName + ".js", {
      headers: {
        "Authorization": "Bearer " + getToken()
      },
    }).then(function (r) {
      if (r.status == 200) {
        var r = r.data;
        // console.log("Loaded Model inside :", typeName)
        if (r.Actions.length > 0) {
          console.log("Register actions", typeName, r.Actions,)
          actionManager.addAllActions(r.Actions);
        }
        that.stateMachines[typeName] = r.StateMachines;
        that.stateMachineEnabled[typeName] = r.IsStateMachineEnabled;
        that.columnKeysCache[typeName] = r;
        callback(r);
      } else {
        callback({}, r)
      }
    }, function (e) {
      callback(e)
    })

  };

  that.isStateMachineEnabled = function (typeName) {
    return that.stateMachineEnabled[typeName] == true;
  };

  that.getColumnKeysWithErrorHandleWithThisBuilder = function () {
    return function (typeName, callback) {
      // console.log("load model", typeName);
      return that.getColumnKeys(typeName, function (a, e, s) {
        // console.log("get column kets respone: ", arguments)
        if (e === "error" && s === "Unauthorized") {
          that.logout();
        } else {
          callback(a, e, s)
        }
      })
    }
  };


  that.GetJsonApiModel = function (columnModel) {
    // console.log('get json api model for ', columnModel);
    var model = {};
    if (!columnModel) {
      console.log("Column model is empty", columnModel);
      return model;
    }

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

  };

  var logoutHandler = ";";

  that.modelLoader = that.getColumnKeysWithErrorHandleWithThisBuilder(logoutHandler);

  that.worlds = [];

  that.getWorlds = function () {
    console.log("GET WORLDS", that.worlds)
    return that.worlds;
  };
  that.getWorldByName = function (name) {
    return that.worlds.filter(function (e) {
      return e.table_name == name;
    })[0];
    // console.log("GET WORLDS", that.worlds)
    // return that.worlds;
  };

  that.systemActions = [];


  that.getSystemActions = function () {
    return that.systemActions;
  }

  that.loadModels = function () {
    var promise = new Promise(function (resolve, reject) {
      // do a thing, possibly async, then…
      that.modelLoader("user", function (columnKeys) {
        jsonApi.define("user", that.GetJsonApiModel(columnKeys.ColumnModel));
        that.modelLoader("usergroup", function (columnKeys) {
          jsonApi.define("usergroup", that.GetJsonApiModel(columnKeys.ColumnModel));

          that.modelLoader("world", function (columnKeys) {
            jsonApi.define("world", that.GetJsonApiModel(columnKeys.ColumnModel));
            // console.log("world column keys", columnKeys, that.GetJsonApiModel(columnKeys.ColumnModel))
            console.log("Defined world", columnKeys.ColumnModel);
            that.systemActions = columnKeys.Actions;
            jsonApi.findAll('world', {
              page: {number: 1, size: 50},
              include: ['world_column']
            }).then(function (res) {
              that.worlds = res;
              store.commit("SET_WORLDS", res)
              console.log("Get all worlds result", res)
              // resolve("Stuff worked!");
              var total = res.length;

              for (var t = 0; t < res.length; t++) {
                (function (typeName) {
                  that.modelLoader(typeName, function (model) {
                    console.log("Loaded model", typeName, model);

                    total -= 1;

                    if (total < 1 && promise !== null) {
                      resolve("Stuff worked!");
                      promise = null;
                    }

                    jsonApi.define(typeName, that.GetJsonApiModel(model.ColumnModel));
                  })
                })(res[t].table_name)

              }
            });

          })
        });
      });


    });


    return promise;
  }


};


const worldmanager = new WorldManager();

export default worldmanager;

