export function loggedIn(state) {
  return !!state.token
}

export function tables(state) {
  console.log("Get tables, ", state.tables)
  return Object.keys(state.tables).filter(function (tableName) {
    return tableName.indexOf("_has_") === -1;
  }).map(e => state.tables[e]).filter(function (e) {
    if (state.showHiddenTables) {
      return true;
    }
    if (e.is_hidden === 0) {
      return false;
    }
  });
}

