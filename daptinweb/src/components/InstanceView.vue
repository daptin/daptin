<template>
  <!-- Content Wrapper. Contains page content -->
  <div class="content-wrapper">
    <!-- Content Header (Page header) -->
    <section class="content-header">
      <h1>
        {{selectedTable | titleCase}} - <b>{{selectedRow | chooseTitle | titleCase}}</b>
        <small>{{ $route.meta.description }}</small>
      </h1>
      <ol class="breadcrumb">
        <li>
          <a href="javascript:;">
            <i class="fa fa-home"></i>Home</a>
        </li>
        <li v-for="crumb in $route.meta.breadcrumb">
          <template v-if="crumb.to">
            <router-link :to="crumb.to">{{crumb.label}}</router-link>
          </template>
          <template v-else>
            {{crumb.label}}
          </template>
        </li>
      </ol>
      <div class="pull-right">
        <div class="ui icon buttons">
          <button class="btn btn-box-tool" @click.prevent="editRow()"><i
            class="fas fa-edit fa-3x "></i>
          </button>
          <button class="btn btn-box-tool" @click.prevent="refreshRow()"><i
            class="fas fa-sync fa-3x "></i>
          </button>
        </div>
      </div>
    </section>
    <section class="content">

      <div class="col-md-12" v-if="showAddEdit">
        <div class="row" v-if="selectedAction != null">
          <action-view @cancel="showAddEdit = false" @action-complete="showAddEdit = false"
                       :action-manager="actionManager" :action="selectedAction"
                       :json-api="jsonApi" :model="selectedRow"></action-view>
        </div>
        <div class="row" v-if="rowBeingEdited != null">
          <model-form @save="saveRow(rowBeingEdited)" :json-api="jsonApi"
                      @cancel="showAddEdit = false"
                      v-bind:model="rowBeingEdited"
                      v-bind:meta="selectedTableColumns" ref="modelform"></model-form>
        </div>
      </div>
      <div class="col-md-9">

        <detailed-table-row :model="selectedRow" v-if="selectedRow" :json-api="jsonApi"
                            :json-api-model-name="selectedTable"></detailed-table-row>

        <!--<div class="row" v-if="showAddEdit && rowBeingEdited != null">-->


        <!--<model-form @save="saveRow(rowBeingEdited)" :json-api="jsonApi"-->
        <!--v-if="selectedSubTable"-->
        <!--@cancel="showAddEdit = false"-->
        <!--v-bind:model="rowBeingEdited"-->
        <!--v-bind:meta="subTableColumns" ref="modelform"></model-form>-->


        <!--</div>-->


      </div>
      <div class="col-md-3">


        <div class="row" v-if="stateMachines != null && stateMachines.length > 0">
          <div class="col-md-12">
            <h2>Start Tracking</h2>
          </div>
          <div class="col-md-12" v-for="a, k in stateMachines">
            <button class="btn btn-default" style="width: 100%" @click="addStateMachine(a)">{{a.label}}</button>
          </div>
        </div>


        <div class="row" v-if="actions != null">
          <div class="col-md-12">
            <h2>Actions</h2>
          </div>
          <div class="col-md-12" v-for="a, k in actions" v-if="!a.InstanceOptional">
            <button class="btn btn-default" style="width: 100%" @click="doAction(a)">{{a.Label}}</button>
          </div>
        </div>

        <div class="row" v-if="visibleWorlds.length > 0">
          <div class="col-md-12">
            <h2>Related</h2>
          </div>
          <div class="col-md-12" v-for="world in visibleWorlds">
            <router-link v-if="selectedInstanceReferenceId" style=" width: 100%" class="btn btn-default"
                         :to="{name: 'Relation', params: {tablename: selectedTable, refId: selectedInstanceReferenceId, subTable: world.table_name}}">
              {{world.table_name | titleCase}}
            </router-link>
          </div>
        </div>


      </div>


      <div class="col-md-12" v-if="objectStates.length > 0">
        <h3>Status tracks</h3>
        <div class="row">
          <div class="col-md-3" v-for="state, k in objectStates">
            <div class="box">
              <div class="box-header">
                <div class="box-title">
                  <small>{{state.smd.label}}</small>
                </div>
                <div class="box-title pull-right">
                  {{state.current_state | titleCase}}
                </div>
              </div>
              <div class="box-body">
                <div class="col-md-12" v-for="action in state.possibleActions">
                  <button @click="doEvent(state, action)" class="btn btn-primary btn-xs btn-flat"
                          style="width: 100%; border-radius: 5px; margin: 5px;">{{action.label}}
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

    </section>
  </div>


</template>

<script>
  import {Notification} from 'element-ui';
  import worldManager from "../plugins/worldmanager"
  import jsonApi from "../plugins/jsonapi"
  import actionManager from "../plugins/actionmanager"
  import {mapGetters, mapState} from 'vuex'


  export default {
    name: 'InstanceView',
    data() {
      return {
        jsonApi: jsonApi,
        actionManager: actionManager,
        showAddEdit: false,
        stateMachines: [],
        selectedWorldAction: {},
        objectStates: [],
        rowBeingEdited: {},
        truefalse: [],
      }
    },
    methods: {
      editRow() {
        console.log("edit row");
        this.$store.commit("SET_SELECTED_ACTION", null);
        this.showAddEdit = true;
        this.rowBeingEdited = this.selectedRow;
      },
      refreshRow() {
        var that = this;
        let tableName = that.$route.params.tablename;
        let selectedInstanceId = that.$route.params.refId;

        jsonApi.find(tableName, selectedInstanceId).then(function (res) {
          console.log("got object", res);
          res = res.data;
          that.$store.commit("SET_SELECTED_ROW", res);
        }, function (err) {
          console.log("Errors", err)
        });
      },
      doEvent(action, event) {
        var that = this;
        console.log("do event", action, event);
        worldManager.trackObjectEvent(this.selectedTable, action.id, event.name).then(function () {
          Notification.success({
            title: "Updated",
            message: that.selectedTable + " status was updated for this track"
          });
          that.updateStates();
        }, function () {
          Notification.error({
            title: "Failed",
            message: "Object status was not updated"
          })
        });
      },
      addStateMachine(machine) {
        console.log("Add state machine", machine);
        console.log("Selected row", this.selectedRow);
        var that = this;
        worldManager.startObjectTrack(this.selectedTable,
          this.selectedRow["id"],
          machine["reference_id"]).then(function (res) {
          Notification.success({
            title: "Done",
            message: "Started tracking status for " + that.selectedTable
          });
          that.updateStates();
        });
      },
      doAction(action) {
        this.$store.commit("SET_SELECTED_ACTION", action);
        this.rowBeingEdited = null;
        this.showAddEdit = true;
      },
      saveRow(row) {
        var that = this;

        var currentTableType = this.selectedTable;

        if (that.selectedSubTable && that.selectedInstanceReferenceId) {
          row[that.selectedTable + "_id"] = {
            "id": that.selectedInstanceReferenceId,
          };
        }

        var newRow = {};
        var keys = Object.keys(row);
        for (var i = 0; i < keys.length; i++) {
          if (row[keys[i]] != null) {
            newRow[keys[i]] = row[keys[i]];
          }
        }
        row = newRow;


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
      setTable() {
        const that = this;

        console.log("Instance View: ", that.$route.params);

        that.actionManager = actionManager;
        const worldActions = actionManager.getActions("world");

        let tableName = that.$route.params.tablename;
        let selectedInstanceId = that.$route.params.refId;

        if (!tableName) {
          alert("no table name");
          return;
        }
        that.$route.meta.breadcrumb = [{
          label: tableName,
          to: {
            name: "Entity",
            params: {
              tablename: tableName
            }
          }
        }, {
          label: selectedInstanceId
        }];

        that.$store.commit("SET_SELECTED_TABLE", tableName);
        that.$store.commit("SET_ACTIONS", worldActions);

        that.$store.commit("SET_SELECTED_INSTANCE_REFERENCE_ID", selectedInstanceId);
        console.log("Get instance: ", tableName, selectedInstanceId);


        jsonApi.find(tableName, selectedInstanceId).then(function (res) {
          console.log("got object", arguments);
          res = res.data;
          that.$store.commit("SET_SELECTED_ROW", res);
        }, function (err) {
          console.log("Errors", err)
        });


        that.$store.commit("SET_SELECTED_TABLE", tableName);


        let all = {};

        console.log("Admin set table -", that.$store, that.selectedTable, that.selectedTable);
        all = jsonApi.all(that.selectedTable);
        tableName = that.selectedTable;


        if (that.selectedTable) {
          worldManager.getColumnKeys(that.selectedTable, function (model) {
            console.log("Set selected world columns", model.ColumnModel);
            that.$store.commit("SET_SELECTED_TABLE_COLUMNS", model.ColumnModel)
          });
        }


        that.$store.commit("SET_FINDER", all.builderStack);
        console.log("Finder stack: ", that.finder);


        console.log("Selected sub table: ", that.selectedSubTable);
        console.log("Selected table: ", that.selectedTable);

        that.$store.commit("SET_ACTIONS", actionManager.getActions(that.selectedTable));

        all.builderStack = [];


        worldManager.getStateMachinesForType(that.selectedTable).then(function (machines) {
          console.log("state machines for ", that.selectedTable, machines)
          that.stateMachines = machines;
        });

        that.updateStates();

        if (that.$refs.tableview1) {
          console.log("setTable for [tableview1]: ", tableName);
          that.$refs.tableview1.reloadData(tableName)
        }

      },
      logout: function () {
        this.$parent.logout();
      },
      updateStates: function () {
        var that = this;

        let tableName = that.$route.params.tablename;
        let selectedInstanceId = that.$route.params.refId;

        console.log("Start get states for ", tableName, selectedInstanceId);
        var tableModel = jsonApi.modelFor(tableName);
        console.log("json api model", tableModel);


        if (worldManager.isStateMachineEnabled(tableName)) {
          jsonApi.one(tableName, selectedInstanceId).all(tableName + "_has_state").get({
            page: {
              number: 1,
              size: 20
            }
          }).then(function (states) {
            states = states.data;
            console.log("states", states);
            states.map(function (e) {
              e.smd = e[tableName + "_smd"];
              e.smd.events = JSON.parse(e.smd.events);
              e.possibleActions = e.smd.events.filter(function (t) {
                return t.Src.indexOf(e.current_state) > -1
              }).map(function (er) {
                return {
                  name: er.Name,
                  label: er.Label,
                }
              });
              console.log(e)
            });

            that.objectStates = states;
          });

        }

      },
    },

    mounted() {
      var that = this;
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
        var that = this;

        console.log("tablename, path changed: ", arguments, this.$route.params.refId);
        this.$store.commit("SET_SELECTED_TABLE", to);
        this.$store.commit("SET_SELECTED_SUB_TABLE", null);
        that.$store.commit("SET_SELECTED_INSTANCE_REFERENCE_ID", this.$route.params.refId);
        this.showAddEdit = false;

        jsonApi.one(that.selectedTable, this.$route.params.refId).get().then(function (r) {
          console.log("TableName SET_SELECTED_ROW", r);
          that.$store.commit("SET_SELECTED_ROW", r);
          that.$store.commit("SET_SELECTED_INSTANCE_REFERENCE_ID", r["id"])
        });
        this.setTable();
      },
      '$route.params.refId': function (to, from) {
        var that = this;

        console.log("refId page, path changed: ", arguments, this.$route.params.refId);
        this.$store.commit("SET_SELECTED_TABLE", to);
        this.$store.commit("SET_SELECTED_SUB_TABLE", null);
        that.$store.commit("SET_SELECTED_INSTANCE_REFERENCE_ID", this.$route.params.refId);
        this.showAddEdit = false;

        jsonApi.one(that.selectedTable, this.$route.params.refId).get().then(function (r) {
          console.log("TableName SET_SELECTED_ROW", r);
          that.$store.commit("SET_SELECTED_ROW", r);
          that.$store.commit("SET_SELECTED_INSTANCE_REFERENCE_ID", r["id"])
        });
        this.setTable();
      }
    }
  }
</script>
