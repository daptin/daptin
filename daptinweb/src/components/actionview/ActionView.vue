<template>

  <div class="box" v-if="action">
    <div v-if="!hideTitle" class="box-header">
      <div class="box-title">
        <h1> {{action.Label}}</h1>
      </div>
    </div>
    <div class="box-body">
      <div class="col-md-12" v-if="!finalModel && !action.InstanceOptional">
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


  // import actionManager from '../../plugins/actionmanager'
  // import jsonApi from '../../plugins/jsonapi'
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
      },
      values: {
        type: Object,
        required: false
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
    created() {
    },
    computed: {},
    methods: {
      setModel(m1) {
        console.log("set model", m1)
        this.finalModel = m1;
      },
      doAction(actionData) {
        var that = this;

        if (!this.finalModel && !this.action.InstanceOptional) {
          Notification.error({
            title: "Error",
            message: "Please select a " + this.action.OnType,
          });
          return
        }
        console.log("perform action", actionData, this.finalModel);
        if (this.finalModel && Object.keys(this.finalModel).indexOf("id") > -1) {
          actionData[this.action.OnType + "_id"] = this.finalModel["id"]
        } else {
        }
        that.actionManager.doAction(that.action.OnType, that.action.Name, actionData).then(function () {
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

        if (that.values) {
          console.log("values", that.values);
          var keys = Object.keys(that.values);
          for (var i = 0; i < keys.length; i++) {
            let key = keys[i];
            that.model[key] = that.values[key]
          }

        }

        console.log("render action ", that.action, " on ", that.model);


        that.finalModel = that.model;
        var worldName = that.action.OnType;
        that.modelSchema = {
          inputType: worldName,
          value: null,
          multiple: false,
          name: that.action.OnType,
        };

        var meta = {};

        for (var i = 0; this.action.InFields && i < this.action.InFields.length; i++) {
          meta[this.action.InFields[i].ColumnName] = that.action.InFields[i]
        }

        if (this.action.InFields && this.action.InFields.length == 0 && this.action.InstanceOptional) {

          var payload = this.model;
          if (!payload) {
            payload = {};
          }

          if (this.finalModel && this.finalModel["id"]) {
            payload[this.action.OnType + "_id"] = this.finalModel["id"];
          }

          this.actionManager.doAction(this.action.OnType, this.action.Name, payload).then(function () {
          }, function () {

          });
          setTimeout(function () {
            that.$emit("cancel");
          }, 400);
        }
        console.log("action meta", meta);
        this.meta = meta;
      },
    },
    mounted: function () {
      console.log("Mounted action view");
      this.init();

    },
    watch: {
      'action': function (newValue) {
        console.log("ActionView: action changed");
        this.init();
      },
    },
  }
</script>s
