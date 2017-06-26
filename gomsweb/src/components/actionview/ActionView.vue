<template>

  <div class="box" v-if="action">
    <div v-if="!hideTitle" class="box-head">
      <div class="box-title">
        <h3 class="text-center"> {{action.label}}</h3>
      </div>
    </div>
    <div class="box-body">
      <div class="col-md-12">
        <model-form :hide-title="true" :hide-cancel="hideCancel" v-if="meta != null" @save="doAction(data)" @cancel="cancel()" :meta="meta"
                    :model.sync="data"></model-form>
      </div>
    </div>
  </div>
</template>

<script>


  import actionManager from '../../plugins/actionmanager'

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
          return {}
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
      }
    },
    created () {
    },
    computed: {},
    methods: {
      doAction(actionData){
        var that = this;
        console.log("perform action", actionData, this.model);
        if (this.model && Object.keys(this.model).indexOf("id") > -1) {
          actionData[this.action.onType + "_id"] = this.model["id"]
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
        var modelName = "_actionmodel_" + that.action.name;
        console.log("render action ", that.action, " on ", that.model);

        var meta = {};

        for (var i = 0; this.action.fields && i < this.action.fields.length; i++) {
          meta[this.action.fields[i].ColumnName] = that.action.fields[i]
        }

        if (this.action.fields && this.action.fields.length == 0) {
          this.actionManager.doAction(this.action.onType, this.action.name, {}).then(function () {
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
