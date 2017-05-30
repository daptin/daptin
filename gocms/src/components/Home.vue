<template>


  <div class="ui three column grid">


    <div class="four wide column">
      <div class="ui segment top attached">
        <h2 v-if="!selectedInstanceReferenceId">Tables</h2>
        <h2 v-if="!selectedSubTable && selectedInstanceReferenceId">{{selectedWorld | titleCase}}</h2>
        <h2 v-if="selectedSubTable">{{selectedWorld | titleCase}}</h2>
      </div>

      <div class="ui segment attached">
        <ul class="ui relaxed list">
          <div class="item" v-for="w in visibleWorlds">
            <div class="content">


              <router-link v-if="!selectedInstanceReferenceId" v-bind:to="w.table_name">{{w.table_name | titleCase}}
              </router-link>

              <router-link v-if="selectedInstanceReferenceId"
                           :to="{ name: 'SubTables', params: { tablename: selectedWorld, refId:selectedInstanceReferenceId, subTable: w.table_name  }}">
                {{w.table_name | titleCase}}
              </router-link>


              <!--<a class="header" href="#" style="text-transform: capitalize;" @click.prevent="setTable(w.table_name)">-->
              <!--{{w.table_name}}</a>-->
            </div>
          </div>
        </ul>
      </div>
    </div>

    <div class="eight wide column" v-if="selectedWorld != null">
      <div class="ui segment attached top grid">

        <div class="four wide column">
          <h2>
            {{selectedSubTable | titleCase}}
            <!--<el-button @click="newRow()"><span class="fa fa-plus"></span></el-button>-->
          </h2>
          <h2 v-if="!selectedSubTable">
            {{selectedWorld | titleCase}}
            <!--<el-button @click="newRow()"><span class="fa fa-plus"></span></el-button>-->
          </h2>

        </div>
        <div class="four wide column right floated" style="text-align: right;">
          <div class="ui icon buttons">
            <el-button class="ui button" @click.prevent="viewMode = 'table'"><i class="fa fa-table"></i></el-button>
            <el-button class="ui button" @click.prevent="viewMode = 'items'"><i class="fa fa-th-large"></i></el-button>
            <el-button class="ui button" @click.prevent="showAddEdit = true"><i class="fa fa-plus"></i></el-button>
          </div>
        </div>
      </div>

      <div class="ui column segment attached bottom" v-if="showAddEdit && selectedRow != null">

        <div class="row">
          <div class="sixteen column">
            <!--{{selectedWorldColumns}}-->

            <model-form @save="saveRow(selectedRow)" v-if="!selectedSubTable" @cancel="showAddEdit = false"
                        v-bind:model="selectedRow"
                        v-bind:meta="selectedWorldColumns" ref="modelform"></model-form>

            <model-form @save="saveRow(selectedRow)" v-if="selectedSubTable" @cancel="showAddEdit = false"
                        v-bind:model="selectedRow"
                        v-bind:meta="subTableColumns" ref="modelform"></model-form>

          </div>


        </div>

      </div>
      <table-view @newRow="newRow()" @editRow="editRow"
                  v-if="viewMode == 'table' && !selectedSubTable" :finder="finder"
                  ref="tableview" :json-api="jsonApi"
                  :json-api-model-name="selectedWorld"></table-view>
      <table-view @newRow="newRow()" @editRow="editRow"
                  v-if="viewMode == 'table' && selectedSubTable" :finder="finder"
                  ref="tableview" :json-api="jsonApi"
                  :json-api-model-name="selectedSubTable"></table-view>


    </div>
    <div class="four wide column" v-if="showAddEdit && selectedRow != null">
      {{selectedRow}}
    </div>


  </div>
</template>

<script>
  import {Notification} from 'element-ui';

  export default {
    name: 'Home',
    filters: {
      titleCase: function (str) {
        if (!str) {
          return str;
        }
        return str.replace(/[-_]/g, " ").split(' ')
            .map(w => w[0].toUpperCase() + w.substr(1).toLowerCase())
            .join(' ')
      }
    },
    props: {
      tablename: {
        type: String,
        default: 'world'
      },
      refId: {
        type: String,
        default: null
      },
      subTable: {
        type: String,
        default: null
      },

    },
    data () {
      return {
        world: [],
        viewMode: 'table',
        msg: "message",
        selectedWorld: null,
        filterText: "",
        selectedWorldColumns: [],
        showAddEdit: false,
        tableData: [],
        jsonApi: jsonApi,
        selectedRow: {},
        selectedInstanceReferenceId: null,
        subTableColumns: null,
        selectedSubTable: null,
        selectedInstanceType: null,
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

        var that = this;

        if (!that.selectedSubTable || !that.selectedInstanceReferenceId) {


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

        } else {

          row[that.selectedWorld + "_id"] = {
            "id": that.selectedInstanceReferenceId,
          };

          console.log("save row", row);
          if (row["reference_id"]) {
            var that = this;
            jsonApi.update(that.selectedSubTable, row).then(function () {
              that.setTable(that.selectedWorld);
              that.showAddEdit = false;
            });
          } else {
            var that = this;
            jsonApi.create(that.selectedSubTable, row).then(function () {
              that.showAddEdit = false;
              that.$refs.tableview.reloadData(that.selectedSubTable)
            });
          }


        }


      },
      newRow() {
        console.log("new row", this.selectedWorld);
        this.selectedRow = {};
        this.showAddEdit = true;
      },
      editRow(row) {
        console.log("new row", this.selectedWorld);
        this.selectedRow = row;
        this.showAddEdit = true;
      },
      setTable(tableName) {

        if (!tableName) {
          tableName = this.selectedWorld;
        }
        var that = this;
        console.log("Set table selected world", tableName)


        that.selectedWorld = tableName;

        var all = {};
        if (!that.selectedSubTable) {
          var all = jsonApi.all(tableName);
        } else {
          var all = jsonApi.one(that.selectedWorld, that.selectedInstanceReferenceId).all(that.selectedSubTable + "_id");
          that.subTableColumns = jsonApi.modelFor(that.selectedSubTable)["attributes"];
          console.log("Set subtable columns: ", that.subTableColumns)
        }


        that.finder = all.builderStack;
        console.log("finder stack for this view table", that.selectedSubTable, that.selectedWorld, that.finder);
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
    computed: {
      visibleWorlds: function () {

        var that = this;

        return this.world.filter(function (w, r) {
          if (!that.selectedInstanceReferenceId) {
            return w.is_top_level == '1' && w.is_hidden == '0';
          } else {
            console.log("check visibility of ", w);
            var model = that.jsonApi.modelFor(w.table_name);
            console.log("model  ", model);
            var attrs = model["attributes"];
            var keys = Object.keys(attrs);
            console.log("keys ", attrs, keys, that.selectedWorld + "_id");
            if (keys.indexOf(that.selectedWorld + "_id") > -1) {
              return w.is_top_level == '0' && w.is_hidden == '0';
            }
            return false;


          }
        });


      }
    },
    mounted() {
      var that = this;
      console.log("Set table", that.$route.params.tablename)


      if (that.$route.params.tablename) {
        var tableName = this.$route.params.tablename;

        that.selectedWorld = tableName;
        var all = jsonApi.all(tableName);
        that.finder = all.builderStack;
        all.builderStack = [];
        that.selectedWorldColumns = jsonApi.modelFor(tableName)["attributes"];
      }

      if (that.$route.params.refId) {
        that.selectedInstanceReferenceId = that.$route.params.refId;
      }


      that.modelLoader = getColumnKeysWithErrorHandleWithThisBuilder(that);

      jsonApi.findAll('world', {
        page: {number: 1, size: 50},
        include: ['world_column']
      }).then(function (res) {

        console.log("worlds ", res)
        that.world = res.sort(function (a, b) {
          if (a.table_name < b.table_name) {
            return -1;
          } else if (a.table_name > b.table_name) {
            return 1;
          }
          return 0;
        });
        console.log("got world", res);


      });


    },
    watch: {
      '$route.params.tablename': function (to, from) {
        console.log("path changed", arguments);
        this.setTable(to);
      },
      '$route.params.refId': function (to, from) {
        var that = this;
        console.log("refId changed", arguments);
        this.selectedInstanceReferenceId = to;
        this.setTable();
      },
      '$route.params.subTable': function (to, from) {
        var that = this;
        console.log("subTable  changed", arguments);
        this.selectedSubTable = to;
        this.setTable();
      }

    }
  }
</script>
