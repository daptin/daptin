



export function loggedIn(state) {
  let token = state.token;
  return !!token
}

export function endpoint(state) {
  return state.endpoint;
}

export function authToken(state) {
  return state.token
}
export function hideNavigationDrawer(state) {
  return state.hideNavigationDrawer
}

export function decodedAuthToken(state) {
  if (state.decodedAuthToken) {
    return state.decodedAuthToken
  }
  return state.decodedAuthToken
}

export function tables(state) {
  console.log("Get tables, ", state.tables);
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

