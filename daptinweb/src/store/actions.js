export default {
  setQuery({commit}, query) {
    commit("SET_QUERY", query)
  },
  setLanguage({commit}, language) {
    commit("SET_LANGUAGE", language)
  },
  setStreams({commit}, streams) {
    commit("SET_STREAMS", streams)
  },
}
