import jsonApi from  "~/plugins/jsonapi"

export const getters = {
  worlds () {
    return jsonApi.findAll("world", {
      page: {number: 1, size: 50},
      include: ['world_column']
    })
  }
};
