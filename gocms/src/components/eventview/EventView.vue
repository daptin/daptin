<template>

  <div class="ui one column grid">

    <div class="ui column">
      <h3>{{action.label}}</h3>
    </div>
    <div class="ui column">
      <model-form @save="doAction(data)" @cancel="cancel()" :meta="meta" :model.sync="data"
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

        console.log("perform action", actionData, this.model["id"], this.model)
        actionData[this.action.onType + "_id"] = this.model["id"]
        this.actionManager.doAction(this.action.onType, this.action.name, actionData);
      },
      cancel() {
        this.$emit("cancel");
      },
    },
    mounted: function () {
      var modelName = "_actionmodel_" + this.action.name;
      console.log("render action ", this.action, " on ", this.model);


      var jsonApiModel = this.jsonApi.modelFor(modelName);
      if (!jsonApiModel) {

        var fieldMap = {};
        for (var i = 0; i < this.action.fields.length; i++) {
          fieldMap[this.action.fields[i].ColumnName] = this.action.fields[i];
        }

        jsonApiModel = GetJsonApiModel(fieldMap);
        console.log("new json model defiintion ", jsonApiModel)
        this.jsonApi.define(modelName, jsonApiModel);
      } else {
        jsonApiModel = jsonApiModel["attributes"];
      }

      console.log("build meta", modelName, jsonApiModel)
      this.meta = this.jsonApi.modelFor(modelName)["attributes"];
    },
    watch: {},
  }
</script>s
