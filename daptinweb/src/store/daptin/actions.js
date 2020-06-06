import {DaptinClient} from 'daptin-client';

// const daptinClient = new DaptinClient(window.location.protocol + "//" + window.location.hostname, false, function () {
let endpoint = window.location.hostname === "site.daptin.com" ? "http://localhost:6336" : window.location.protocol + "//" + window.location.hostname + (window.location.port === "80" ? "" : window.location.port);

var daptinClient = new DaptinClient(endpoint, false, {
  getToken: function () {
    return localStorage.getItem("token");
  }
});

daptinClient.worldManager.init();


export function load({commit}) {
  console.log("Load tables");
  daptinClient.worldManager.loadModels().then(function (worlds) {
    console.log("All models loaded", arguments);
    commit('setTables', worlds)
  }).catch(function (e) {
    console.log("Failed to connect to backend", e);
  })
}

export function setToken({commit}) {
  let token = localStorage.getItem("token");
  if (!token) {
    throw "Failed to login";
  }
  commit('setToken', token)
}

export function setSelectedTable({commit}, tableName) {
  commit("setSelectedTable", tableName)
}

export function executeAction({commit}, params) {
  var tableName = params.tableName;
  var actionName = params.actionName;
  return daptinClient.actionManager.doAction(tableName, actionName, params.params);
}

export function updateRow({commit}, row) {
  var tableName = row.tableName;
  delete row.tableName;
  return daptinClient.jsonApi.update(tableName, row)
}

export function loadData({commit}, params) {
  var tableName = params.tableName;
  var params = params.params;
  return daptinClient.jsonApi.findAll(tableName, params);
}

export function getTableSchema({commit}, tableName) {
  return new Promise(function (resolve, reject) {
    daptinClient.worldManager.loadModel(tableName).then(function () {
      resolve(daptinClient.worldManager.getColumnKeys(tableName));
    }).catch(reject)
  })
}

export function loadModel({commit}, tableName) {
  return daptinClient.worldManager.loadModel(tableName);
}

export function refreshTableSchema({commit}, tableName) {
  daptinClient.worldManager.loadModels().then(function (worlds) {
    console.log("All models loaded", arguments);
    commit('setTables', worlds)
  }).catch(function (e) {
    console.log("Failed to connect to backend", e);
  });
  daptinClient.worldManager.refreshWorld(tableName).then(function (e) {

  });
}
