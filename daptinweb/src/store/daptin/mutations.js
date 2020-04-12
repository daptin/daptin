import Vue from "vue";

export function setToken(state, token) {
  state.token = token;
}

export function setTables(state, tables) {
  for (var tableName in tables) {
    Vue.set(state.tables, tableName, tables[tableName])
  }
  console.log("Tables set to ", state.tables)
}
