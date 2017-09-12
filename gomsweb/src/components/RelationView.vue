<template>

  <!-- Content Wrapper. Contains page content -->
  <div class="content-wrapper">
    <!-- Content Header (Page header) -->
    <section class="content-header">
      <h1>
        {{selectedSubTable | titleCase}}
        <small>{{ $route.meta.description }}</small>
      </h1>
      <ol class="breadcrumb">
        <li>
          <a href="javascript:;">
            <i class="fa fa-home"></i>Home</a>
        </li>
        <li>
          <router-link :to="{name: 'Entity', params: {tablename: selectedTable}}">
            {{selectedTable | titleCase}}
          </router-link>
        </li>
        <li class="active">
          <router-link :to="{name: 'Instance', params: {tablename: selectedTable, refId: $route.params.refId}}">
            {{selectedRow | chooseTitle | titleCase}}
          </router-link>
        </li>
      </ol>
      <div class="box-tools pull-right">
        <div class="ui icon buttons">
          <button class="btn btn-box-tool" @click.prevent="viewMode = 'table'"><i
            class="fa  fa-2x fa-table blue "></i>
          </button>
          <button class="btn btn-box-tool" @click.prevent="viewMode = 'items'"><i
            class="fa  fa-2x fa-th-large blue"></i>
          </button>
          <button class="btn btn-box-tool" @click.prevent="newRow()"><i class="fa fa-2x fa-plus green "></i>
          </button>
          <button class="btn btn-box-tool" @click.prevent="reloadData()"><i class="fa fa-2x fa-refresh orange"></i>
          </button>
        </div>
      </div>
    </section>
    <section class="content">

      <div class="col-md-12" v-if="showAddEdit && rowBeingEdited != null">
        <model-form @save="saveRow(rowBeingEdited)" :json-api="jsonApi"
                    v-if="showAddEdit"
                    @cancel="showAddEdit = false"
                    v-bind:model="rowBeingEdited"
                    v-bind:meta="subTableColumns" ref="modelform"></model-form>

      </div>

      <div class="col-md-12">

        <table-view @newRow="newRow()" @editRow="editRow"
                    v-if="selectedSubTable" :finder="finder"
                    ref="tableview2" :json-api="jsonApi"
                    :json-api-model-name="selectedSubTable"></table-view>

      </div>
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
    name: 'AdminView',
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
    data() {
      return {
        jsonApi: jsonApi,
        actionManager: actionManager,
        showAddEdit: false,
        selectedWorldAction: {},
      }
    },
    methods: {
      hideModel() {
        console.log("Call to hide model")
        $('#uploadJson').modal('hide all');
      },
      doAction(action) {
        this.$store.commit("SET_SELECTED_ACTION", action)
        this.showAddEdit = true;
      },
      uploadJsonSchemaFile() {
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
      },
      getCurrentTableType() {
        var that = this;
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
            that.$refs.tableview2.reloadData(currentTableType)
          }, function (r) {
            console.error(r)
          });
        }


      },
      reloadData: function () {
        var currentTableType = this.getCurrentTableType();
        var that = this;


        that.$refs.tableview2.reloadData(currentTableType)
      },
      newRow() {
        var that = this;
        console.log("new row", that.selectedSubTable);
        this.rowBeingEdited = {};
        this.showAddEdit = true;
      },
      editRow(row) {
        var that = this;
        console.log("new row", that.selectedSubTable);
        this.rowBeingEdited = row;
        this.showAddEdit = true;
      },
      setTable() {
        const that = this;
        var tableName;

        let all = {};
        console.log("Admin set table -", that.visibleWorlds)
        console.log("Admin set table -", that.$store, that.selectedTable, that.selectedTable)

        tableName = that.selectedSubTable;
        all = jsonApi.one(that.selectedTable, that.selectedInstanceReferenceId).all(that.selectedSubTable + "_id");
        console.log("Set subtable columns: ", that.subTableColumns)


        worldManager.getColumnKeys(that.selectedSubTable, function (model) {
          console.log("Set selected world columns", model.ColumnModel);
          that.$store.commit("SET_SUBTABLE_COLUMNS", model.ColumnModel)
        });


        that.$store.commit("SET_FINDER", all.builderStack);
        console.log("Finder stack: ", that.finder);


        console.log("Selected sub table: ", that.selectedSubTable);
        console.log("Selected table: ", that.selectedTable);

        that.$store.commit("SET_ACTIONS", actionManager.getActions(that.selectedTable));

        all.builderStack = [];


        that.$refs.tableview2.reloadData(tableName)

      },
      logout: function () {
        this.$parent.logout();
      }
    },

    mounted() {
      var that = this;
//      that.$store.dispatch("LOAD_WORLDS");
      console.log("Enter tablename: ", that);

      that.actionManager = actionManager;
      const worldActions = actionManager.getActions("world");

      let tableName = that.$route.params.tablename;
      let subTableName = that.$route.params.subTable;
      let selectedInstanceId = that.$route.params.refId;

      if (!tableName) {
        tableName = "user";
      }
      console.log("Set table 1", tableName, subTableName);
      that.$store.commit("SET_SELECTED_TABLE", tableName);
      that.$store.commit("SET_ACTIONS", worldActions);

      that.$store.commit("SET_SELECTED_INSTANCE_REFERENCE_ID", selectedInstanceId);
      jsonApi.one(tableName, selectedInstanceId).get(function (res) {
        console.log("got object", res);
        that.$store.commit("SET_SELECTED_ROW", res);
      })

      that.$store.commit("SET_SELECTED_SUB_TABLE", subTableName);


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
