<template>

  <div class="row">
    <div class="col-md-12" v-if="!hideTitle">
      {{title}}
    </div>
    <div class="col-md-12">
      <vue-form-generator :schema="formModel" :model="model"></vue-form-generator>
    </div>
    <div class="col-md-12">
      <el-button type="submit" :class="loading" @click.prevent="saveRow()"> Submit </el-button>
      <el-button v-if="!hideCancel" @click="cancel()">Cancel</el-button>
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
        loading: "",
      }
    },
    methods: {
      getTextInputType(columnMeta) {
        let inputType = columnMeta.ColumnType;

        if (inputType.indexOf(".") > 0) {
          var inputTypeParts = inputType.split(".")
          if (inputTypeParts[0] == "file") {
            return inputTypeParts[1];
          }
        }


        switch (inputType) {
          case "hidden":
            inputType = "hidden";
            break;
          case "password":
            inputType = "password";
            break;
          case "content":
            inputType = "";
            break;
          case "json":
            inputType = "";
            break;
          default:
            inputType = "text";
            break;
        }
        return inputType;
      },
      getInputType(columnMeta) {
        let inputType = columnMeta.ColumnType;

        if (inputType.indexOf(".") > 0) {
          var inputTypeParts = inputType.split(".");
          if (inputTypeParts[0] == "file") {
            return "fileUpload";
          }
        }

        switch (inputType) {
          case "truefalse":
            inputType = "checkbox";
            break;
          case "content":
            inputType = "textArea";
            break;
          case "json":
            inputType = "textArea";
            break;
          default:
            inputType = "input";
            break;
        }
        return inputType;
      },
      saveRow: function () {
        console.log("save row", this.model);
        this.loading = "loading";
        this.$emit('save', this.model)
      },
      cancel: function () {
        this.$emit('cancel')
      },
      titleCase: function (str) {
        if (!str) {
          return str;
        }
        return str.replace(/[-_]/g, " ").trim().split(' ')
          .map(w => w[0].toUpperCase() + w.substr(1).toLowerCase()).join(' ')
      },
      init() {


        // todo: convert strings to booleans and numbers

        var that = this;
        var formFields = [];
        console.log("that mode", that.model);
        that.formValue = that.model;

        console.log("model form for ", this.meta);
        var columnsKeys = Object.keys(this.meta);
        that.formModel = {};


        var skipColumns = [
          "reference_id",
          "id",
          "updated_at",
          "created_at",
          "deleted_at",
          "status",
          "user_id",
          "usergroup_id"
        ];

        formFields = columnsKeys.map(function (columnName) {


//        const columnName = columns[i];
          if (skipColumns.indexOf(columnName) > -1) {
            return null
          }

          const columnMeta = that.meta[columnName];
          const columnLabel = that.titleCase(columnMeta.Name);

          if (columnMeta.columnType && columnMeta.columnType === "entity") {
            console.log("Skip relation", columnName);
            return null;
          }

          if (columnMeta.ColumnType == "hidden") {
            return null;
          }


          let inputType = that.getInputType(columnMeta);
          const textInputType = that.getTextInputType(columnMeta);


          console.log("Add column model ", columnName, columnMeta);

          return {
            type: inputType,
            inputType: textInputType,
            label: columnLabel,
            model: columnMeta.ColumnName,
            name: columnMeta.ColumnName,
            id: "id",
            readonly: false,
            value: columnName.DefaultValue,
            featured: true,
            disabled: !!columnName.DefaultValue,
            required: !columnName.IsNullable,
            "default": columnName.DefaultValue,
            validator: null,
            onChanged: function (model, newVal, oldVal, field) {
//              console.log(`Model's name changed from ${oldVal} to ${newVal}. Model:`, model);
            },
            onValidated: function (model, errors, field) {
              if (errors.length > 0)
                console.warn("Validation error in Name field! Errors:", errors);
            }
          };
        }).filter(function (e) {
          return !!e
        });


        console.log("all form fields", formFields);
        that.formModel.fields = formFields;
      }
    },
    mounted: function () {
      this.init();

    },
    watch: {
      "model": function () {
        var that = this;
        console.log("ModelForm: model changed", that.model);
        this.init();
      },
      "meta": function () {
        var that = this;
        console.log("ModelForm: meta changed", that.meta);
        this.init();
      },

    },
  }
</script>s
