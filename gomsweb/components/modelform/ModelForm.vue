<template>

  <div class="ui one column grid">
    <div class="ui column" v-if="title">
      {{title}}
    </div>
    <div class="ui column">
      <vue-form-generator :schema="formModel" :model="model"></vue-form-generator>
    </div>
    <div class="ui column">
      <el-button @click.prevent="saveRow()">
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
    props: {
      model: {
        type: Object,
        required: false,
        default: function () {
          return {}
        }
      },
      jsonApi: {
        type: Object,
        required: true
      },
      meta: {
        type: Object,
        required: true,
      },
      title: {
        type: String,
        required: false,
      }
    },
    components: {
      "vue-form-generator": VueFormGenerator.component
    },
    data: function () {
      return {
        formModel: null,
        formValue: {},
      }
    },
    methods: {
      saveRow: function () {
        console.log("save row", this.model);
        this.$emit('save', this.model)
      },
      cancel: function () {
        console.log("canel row");
        this.$emit('cancel')
      },
      titleCase: function (str) {
        if (!str) {
          return str;
        }
        return str.replace(/[-_]/g, " ").trim().split(' ')
          .map(w => w[0].toUpperCase() + w.substr(1).toLowerCase()).join(' ')
      }
    },
    mounted: function () {
      var that = this;
      var formFields = [];
      console.log("that mode", that.model)
      that.formValue = that.model;

      console.log("model form for ", this.meta);
      var columns = Object.keys(this.meta);
      that.formModel = {};


      var skipColumns = [
        "reference_id",
        "id",
        "updated_at",
        "created_at",
        "deleted_at",
        "status",
        "permission",
        "user_id",
        "usergroup_id"
      ];

      for (var i = 0; i < columns.length; i++) {

        var columnName = columns[i];


        var columnMeta = that.meta[columnName];

        var columnLabel = that.titleCase(columns[i]);


        if (columnMeta.columnType && columnMeta.columnType == "entity") {
          console.log("Skip relation", columnName);
          continue;
        }


        var hint = "";
        skipColumns.indexOf(columnLabel.ColumnName)

        if (columnMeta.DefaultValue) {
          continue;
        }

        if (columnMeta.IsForeignKey) {
          continue;
        }

        if (columnMeta.ColumnType == "hidden") {
          continue;
        }

        if (skipColumns.indexOf(columnName) > -1) {
          continue
        }

        console.log("Add column model ", columnName, columnMeta);

        var field = {
          type: "input",
          inputType: "text",
          label: columnLabel,
          model: columnMeta.ColumnName,
          id: "id",
          readonly: false,
          value: columnName.DefaultValue,
          featured: true,
          disabled: false,
          required: !columnName.IsNullable,
          "default": columnName.DefaultValue,
          validator: null,
          onChanged: function (model, newVal, oldVal, field) {
            console.log(`Model's name changed from ${oldVal} to ${newVal}. Model:`, model);
          },
          onValidated: function (model, errors, field) {
            if (errors.length > 0)
              console.warn("Validation error in Name field! Errors:", errors);
          }
        }
        formFields.push(field)
      }
      console.log("all form fields", formFields)
      that.formModel.fields = formFields;

    },
    watch: {
      "model": function () {
        var that = this;
        console.log("moidel changed", that.model)
        that.formValue = that.model;
      }
    },
  }
</script>s
