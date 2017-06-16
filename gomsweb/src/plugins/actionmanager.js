import axios from "axios"
import appConfig from "./appconfig"
import {getToken} from "../utils/auth"
import {Notification} from "element-ui"
import jwtDecode from 'jwt-decode'

const ActionManager = function () {

  const that = this;
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
  }

  setTimeout(function () {
    that.a = document.createElement("a");
    document.body.appendChild(that.a);
    that.a.style = "display: none";
    return function (downloadData) {
      var blob = new Blob([atob(downloadData.content)], {type: downloadData.contentType}),
        url = window.URL.createObjectURL(blob);
      that.a.href = url;
      that.a.download = downloadData.name;
      that.a.click();
      window.URL.revokeObjectURL(url);
    };
  })

  this.saveByteArray = function (downloadData) {
    var blob = new Blob([atob(downloadData.content)], {type: downloadData.contentType}),
      url = window.URL.createObjectURL(blob);
    that.a.href = url;
    that.a.download = downloadData.name;
    that.a.click();
    window.URL.revokeObjectURL(url);
  };


  this.getGuestActions = function () {
    return new Promise(function (resolve, reject) {
      axios({
        url: appConfig.apiRoot + "/actions",
        method: "GET"
      }).then(function (respo) {
        console.log("Guest actions list: ", respo)
        resolve(respo.data)
      }, function (rs) {
        console.log("get actions list fetch failed", arguments);
        reject(rs)
      })
    });
  };

  this.doAction = function (type, actionName, data) {
    // console.log("invoke action", type, actionName, data);
    var that = this;
    return new Promise(function (resolve, reject) {
      axios({
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
                Notification(data);
                break;
              case "client.store.set":
                console.log("notify client", data);
                window.localStorage.setItem(data.key, data.value);
                if (data.key == "token") {
                  window.localStorage.setItem('user', JSON.stringify(jwtDecode(data.value)));
                }
                break;
              case "client.file.download":
                that.saveByteArray(data)
                break;
              case "client.redirect":
                (function (redirectAttrs) {

                  Notification.success({
                    message: "Redirecting in " + (redirectAttrs.delay / 1000) + " seconds",
                  });
                  setTimeout(function () {

                    var target = redirectAttrs["window"]

                    if (target == "self") {
                      window.location = redirectAttrs.location;
                    } else {
                      window.open(redirectAttrs.location, "_target")
                    }

                  }, redirectAttrs.delay)

                })(data)
                break;

            }
          }
        } else {
          Notification.success("Action " + actionName + " finished.")
        }
      }, function (res) {
        reject("Failed")
        Notification.error("Action " + actionName + " failed.")
      })

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
