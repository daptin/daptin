<template>

  <div class="ui one column grid" v-if="action">
    <!-- EventView -->

    <div class="ui column" v-if="showTitle">
      <h3>{{action.label}}</h3>
    </div>

    <model-form @save="doAction(data)" :json-api="jsonApi" @cancel="cancel()" :meta="meta" :model.sync="data"
                v-if="data != null && meta != null"></model-form>


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
        required: false
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
    },
    mounted: function () {
      var modelName = "_actionmodel_" + this.action.name;
      console.log("render action ", this.action, " on ", this.model);

      var meta = {};

      for (var i = 0; i < this.action.fields.length; i++) {
        meta[this.action.fields[i].ColumnName] = this.action.fields[i]
      }

      this.meta = meta;
    },
    watch: {},
  }
</script>s
