/**
 * Created by artpar on 6/14/17.
 */
import appconfig from "./appconfig"

import axios from "axios"
import {getToken} from "../utils/auth"

const ConfigManager = function () {


  this.getConfig = function (name) {
    return new Promise(function (resolve, reject) {
      axios({
        url: appconfig.apiRoot + "/_config/backend/" + name,
        method: "GET",
        headers: {
          "Authorization": "Bearer " + getToken(),
        },
      }).then(function (r) {
        resolve(r.data)
      }, function (r) {
        reject(r);
      });
    })
  };


  this.setConfig = function (name, value) {
    return new Promise(function (resolve, reject) {
      axios({
        url: appconfig.apiRoot + "/_config/backend/" + name,
        method: "POST",
        headers: {
          "Authorization": "Bearer " + getToken(),
        },
        data: value
      }).then(function (r) {
        resolve(r.data)
      }, function (r) {
        reject(r);
      });
    })
  };

  this.getAllConfig = function () {
    var p = new Promise(function (resolve, reject) {
      axios({
        url: appconfig.apiRoot + "/_config",
        method: "GET",
        headers: {
          "Authorization": "Bearer " + getToken(),
        },
      }).then(function (r) {
        resolve(r.data)
      }, function (r) {
        reject(r);
      });

    });
    return p
  };

  return this;

};


export default new ConfigManager();
