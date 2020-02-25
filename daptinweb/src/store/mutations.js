export default {
  TOGGLE_LOADING(state) {
    state.callingAPI = !state.callingAPI
  },
  TOGGLE_SEARCHING(state) {
    state.searching = (state.searching === '') ? 'loading' : ''
  },
  SET_USER(state, user) {
    state.user = user
  },
  SET_LANGUAGE(state, language) {
    console.log("set language")
    localStorage.setItem("LANGUAGE", language)
    state.language = language;
  },
  SET_LAST_URL(state, route) {
    if (route) {
      window.localStorage.setItem("last_route", JSON.stringify(route));
    } else {
      window.localStorage.removeItem("last_route");
    }
  },
  SET_TOKEN(state, token) {
    window.localStorage.setItem("token", token)
  },
  SET_ACTIONS(state, actions) {
    state.actions = actions;
  },
  SET_WORLDS(state, worlds) {
    console.log("\t\t\tSet worlds: ", worlds)
    state.worlds = worlds;
    state.visibleWorlds = worlds;
  },
  SET_WORLD_ACTIONS(state, actions) {
    state.systemActions = actions;
  },
  SET_SELECTED_TABLE(state, selectedTable) {
    console.log("SET_SELECTED_TABLE", selectedTable);
    state.selectedTable = selectedTable;
  },
  SET_STREAMS(state, streams) {
    state.streams = streams;
  },
  SET_SELECTED_SUB_TABLE(state, selectedSubTable) {
    state.selectedSubTable = selectedSubTable;
  },
  SET_QUERY(state, query) {
    state.query = query;
  },
  SET_SELECTED_ROW(state, selectedRow) {
    state.selectedRow = selectedRow;
  },
  SET_SUBTABLE_COLUMNS(state, columns) {
    state.subTableColumns = columns;
  },
  SET_SELECTED_TABLE_COLUMNS(state, columns) {
    state.selectedTableColumns = columns;
  },
  SET_SELECTED_ACTION(state, action) {
    state.selectedAction = action
  },
  SET_FINDER(state, finder) {
    console.log("SET_FINDER", finder);
    state.finder = finder;
  },
  SET_SELECTED_INSTANCE_REFERENCE_ID(state, refId) {
    state.selectedInstanceReferenceId = refId
  },
  LOGOUT(state) {
    window.localStorage.clear("token");
    window.localStorage.clear("user");
  }
}
