import jsonApi from "./plugins/jsonapi"
import Vuex from "vuex"
import Vue from "vue"
import worldManager from "./plugins/worldmanager"


Vue.use(Vuex)
console.log("STORE IMPORTED ")

const state = {
  user: null,
  selectedTable: null,
  authToken: null,
  authUser: null,
  selectedSubTable: null,
  selectedAction: null,

  viewMode: 'table',


  selectedTableColumns: [],
  showAddEdit: false,

  tableData: [],
  fileList: [],
  jsonApi: jsonApi,
  selectedRow: null,
  finder: [],
  systemActions: [],
  actionManager: null,
  visibleWorlds: [],
  selectedInstanceReferenceId: null,
  worlds: [],
  selectedInstanceTitle: null,
  subTableColumns: null,
  actions: null,
  selectedInstanceType: null,
};

const actions = {
  LOAD_WORLDS ({commit}) {
    console.log("LOAD_WORLDS request", state.worlds)
    commit("SET_WORLDS", worldManager.getWorlds())
    console.log("SET_WORLD_ACTIONS request", worldManager.getSystemActions())
    commit("SET_WORLD_ACTIONS", worldManager.getSystemActions())
  },
};


const mutations = {
  SET_USER (state, user) {
    state.authUser = user || null
  },
  SET_TOKEN (state, token) {
    window.localStorage.setItem("token", token)
  },
  SET_ACTIONS (state, actions) {
    state.actions = actions;
  },
  SET_WORLDS (state, worlds) {
    state.worlds = worlds;
    state.visibleWorlds = worlds;
  },
  SET_WORLD_ACTIONS (state, actions) {
    state.systemActions = actions;
  },
  SET_SELECTED_TABLE (state, selectedTable) {
    console.log("SET_SELECTED_TABLE", selectedTable)
    state.selectedTable = selectedTable;
  },
  SET_SELECTED_SUB_TABLE (state, selectedSubTable) {
    state.selectedSubTable = selectedSubTable;
  },
  SET_SELECTED_ROW (state, selectedRow) {
    state.selectedRow = selectedRow;
  },
  SET_SUBTABLE_COLUMNS (state, columns) {
    state.subTableColumns = columns;
  },
  SET_SELECTED_TABLE_COLUMNS (state, columns) {
    state.selectedTableColumns = columns;
  },
  SET_SELECTED_ACTION (state, action) {
    state.selectedAction = action
  },
  SET_FINDER (state, finder) {
    console.log("SET_FINDER", finder)
    state.finder = finder;
  },
  SET_SELECTED_INSTANCE_REFERENCE_ID (state, refId) {
    state.selectedInstanceReferenceId = refId
  },
  LOGOUT (state) {
    window.localStorage.clear("token");
    window.localStorage.clear("user");
  }
};

const getters = {
  subTableColumns(state) {
    return state.subTableColumns
  },
  isAuthenticated (state) {
    console.log("check is authenticated: ", window.localStorage.getItem("token"))
    return !!window.localStorage.getItem("token")
  },
  systemActions(state) {
    return state.systemActions;
  },
  authToken (state) {
    return window.localStorage.getItem("token")
  },
  selectedAction (state) {
    return state.selectedAction;
  },
  selectedInstanceReferenceId (state) {
    return state.selectedInstanceReferenceId
  },
  loggedUser (state) {
    return state.authUser
  },
  actions (state) {
    return state.actions;
  },
  selectedTable (state) {
    console.log("get selected table", state.selectedTable)
    return state.selectedTable;
  },
  finder (state) {
    return state.finder
  },
  selectedRow (state) {
    return state.selectedRow;
  },
  selectedTableColumns (state) {
    return state.selectedTableColumns;
  },
  selectedInstanceReferenceId(state) {
    return state.selectedInstanceReferenceId
  },
  selectedSubTable (state) {
    return state.selectedSubTable
  },
  showAddEdit (state) {
    return state.showAddEdit;
  },
  visibleWorlds (state) {
    let filtered = state.worlds.filter(function (w, r) {
      if (!state.selectedInstanceReferenceId) {
        // console.log("No selected item. Return top level tables")
        return w.is_top_level == '1' && w.is_hidden == '0';
      } else {
        console.log("Selected item found. Return child tables")
        const model = jsonApi.modelFor(w.table_name);
        const attrs = model["attributes"];
        const keys = Object.keys(attrs);
        if (keys.indexOf(state.selectedTable + "_id") > -1) {
          return w.is_top_level == '0';
        }
        return false;
      }
    });
    console.log("filtered worlds: ", filtered)

    return filtered;
  },
};

const store = new Vuex.Store({
  state: state,
  mutations: mutations,
  getters: getters,
  actions: actions,
});


export default store
