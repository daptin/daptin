import axios from "axios"
import appConfig from "~/plugins/appconfig"


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
        "Authorization": "Bearer " + window.localStorage.getItem("id_token")
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
    // console.log("actions for ", typeName, that.actionMap[typeName])
    return that.actionMap[typeName];
  };

  this.getActionModel = function (typeName, actionName) {
    return that.actionMap[typeName].filter(function (i, r) {
      return r.ActionName == actionName;
    })[0];
  };

  return this;
};


var actionmanager = new ActionManager();
export default actionmanager
