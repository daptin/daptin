export default function () {
  return {
    tables: {},
    token: localStorage.getItem("token"),
    showHiddenTables: true,
    endpoint: window.location.hostname === "site.daptin.com" ? "http://localhost:6336" : window.location.protocol + "//" + window.location.hostname + (window.location.port === "80" ? "" : window.location.port),
  }
}
