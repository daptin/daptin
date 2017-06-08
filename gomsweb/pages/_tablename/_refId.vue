<template>


  <div class="ui three column grid">


    <!-- Home -->

    <div class="three wide column">
      <div class="ui modal" id="uploadJson">
        <i class="close icon"></i>
        <div class="header">
          Add site features from json file
        </div>
        <div class="content">
          <div class="description">
            <!--<action-view :action-manager="actionManager" :action="selectedAction"-->
            <!--:json-api="jsonApi" :model="$store.getters.selectedRow"></action-view>-->
          </div>
        </div>
      </div>

      <div class="ui two column grid segment top attached">
        <div class="four wide column left floated">
          <h2 v-if="!$store.selectedInstanceReferenceId">Tables</h2>
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


        <h2 v-if="!$store.getters.selectedSubTable && $store.selectedInstanceReferenceId">
          <router-link :to="{ name: 'Home', params: { tablename: $store.getters.selectedTable }}">
            {{$store.getters.selectedTable | titleCase}}
          </router-link>
        </h2>
        <h2 v-if="$store.getters.selectedSubTable">
          <router-link :to="{ name: 'Home', params: { tablename: $store.getters.selectedTable }}">
            {{$store.getters.selectedTable | titleCase}}
          </router-link>
          {{selectedInstanceTitle}}
        </h2>
      </div>

      <div class="ui segment bottom attached" v-if="$store.getters.visibleWorlds.length > 0">
        <div class="ui secondary vertical pointing menu">
          <template v-for="w in $store.getters.visibleWorlds">


            <router-link v-bind:class="{item: true, active: $store.getters.selectedTable == w.table_name}"
                         v-if="!$store.selectedInstanceReferenceId" v-bind:to="w.table_name">
              {{w.table_name | titleCase}}
            </router-link>

            <router-link v-bind:class="{item: true, active: $store.getters.selectedTable == w.table_name}"
                         v-if="$store.selectedInstanceReferenceId"
                         :to="{ name: 'SubTables', params: { tablename: $store.getters.selectedTable, refId:$store.selectedInstanceReferenceId, subTable: w.table_name  }}">
              {{w.table_name | titleCase}}
            </router-link>

          </template>
        </div>
      </div>


    </div>

    <div class="thirteen wide column" v-if="$store.getters.selectedRow != null && $store.getters.selectedRow['id']">

      <div class="ui segment" v-if="selectedAction != null">
        <action-view @cancel="selectedAction = null" :action-manager="actionManager" :action="selectedAction"
                     :json-api="jsonApi" :model="$store.getters.selectedRow"></action-view>
      </div>

      <div class="ui segment" v-if="$store.getters.selectedRow != null">
        <h2>{{$store.getters.selectedRow | chooseTitle | titleCase}}</h2>
      </div>

      <div class="ui segment" v-if="actions != null">
        <ul class="ui relaxed list">
          <div class="item" v-for="a, k in actions">
            <el-button @click="$store.getters.selectedAction = a">{{a.label}}</el-button>
          </div>
        </ul>
      </div>


      <detailed-table-row :model="$store.getters.selectedRow" :json-api="jsonApi"
                          :json-api-model-name="$store.getters.selectedTable"></detailed-table-row>


    </div>
    <div class="thirteen wide column right floated">
      <div class="ui segment attached top grid">

        <div class="four wide column left floated">
          <h2 v-if="$store.getters.selectedSubTable">
            {{$store.getters.selectedSubTable | titleCase}}
            <!--<el-button @click="newRow()"><span class="fa fa-plus"></span></el-button>-->
          </h2>
          <h2 v-if="!$store.getters.selectedSubTable">
            {{$store.getters.selectedTable | titleCase}}
            <!--<el-button @click="newRow()"><span class="fa fa-plus"></span></el-button>-->
          </h2>

        </div>
        <div class="four wide column right floated" style="text-align: right;">
          <div class="ui icon buttons">
            <el-button class="ui button" @click.prevent="$store.viewMode = 'table'"><i class="fa fa-table"></i>
            </el-button>
            <el-button class="ui button" @click.prevent="$store.viewMode = 'items'"><i class="fa fa-th-large"></i>
            </el-button>
            <el-button class="ui button" @click.prevent="newRow()"><i class="fa fa-plus"></i></el-button>
          </div>
        </div>
      </div>

      <div class="ui column segment attached bottom" v-if="showAddEdit && $store.getters.selectedRow != null">

        <div class="row">
          <div class="sixteen column">
            <!--{{selectedTableColumns}}-->

            <model-form @save="saveRow($store.getters.selectedRow)" :json-api="jsonApi"
                        v-if="!$store.getters.selectedSubTable"
                        @cancel="showAddEdit = false"
                        v-bind:model="$store.getters.selectedRow"
                        v-bind:meta="selectedTableColumns" ref="modelform"></model-form>

            <model-form @save="saveRow($store.getters.selectedRow)" :json-api="jsonApi"
                        v-if="$store.getters.selectedSubTable"
                        @cancel="showAddEdit = false"
                        v-bind:model="$store.getters.selectedRow"
                        v-bind:meta="subTableColumns" ref="modelform"></model-form>

          </div>


        </div>

      </div>
      <table-view @newRow="newRow()" @editRow="editRow"
                  :finder="$store.getters.finder"
                  ref="tableview1" :json-api="jsonApi"
                  :json-api-model-name="$store.getters.selectedTable"></table-view>

      <table-view @newRow="newRow()" @editRow="editRow"
                  v-if="$store.viewMode == 'table' && selectedSubTable" :finder="finder"
                  ref="tableview2" :json-api="jsonApi"
                  :json-api-model-name="selectedSubTable"></table-view>


    </div>

  </div>
</template>

<script>
  import {Notification} from 'element-ui';
  import worldManager from "~/plugins/worldmanager"
  import jsonApi from "~/plugins/jsonapi"
  import actionManager from "~/plugins/actionmanager"
  import {mapGetters} from 'vuex'

  export default {
    name: 'Home',
    filters: {
      titleCase: function (str) {
//        console.log("ttilec ase", str)
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
        jsonApi: jsonApi,
        showAddEdit: false,
      }
    },
    methods: {
      uploadJsonSchemaFile(){
        console.log("this files list", this.$refs.upload)
      },
      handleCommand(command) {
        console.log(command);
        if (command === "json") {
          jQuery('#uploadJson').modal('show');
        }
      },
      getCurrentTableType() {
        var that = this;
        if (!that.$store.selectedSubTable || !that.$store.selectedInstanceReferenceId) {
          return that.selectedTable;
        }

        return that.selectedSubTable;

      },
      deleteRow(row) {
        var that = this;
        console.log("delete row", this.getCurrentTableType());

        jsonApi.destroy(this.getCurrentTableType(), row["reference_id"]).then(function () {
          that.setTable();
        })
      },
      saveRow(row) {

        var that = this;

        var currentTableType = this.getCurrentTableType();

        if (that.selectedSubTable && that.selectedInstanceReferenceId) {
          row[that.selectedWorld + "_id"] = {
            "id": that.selectedInstanceReferenceId,
          };
        }


        console.log("save row", row);
        if (row["id"]) {
          var that = this;
          jsonApi.update(currentTableType, row).then(function () {
            that.setTable();
            that.showAddEdit = false;
          });
        } else {
          var that = this;
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
      setTable() {
        const that = this;
        var tableName;

        let all = {};

        if (!that.$store.getters.selectedSubTable) {
          all = jsonApi.all(that.$store.getters.selectedTable);
          tableName = that.$store.getters.selectedTable;
        } else {
          tableName = that.$store.getters.selectedSubTable;
          all = jsonApi.one(that.$store.getters.selectedWorld, that.$store.getters.selectedInstanceReferenceId).all(that.$store.getters.selectedSubTable + "_id");
          worldManager.getColumnKeys(that.$store.getters.selectedSubTable, function (r) {
            console.log("Set selected sub table columns", r.ColumnModel);
            that.$store.commit("SET_SUBTABLE_COLUMNS", r.ColumnModel)
          });
          console.log("Set subtable columns: ", that.$store.getters.subTableColumns)
        }

        that.$store.commit("SET_FINDER", all.builderStack);


        console.log("Selected sub table", that.$store.getters.selectedSubTable);
        console.log("Selected table", that.$store.getters.selectedTable);

        worldManager.getColumnKeys(tableName, function (model) {
          console.log("Set selected world columns", model.ColumnModel);
          that.$store.commit("SET_SELECTED_TABLE_COLUMNS", model.ColumnModel)
        });

        that.$store.commit("SET_ACTIONS", actionManager.getActions(that.selectedWorld));

        all.builderStack = [];


        console.log("reload data table", tableName)
        if (!that.$store.getters.selectedSubTable) {
          console.log("reload data for selected table", tableName);
          that.$refs.tableview1.reloadData(tableName)
        } else if (that.$store.getters.selectedSubTable) {
          console.log("reload data for selected sub table", tableName);
          that.$refs.tableview2.reloadData(tableName)
        } else {
//          console.error("no table is active")
        }

      },
      logout: function () {
        this.$parent.logout();
      }
    },

    mounted() {
      console.log("Enter tablename-refId", that.$route.params.tablename, that.$route.params.refId)
      var that = this;
      that.$store.commit("LOAD_WORLDS");
      this.$store.commit("SET_SELECTED_TABLE", that.$route.params.tablename)
      that.$store.commit("SET_SELECTED_INSTANCE_REFERENCE_ID", that.$route.params.refId)
      that.actionManager = actionManager;
      var worldActions = actionManager.getActions("world");

      console.log("Set table 1", that.$route.params.tablename);

      if (that.$route.params.tablename) {
        var tableName = this.$route.params.tablename;

        worldManager.getColumnKeys(tableName, function (model) {
          console.log("Set selected world columns", model.ColumnModel);
//          that.selectedWorldColumns = model.ColumnModel
          that.$store.commit("SET_SELECTED_TABLE_COLUMNS", model.ColumnModel)
        });
      }

      that.setTable();


    },
    watch: {
      '$route.params.tablename': function (to, from) {
        console.log("path changed", arguments);
        this.$store.commit("SET_SELECTED_TABLE", to)
        this.showAddEdit = false;
        this.setTable();
      },
      '$route.params.refId': function (to, from) {
        var that = this;
        console.log("refId changed", arguments);
        this.showAddEdit = false;
        this.selectedInstanceReferenceId = to;
        var that = this;
        if (!to) {
          this.$store.commit("SET_SELECTED_ROW", null)
        } else {
          jsonApi.one(this.selectedWorld, to).get().then(function (r) {
            console.log("selected world instance", r);
            that.$store.commit("SET_SELECTED_ROW", r)
            that.$store.commit("SET_SELECTED_ROW", r["id"])
            that.selectedRow = r;
          });
        }
        this.setTable();
      },
      '$route.params.subTable': function (to, from) {
        var that = this;
        this.showAddEdit = false;
        console.log("subTable  changed", arguments);
        this.selectedSubTable = to;
        this.setTable();
      }
    }
  }
</script>
