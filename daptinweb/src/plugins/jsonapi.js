import JsonApi from "devour-client"
import {Notification} from "element-ui"
import appConfig from "../plugins/appconfig"
import {getToken, unsetToken} from '../utils/auth'
import {mapState} from 'vuex';


const jsonapi = new JsonApi({
  apiUrl: appConfig.apiRoot + '/api',
  pluralize: false,
  logger: false
});


jsonapi.replaceMiddleware('errors', {
  name: 'nothing-to-see-here',
  error: function (response) {
    console.log("errors", response);
    response = response.response.data.errors[0];
    if (response.status === 401) {
      Notification.error({
        "title": "Failed",
        "message": response.data
      });
      unsetToken();
      return;
    }

    if (response.status == 400 || response.status == 500) {
      Notification.error({
        "title": "Failed",
        "message": response.title
      });
      return {};
    }

    if (response.data && !response.data.errors) {
      Notification.error({
        "title": "Warn",
        "message": "Massive"
      });
      console.log("we dont know about this entity");
      return {};
    }


    for (var i = 0; i < response.data.errors.length; i++) {
      Notification.error({
        "title": "Failed",
        "message": response.data.errors[i].title
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

jsonapi.insertMiddlewareBefore('HEADER', {
  name: 'insert-query',
  req: function (payload) {

    let lang = localStorage.getItem("LANGUAGE");
    payload.req.headers["Accept-Language"] = lang;


    if (payload.req.method.toLowerCase() !== "get") {
      return payload;
    }

    var query = $("#navbar-search-input").val();

    if (query && query.length > 2) {
      console.log("change payload for query", query);
      payload.req.params.filter = encodeURIComponent(query);
    }
    return payload;
  }
});
jsonapi.insertMiddlewareAfter('response', {
  name: 'track-request',
  req: function (payload) {
    // console.log("request initiate", payload);
    let requestMethod = payload.config.method.toUpperCase();
    if (requestMethod !== 'GET' && requestMethod !== 'OPTIONS') {

      // console.log("Create request complete: ", payload, payload.status / 100);
      if (parseInt(payload.status / 100) === 2) {
        let action = "Created ";

        if (requestMethod === "DELETE") {
          action = "Deleted "
        } else if (requestMethod === "PUT" || requestMethod === "PATCH") {
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
