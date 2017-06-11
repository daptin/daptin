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
          name: "tablename",
          params: {
            tablename: this.tablename,
          }
        });
      }
    },
    mounted () {
      console.log("loaded action view", this.$route.params);
      this.tablename = this.$route.params.tablename;
      this.actionname = this.$route.params.actionname;

      var action = actionManager.getActionModel(this.tablename, this.actionname);


      this.action = action;

    }
  }
</script>
