<template>


  <div class="ui two column grid" style="overflow-y:auto;white-space:nowrap;">


    <div class="four wide column">
      <div class="ui segment top attached">
        <h2>Tables</h2>
      </div>

      <div class="ui segment attached">
        <ul class="ui relaxed list">
          <div class="item" v-for="w in world" v-if="w.is_top_level == '1' && w.is_hidden == '0'">
            <div class="content">
              <a class="header" href="#" style="text-transform: capitalize;"
                 @click.prevent="setTable(w.table_name)">
                {{w.table_name | titleCase}}
              </a>
            </div>
          </div>
        </ul>
      </div>
    </div>


    <div class="twelve column wide" v-if="selectedWorld != null">
      <div class="ui segment top attached">
        <h3>
          {{selectedWorld | titleCase}}
          <el-button @click="newRow()"><span class="fa fa-plus"></span></el-button>
        </h3>
      </div>

      <div class="ui segment column attached" v-if="showAddEdit && selectedRow != null">

        <div class="row">
          <div class="sixteen column">
            <!--{{selectedWorldColumns}}-->
            <model-form @save="saveRow(selectedRow)" @cancel="showAddEdit = false" v-bind:model="selectedRow"
                        v-bind:meta="selectedWorldColumns" ref="modelform"></model-form>
          </div>
        </div>

      </div>

      <div class="ui segment column attached bottom">
        <table-view :finder="finder" ref="tableview" :json-api="jsonApi"
                    :json-api-model-name="selectedWorld"></table-view>
      </div>

    </div>


  </div>
</template>

<script>
  import JsonApi from 'devour-client'
  import {Notification} from 'element-ui';

  window.jsonApi = new JsonApi({
    apiUrl: 'http://localhost:6336/api',
    pluralize: false,
  });
  jsonApi.replaceMiddleware('errors', {
    name: 'nothing-to-see-here',
    error: function (payload) {
      console.log("errors", payload);
      for (var i = 0; i < payload.data.errors.length; i++) {
        Notification.error({
          "title": "Failed",
          "message": payload.data.errors[i].title
        })
      }
      return {errors: []}
    }
  });


  var requests = {};

  jsonApi.insertMiddlewareBefore('response', {
    name: 'track-request',
    req: function (payload) {
      console.log("request initiate", payload);
      if (payload.config.method === 'POST') {
        console.log("Create request complete: ", payload, payload.status / 100);
        if (parseInt(payload.status / 100) == 2) {
          Notification.success({
            title: "Created " + payload.config.model
          })
        } else {
          Notification.warn({
            "title": "Unidentified status"
          })
        }
      }
      return payload
    }
  });


  jsonApi.insertMiddlewareAfter('response', {
    name: 'success-notification',
    res: function (payload) {
      console.log("request complete", arguments);
      return payload
    }
  });


  window.jsonApi.headers['Authorization'] = 'Bearer ' + localStorage.getItem('id_token');


  function getColumnKeys(typeName, callback) {
    jQuery.ajax({
      url: 'http://localhost:6336/jsmodel/' + typeName + ".js",
      headers: {
        "Authorization": "Bearer " + localStorage.getItem("id_token")
      },
      success: function (r, e, s) {
//        console.log("in success", arguments)
        callback(r, e, s);
      },
      error: function (r, e, s) {
        callback(r, e, s)
      },
    })
  }

  function getColumnKeysWithErrorHandleWithThisBuilder(that) {
//    console.log("builder column model getter")
    return function (typeName, callback) {
      return getColumnKeys(typeName, function (a, e, s) {
//        console.log("get column kets respone: ", arguments)
        if (e == "error" && s == "Unauthorized") {
          that.logout();
        } else {
          callback(a, e, s)
        }
      })
    }
  }

  export default {
    name: 'hello',
    filters: {
      titleCase: function (str) {
        return str.replace(/[-_]/g, " ").split(' ')
            .map(w => w[0].toUpperCase() + w.substr(1).toLowerCase())
            .join(' ')
      }
    },
    data () {
      return {
        world: [],
        msg: "message",
        selectedWorld: null,
        filterText: "",
        selectedWorldColumns: [],
        showAddEdit: false,
        tableData: [],
        jsonApi: jsonApi,
        selectedRow: {},
        tableMap: {},
        modelLoader: null,
      }
    },
    methods: {
      deleteRow(row) {
        var that = this;
        console.log("delete row", this.selectedWorld);
        jsonApi.destroy(this.selectedWorld, row["reference_id"]).then(function () {
          that.setTable(that.selectedWorld);
        })
      },
      saveRow(row) {
        console.log("save row", row);
        if (row["reference_id"]) {
          var that = this;
          jsonApi.update(this.selectedWorld, row).then(function () {
            that.setTable(that.selectedWorld);
            that.showAddEdit = false;
          });
        } else {
          var that = this;
          jsonApi.create(this.selectedWorld, row).then(function () {
            that.setTable(that.selectedWorld);
            that.showAddEdit = false;
            that.$refs.tableview.reloadData(that.selectedWorld)
          });
        }
      },
      newRow() {
        console.log("new row", this.selectedWorld);
        this.selectedRow = {};
        this.showAddEdit = true;
      },
      edit(row) {
        console.log("new row", this.selectedWorld);
        this.selectedRow = row;
        this.showAddEdit = true;
      },
      setTable(tableName) {
        var that = this;
        that.selectedWorld = tableName;
        var all = jsonApi.all(tableName);
        that.finder = all.builderStack;
        that.selectedWorldColumns = jsonApi.modelFor(tableName)["attributes"];
        all.builderStack = [];
        if (that.$refs.tableview) {
          that.$refs.tableview.reloadData(tableName)
        }

      },
      logout: function () {
        this.$parent.logout();
      }
    },
    mounted() {
      var that = this;

      that.modelLoader = getColumnKeysWithErrorHandleWithThisBuilder(that);

      that.modelLoader("user", function (columnKeys) {
        jsonApi.define("user", columnKeys);
        that.modelLoader("usergroup", function (columnKeys) {
          jsonApi.define("usergroup", columnKeys);

          that.modelLoader("world", function (columnKeys) {
            jsonApi.define("world", columnKeys);

            jsonApi.findAll('world', {
              page: {number: 1, size: 50},
              include: ['world_column']
            }).then(function (res) {
              for (var t = 0; t < res.length; t++) {


                (function (typeName) {
                  that.modelLoader(typeName, function (model) {
                    console.log("Loaded model", typeName, model);
                    jsonApi.define(typeName, model);
                  })
                })(res[t].table_name)

              }
              that.world = res.sort(function (a, b) {
                if (a.table_name < b.table_name) {
                  return -1;
                } else if (a.table_name > b.table_name) {
                  return 1;
                }
                return 0;
              });
              console.log("got world", res)
            });

          })


        });

      });


    }
  }
</script>
