<template>

  <!-- Content Wrapper. Contains page content -->
  <div class="content-wrapper">
    <!-- Content Header (Page header) -->
    <section class="content-header">
      <h1>
        <small>{{ $route.actionname }}</small>
      </h1>
      <ol class="breadcrumb">
        <li>
          <a href="javascript:;">
            <i class="fa fa-home"></i>Home</a>
        </li>
        <li class="active">{{$route.name.toUpperCase()}}</li>
      </ol>

    </section>
    <section class="content">


      <div class="col-md-12">
        <action-view ref="systemActionView" v-if="action" :hide-title="false" @cancel="cancel"
                     :action-manager="actionManager"
                     :action="action" :model="model"
                     :json-api="jsonApi"></action-view>

      </div>
      <h3 v-if="!action">404, Action not found</h3>


    </section>
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
        model: {},
        actionname: null,
        actionManager: actionManager,
      }
    },
    methods: {
      cancel: function () {
        console.log("cancel action")
        window.history.back();
//        this.$router.push({
//          name: "Entity",
//          params: {
//            tablename: this.tablename,
//          }
//        });
      },
      init() {
        this.model = this.$route.query;
        console.log("action model", this.model)
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
