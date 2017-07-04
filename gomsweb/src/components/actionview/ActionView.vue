<template>

  <div class="box" v-if="action">
    <div v-if="!hideTitle" class="box-header">
      <div class="box-title">
        <h1> {{action.label}}</h1>
      </div>
    </div>
    <div class="box-body">
      <div class="col-md-12" v-if="!finalModel && !action.instanceOptional">
        <select-one-or-more :value="finalModel" :schema="modelSchema" @save="setModel"></select-one-or-more>
      </div>
      <div class="col-md-12">
        <model-form :hide-title="true" :hide-cancel="hideCancel" v-if="meta != null" @save="doAction(data)"
                    @cancel="cancel()" :meta="meta"
                    :model.sync="data"></model-form>
      </div>
    </div>
  </div>
</template>

<script>


  import actionManager from '../../plugins/actionmanager'
  import jsonApi from '../../plugins/jsonapi'
  import {Notification} from "element-ui"

  export default {
    props: {
      hideTitle: {
        type: Boolean,
        required: false,
        default: false
      },
      hideCancel: {
        type: Boolean,
        required: false,
        default: false
      },
      action: {
        type: Object,
        required: true
      },
      model: {
        type: Object,
        required: false,
        default: function () {
          return null
        }
      },
      actionManager: {
        type: Object,
        required: true
      }
    },
    data: function () {
      return {
        meta: null,
        data: {},
        modelSchema: {},
        finalModel: null,
      }
    },
    created () {
    },
    computed: {},
    methods: {
      setModel (m1){
        console.log("set model", m1)
        this.finalModel = m1;
      },
      doAction(actionData){
        var that = this;

        if (!this.finalModel && !this.action.instanceOptional) {
          Notification.error({
            title: "Error",
            message: "Please select a " + this.action.onType,
          });
          return
        }
        console.log("perform action", actionData, this.finalModel);
        if (this.finalModel && Object.keys(this.finalModel).indexOf("id") > -1) {
          actionData[this.action.onType + "_id"] = this.finalModel["id"]
        } else {
        }
        that.actionManager.doAction(that.action.onType, that.action.name, actionData).then(function () {
          that.$emit("action-complete", that.action);
        }, function () {
          console.log("not clearing out the form")
        });
      },
      cancel() {
        this.$emit("cancel");
      },
      init() {

        if (!this.action) {
          return;
        }


        var that = this;
        console.log("render action ", that.action, " on ", that.model);
        that.finalModel = that.model;
        var worldName = that.action.onType;
        var worldSchema = jsonApi.modelFor(worldName);

        that.modelSchema = {
          inputType: worldName,
          value: null,
          multiple: false,
          name: that.action.onType,
        };

        var meta = {};

        for (var i = 0; this.action.fields && i < this.action.fields.length; i++) {
          meta[this.action.fields[i].ColumnName] = that.action.fields[i]
        }

        if (this.action.fields && this.action.fields.length == 0 && this.action.instanceOptional) {

          var payload = {};

          if (this.finalModel && this.finalModel["id"]) {
            payload[this.action.onType + "_id"] = this.finalModel["id"];
          }

          this.actionManager.doAction(this.action.onType, this.action.name, payload).then(function () {
          }, function () {

          });
          setTimeout(function () {
            that.$emit("cancel");
          }, 400);
        }

        this.meta = meta;
      },
    },
    mounted: function () {
      console.log("Mounted action view");
      this.init();
    },
//    updated: function () {
//      console.log("Updated action view");
//      this.init();
//    },
    watch: {
      'action': function (newValue) {
        console.log("ActionView: action changed");
//        this.action = actionManager.getActionModel(this.$route.params.tablename, newValue);
        this.init();
      },
//      '$route.params.actionname': function (newValue) {
//        console.log("ActionView: action changed");
//        this.action = actionManager.getActionModel(this.$route.params.tablename, newValue);
//        this.init();
//      },
//      '$route.params.tablename': function (newValue) {
//        console.log("ActionView: world changed");
//        this.action = actionManager.getActionModel(newValue, this.$route.params.actionname);
//        this.init();
//      },
    },
  }
</script>s
