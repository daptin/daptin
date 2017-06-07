<template>


  <div class="ui three column grid">

    <div class="hidden" style="display: none">
      <nuxt-child/>
      <div class="ui modal" id="uploadJson">
        <i class="close icon"></i>
        <div class="header">
          Add site features from json file
        </div>
        <div class="content">
          <div class="description">
            <!--<action-view :action-manager="actionManager" :action="selectedAction"-->
            <!--:json-api="jsonApi" :model="selectedRow"></action-view>-->
          </div>
        </div>
      </div>
    </div>
    <!-- Home -->

    <div class="three wide column">
      {{selectedWorld}} - {{selectedSubTable}} - {{viewMode}}

      <div class="ui two column grid segment top attached">
        <div class="four wide column left floated">
          <h3 v-if="!selectedInstanceReferenceId">Tables</h3>
          <h3 v-if="!selectedSubTable && selectedInstanceReferenceId">
            <router-link :to="{ name: 'tablename', params: { tablename: selectedWorld }}">{{selectedWorld | titleCase}}
            </router-link>
          </h3>
          <h3 v-if="selectedSubTable">
            <router-link :to="{ name: 'tablename', params: { tablename: selectedWorld }}">{{selectedWorld | titleCase}}
            </router-link>
            {{selectedInstanceTitle}}
          </h3>
        </div>

        <div class="four wide column right floated" style="text-align: right">
          <el-dropdown @command="handleCommand">
            <button class="ui icon button el-dropdown-link">
              <i class="setting icon"></i>
            </button>
            <el-dropdown-menu slot="dropdown">
              <el-dropdown-item command="json">Load features from json</el-dropdown-item>
              <el-dropdown-item command="sample">Load features from sample</el-dropdown-item>
              <el-dropdown-item command="restart">Restart</el-dropdown-item>
            </el-dropdown-menu>
          </el-dropdown>

        </div>


      </div>

      <div class="ui segment bottom attached" v-if="visibleWorlds.length > 0">
        <div class="ui secondary vertical pointing menu">
          <template v-for="w in visibleWorlds">


            <nuxt-link v-bind:class="{item: true, active: selectedWorld == w.table_name}"
                       v-if="!selectedInstanceReferenceId"
                       v-bind:to="{name: 'tablename', params: {tablename: w.table_name}}">
              {{w.table_name | titleCase}} - {{w.table_name}}
            </nuxt-link>

            <nuxt-link v-bind:class="{item: true, active: selectedWorld == w.table_name}"
                       v-if="selectedInstanceReferenceId"
                       :to="{ name: 'tablename-refId-subTable', params: { tablename: selectedWorld, refId:selectedInstanceReferenceId, subTable: w.table_name  }}">
              {{w.table_name | titleCase}} - {{selectedWorld}}
            </nuxt-link>

          </template>
        </div>
      </div>


    </div>

    <div class="thirteen wide column" v-if="selectedRow != null && selectedRow['id']">

      <div class="ui segment" v-if="selectedAction != null">
        <action-view @cancel="selectedAction = null" :action-manager="actionmanager" :action="selectedAction"
                     :json-api="jsonApi" :model="selectedRow"></action-view>
      </div>

      <div class="ui segment" v-if="selectedRow != null">
        <h2>{{selectedRow | chooseTitle | titleCase}}</h2>
      </div>

      <div class="ui segment" v-if="actions != null">
        <ul class="ui relaxed list">
          <div class="item" v-for="a, k in actions">
            <el-button @click="selectedAction = a">{{a.label}}</el-button>
          </div>
        </ul>
      </div>


      <detailed-table-row :model="selectedRow" :json-api="jsonApi"
                          :json-api-model-name="selectedWorld"></detailed-table-row>


    </div>

    <div class="thirteen wide column right floated" v-if="selectedWorld != null">
      <div class="ui segment attached top grid">

        <div class="four wide column left floated">
          <h2 v-if="selectedSubTable">
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
            <el-button class="ui button" @click.prevent="newRow()"><i class="fa fa-plus"></i></el-button>
          </div>
        </div>
      </div>

      <div class="ui column segment attached bottom" v-if="showAddEdit && selectedRow != null">

        <div class="row">
          <div class="sixteen column">
            <!--{{selectedWorldColumns}}-->

            <model-form @save="saveRow(selectedRow)" :json-api="jsonApi" v-if="!selectedSubTable"
                        @cancel="showAddEdit = false"
                        v-bind:model="selectedRow"
                        v-bind:meta="selectedWorldColumns" ref="modelform"></model-form>

            <model-form @save="saveRow(selectedRow)" :json-api="jsonApi" v-if="selectedSubTable"
                        @cancel="showAddEdit = false"
                        v-bind:model="selectedRow"
                        v-bind:meta="subTableColumns" ref="modelform"></model-form>

          </div>


        </div>

      </div>
      <table-view @newRow="newRow()" @editRow="editRow"
                  v-if="viewMode == 'table' && !selectedSubTable" :finder="finder"
                  ref="tableview1" :json-api="jsonApi"
                  :json-api-model-name="selectedWorld"></table-view>

      {{selectedSubTable}} - {{selectedSubTable}}
      <table-view @newRow="newRow()" @editRow="editRow"
                  v-if="viewMode == 'table' && selectedSubTable" :finder="finder"
                  ref="tableview2" :json-api="jsonApi"
                  :json-api-model-name="selectedSubTable"></table-view>


    </div>

  </div>
</template>

<script>
  import {mapGetters} from 'vuex'
  import worldManager from "~/plugins/worldmanager"
  import jsonApi from "~/plugins/jsonapi"
  import actionManager from "~/plugins/actionmanager"
  import Notification from "element-ui"

  export default {
    middleware: 'authenticated',
    components: [
      actionManager,
    ],
    filters: {
      titleCase: function (str) {
        if (!str || str.length < 2) {
          return str;
        }
        return str.replace(/[-_]+/g, " ").trim().split(' ')
          .map(w => (w[0] ? w[0].toUpperCase() : "") + w.substr(1).toLowerCase()).join(' ')
      },
      chooseTitle: function (obj) {
        var keys = Object.keys(obj);
        for (var i = 0; i < keys.length; i++) {
          console.log("check key", keys[i],);
          if (keys[i].indexOf("name") > -1 && typeof obj[keys[i]] == "string" && obj[keys[i]].length > 0) {
            console.log("title value", keys[i], obj[keys[i]], typeof obj[keys[i]]);
            return obj[keys[i]];
          }
        }
        console.log("title value", "Reference id", obj);
        return obj["type"] + " #" + obj["id"];

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
        selectedAction: null,
        jsonUploadAction: null,
        filterText: "",
        selectedWorldColumns: [],
        showAddEdit: false,
        tableData: [],
        fileList: [],
        jsonApi: jsonApi,
        selectedRow: null,
        finder: [],
        actionManager: actionManager,
        selectedInstanceReferenceId: null,
        selectedInstanceTitle: null,
        subTableColumns: null,
        actions: null,
        selectedSubTable: null,
        selectedInstanceType: null,
        tableMap: {},
        modelLoader: null,
      }
    },
    methods: {
      uploadJsonSchemaFile(){
        console.log("this files list", this.$refs.upload)
      },
      handleCommand(command) {
        console.log(command);
        if (command === "json") {
          document.getElementById('uploadJson').modal('show');
        }
      },
      getCurrentTableType() {
        const that = this;
        if (!that.selectedSubTable || !that.selectedInstanceReferenceId) {
          return that.selectedWorld;
        }

        return that.selectedSubTable;

      },
      deleteRow(row) {
        const that = this;
        console.log("delete row", this.getCurrentTableType());

        jsonApi.destroy(this.getCurrentTableType(), row["reference_id"]).then(function () {
          that.setTable();
        })
      },
      saveRow(row) {

        let that = this;

        const currentTableType = this.getCurrentTableType();

        if (that.selectedSubTable && that.selectedInstanceReferenceId) {
          row[that.selectedWorld + "_id"] = {
            "id": that.selectedInstanceReferenceId,
          };
        }


        console.log("save row", row);
        if (row["id"]) {
          that = this;
          jsonApi.update(currentTableType, row).then(function () {
            that.setTable();
            that.showAddEdit = false;
          });
        } else {
          that = this;
          jsonApi.create(currentTableType, row).then(function () {
            console.log("create complete", arguments);
            that.setTable();
            that.showAddEdit = false;
            that.$refs.tableview1.reloadData(currentTableType);
            that.$refs.tableview2.reloadData(currentTableType)
          }, function (r) {
            console.error(r)
          });
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
          console.log("no table name in argument, getting from present")
          tableName = this.getCurrentTableType();
        }
        const that = this;
        console.log("Set table selected world :: ", tableName);

        let all = {};
        if (!that.selectedSubTable) {
          all = jsonApi.all(tableName);
        } else {
          all = jsonApi.one(that.selectedWorld, that.selectedInstanceReferenceId).all(that.selectedSubTable + "_id");
          worldManager.getColumnKeys(that.selectedSubTable, function (r) {
            console.log("Set selected sub table columns", r.ColumnModel);
            that.subTableColumns = r.ColumnModel;
          });
          console.log("Set subtable columns: ", that.subTableColumns)
        }


        that.finder = all.builderStack;
        console.log("finder stack for this view table", that.selectedSubTable, that.selectedWorld, that.finder);
        worldManager.getColumnKeys(tableName, function (model) {
          console.log("Set selected world columns", model.ColumnModel);
          that.selectedWorldColumns = model.ColumnModel
        });


        that.actions = that.actionManager.getActions(that.selectedWorld);

        all.builderStack = [];
        if (that.$refs.tableview1) {
          console.log("reload data for ", tableName);
          that.$refs.tableview1.reloadData(tableName)
        }
        if (that.$refs.tableview2) {
          that.$refs.tableview2.reloadData(tableName)
        }

      },
      logout: function () {
        this.$parent.logout();
      }
    },
    computed: {
      visibleWorlds: function () {
        console.log("get visible worlds", this.world, "0");
        var that = this;

        let filtered = this.world.filter(function (w, r) {
          if (!that.selectedInstanceReferenceId) {
            return w.is_top_level === '1' && w.is_hidden == '0';
          } else {
            console.log("check visibility of ", w);
            var model = that.jsonApi.modelFor(w.table_name);
            console.log("model  ", model);
            var attrs = model["attributes"];
            var keys = Object.keys(attrs);
            console.log("keys ", attrs, keys, that.selectedWorld + "_id");
            if (keys.indexOf(that.selectedWorld + "_id") > -1) {
              return w.is_top_level == '0' && w.is_hidden === '0';
            }
            return false;


          }
        });
        console.log("Filtered visible worlds", filtered)
        return filtered;
      },
      ...mapGetters([
        'isAuthenticated',
        'loggedUser'
      ]),
    },
    created() {
      console.log("created ")
    },
    mounted() {
      const that = this;
      console.log("Set table", that.$route.params.tablename);
      const worldActions = that.actionManager.getActions("world");
      console.log("world actions", worldActions);
      console.log("that is a ", that.$route);
      console.log("this is a ", this);
      if (that.$route.params.tablename) {
        const tableName = that.$route.params.tablename;

//        console.log("World Selected change", tableName);
//        that.selectedWorld = tableName;
        const all = jsonApi.all(tableName);
        that.finder = all.builderStack;
        all.builderStack = [];
        worldManager.getColumnKeys(tableName, function (model) {
          console.log("Set selected world columns", model.ColumnModel);
          that.selectedWorldColumns = model.ColumnModel
        });
      }

      if (that.$route.params.refId) {
        that.selectedInstanceReferenceId = that.$route.params.refId;
      }


      that.modelLoader = worldManager.getColumnKeysWithErrorHandleWithThisBuilder(that);

      jsonApi.findAll('world', {
        page: {number: 1, size: 50},
        include: ['world_column']
      }).then(function (res) {

//          console.log("worlds ", res);
        that.world = res.sort(function (a, b) {
          if (a.table_name < b.table_name) {
            return -1;
          } else if (a.table_name > b.table_name) {
            return 1;
          }
          return 0;
        });
//          console.log("got world", res);


      });


    },
    watch: {
      'selectedWorld': function (to, from) {
        console.log("World Selected changed", from, " => ", to)
      },
      '$route.params.tablename': function (to, from) {
        console.log("Watch $route.params.tablename")
        console.log("World Selected change", to);
        this.selectedWorld = to;
        this.selectedSubTable = null;
        this.selectedRow = null;
        this.showAddEdit = false;
        this.setTable(to);
      },
//      '$route.params.refId': function (to, from) {
//        console.log("Watch $route.params.refId")
//        let that = this;
//        console.log("refId changed", arguments);
//        this.showAddEdit = false;
//        this.selectedInstanceReferenceId = to;
//        that = this;
//        if (!to) {
//          this.selectedRow = null;
//        } else {
//          this.jsonApi.one(this.selectedWorld, to).get().then(function (r) {
//            console.log("selected world instance", r);
//            that.selectedRow = r;
//          });
//        }
//        this.setTable();
//      },
//      '$route.params.subTable': function (to, from) {
//        console.log("Watch $route.params.subTable")
//        var that = this;
//        this.showAddEdit = false;
//        console.log("World Sub change", to);
//        console.log("World Selected", to);
//        this.selectedSubTable = to;
//        this.setTable();
//      }
    }
  }
</script>
