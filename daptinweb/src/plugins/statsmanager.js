import axios from "axios"
import appConfig from "./appconfig"
import {getToken} from "../utils/auth"


const StatsManager = function () {
  const that = this;

  that.queryToParams = function (statsRequest) {

    var keys = Object.keys(statsRequest);
    var list = [];

    for (var i = 0; i < keys.length; i++) {

      var key = keys[i];
      var values = statsRequest[key];

      if (!(values instanceof Array)) {
        values = [values]
      }

      for (var j = 0; j < values.length; j++) {
        list.push(encodeURIComponent(key) + "=" + encodeURIComponent(values));
      }

    }

    return "?" + list.join("&");


  };

  that.getStats = function (tableName, statsRequest) {

    // console.log("create stats request", tableName, statsRequest)
    return axios({
      url: appConfig.apiRoot + "/stats/" + tableName + that.queryToParams(statsRequest),
      headers: {
        "Authorization": "Bearer " + getToken(),
        "Accept-Language": localStorage.getItem("LANGUAGE") || window.language
      }
    })

  }


};


const statsManager = new StatsManager();


export default statsManager
