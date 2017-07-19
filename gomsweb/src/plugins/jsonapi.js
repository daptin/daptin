import  JsonApi from "devour-client"
import {Notification} from "element-ui"
import appConfig from "../plugins/appconfig"
import {getToken, unsetToken} from '../utils/auth'

const jsonapi = new JsonApi({
  apiUrl: appConfig.apiRoot + '/api',
  pluralize: false
});


jsonapi.replaceMiddleware('errors', {
  name: 'nothing-to-see-here',
  error: function (payload) {
    // console.log("errors", payload);

    if (payload.status === 401) {
      Notification.error({
        "title": "Failed",
        "message": payload.data
      });
      unsetToken();
      return;
    }


    for (var i = 0; i < payload.data.errors.length; i++) {
      Notification.error({
        "title": "Failed",
        "message": payload.data.errors[i].title
      })
    }
    return {errors: []}
  }
});


jsonapi.insertMiddlewareBefore("HEADER", {
  name: "Auth Header middleware",
  req: function (req) {
    // console.log("intercept before request");
    jsonapi.headers['Authorization'] = 'Bearer ' + getToken();
    return req
  }
});

jsonapi.insertMiddlewareAfter('response', {
  name: 'track-request',
  req: function (payload) {
    // console.log("request initiate", payload);
    if (payload.config.method !== 'GET' && payload.config.method !== 'OPTIONS') {


      // console.log("Create request complete: ", payload, payload.status / 100);
      if (parseInt(payload.status / 100) === 2) {
        let action = "Created ";

        if (payload.config.method === "DELETE") {
          action = "Deleted "
        } else if (payload.config.method === "PUT" || payload.config.method === "PATCH") {
          action = "Updated "
        }

        Notification.success({
          title: action + payload.config.model
        });
        console.log("return payload from response middleware")
      } else {
        Notification.warn({
          "title": "Unidentified status"
        })
      }
    }
    return payload
  },
  res: function (r) {
    return r
  }
});


jsonapi.insertMiddlewareAfter('response', {
  name: 'success-notification',
  res: function (payload) {
    // console.log("request complete", arguments);
    return payload
  }
});


export default jsonapi
