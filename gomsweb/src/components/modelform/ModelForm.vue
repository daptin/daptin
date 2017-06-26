<template>

  <div class="box">
    <div class="box-header" v-if="!hideTitle">
      <div class="box-title">
        {{title}}
      </div>
    </div>
    <div class="box-body">
      <div :class="{'col-md-6': relations.length > 0, 'col-md-12': relations.length == 0 }">
        <vue-form-generator :schema="formModel" :model="model"></vue-form-generator>
      </div>
      <div class="col-md-6" v-if="relations.length > 0">

        <div class="row">
          <div class="col-md-12" v-for="item in relations">
            <select-one-or-more :value="item.value" :schema="item" @save="setRelation"></select-one-or-more>
          </div>
        </div>

      </div>
    </div>
    <div class="box-footer">
      <el-button class="bg-yellow" type="submit" v-loading.body="loading" @click.prevent="saveRow()"> Submit
      </el-button>
      <el-button class="bg-red" v-if="!hideCancel" @click="cancel()">Cancel</el-button>
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
      title: {
        type: String,
        required: false,
        default: ""
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
        loading: false,
        relations: [],
      }
    },
    methods: {
      setRelation(item){
        console.log("save relation", item);

        var meta = this.meta[item.name];

        if (meta.jsonApi == "hasOne") {

          this.model[item.name] = {
            type: meta.ColumnType,
            id: item.id
          }
        } else {
          this.model[item.name] = [
            {
              type: meta.ColumnType,
              id: item.id
            }
          ]
        }
      },
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
          case "entity":
            inputType = columnMeta.type;
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
          case "entity":
            inputType = "selectOneOrMore";
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
        var that = this;
        console.log("save row", this.model);
        this.loading = true;
        setTimeout((function () {
          that.loading = false;
        }), 3000);
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

        var foreignKeys = [];

        formFields = columnsKeys.map(function (columnName) {


//        const columnName = columns[i];
          if (skipColumns.indexOf(columnName) > -1) {
            return null
          }

          const columnMeta = that.meta[columnName];
          columnMeta.ColumnName = columnName;
          const columnLabel = that.titleCase(columnName);

          if (columnMeta.columnType && columnMeta.columnType === "entity") {
            columnMeta.ColumnType = columnMeta.columnType;
//            console.log("Skip relation", columnName);
//            return null;
          }

          if (columnMeta.ColumnType == "hidden") {
            return null;
          }


          if (!that.model[columnMeta.ColumnName] && columnMeta.DefaultValue) {

            if (columnMeta.DefaultValue[0] == "'") {
              that.model[columnMeta.ColumnName] = columnMeta.DefaultValue.substring(1, columnMeta.DefaultValue.length - 1);
            } else {
              that.model[columnMeta.ColumnName] = columnMeta.DefaultValue;
            }

          }

          if (columnMeta.ColumnType == "truefalse") {
            that.model[columnMeta.ColumnName] = that.model[columnMeta.ColumnName] === "1" ? true : false;
          }


          let inputType = that.getInputType(columnMeta);
          const textInputType = that.getTextInputType(columnMeta);


          console.log("Add column model ", columnName, columnMeta);

          var resVal = {
            type: inputType,
            inputType: textInputType,
            label: columnLabel,
            model: columnMeta.ColumnName,
            name: columnName,
            id: "id",
            readonly: false,
            value: columnMeta.DefaultValue,
            featured: true,
            disabled: false,
            required: !columnMeta.IsNullable,
            "default": columnMeta.DefaultValue,
            validator: null,
            onChanged: function (model, newVal, oldVal, field) {
//              console.log(`Model's name changed from ${oldVal} to ${newVal}. Model:`, model);
            },
            onValidated: function (model, errors, field) {
              if (errors.length > 0)
                console.warn("Validation error in Name field! Errors:", errors);
            }
          };

          if (columnMeta.ColumnType == "entity") {
            if (columnMeta.jsonApi == "hasOne") {
              resVal.value = that.model[resVal.ColumnName];
              resVal.multiple = false;
              foreignKeys.push(resVal);
            } else {
              resVal.value = that.model[resVal.ColumnName];
              resVal.multiple = false;
              foreignKeys.push(resVal);
            }
            return null;
          }

          return resVal;
        }).filter(function (e) {
          return !!e
        });


        console.log("all form fields", formFields, foreignKeys);
        that.formModel.fields = formFields;
        that.relations = foreignKeys;
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
