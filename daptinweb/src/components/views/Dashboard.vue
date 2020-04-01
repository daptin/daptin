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

      <div class="row">
        <div class="col-md-6">
          <div class="row">


            <div class="col-lg-4 col-xs-6" v-bind:key="world.id" v-for="world in worlds">
              <!-- small box -->
              <div class="small-box" :style="{backgroundColor: stringToColor(world.TableName), color: 'white'}">
                <div class="inner">
                  <h3>{{world.Count}}</h3>

                  <p>{{world.TableName | titleCase}}s </p>
                </div>

                <div class="icon">
                  <i style="color: #bbb" :class="'fa ' + world.Icon"></i>
                </div>
                <router-link :to="{name: 'Entity', params: { tablename: world.TableName}}" class="small-box-footer">
                  <i class="fa fa-arrow-circle-right"></i>
                </router-link>
              </div>
            </div>
          </div>
        </div>

        <div class="col-md-6">


          <div class="row">
            <div class="col-md-12">
              <div class="row">
                <div class="col-sm-12">
                  <router-link :to="{name: 'Action', params: {tablename: 'user_account', actionname: 'signup'}}"

                               class="btn btn-lg btn-app dashboard_button">
                    <i class="fas fa-user-plus"></i><br/>Create new user
                  </router-link>

                  <router-link :to="{name: 'NewEntity', params:{tablename: 'usergroup'}}"

                               class="btn btn-lg btn-app dashboard_button">
                    <i class="fas fa-users"></i><br/>Create new user group
                  </router-link>

                  <router-link
                    :to="{name : 'Action', params: {tablename: 'world', actionname: 'become_an_administrator'}}"
                    class="btn btn-lg btn-app dashboard_button">
                    <i class="fas fa-lock"></i><br/>Become admin
                  </router-link>

                  <router-link :to="{name : 'Action', params: {tablename: 'world', actionname: 'restart_daptin'}}"
                               class="btn btn-lg btn-app dashboard_button">
                    <i class="fas fa-retweet"></i><br/>Restart
                  </router-link>
                </div>
              </div>


              <h3>Backup</h3>
              <div class="row">

                <div class="col-sm-12">
                  <router-link
                    :to="{name : 'Action', params: {tablename: 'world', actionname: 'download_system_schema'}}"
                    class="btn btn-lg btn-app dashboard_button">
                    <i class="fas fa-object-group"></i><br/>Download JSON schema
                  </router-link>

                  <router-link :to="{name : 'Action', params: {tablename: 'world', actionname: 'export_data'}}"
                               class="btn btn-lg btn-app dashboard_button">
                    <i class="fas fa-database"></i><br/>Download JSON dump
                  </router-link>
                </div>
              </div>


            </div>

            <!--<div class="col-md-3">-->
              <!--<div class="row">-->
                <!--<div class="col-md-12" v-for="(worlds, tableName) in worldActions" v-if="worlds.length > 0"-->
                     <!--v-bind:key="tableName">-->

                  <!--<div class="box box-solid" v-if="worlds.filter(function(e){return e.InstanceOptional}).length > 0">-->
                    <!--<div class="box-header with-border">-->
                      <!--<h3 class="box-title">{{tableName | titleCase}}</h3>-->

                      <!--<div class="box-tools">-->
                        <!--<button type="button" class="btn btn-box-tool" data-widget="collapse"><i-->
                          <!--class="fa fa-minus"></i>-->
                        <!--</button>-->
                      <!--</div>-->
                    <!--</div>-->
                    <!--<div class="box-body no-padding">-->
                      <!--<ul class="nav nav-pills nav-stacked">-->
                        <!--<li v-for="action in worlds" v-if="action.InstanceOptional" v-bind:key="action.Name">-->
                          <!--<router-link-->
                            <!--:to="{name: 'Action', params: {tablename: action.OnType, actionname: action.Name}}">-->
                            <!--{{action.Label}}-->
                          <!--</router-link>-->
                        <!--</li>-->

                      <!--</ul>-->
                    <!--</div>-->
                    <!--&lt;!&ndash; /.box-body &ndash;&gt;-->
                  <!--</div>-->


                <!--</div>-->
              <!--</div>-->
            <!--</div>-->
          </div>


        </div>

      </div>

      <!-- Main row -->
      <!-- /.row -->


      <div class="row">


      </div>

      <div class="row">
      </div>
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

        return "#333";

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
                parse.Icon = !parse.Icon || parse.Icon == "" ? "fa-star" : parse.Icon;
                parse.Count = 0;
                return parse;
              })
              .filter(function (e) {
                // console.log("filter ", e);
                return (
                  !e.IsHidden &&
                  !e.IsJoinTable &&
                  e.TableName.indexOf("_state") === -1
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
                    // console.log("Stats received", stats);

                    const rows = stats.data;
                    const totalCount = rows[0]["count"];
                    w.Count = totalCount;
                  },
                  function (error) {
                    console.log("Failed to query stats", error);
                  }
                );
            });

            that.worlds.sort(function (a, b) {

              const nameA = a.TableName;
              const nameB = b.TableName;
              if (nameA < nameB) //sort string ascending
                return -1;
              if (nameA > nameB)
                return 1;
              return 0
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

            // console.log("load world actions tables");
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
