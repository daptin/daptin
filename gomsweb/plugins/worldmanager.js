/**
 * Created by artpar on 6/7/17.
 */


import axios from "axios"
import jsonApi from "~/plugins/jsonapi"
import appconfig from "~/plugins/appconfig"
import {getToken} from '~/utils/auth'


const WorldManager = function () {
  const that = this;
  that.columnKeysCache = {};


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
        // console.log("Loaded Model :", typeName)
        if (r.Actions.length > 0) {
          // console.log("register actions", r.Actions)
          actionmanager.addAllActions(r.Actions);
        }
        that.columnKeysCache[typeName] = r;
        callback(r);
      } else {
        callback({}, r)
      }
    }, function (e) {
      callback(e)
    })

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

            jsonApi.findAll('world', {
              page: {number: 1, size: 50},
              include: ['world_column']
            }).then(function (res) {
              console.log("Get all worlds result", res)
              var total = res.length;

              for (var t = 0; t < res.length; t++) {
                (function (typeName) {
                  that.modelLoader(typeName, function (model) {
                    console.log("Loaded model", typeName, model);

                    total -= 1;

                    if (total < 3 && promise !== false) {
                      resolve("Stuff worked!");
                      promise = false;
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

