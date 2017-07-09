<template>
  <!-- Main content -->
  <section class="content">
    <!-- Info boxes -->
    <div class="row">
      <div class="col-md-12">
        <h3>Dashboard is empty for now. You can take a tour.</h3>
      </div>
    </div>
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
                <router-link :style="'color: ' + stringToColor(action.name)"
                             :to="{name: 'Action', params: {tablename: action.onType, actionname: action.name}}">
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
                <router-link :style="'color: ' + stringToColor(world.name)"
                             :to="{name: 'Action', params: {tablename: world.onType, actionname: world.name}}">
                  {{world.label}}
                </router-link>
              </li>

            </ul>
          </div>
          <!-- /.box-body -->
        </div>


      </div>

    </div>

    <div class="row">
      <div class="col-md-12">

        <router-link :to="{name: 'NewEntity', params: {tablename: 'world'}}"
                     style="min-width: 120px; height: 90px; font-size: 20px" class="btn btn-lg btn-app">
          <i style="font-size: 30px" class="fa fa-3x fa-plus green"></i>New Entity
        </router-link>

        <router-link :to="{name: 'NewEntity', params: {tablename: 'data_exchange'}}"
                     style="min-width: 120px; height: 90px; font-size: 20px" class="btn btn-lg btn-app">
          <i style="font-size: 30px" class="fa fa-3x fa-level-up orange"></i>Add Export
        </router-link>

        <a style="min-width: 120px; height: 90px; font-size: 20px" class="btn btn-lg btn-app">
          <i style="font-size: 30px" class="fa fa-3x fa-level-down maroon"></i>Add Import
        </a>

        <a style="min-width: 120px; height: 90px; font-size: 20px" class="btn btn-lg btn-app">
          <i style="font-size: 30px" class="fa fa-3x fa-upload yellow"></i>Upload Csv/Xls
        </a>

      </div>
    </div>
  </section>
  <!-- /.content -->
</template>

<script>
  import jsonApi  from '../../plugins/jsonapi'
  import actionManager from '../../plugins/actionmanager'
  import worldManager from '../../plugins/worldmanager'

  export default {
    data () {
      return {
        worldActions: {},
        actionGroups: {},
        generateRandomNumbers (numbers, max, min) {
          var a = []
          for (var i = 0; i < numbers; i++) {
            a.push(Math.floor(Math.random() * (max - min + 1)) + max)
          }
          return a
        }
      }
    },
    computed: {},
    methods: {
      stringToColor (str) {
//        console.log("String to color", str, window.stringToColor(str))
        return "#" + window.stringToColor(str)
      },
    },
    mounted () {

      var that = this;
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

          console.log("actions for ", tableName, actions)
          if (!actions) {
            continue
          }
          var actionKeys = Object.keys(actions);
          for (var j = 0; j < actionKeys.length; j++) {
            var action = actions[actionKeys[j]];
            console.log("dashboard action", action)
            var onType = action.onType;
            var onWorld = worldManager.getWorldByName(onType)
            console.log("on world", onWorld)

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
