var jwtDecode = require('jwt-decode');

export default function () {
  return {
    tables: {},
    token: localStorage.getItem("token"),
    decodedAuthToken: localStorage.getItem("token") == null ? null : jwtDecode(localStorage.getItem("token")),
    defaultCloudStore: null,
    showHiddenTables: true,
    endpoint: window.location.hostname === "site.daptin.com" && window.location.port == "8080" ? "http://localhost:6336" : window.location.protocol + "//" + window.location.hostname + (window.location.port === "80" ? "" : ':' + window.location.port),
  }
}
