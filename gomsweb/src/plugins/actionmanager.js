import axios from "axios"
import appConfig from "./appconfig"
import {getToken} from "../utils/auth"
import {Notification} from "element-ui"

const ActionManager = function () {

  const that = this;
  that.actionMap = {};

  this.setActions = function (typeName, actions) {
    that.actionMap[typeName] = actions;
  };

  this.doAction = function (type, actionName, data) {
    // console.log("invoke action", type, actionName, data);
    return axios({
      url: appConfig.apiRoot + "/action/" + actionName,
      method: "POST",
      headers: {
        "Authorization": "Bearer " + getToken()
      },
      data: {
        type: type,
        action: actionName,
        attributes: data
      }
    }).then(function (res) {
      Notification.success("Action " + actionName + " finished.")
    }, function (res) {
      Notification.error("Action " + actionName + " failed.")
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
    // console.log("actions for ", typeName, that.actionMap[typeName])
    return that.actionMap[typeName];
  };

  this.getActionModel = function (typeName, actionName) {
    return that.actionMap[typeName][actionName];
  };

  return this;
};


var actionmanager = new ActionManager();
export default actionmanager
