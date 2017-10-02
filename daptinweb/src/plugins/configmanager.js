/**
 * Created by artpar on 6/14/17.
 */
import appconfig from "./appconfig"

import axios from "axios"

const ConfigManager = function () {


  this.getAllConfig = function () {
    var p = new Promise(function (resolve, reject) {
      axios({
        url: appconfig.apiRoot + "/config",
        method: "GET",
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
