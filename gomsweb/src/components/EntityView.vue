<template>


  <div class="box">
    <div class="box-header">
      <div class="box-title">
        <h2>{{selectedTable | titleCase}}</h2>
      </div>
      <div class="box-tools pull-right">
        <div class="ui icon buttons">
          <button class="btn btn-box-tool" @click.prevent="viewMode = 'table'"><i class="fa  fa-2x fa-table grey "></i></button>
          <button class="btn btn-box-tool" @click.prevent="viewMode = 'items'"><i class="fa  fa-2x fa-th-large grey"></i></button>
          <button class="btn btn-box-tool" @click.prevent="newRow()"><i class="fa fa-2x fa-plus green "></i></button>
          <button class="btn btn-box-tool" @click.prevent="reloadData()"><i class="fa fa-2x fa-refresh orange"></i></button>
        </div>

      </div>
    </div>
    <div class="box-body">

      <div class="row" v-if="showAddEdit && rowBeingEdited != null">
        <model-form class="col-md-12" @save="saveRow(rowBeingEdited)" :json-api="jsonApi"
                    @cancel="showAddEdit = false"
                    v-bind:model="rowBeingEdited"
                    v-bind:meta="selectedTableColumns" ref="modelform"></model-form>
      </div>

      <table-view @newRow="newRow()" @editRow="editRow" v-if="selectedTable"
                  :finder="finder" ref="tableview1" :json-api="jsonApi"
                  :json-api-model-name="selectedTable"></table-view>
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
          name: 'Action',
          params: {
            tablename: "world",
            actionname: command,
          }
        });

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
