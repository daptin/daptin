<template>


  <div class="ui column">

    <div class="column" v-if="!showAll">
      <div class="ui three column grid" v-if="truefalse.length > 0">
        <div class="column" v-for="tf in truefalse">
          <div class="ui checkbox">
            <input type="checkbox" :checked="tf.value" name="tf.name">
            <label>{{tf.label}}</label>
          </div>
        </div>
      </div>


      <div class="ui two column grid" v-for="col in normalFields" :id="col.name">
        <div class="ui column"><h5>{{col.label}}</h5></div>
        <div :style="col.style" class="ui column description">{{col.value}}</div>
      </div>


    </div>

    <el-tabs v-model="activeTabName" v-if="showAll">
      <el-tab-pane :label="jsonApiModelName" name="first">


        <div class="column ten wide">
          <div class="ui three column grid" v-if="truefalse.length > 0">
            <div class="column" v-for="tf in truefalse">
              <div class="ui checkbox">
                <input type="checkbox" :checked="tf.value" name="tf.name">
                <label>{{tf.label}}</label>
              </div>
            </div>
          </div>


          <div class="ui two column grid" v-for="col in normalFields" :id="col.name">
            <div class="ui column"><h5>{{col.label}}</h5></div>
            <div :style="col.style" class="ui column description">{{col.value}}</div>
          </div>


        </div>

      </el-tab-pane>


      <el-tab-pane :label="relation.name" :name="relation.name" v-for="relation in relations">
        <div class="column six wide">

          <!--<table-view :json-api="jsonApi"-->
          <!--:json-api-model-name="relation.type" :autoload="false" :finder="relation.finder"></table-view>-->

          <list-view :json-api="jsonApi"
                     :json-api-model-name="relation.type" :autoload="false" :finder="relation.finder"></list-view>


        </div>
      </el-tab-pane>
    </el-tabs>


  </div>
</template>

<script>


  import "json-editor"

  export default {
    props: {
      model: {
        type: Object,
        required: true
      },
      showAll: {
        type: Boolean,
        required: false,
        default: true
      },
      jsonApi: {
        type: Object,
        required: true
      },
      jsonApiModelName: {
        type: String,
        required: true
      },
      renderNextLevel: {
        type: Boolean,
        required: false,
        default: function () {
          return false
        }
      }
    },
    data () {
      return {
        meta: {},
        metaMap: {},
        activeTabName: "first",
        editData: null,
        attributes: null,
        visible2: false,
        normalFields: [],
        relatedData: {},
        relations: [],
        relationFinder: {},
        truefalse: []
      }
    },
    created () {
    },
    computed: {},
    methods: {
      saveRow: function (newRow) {
        newRow.data[this.jsonApiModelName + "_id"] = {
          type: this.jsonApiModelName,
          id: this.model["reference_id"]
        };
        console.log("save row", newRow.name, newRow.data)
        this.jsonApi.create(newRow.name, newRow.data)
      },
      titleCase: function (str) {
        return str.replace(/[-_]/g, " ").split(' ')
            .map(w => w[0].toUpperCase() + w.substr(1).toLowerCase())
            .join(' ')
      },
      reloadData: function (relation) {

      },
      init: function() {
        var that = this;
        console.log("data for detailed row ", this.model)
        this.meta = this.jsonApi.modelFor(this.jsonApiModelName);
        this.attributes = this.meta["attributes"];

        var attributes = this.meta["attributes"];

        var normalFields = [];
        that.relations = [];

        var columnKeys = Object.keys(attributes);
        console.log("keys ", columnKeys, attributes);
        for (var i = 0; i < columnKeys.length; i++) {
          var colName = columnKeys[i];


          var item = {
            name: colName,
            value: this.model[colName]
          };

          var type = attributes[colName];
          if (typeof type == "string") {
            type = {
              type: type
            }
          }

          item.type = type.type;
          item.valueType = type.columnType;
          var columnNameTitleCase = this.titleCase(item.name)
          item.label = columnNameTitleCase;
          item.title = columnNameTitleCase;
          item.style = "";

          if (item.valueType == "entity") {


            (function (item) {

              var columnName = item.name;
              columnNameTitleCase = item.name

              console.log("relation", item, that.jsonApiModelName, that.model);

              var builderStack = that.jsonApi.one(that.jsonApiModelName, that.model["id"]).all(item.name);
              var finder = builderStack.builderStack;
              builderStack.builderStack = [];
              console.log("finder: ", finder)
              that.relations.push({
                name: columnName,
                title: item.title,
                finder: finder,
                label: item.label,
                type: item.type,
                jsonModelAttrs: that.jsonApi.modelFor(columnName),
              });


            })(item);

            continue;
          } else if (item.type == "truefalse") {
            this.truefalse.push(item);
            continue;
          }


          if (item.type == "datetime") {
            continue;
          }

          if (item.type == "json") {
            item.originalValue = item.value;
            item.value = "";
            item.style = "width: 500px; min-height: 300px;"
          }

          if (item.name == "permission") {
            continue
          }

          if (item.name == "reference_id") {
            continue
          }

          if (item.name == "password") {
            continue
          }

          if (item.name == "status") {
            continue
          }


          console.log("row ", item);

          if (item.type == "label") {
            normalFields.unshift(item)
          } else {
            normalFields.push(item);
          }

        }


        this.normalFields = normalFields;


        console.log("Created detailed row", this.jsonApiModelName, this.model, this.meta)

      }
    }, // end: methods
    created () {
      JSONEditor.defaults.options.theme = 'html';

      this.init()

      var that = this;
      setTimeout(function () {
        for (var i = 0; i < that.normalFields.length; i++) {
          var field = that.normalFields[i];
          if (field.type == "json") {
            var element = jQuery("#" + field.name).find(".description")[0];
            console.log("element", element)
            var editor = new JSONEditor(element, {
              schema: {}
            });
            console.log("Set value", field)
            editor.setValue(JSON.parse(field.originalValue));
          }
        }
      }, 200)

    },
    watch: {
      "model": function() {
        console.log("model changed, rerender detailed view  ")
        this.init();
      }
    },
  }
</script>s
