<template>
  <!-- Content Wrapper. Contains page content -->
  <div class="content-wrapper">
    <!-- Content Header (Page header) -->
    <section class="content-header">
      <h1>
        Dashboard
      </h1>
      <ol class="breadcrumb">
        <li>
          <a href="javascript:;">
            <i class="fa fa-home"></i>Home </a>
        </li>
        <li v-for="crumb in $route.meta.breadcrumb">
          {{crumb.label}}
        </li>
      </ol>

    </section>

    <!-- Main content -->
    <section class="content">

      <!-- Main row -->
      <!-- /.row -->
      <div class="row">
        <div class="col-md-3" v-for="(worlds, tableName) in worldActions" v-if="worlds.length > 0">

          <div class="box box-solid">
            <div class="box-header with-border">
              <h3 class="box-title">{{tableName | titleCase}}</h3>

              <div class="box-tools">
                <button type="button" class="btn btn-box-tool" data-widget="collapse"><i class="fa fa-minus"></i>
                </button>
              </div>
            </div>
            <div class="box-body no-padding">
              <ul class="nav nav-pills nav-stacked">
                <li v-for="action in worlds" v-if="action.instanceOptional">
                  <router-link :to="{name: 'Action', params: {tablename: action.onType, actionname: action.name}}">
                    {{action.label}}
                  </router-link>
                </li>

              </ul>
            </div>
            <!-- /.box-body -->
          </div>


        </div>
      </div>

      <div class="row">
        <div class="col-md-3" v-for="(worlds, tableName) in actionGroups" v-if="worlds.length > 0">

          <div class="box box-solid collapsed-box">
            <div class="box-header with-border">
              <h3 class="box-title">{{tableName | titleCase}}</h3>

              <div class="box-tools">
                <button type="button" class="btn btn-box-tool" data-widget="collapse"><i class="fa fa-minus"></i>
                </button>
              </div>
            </div>
            <div class="box-body no-padding">
              <ul class="nav nav-pills nav-stacked">
                <li v-for="world in worlds">
                  <router-link :to="{name: 'Action', params: {tablename: world.onType, actionname: world.name}}">
                    {{world.label}}
                  </router-link>
                </li>

              </ul>
            </div>
            <!-- /.box-body -->
          </div>


        </div>

        <div class="col-md-9">

          <div class="row">
            <div class="col-md-12">
              <h3>New Entity</h3>

              <div class="row">

                <router-link :to="{name : 'Action', params: {tablename: 'world', actionname: 'upload_system_schema'}}"
                             style="min-width: 120px; height: 90px; font-size: 20px" class="btn btn-lg btn-app">
                  <i style="font-size: 30px" class="fa fa-3x fa-plus black"></i>Upload Schema JSON
                </router-link>


                <router-link
                  :to="{name : 'Action', params: {tablename: 'world', actionname: 'upload_xls_to_system_schema'}}"
                  style="min-width: 120px; height: 90px; font-size: 20px" class="btn btn-lg btn-app">
                  <i style="font-size: 30px" class="fa fa-3x fa-file-excel-o black"></i>Upload XLS
                </router-link>

                <router-link :to="{name : 'Action', params: {tablename: 'world', actionname: 'import_data'}}"
                             style="min-width: 120px; height: 90px; font-size: 20px" class="btn btn-lg btn-app">
                  <i style="font-size: 30px" class="fa fa-3x fa-upload black"></i>Upload Data JSON
                </router-link>


              </div>

              <h3>Backup</h3>
              <div class="row">
                <router-link :to="{name : 'Action', params: {tablename: 'world', actionname: 'download_system_schema'}}"
                             style="min-width: 120px; height: 90px; font-size: 20px" class="btn btn-lg btn-app">
                  <i style="font-size: 30px" class="fa fa-3x fa-object-group black"></i>Download schema JSON
                </router-link>

                <router-link :to="{name : 'Action', params: {tablename: 'world', actionname: 'export_data'}}"
                             style="min-width: 120px; height: 90px; font-size: 20px" class="btn btn-lg btn-app">
                  <i style="font-size: 30px" class="fa fa-3x fa-cloud-download black"></i>Download dump JSON
                </router-link>

              </div>


              <h3>People</h3>
              <div class="row">
                <router-link :to="{name: 'NewEntity', params: {tablename: 'user'}}"
                             style="min-width: 120px; height: 90px; font-size: 20px"
                             class="btn btn-lg btn-app">
                  <i style="font-size: 30px" class="fa fa-3x fa-user-plus black"></i>Create new user
                </router-link>

                <router-link :to="{name: 'NewEntity', params:{tablename: 'usergroup'}}"
                             style="min-width: 120px; height: 90px; font-size: 20px"
                             class="btn btn-lg btn-app">
                  <i style="font-size: 30px" class="fa fa-3x fa-users black"></i>Create new user group
                </router-link>

                <router-link :to="{name : 'Action', params: {tablename: 'world', actionname: 'invoke_become_admin'}}"
                             style="min-width: 120px; height: 90px; font-size: 20px" class="btn btn-lg btn-app">
                  <i style="font-size: 30px" class="fa fa-3x fa-expeditedssl black"></i>Become admin
                </router-link>

              </div>


              <h3>Others</h3>
              <div class="row">
                <router-link :to="{name: 'Entity', params:{tablename: 'data_exchange'}}"
                             style="min-width: 120px; height: 90px; font-size: 20px"
                             class="btn btn-lg btn-app">
                  <i style="font-size: 30px" class="fa fa-3x fa-exchange black"></i>Data Exchange
                </router-link>

                <router-link :to="{name: 'Entity', params:{tablename: 'oauth_token'}}"
                             style="min-width: 120px; height: 90px; font-size: 20px"
                             class="btn btn-lg btn-app">
                  <i style="font-size: 30px" class="fa fa-3x fa-key black"></i>Oauth Tokens
                </router-link>

                <router-link :to="{name: 'Entity', params:{tablename: 'oauth_connect'}}"
                             style="min-width: 120px; height: 90px; font-size: 20px"
                             class="btn btn-lg btn-app">
                  <i style="font-size: 30px" class="fa fa-3x fa-institution black"></i>Oauth Connections
                </router-link>

                <router-link :to="{name: 'Entity', params:{tablename: 'cloud_store'}}"
                             style="min-width: 120px; height: 90px; font-size: 20px"
                             class="btn btn-lg btn-app">
                  <i style="font-size: 30px" class="fa fa-3x fa-cloud black"></i>Storage
                </router-link>

                <router-link :to="{name: 'Entity', params:{tablename: 'site'}}"
                             style="min-width: 120px; height: 90px; font-size: 20px"
                             class="btn btn-lg btn-app">
                  <i style="font-size: 30px" class="fa fa-3x fa-television black"></i>Sub sites
                </router-link>
                <router-link :to="{name: 'Entity', params:{tablename: 'stream'}}"
                             style="min-width: 120px; height: 90px; font-size: 20px"
                             class="btn btn-lg btn-app">
                  <i style="font-size: 30px" class="fa fa-3x fa-film black"></i>Data views
                </router-link>
                <router-link :to="{name: 'Entity', params:{tablename: 'json_schema'}}"
                             style="min-width: 120px; height: 90px; font-size: 20px"
                             class="btn btn-lg btn-app">
                  <i style="font-size: 30px" class="fa fa-3x fa-puzzle-piece black"></i>Field Json Schema
                </router-link>

              </div>


            </div>
          </div>


        </div>


      </div>

      <div class="row">
      </div>
    </section>
    <!-- /.content -->
  </div>

</template>

<script>
  import jsonApi from '../../plugins/jsonapi'
  import actionManager from '../../plugins/actionmanager'
  import worldManager from '../../plugins/worldmanager'

  export default {
    data() {
      return {
        worldActions: {},
        actionGroups: {},
        generateRandomNumbers(numbers, max, min) {
          var a = []
          for (var i = 0; i < numbers; i++) {
            a.push(Math.floor(Math.random() * (max - min + 1)) + max)
          }
          return a
        }
      }
    },
    computed: {
      sortedWorldActions: function () {

        console.log("return sorted world actions", this.worldActions)
        var keys = Object.keys(this.worldActions);

        keys.sort();

        var res = {};


        for (var key in keys) {
          res[key] = this.worldActions[key];
        }

        console.log("returning sorted worlds", res);
        return res;
      }
    },
    methods: {
      stringToColor(str) {
//        console.log("String to color", str, window.stringToColor(str))
        return "#" + window.stringToColor(str)
      },
    },
    mounted() {

      var that = this;
      that.$route.meta.breadcrumb = [
        {
          label: 'Dashboard'
        }
      ];
      var newWorldActions = {};
      jsonApi.all("world").get({
        page: {
          number: 1,
          size: 200,
        }
      }).then(function (worlds) {

        var actionGroups = {
          "System": [],
          "User": []
        };
        console.log("worlds in dashboard", worlds);
        for (var i = 0; i < worlds.length; i++) {
          var tableName = worlds[i].table_name;
          var actions = actionManager.getActions(tableName);

//          console.log("actions for ", tableName, actions)
          if (!actions) {
            continue
          }
          var actionKeys = Object.keys(actions);
          for (var j = 0; j < actionKeys.length; j++) {
            var action = actions[actionKeys[j]];
//            console.log("dashboard action", action)
            var onType = action.onType;
            var onWorld = worldManager.getWorldByName(onType)
//            console.log("on world", onWorld)

            if (onWorld.is_hidden == "1") {
              actionGroups["System"].push(action)
            } else if (onWorld.table_name == "user") {
              actionGroups["User"].push(action)
            } else if (onWorld.table_name == "usergroup") {
              actionGroups["User"].push(action)
            } else {
              if (!newWorldActions[onWorld.table_name]) {
                newWorldActions[onWorld.table_name] = [];
              }
              newWorldActions[onWorld.table_name].push(action)
            }
          }
        }

        that.worldActions = newWorldActions;
        that.actionGroups = actionGroups;
      });


    }
  }
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
