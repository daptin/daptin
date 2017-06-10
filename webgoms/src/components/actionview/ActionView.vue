<template>

  <div class="ui one column grid" v-if="action">
    <!-- EventView -->

    <div class="ui column" v-if="showTitle">
      <h3>{{action.label}}</h3>
    </div>

    <model-form v-if="meta != null" @save="doAction(data)" :json-api="jsonApi" @cancel="cancel()" :meta="meta"
                :model.sync="data"></model-form>


  </div>

</template>

<script>
  export default {
    props: {
      showTitle: {
        type: Boolean,
        required: false,
        default: true,
      },
      action: {
        type: Object,
        required: true
      },
      jsonApi: {
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
        console.log("perform action", actionData, this.model)
        if (this.model && Object.keys(this.model).indexOf("id") > -1) {
          actionData[this.action.onType + "_id"] = this.model["id"]
        }
        this.actionManager.doAction(this.action.onType, this.action.name, actionData).then(function () {
          that.$emit("cancel");
        }, function () {
          console.log("not clearing out the form")
        });
      },
      cancel() {
        this.$emit("cancel");
      },
      init() {
        var that = this;
        var modelName = "_actionmodel_" + that.action.name;
        console.log("render action ", that.action, " on ", that.model);

        var meta = {};

        for (var i = 0; i < this.action.fields.length; i++) {
          meta[this.action.fields[i].ColumnName] = that.action.fields[i]
        }

        if (this.action.fields.length == 0) {
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
      this.init();
    },
    watch: {
      'action': function () {
        console.log("ActionView: action changed")
        this.init();
      },
    },
  }
</script>s
