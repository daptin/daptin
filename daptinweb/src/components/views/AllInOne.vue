<template>
  <!-- Content Wrapper. Contains page content -->
  <div class="content-wrapper">
    <!-- Content Header (Page header) -->
    <section class="content-header">
      <ol class="breadcrumb">
        <li>
          <a href="javascript:">
            <i class="fa fa-home"></i>Home </a>
        </li>
        <li v-for="crumb in $route.meta.breadcrumb" v-bind:key="crumb.label">
          <template v-if="crumb.to">

          </template>
          <template v-else>
            {{crumb.label}}
          </template>
        </li>
      </ol>

    </section>

    <!-- Main content -->
    <section class="content">

      <el-tabs type="card">
        <el-tab-pane :key="world.TableName" v-for="world in worlds" :label="world.TableName | titleCase">
          <daptable
            :json-api="jsonApi"
            data-path="data"
            :json-api-model-name="world.TableName">
          </daptable>
        </el-tab-pane>
      </el-tabs>
    </section>
    <!-- /.content -->
  </div>

</template>

<style>
  .dashboard_button {
    width: 230px;
    height: 90px;
    font-size: 20px;
  }

  .dashboard_button i {
    font-size: 40px;
  }

  .dashboard_button i {
    color: #534da7;
  }
</style>

<script>
  import jsonApi from "../../plugins/jsonapi";
  import actionManager from "../../plugins/actionmanager";
  import worldManager from "../../plugins/worldmanager";
  import statsManger from "../../plugins/statsmanager";
  import {mapState} from "vuex";

  export default {
    data() {
      return {
        worldActions: {},
        actionGroups: {},
        jsonApi: jsonApi,
        generateRandomNumbers(numbers, max, min) {
          let a = [];
          for (let i = 0; i < numbers; i++) {
            a.push(Math.floor(Math.random() * (max - min + 1)) + max);
          }
          return a;
        },
        worlds: []
      };
    },
    computed: {
      ...mapState(["query"]),
      sortedWorldActions: function () {
        console.log("return sorted world actions", this.worldActions);
        let keys = Object.keys(this.worldActions);

        keys.sort();

        let res = {};

        for (let key in keys) {
          res[key] = this.worldActions[key];
        }

        console.log("returning sorted worlds", res);
        return res;
      }
    },
    methods: {
      stringToColor(str) {
        //        console.log("String to color", str, window.stringToColor(str))
        return "#" + window.stringToColor(str);
      },
      reloadData() {
        let that = this;
        let newWorldActions = {};
        jsonApi
          .all("world")
          .get({
            page: {
              number: 1,
              size: 200
            }
          })
          .then(function (worlds) {
            worlds = worlds.data;
            console.log("got worlds", worlds);
            that.worlds = worlds
              .map(function (e) {
                let parse = JSON.parse(e.world_schema_json);
                parse.Icon = e.icon;
                parse.Count = 0;
                return parse;
              })
              .filter(function (e) {
                console.log("filter ", e);
                return (
                  !e.IsHidden &&
                  !e.IsJoinTable &&
                  e.TableName.indexOf("_state") == -1
                );
              });
            that.worlds.forEach(function (w) {
              // console.log("call stats", w);

              statsManger
                .getStats(w.TableName, {
                  column: ["count"]
                })
                .then(
                  function (stats) {
                    stats = stats.data;
                    console.log("Stats received", stats);

                    const rows = stats.data;
                    const totalCount = rows[0]["count"];
                    w.Count = totalCount;
                  },
                  function (error) {
                    console.log("Failed to query stats", error);
                  }
                );
            });

            let actionGroups = {
              System: [],
              User: []
            };
            // console.log("worlds in dashboard", worlds);
            for (let i = 0; i < worlds.length; i++) {
              let tableName = worlds[i].table_name;
              let actions = actionManager.getActions(tableName);

              if (!actions) {
                continue;
              }
              // console.log("actions for ", tableName, actions);
              let actionKeys = Object.keys(actions);
              for (let j = 0; j < actionKeys.length; j++) {
                let action = actions[actionKeys[j]];
                //            console.log("dashboard action", action)
                let onType = action.OnType;
                let onWorld = worldManager.getWorldByName(onType);
                //            console.log("on world", onWorld)

                if (onWorld.is_hidden == "1") {
                  actionGroups["System"].push(action);
                } else if (onWorld.table_name == "user_account") {
                  actionGroups["User"].push(action);
                } else if (onWorld.table_name == "usergroup") {
                  actionGroups["User"].push(action);
                } else {
                  if (!newWorldActions[onWorld.table_name]) {
                    newWorldActions[onWorld.table_name] = [];
                  }
                  newWorldActions[onWorld.table_name].push(action);
                }
              }
            }

            console.log("load world actions tabld");
            that.worldActions = newWorldActions;
            that.actionGroups = actionGroups;
          });

      }
    },
    updated() {
      document.getElementById("navbar-search-input").value = "";
    },
    watch: {
      query: function (oldVal, newVal) {
        console.log("query change", arguments);
        this.reloadData();
      }
    },
    mounted() {
      //      $(".content").popover();

      let that = this;
      that.$route.meta.breadcrumb = [
        {
          label: "Dashboard"
        }
      ];
      this.reloadData();
    }
  };
</script>
<style>
  .info-box {
    cursor: pointer;
  }

  .info-box-content {
    text-align: center;
    vertical-align: middle;
    display: inherit;
  }

  .fullCanvas {
    width: 100%;
  }
</style>
