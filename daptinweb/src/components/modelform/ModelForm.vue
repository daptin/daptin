<template>

  <div class="box action-form-body">
    <div class="box-header" v-if="!hideTitle">
      <div class="box-title">
        {{title}}
      </div>
    </div>
    <div class="box-body">
      <div
        :class="{'col-md-12': relations.length == 0 && !hasPermissionField, 'col-md-6': relations.length > 0 || hasPermissionField }">
        <vue-form-generator :schema="formModel" :model="model"></vue-form-generator>
      </div>
      <div class="col-md-3" v-if="relations.length > 0">
        <div class="row">
          <div class="col-md-12" v-bind:key="item.value" v-for="item in relations">
            <select-one-or-more :value="item.value" :schema="item" @save="setRelation"></select-one-or-more>
          </div>
        </div>
        <div class="col-md-6" v-if="hasPermissionField && model.reference_id">
          <fieldPermissionInput :value="model.permission"></fieldPermissionInput>
        </div>
      </div>

    </div>
    <div class="box-footer">
      <el-button class="bg-yellow" type="submit" v-loading.body="loading" @click.prevent="saveRow()"> Submit
      </el-button>
      <el-button class="bg-red" v-if="!hideCancel" @click="cancel()">Cancel</el-button>
      <!--<el-button class="bg-orange" v-if="!hideCancel" @click="loadLast()">Load last submission</el-button>-->
    </div>
  </div>

</template>
<style>
  .vue-form-generator fieldset {
    min-width: 100% !important;
  }
</style>
<script>
  import VueFormGenerator from "vue-form-generator";
  //  import 'vue-form-generator/dist/vfg.css'

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
      }
    },
    components: {
      "vue-form-generator": VueFormGenerator.component
    },
    data: function () {
      return {
        formModel: null,
        formValue: {},
        focusSet: false,
        loading: false,
        relations: [],
        hasPermissionField: false,
      }
    },
    methods: {
      loadLast() {
//        var that = this;
//        var stored = window.localStorage.getItem(this.$route.path)
//        if (stored) {
//          var obj = JSON.parse(stored);
//          var keys = Object.keys(obj);
//          for (var i=0;i<keys.length;i++){
//            var key = keys[i];
//            var val = obj[key];
//            that.model[key] = val;
//          }
////          that.model = obj;
//        }
      },
      setRelation(item) {
        console.log("save relation", item);

        const meta = this.meta[item.name];

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
        console.log("get text input type for ", columnMeta);
        if (inputType.indexOf(".") > 0) {
          const inputTypeParts = inputType.split(".");
          if (inputTypeParts[0] == "file") {
            inputTypeParts.shift();
            return inputTypeParts.join(".");
          } else if (inputTypeParts[0] == "audio") {
            inputTypeParts.shift();
            return inputTypeParts.join(".");
          } else if (inputTypeParts[0] == "video") {
            inputTypeParts.shift();
            return inputTypeParts.join(".");
          } else if (inputTypeParts[0] == "image") {
            inputTypeParts.shift();
            return inputTypeParts.join(".");
          } else if (inputTypeParts[0] == "json") {
            inputTypeParts.shift();
            return inputTypeParts.join(".")
          }
        }

        if (["json", "yaml"].indexOf(columnMeta.ColumnType) > -1) {
          console.log("get text input type for json ", this.model);
          return columnMeta.ColumnName
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
          case "measurement":
            inputType = "number";
            break;
          case "date":
            inputType = "date";
            break;
          case "time":
            inputType = "time";
            break;
          case "datetime":
            inputType = "datetime";
            break;
          case "content":
            inputType = "";
            break;
          default:
            inputType = "text";
            break;
        }
        return inputType;
      },
      getInputType(columnMeta) {
        console.log("get input type for", columnMeta);
        let inputType = columnMeta.ColumnType;
        if (inputType.indexOf(".") > 0) {
          const inputTypeParts = inputType.split(".");
          if (inputTypeParts[0] == "file") {
            return "fileUpload";
          }
          if (inputTypeParts[0] == "video") {
            return "fileUpload"
          }
          if (inputTypeParts[0] == "audio") {
            return "fileUpload"
          }
          if (inputTypeParts[0] == "image") {
            return "fileUpload";
          }
        }

        if (columnMeta.ColumnName == "default_permission" || columnMeta.ColumnName == "permission") {
          return "permissionInput";
        }

        switch (inputType) {
          case "truefalse":
            inputType = "fancyCheckBox";
            break;
          case "entity":
            inputType = "selectOneOrMore";
            break;
          case "date":
            inputType = "dateSelect";
            break;
          case "measurement":
            inputType = "input";
            break;
          case "content":
            inputType = "textArea";
            break;
          case "json":
            inputType = "jsonEditor";
            break;
          case "yaml":
            inputType = "textArea";
            break;
          case "html":
            inputType = "textArea";
            break;
          case "markdown":
            inputType = "textArea";
            break;
          default:
            inputType = "input";
            break;
        }
        return inputType;
      },
      saveRow: function () {
        const that = this;
        console.log("save row", this.model);
//        window.localStorage.setItem(that.$route.path, JSON.stringify(this.model))
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

        const that = this;
        let formFields = [];
        console.log("that mode", that.model);
        that.formValue = that.model;

        console.log("model form for ", this.meta);
        const columnsKeys = Object.keys(this.meta);
        that.formModel = {};


        let skipColumns = [
          "reference_id",
          "id",
          "updated_at",
          "created_at",
          "deleted_at",
          "user_id",
          "usergroup_id"
        ];

        let foreignKeys = [];

        formFields = columnsKeys.map(function (columnName) {


//        const columnName = columns[i];
          if (skipColumns.indexOf(columnName) > -1) {
            return null
          }

          const columnMeta = that.meta[columnName];
          columnMeta.ColumnName = columnName;
          const columnLabel = that.titleCase(columnMeta.Name);

          if (columnMeta.columnType && !columnMeta.ColumnType) {
            columnMeta.ColumnType = columnMeta.columnType;
          }

          if (columnMeta.ColumnType == "hidden") {
            return null;
          }


          if (columnMeta.ColumnName == "permission") {
//            that.hasPermissionField = true;
//            return null;
          }


          if (!that.model["reference_id"]) {

            if (!that.model[columnMeta.ColumnName] && columnMeta.DefaultValue) {
              if (columnMeta.DefaultValue[0] == "'") {
                that.model[columnMeta.ColumnName] = columnMeta.DefaultValue.substring(1, columnMeta.DefaultValue.length - 1);
              } else {
                that.model[columnMeta.ColumnName] = columnMeta.DefaultValue;
              }
            }
          }

          if (columnMeta.ColumnType == "truefalse") {
            that.model[columnMeta.ColumnName] = that.model[columnMeta.ColumnName] === "1" || that.model[columnMeta.ColumnName] === 1 || that.model[columnMeta.ColumnName] === "true" || that.model[columnMeta.ColumnName] === true;
          }

          if (columnMeta.ColumnType == "date") {
            var parseTime = Date.parse(that.model[columnMeta.ColumnName]);
            if (!isNaN(parseTime)) {
              console.log("parsed time is not nan", parseTime);
              that.model[columnMeta.ColumnName] = new Date(parseTime)
            }
          }


          let inputType = that.getInputType(columnMeta);
          const textInputType = that.getTextInputType(columnMeta);


          console.log("Add column model ", columnName, columnMeta, that.model[columnMeta.ColumnName]);

          var resVal = {
            type: inputType,
            inputType: textInputType,
            label: columnLabel,
            model: columnMeta.ColumnName,
            name: columnName,
            id: "id",
            readonly: false,
            rows: 10,
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
          console.log("check column meta for entity", columnMeta);
          if (columnMeta.ColumnType == "entity") {
            if (columnMeta.jsonApi == "hasOne") {
              resVal.value = that.model[resVal.ColumnName];
              resVal.multiple = false;
              foreignKeys.push(resVal);
            } else {
              resVal.value = that.model[resVal.ColumnName];
              resVal.multiple = false;
//              foreignKeys.push(resVal);
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

        setTimeout(function () {
          if (that.focusSet) {
            return
          }
          console.log("set focus to first input field")
          that.focusSet = true;
          document.querySelector(".action-form-body input").focus()
        }, 100)


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
