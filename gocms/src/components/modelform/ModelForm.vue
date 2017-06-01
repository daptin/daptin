<template>

  <div class="ui one column grid">
    <!--<div class="ui segment top">-->
    <!--<form class="form ui" @submit.prevent="saveRow(model)">-->

    <!--<div class="field" v-for="col in formBuildData">-->

    <!--<label :for="col"> {{col.label}} </label>-->
    <!--<input class="form-control" :id="col" :value="model[col.name]" v-model="model[col.name]">-->

    <!--</div>-->
    <!--<el-button @click.prevent="saveRow(model)">-->
    <!--Save-->
    <!--</el-button>-->
    <!--<el-button @click="cancel()">Cancel</el-button>-->
    <!--</form>-->
    <!--</div>-->
    <div class="ui column">
      <vue-form-generator :schema="formModel" :model.sync="localModel"></vue-form-generator>
    </div>
    <div class="ui column">
      <el-button @click.prevent="saveRow(localModel)">
        Save
      </el-button>
      <el-button @click="cancel()">Cancel</el-button>

    </div>
  </div>

</template>

<script>
  import VueFormGenerator from "vue-form-generator";
  import 'vue-form-generator/dist/vfg.css'

  export default {
    props: [
      "model",
      "meta"
    ],
    components: {
      "vue-form-generator": VueFormGenerator.component
    },
    data: function () {
//      console.log("this data", this);
//      console.log(arguments);
//      console.log(this.model);
      return {
        currentElement: "el-input",
        formBuildData: [],
        previousSubmit: null,
        formModel: null,
        localModel: null,
      }
    },
    created () {
      this.localModel = this.model;
    },
    computed: {},
    methods: {
      titleCase: function (str) {
        return str.replace(/[-_]/g, " ").split(' ')
            .map(w => w[0].toUpperCase() + w.substr(1).toLowerCase()
            ).join(' ')
      },
      saveRow: function () {
        console.log("save row");
        this.$emit('save', this.model)
        this.previousSubmit = this.model;
        this.localModel = {};
      },
      cancel: function () {
        console.log("canel row");
        this.$emit('cancel')
      },
      endsWith: function (str1, str2) {
        if (str1.length < str2.length) {
          return false;
        }
        if (str1.substring(str1.length - str2.length) == str2) {
          return true;
        }
        return false;
      },
      reinit: function () {
        var that = this;

        var colKeys = Object.keys(this.meta);
        var formModel = {fields: []};

        console.log("model form", this.meta, that.model, that.model["arguments"]);

        for (var i = 0; i < colKeys.length; i++) {

          var column = colKeys[i];
          var colMeta = this.meta[column];
          var label = this.titleCase(column);
          var formField = {
            type: "input",
            inputType: "text",
            label: label,
            model: column
          };


          if (!that.model[column]) {
            that.model[column] = "";
          }

          if (typeof colMeta == "string") {
            colMeta = {
              name: column,
              columnType: colMeta,
            }
          } else {
            colMeta.name = column
          }

          if (colMeta.columnType == "datetime") {
            continue;
          }

          if (colMeta.columnType == "entity") {
            continue;
          }

          if (colMeta.name == "status" || colMeta.name == "pending" || colMeta.name == "permission" || colMeta.name == "reference_id") {
            continue
          }

          if (colMeta.columnType == "content") {
            formField.type = "textArea"
            formField.rows = "5"
          } else {
            formField.type = "input"
          }

          if (colMeta.columnType == "label") {
            formModel.fields.unshift(formField);
          } else {

            formModel.fields.push(formField);
          }



          console.log("that model", that.model)
        }
        that.formModel = formModel;
      }
    },
    mounted: function () {
      this.reinit()
    },
    watch: {},
  }
</script>s
