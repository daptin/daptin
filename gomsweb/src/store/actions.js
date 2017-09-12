export default {
  setQuery({commit}, query) {
    commit("SET_QUERY", query)
  },
  setStreams({commit}, streams) {
    commit("SET_STREAMS", streams)
  },
}
