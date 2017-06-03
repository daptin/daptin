<template>

  <div class="ui one column grid">
    <!-- EventView -->

    <div class="ui column">
      <h3>{{action.label}}</h3>
    </div>
    <div class="ui column">
      <model-form @save="doAction(data)" :json-api="jsonApi" @cancel="cancel()" :meta="meta" :model.sync="data"
                  v-if="data != null && meta != null"></model-form>
    </div>

  </div>

</template>

<script>
  export default {
    props: {
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
        required: true
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
        console.log("perform action", actionData, this.model["id"], this.model)
        actionData[this.action.onType + "_id"] = this.model["id"]
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

      for(var i=0;i<this.action.fields.length;i++) {
        meta[this.action.fields[i].ColumnName] = this.action.fields[i]
      }

      this.meta = meta;
    },
    watch: {},
  }
</script>s
