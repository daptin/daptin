import Vue from "vue";
var jwtDecode = require('jwt-decode');

export function setToken(state, token) {
  state.token = token;
  var decodedAuthToken = jwtDecode(token);
  state.decodedAuthToken = decodedAuthToken;


}

export function setDecodedAuthToken(state, token) {
  state.decodedAuthToken = token;
}

export function setDefaultCloudStore(state, cloudStore) {
  state.defaultCloudStore = cloudStore;
}

export function setSelectedTable(state, tableName) {
  console.log("set selected table", tableName);
  state.selectedTable = tableName;
}

export function clearTablesCache(state) {

}

export function setTables(state, tables) {
  state.tables = {}
  for (var tableName in tables) {
    Vue.set(state.tables, tableName, tables[tableName])
  }
  console.log("Tables set to ", state.tables)
}
