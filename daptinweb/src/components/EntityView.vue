<template>

  <!-- Content Wrapper. Contains page content -->
  <div class="content-wrapper">
    <!-- Content Header (Page header) -->
    <section class="content-header">
      <h1>

        {{selectedTable | titleCase}}
        <small>{{ $route.meta.description }}</small>
      </h1>

      <ol class="breadcrumb">
        <li>
          <a href="javascript:">
            <i class="fa fa-home"></i>Home</a>
        </li>
        <li v-for="crumb in $route.meta.breadcrumb">
          <template v-if="crumb.to">

          </template>
          <template v-else>
            {{crumb.label}}
          </template>
        </li>
      </ol>
      <div class="box-tools pull-right">
        <div class="ui icon buttons">
          <button class="btn btn-box-tool" @click.prevent="viewMode = 'table'; currentViewType = 'table-view';"><i
            class="fas fa-table fa-2x"></i>
          </button>
          <button class="btn btn-box-tool" @click.prevent="viewMode = 'card'; currentViewType = 'table-view';"><i
            class="fas fa-th-large fa-2x  grey"></i></button>
          <button class="btn btn-box-tool" @click.prevent="currentViewType = 'recline-view'"><i
            class="fas fa-chart-bar fa-2x grey"></i></button>
          <!--<button class="btn btn-box-tool" @click.prevent="currentViewType = 'voyager-view'"><i-->
          <!--class="fa  fa-2x fa-area-chart grey"></i></button>-->
          <router-link v-if="selectedTable" :to="{name: 'NewEntity', params: {tablename: selectedTable}}"
                       class="btn btn-box-tool"
                       @click.prevent="newRow()">
            <i class="fa fa-2x fa-plus green "></i>
          </router-link>
          <button class="btn btn-box-tool" @click.prevent="reloadData()">
            <i class="fas fa-sync fa-2x grey"></i>
          </button>
          <router-link
            :to="{name: 'NewItem', query: {table: selectedTable}}"
            class="btn btn-box-tool"><i
            class="fas fa-edit fa-2x grey"></i></router-link>
          <router-link
            :to="{name: 'Action', params: {actionname: 'add_exchange', tablename: 'world'}, query: {id: worldReferenceId}}"
            class="btn btn-box-tool"><i
            class="fas fa-link fa-2x  grey"></i></router-link>
          <router-link
            :to="{name: 'Action', params: {actionname: 'export_data', tablename: 'world'}, query: {world_id: worldReferenceId}}"
            class="btn btn-box-tool"><i
            class="fas fa-download fa-2x  grey"></i></router-link>
          <router-link
            :to="{name: 'Action', params: {actionname: 'export_csv_data', tablename: 'world'}, query: {world_id: worldReferenceId}}"
            class="btn btn-box-tool"><i
            class="fas fa-bars fa-2x  grey"></i></router-link>
        </div>

      </div>


    </section>


    <section class="content">

      <div class="row" v-if="showAddEdit && rowBeingEdited != null">
        <div class="col-md-12">
          <model-form :hideTitle="true" @save="saveRow(rowBeingEdited)" :json-api="jsonApi"
                      @cancel="showAddEdit = false"
                      v-bind:model="rowBeingEdited"
                      v-bind:meta="selectedTableColumns" ref="modelform"></model-form>
        </div>
      </div>

      <template v-if="currentViewType == 'table-view'">
        <table-view @newRow="newRow()" @editRow="editRow"
                    :finder="finder" ref="tableview1" :view-mode="viewMode" :json-api="jsonApi"
                    :json-api-model-name="selectedTable" v-if="selectedTable && !showAddEdit"></table-view>

      </template>

      <template v-else-if="currentViewType == 'recline-view'">
        <recline-view @newRow="newRow()" @editRow="editRow"
                      :finder="finder" ref="tableview1" :view-mode="viewMode" :json-api="jsonApi"
                      :json-api-model-name="selectedTable" v-if="selectedTable && !showAddEdit"></recline-view>
      </template>

      <template v-else-if="currentViewType == 'voyager-view'">
        <voyager-view @newRow="newRow()" @editRow="editRow"
                      :finder="finder" ref="tableview1" :view-mode="viewMode" :json-api="jsonApi"
                      :json-api-model-name="selectedTable" v-if="selectedTable && !showAddEdit"></voyager-view>
      </template>


    </section>

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
    name: 'EntityView',
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
      viewType: {
        type: String,
        default: 'table-view'
      }
    },
    data() {
      return {
        jsonApi: jsonApi,
        currentViewType: null,
        actionManager: actionManager,
        showAddEdit: false,
        selectedWorldAction: {},
        addExchangeAction: null,
        viewMode: "card",
        rowBeingEdited: null,
        worldReferenceId: null,
      }
    },
    methods: {
      hideModel() {
        console.log("Call to hide model");
        $('#uploadJson').modal('hide all');
      },
      doAction(action) {
        console.log("set action", action);

        this.$store.commit("SET_SELECTED_ACTION", action);
        this.rowBeingEdited = true;
        this.showAddEdit = true;
      },
      uploadJsonSchemaFile() {
        console.log("this files list", this.$refs.upload)
      },
      handleCommand(command) {
        if (command == "load-restart") {
          window.location.reload();
          return;
        }

        this.$router.push({
          name: 'Action',
          params: {
            tablename: "world",
            actionname: command,
          }
        });

      },
      getCurrentTableType() {
        var that = this;
        return that.selectedTable;
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
        var newRow = {};
        var keys = Object.keys(row);
        for (var i=0;i<keys.length;i++){
          if (row[keys[i]] != null) {
            newRow[keys[i]] = row[keys[i]];
          }
        }
        row = newRow;

        var currentTableType = this.getCurrentTableType();


        console.log("save row", row);
        if (row["id"]) {
          var that = this;
          jsonApi.update(currentTableType, row).then(function () {
            that.setTable();
            that.showAddEdit = false;
          }, function(err){
            console.log("failed to save row", err)
          });
        } else {
          var that = this;
          jsonApi.create(currentTableType, row).then(function () {
            console.log("create complete", arguments);
            that.setTable();
            that.showAddEdit = false;
            that.$refs.tableview1.reloadData(currentTableType);
          }, function (r) {
            console.error("failed to save row", r)
          });
        }


      },
      reloadData: function () {
        var currentTableType = this.getCurrentTableType();
        var that = this;
        if (that.$refs.tableview1) {
          that.$refs.tableview1.reloadData(currentTableType);

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

        let world = worldManager.getWorldByName(that.selectedTable);

        if (!world) {
          that.$notify({
            type: "error",
            title: "Error",
            message: "We dont yet know about anything like " + window.titleCase(that.selectedTable)
          });
          return
        }

        this.worldReferenceId = world.id;

        let all = {};
        console.log("Admin set table -", that.visibleWorlds);
        console.log("Admin set table -", that.$store, that.selectedTable, that.selectedTable);

        all = jsonApi.all(that.selectedTable);
        tableName = that.selectedTable;

        that.$route.meta.breadcrumb = [{
          label: tableName,
          to: {
            name: "Entity",
            params: {
              tablename: tableName
            }
          }
        }];


        if (that.selectedTable) {
          worldManager.getColumnKeys(that.selectedTable, function (model) {
            console.log("Set selected world columns", model.ColumnModel);
            that.$store.commit("SET_SELECTED_TABLE_COLUMNS", model.ColumnModel)
          });
        }


        that.$store.commit("SET_FINDER", all.builderStack);
        console.log("Finder stack: ", that.finder);


        console.log("Selected table: ", that.selectedTable);

        that.$store.commit("SET_ACTIONS", actionManager.getActions(that.selectedTable));

        all.builderStack = [];


        console.log("setTable for [tableview1]: ", tableName);
        if (that.$refs.tableview1) {
          console.log("tableview 1 is present");
          that.$refs.tableview1.reloadData(tableName)
        }

      },
      logout: function () {
        this.$parent.logout();
      }
    },
    mounted() {

      var that = this;
      that.currentViewType = that.viewType;
      console.log("Entity view: ", that.$route);

      that.actionManager = actionManager;
      const worldActions = actionManager.getActions("world");
      console.log("world actions", worldActions);

      that.addExchangeAction = actionManager.getActionModel("world", "add-exchange");

      if (that.$route.name == "NewEntity") {
        this.rowBeingEdited = {};
        that.showAddEdit = true;
      }

      let tableName = that.$route.params.tablename;
      let subTableName = that.$route.params.subTable;
      let selectedInstanceId = that.$route.params.refId;

      if (!tableName) {
        tableName = "user_account";
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
        "selectedAction",
        "subTableColumns",
        "systemActions",
        "finder",
        "selectedTableColumns",
        "query",
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
      '$route.params.subTable': function (to, from) {
        this.showAddEdit = false;
        console.log("TableName SubTable changed", arguments);
        this.$store.commit("SET_SELECTED_SUB_TABLE", to);
        this.setTable();
      },
      '$route.name': function () {
        if (this.$route.name === "NewEntity") {
          this.showAddEdit = true;
          this.rowBeingEdited = {};
        } else {
          this.showAddEdit = false;
        }
      },
      'showAddEdit': function (newVal) {
        if (!newVal) {
          if (this.$route.name === "NewEntity") {
            console.log("triggr back");
            window.history.back();
          }
        }
      },
      'query': function () {
        this.setTable();
      }
    }
  }
</script>
