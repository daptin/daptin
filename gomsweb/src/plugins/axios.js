/**
 * Created by artpar on 6/7/17.
 */


import axios from "axios"
import {Notification} from "element-ui"
import {unsetToken, extractInfoFromHash} from "../utils/auth"

// Add a response interceptor
axios.interceptors.response.use(function (response) {
  // Do something with response data
  // console.log("intercept response", response)
  return response;
}, function (error) {
  // Do something with response error
  // console.log("intercept error", error, extractInfoFromHash())
  if (error.response && error.response.status == 403) {
    Notification.error({
      "title": "Unauthorized",
      "message": error.message
    })
  } else if (error.response && error.response.status == 401) {
    unsetToken();
    Notification.error({
      "title": "Unauthorized",
      "message": error.message
    })
  }
  return Promise.reject(error);
});
