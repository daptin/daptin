<template>


  <div class="ui three column grid">

    <!--<div class="row">-->
      <!--<div class="ui modal" id="uploadJson" v-if="selectedWorldAction">-->
        <!--<i class="close icon"></i>-->
        <!--<div class="header">-->
          <!--{{selectedWorldAction.label}}-->
        <!--</div>-->
        <!--<div class="content">-->
          <!--<div class="description">-->
            <!--<action-view ref="systemActionView" :hide-title="true" @cancel="hideModel" :action-manager="actionManager"-->
                         <!--:action="selectedWorldAction"-->
                         <!--:json-api="jsonApi"></action-view>-->
          <!--</div>-->
        <!--</div>-->
      <!--</div>-->
    <!--</div>-->

    <!-- Home -->

    <div class="three wide column">

      <div class="ui two column grid segment top attached">
        <div class="four wide column left floated">
          <h2 v-if="!selectedInstanceReferenceId">
            {{selectedTable | titleCase}}
          </h2>
          <h2 v-if="!selectedSubTable && selectedInstanceReferenceId">
            <router-link :to="{ name: 'tablename', params: { tablename: selectedTable }}">
              {{selectedTable | titleCase}}
            </router-link>
          </h2>

          <h2 v-if="selectedSubTable">
            <router-link :to="{ name: 'tablename', params: { tablename: selectedTable }}">
              {{selectedTable | titleCase}}
            </router-link>

          </h2>
        </div>

        <div class="four wide column right floated" style="text-align: right">

          <el-dropdown trigger="click" @command="handleCommand">
            <button class="ui icon button el-dropdown-link">
              <i class="setting icon"></i>
            </button>
            <el-dropdown-menu slot="dropdown">
              <el-dropdown-item :command="action.name" v-for="action in systemActions">{{action.label}}
              </el-dropdown-item>
            </el-dropdown-menu>
          </el-dropdown>


        </div>
      </div>
      <div class="ui segment bottom attached"
           v-if="visibleWorlds && visibleWorlds.length > 0">
        <div class="ui secondary vertical pointing menu">
          <template v-for="w in visibleWorlds">
            <router-link v-bind:class="{item: true, active: selectedTable == w.table_name}"
                         v-if="!selectedInstanceReferenceId"
                         v-bind:to="{name: 'tablename', params: {tablename: w.table_name}}">
              {{w.table_name | titleCase}}
            </router-link>

            <router-link v-bind:class="{item: true, active: selectedTable == w.table_name}"
                         v-if="selectedInstanceReferenceId"
                         :to="{ name: 'tablename-refId-subTable', params: { tablename: selectedTable, refId:selectedInstanceReferenceId, subTable: w.table_name  }}">
              {{w.table_name | titleCase}}
            </router-link>

          </template>
        </div>
      </div>


    </div>

    <div class="thirteen wide column" v-if="selectedRow != null && selectedRow['id'] && !selectedSubTable">

      <div class="ui segment" v-if="selectedAction != null && showAddEdit">
        <action-view @cancel="showAddEdit = false" :action-manager="actionManager"
                     :action="selectedAction"
                     :json-api="jsonApi" :model="selectedRow"></action-view>
      </div>

      <div class="ui segment" v-if="electedRow != null">
        <h2>{{selectedRow | chooseTitle | titleCase}}</h2>
      </div>

      <div class="ui segment" v-if="actions != null">
        <ul class="ui column grid">
          <div class="ui three wide column" v-for="a, k in actions">
            <el-button @click="doAction(a)">{{a.label}}</el-button>
          </div>
        </ul>
      </div>


      <detailed-table-row :model="selectedRow" :json-api="jsonApi"
                          :json-api-model-name="selectedTable"></detailed-table-row>


    </div>


    <div class="thirteen wide column right floated">
      <div class="ui segment attached top grid">

        <div class="four wide column left floated">
          <h2 v-if="selectedSubTable">
            {{selectedSubTable | titleCase}}
            <!--<el-button @click="newRow()"><span class="fa fa-plus"></span></el-button>-->
          </h2>
          <h2 v-if="!selectedSubTable">
            {{selectedTable | titleCase}}
            <!--<el-button @click="newRow()"><span class="fa fa-plus"></span></el-button>-->
          </h2>

        </div>
        <div class="four wide column right floated" style="text-align: right;">
          <div class="ui icon buttons">
            <el-button class="ui button" @click.prevent="viewMode = 'table'"><i class="fa fa-table blue "></i>
            </el-button>
            <el-button class="ui button" @click.prevent="viewMode = 'items'"><i class="fa fa-th-large blue"></i>
            </el-button>
            <el-button class="ui button" @click.prevent="newRow()"><i class="fa fa-plus green "></i></el-button>
            <el-button class="ui button" @click.prevent="reloadData()"><i class="fa fa-refresh orange"></i></el-button>
          </div>
        </div>
      </div>

      <div class="ui column segment attached bottom" v-if="showAddEdit && rowBeingEdited != null">

        <div class="row">
          <div class="sixteen column">
            <!--{{selectedTableColumns}}-->

            <model-form @save="saveRow(rowBeingEdited)" :json-api="jsonApi"
                        v-if="!selectedSubTable"
                        @cancel="showAddEdit = false"
                        v-bind:model="rowBeingEdited"
                        v-bind:meta="selectedTableColumns" ref="modelform"></model-form>

            <model-form @save="saveRow(rowBeingEdited)" :json-api="jsonApi"
                        v-if="selectedSubTable"
                        @cancel="showAddEdit = false"
                        v-bind:model="rowBeingEdited"
                        v-bind:meta="subTableColumns" ref="modelform"></model-form>

          </div>


        </div>

      </div>

      <table-view @newRow="newRow()" @editRow="editRow"
                  :finder="finder" v-if="!selectedSubTable && selectedTable"
                  ref="tableview1" :json-api="jsonApi"
                  :json-api-model-name="selectedTable"></table-view>
      <table-view @newRow="newRow()" @editRow="editRow"
                  v-if="selectedSubTable" :finder="finder"
                  ref="tableview2" :json-api="jsonApi"
                  :json-api-model-name="selectedSubTable"></table-view>


    </div>

  </div>
</template>

<script>
  import {Notification} from 'element-ui';
  import worldManager from "../plugins/worldmanager"
  import jsonApi from "../plugins/jsonapi"
  import actionManager from "../plugins/actionmanager"
  import {mapGetters} from 'vuex'
  import {mapState} from 'vuex'


  export default {
    name: 'AdminView',
    filters: {
      titleCase: function (str) {
//        console.log("TitleCase  : ", str)
        if (!str || str.length < 2) {
          return str;
        }
        return str.replace(/[-_]+/g, " ").trim().split(' ')
          .map(w => (w[0] ? w[0].toUpperCase() : "") + w.substr(1).toLowerCase()).join(' ')
      },
      chooseTitle: function (obj) {
        var keys = Object.keys(obj);
        console.log("choose title for ", obj)
        for (var i = 0; i < keys.length; i++) {
          console.log("check key", keys[i],);
          if (keys[i].indexOf("name") > -1 && typeof obj[keys[i]] == "string" && obj[keys[i]].length > 0) {
            console.log("Choosen title", keys[i], obj[keys[i]], typeof obj[keys[i]]);
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
        actionManager: actionManager,
        showAddEdit: false,
        selectedWorldAction: {},
      }
    },
    methods: {
      hideModel () {
        console.log("Call to hide model")
        $('#uploadJson').modal('hide all');
      },
      doAction (action) {
        this.$store.commit("SET_SELECTED_ACTION", action)
        this.showAddEdit = true;
      },
      uploadJsonSchemaFile(){
        console.log("this files list", this.$refs.upload)
      },
      handleCommand(command) {
        if (command == "load-restart") {
          window.location.reload()
          return;
        }

        this.$router.push({
          name: 'tablename-actionname',
          params: {
            tablename: "world",
            actionname: command,
          }
        });
        return;

//        const action = actionManager.getActionModel("world", command);
//        console.log("initiate action", action)
//
//
//        console.log(command);
//        this.selectedWorldAction = action;
//        this.$refs.systemActionView.init();
//
//        setTimeout(function () {
//          $('#uploadJson').modal('show');
//        }, 300);
//        if (command === "load-json") {
//        }
      },
      getCurrentTableType() {
        var that = this;
        if (!that.selectedSubTable || !that.selectedInstanceReferenceId) {
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
          row[that.selectedTable + "_id"] = {
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
      reloadData: function () {
        var currentTableType = this.getCurrentTableType();
        var that = this;
        if (that.$refs.tableview1) {
          that.$refs.tableview1.reloadData(currentTableType);

        } else if (that.$refs.tableview2) {
          that.$refs.tableview2.reloadData(currentTableType)

        }
      },
      newRow() {
        var that = this;
        console.log("new row", that.selectedTable);
        this.rowBeingEdited = {};
        this.showAddEdit = true;
      },
      editRow(row) {
        var that = this;
        console.log("new row", that.selectedTable);
        this.rowBeingEdited = row;
        this.showAddEdit = true;
      },
      setTable() {
        const that = this;
        var tableName;

        let all = {};
        console.log("Admin set table -", that.visibleWorlds)
        console.log("Admin set table -", that.$store, that.selectedTable, that.selectedTable)
        if (!that.selectedSubTable) {
          all = jsonApi.all(that.selectedTable);
          tableName = that.selectedTable;
        } else {
          tableName = that.selectedSubTable;
          all = jsonApi.one(that.selectedTable, that.selectedInstanceReferenceId).all(that.selectedSubTable + "_id");
          console.log("Set subtable columns: ", that.subTableColumns)
        }


        if (that.selectedTable) {
          worldManager.getColumnKeys(that.selectedTable, function (model) {
            console.log("Set selected world columns", model.ColumnModel);
            that.$store.commit("SET_SELECTED_TABLE_COLUMNS", model.ColumnModel)
          });
        }

        if (that.selectedSubTable) {
          worldManager.getColumnKeys(that.selectedSubTable, function (model) {
            console.log("Set selected world columns", model.ColumnModel);
            that.$store.commit("SET_SUBTABLE_COLUMNS", model.ColumnModel)
          });
        }


        that.$store.commit("SET_FINDER", all.builderStack);
        console.log("Finder stack: ", that.finder);


        console.log("Selected sub table: ", that.selectedSubTable);
        console.log("Selected table: ", that.selectedTable);

        that.$store.commit("SET_ACTIONS", actionManager.getActions(that.selectedTable));

        all.builderStack = [];


        if (that.$refs.tableview1) {
          console.log("setTable for [tableview1]: ", tableName);
//          console.log("reload data for selected table", tableName);
          that.$refs.tableview1.reloadData(tableName)
        } else {
//          console.error("no table is active")
        }

      },
      logout: function () {
        this.$parent.logout();
      }
    },

    mounted() {
      var that = this;
      that.$store.dispatch("LOAD_WORLDS");
      console.log("Enter tablename: ", that);

      that.actionManager = actionManager;
      const worldActions = actionManager.getActions("world");

      let tableName = that.$route.params.tablename;
      let subTableName = that.$route.params.subTable;
      let selectedInstanceId = that.$route.params.refId;

      if (!tableName) {
        tableName = "user";
      }
      console.log("Set table 1", tableName);
      that.$store.commit("SET_SELECTED_TABLE", tableName);
      that.$store.commit("SET_ACTIONS", worldActions);

      if (selectedInstanceId) {
        that.$store.commit("SET_SELECTED_INSTANCE_REFERENCE_ID", selectedInstanceId);
        jsonApi.one(tableName, selectedInstanceId).get(function (res) {
          console.log("got object", res);
          that.$store.commit("SET_SELECTED_ROW", res);
        })
      }

      if (selectedInstanceId && subTableName) {
        that.$store.commit("SET_SELECTED_TABLE", tableName);
      }


      that.setTable();


    },
    computed: {
      ...mapState([
        "selectedSubTable",
        "selectedAction",
        "subTableColumns",
        "systemActions",
        "finder",
        "selectedTableColumns",
        "selectedRow",
        "selectedTable",
        "selectedInstanceReferenceId",
      ]),
      ...mapGetters([
        "visibleWorlds",
        "actions"
      ])
    },
    watch: {
      '$route.params.tablename': function (to, from) {
        console.log("tablename page, path changed: ", arguments);
        this.$store.commit("SET_SELECTED_TABLE", to);
        this.$store.commit("SET_SELECTED_SUB_TABLE", null);
        this.showAddEdit = false;
        this.setTable();
      },
      '$route.params.refId': function (to, from) {
        var that = this;
        console.log("refId changed in tablename path", arguments);
        this.showAddEdit = false;


        if (!to) {
          this.$store.commit("SET_SELECTED_ROW", null);
          that.$store.commit("SET_SELECTED_INSTANCE_REFERENCE_ID", null)
        } else {
          jsonApi.one(that.selectedTable, to).get().then(function (r) {
            console.log("TableName SET_SELECTED_ROW", r);
            that.$store.commit("SET_SELECTED_ROW", r);
            that.$store.commit("SET_SELECTED_INSTANCE_REFERENCE_ID", r["id"])
          });
        }
        this.setTable();
      },
      '$route.params.subTable': function (to, from) {
        this.showAddEdit = false;
        console.log("TableName SubTable changed", arguments);
        this.$store.commit("SET_SELECTED_SUB_TABLE", to);
        this.setTable();
      }
    }
  }
</script>
