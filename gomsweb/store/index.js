export const state = function () {
  return {
    user: null
  }
}
export const mutations = {
  SET_USER (state, user) {
    state.user = user || null
  }
}

export const getters = {
  isAuthenticated (state) {
    return true;
    return !!state.user
  },
  loggedUser (state) {
    return {};
    return state.user
  }
}
