import jsonApi from "~/plugins/jsonapi"
import actionManager from "~/plugins/actionmanager"
export const state = function () {
  return {
    user: null,
    selectedTable: null,
    selectedSubTable: null,
    selectedAction: null,

    viewMode: 'table',


    selectedWorldColumns: [],
    selectedSubTableColumns: [],
    selectedTableColumns: [],
    showAddEdit: false,

    tableData: [],
    fileList: [],
    jsonApi: jsonApi,
    selectedRow: null,
    finder: [],
    actionManager: null,
    selectedInstanceReferenceId: null,
    worlds: [],
    selectedInstanceTitle: null,
    subTableColumns: null,
    actions: null,
    selectedInstanceType: null,
  }
};

export const mutations = {
  SET_USER (state, user) {
    state.user = user || null
  },
  SET_ACTIONS (state, actions) {
    state.actions = actions;
  },
  LOAD_WORLDS (state) {
    jsonApi.findAll("world", {
      page: {number: 1, size: 50},
      include: ['world_column']
    }).then(function (r) {
      state.worlds = r;
    })
  },
  SET_SELECTED_TABLE (state, selectedTable) {
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
    state.finder = finder;
  },
  SET_SELECTED_INSTANCE_REFERENCE_ID (state, refId) {
    state.selectedInstanceReferenceId = refId
  }
};

export const getters = {
  isAuthenticated (state) {
    return !!state.user
  },
  selectedAction (state) {
    return state.selectedAction;
  },
  selectedInstanceReferenceId (state) {
    return state.selectedInstanceReferenceId
  },
  loggedUser (state) {
    return state.user
  },
  actions (state) {
    return state.actions;
  },
  selectedTable (state) {
    return state.selectedTable;
  },
  finder (state) {
    return state.finder
  },
  visibleWorlds (state) {
    if (!state.worlds) {
      return [];
    }

    let filtered = state.worlds.filter(function (w, r) {
      if (!state.selectedInstanceReferenceId) {
        // console.log("No selected item. Return top level tables")
        return w.is_top_level === '1' && w.is_hidden == '0';
      } else {
        // console.log("Selected item found. Return child tables")
        const model = jsonApi.modelFor(w.table_name);
        const attrs = model["attributes"];
        const keys = Object.keys(attrs);
        if (keys.indexOf(state.selectedTable + "_id") > -1) {
          return w.is_top_level === '0' && w.is_hidden === '0';
        }
        return false;
      }
    });
    return filtered;
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
  selectedSubTableColumns (state) {
    return state.selectedSubTableColumns;
  },
  showAddEdit (state) {
    return state.showAddEdit;
  }
};
