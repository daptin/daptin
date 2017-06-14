/**
 * Created by artpar on 6/14/17.
 */
import appconfig from "./appconfig"

import axios from "axios"

const ConfigManager = function () {


  this.getAllConfig = function () {
    return axios({
      url: appconfig.apiRoot + "/config",
      method: "GET",
    })
  }

  return this;

};


export default new ConfigManager();
