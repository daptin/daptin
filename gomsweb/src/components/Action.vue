<template>
  <div class="ui three column grid">

    <div class="column">

    </div>

    <div class="column">
      <h1 class="header" v-if="action">
        {{action.label}}
      </h1>

      <action-view ref="systemActionView" v-if="action" :hide-title="true" @cancel="cancel"
                   :action-manager="actionManager"
                   :action="action"
                   :json-api="jsonApi"></action-view>


    </div>


  </div>
</template>

<script>

  import worldManager from "../plugins/worldmanager"
  import actionManager from "../plugins/actionmanager"
  import jsonApi from "../plugins/jsonapi"


  export default {
    middleware: 'authenticated',
    data: function () {
      return {
        action: null,
        jsonApi: jsonApi,
        tablename: null,
        actionname: null,
        actionManager: actionManager,
      }
    },
    methods: {
      cancel: function () {
        console.log("cancel action")
        this.$router.push({
          name: "Entity",
          params: {
            tablename: this.tablename,
          }
        });
      },
      init() {
        this.action = actionManager.getActionModel(this.tablename, this.actionname);

      }
    },
    mounted () {
      console.log("loaded action view", this.$route.params);
      this.tablename = this.$route.params.tablename;
      this.actionname = this.$route.params.actionname;
      this.init();
    },
    watch: {
      '$route.params.actionname': function (newActionName) {
        console.log("New action name", newActionName)
        this.actionname = newActionName;
        this.init();
      },
      '$route.params.tablename': function (newTableName) {
        console.log("New action name", newTableName)
        this.tablename = newTableName;
        this.init();
      }
    }
  }
</script>
