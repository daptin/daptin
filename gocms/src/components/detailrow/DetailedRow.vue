<template>


  <div class="ui column">


    <el-tabs v-model="activeTabName">
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
          <div class="ui column relaxed divided list">
            <div class="item" v-for="col in normalFields" :id="col.name">
              <!--<i class="large middle aligned icon"></i>-->
              <div class="content">
                <div class="header">{{col.label}}</div>
                <div :style="col.style" class="description">{{col.value}}</div>
              </div>
            </div>
          </div>
        </div>
      </el-tab-pane>
      <el-tab-pane :label="relation.name" :name="relation.name" v-for="relation in relations">
        <div class="column six wide">

          <table-view :json-api="jsonApi"
                      :json-api-model-name="relation.name" :finder="relation.finder"></table-view>

          <!--<detailed-table-row v-if="renderNextLevel" :render-next-level="false" :rowData="relation.data"-->
          <!--:jsonApi="jsonApi"-->
          <!--:jsonApiModelName="relation.name"></detailed-table-row>-->

          <h4 v-if="!relation.data || relation.data.length == 0"> No {{relation.title}} </h4>
        </div>
      </el-tab-pane>
    </el-tabs>


  </div>
</template>

<script>
  export default {
    props: {
      rowData: {
        type: Object,
        required: true
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
          id: this.rowData["reference_id"]
        };
        console.log("save row", newRow.name, newRow.data)
        this.jsonApi.create(newRow.name, newRow.data)
      },
      titleCase: function (str) {
        return str.replace(/[-_]/g, " ").split(' ')
            .map(w => w[0].toUpperCase() + w.substr(1).toLowerCase())
            .join(' ')
      }
    }, // end: methods
    created () {

      var that = this;
      console.log("data for detailed row ", this.rowData)
      this.meta = this.jsonApi.modelFor(this.jsonApiModelName);
      this.attributes = this.meta["attributes"];

      var attributes = this.meta["attributes"];

      var normalFields = [];

      var columnKeys = Object.keys(attributes);
      console.log("keys ", columnKeys);
      for (var i = 0; i < columnKeys.length; i++) {
        var colName = columnKeys[i];


        var item = {
          name: colName,
          value: this.rowData[colName]
        };

        var type = attributes[colName];
        if (typeof type == "string") {
          type = {
            columnType: type
          }
        }

        item.type = type.columnType;
        var columnNameTitleCase = this.titleCase(item.name)
        item.label = columnNameTitleCase;
        item.title = columnNameTitleCase;
        item.style = "";

        if (item.type == "entity") {


          (function (item) {

            var columnName = item.name;
            columnNameTitleCase = item.name

            console.log("relation", item, that.jsonApiModelName, that.rowData);

            var builderStack = that.jsonApi.one(that.jsonApiModelName, that.rowData["id"]).all(item.name);
            var finder = builderStack.builderStack;
            builderStack.builderStack = [];
            console.log("finder: ", finder)
            that.relations.push({
              name: columnName,
              title: item.title,
              finder: finder,
              label: item.label,
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
          item.style = "width: 500px; height: 300px;"
        }


        console.log("row ", item);

        normalFields.push(item);
      }


      this.normalFields = normalFields;


      console.log("Created detailed row", this.jsonApiModelName, this.rowData, this.meta)
      setTimeout(function () {
//        $(".dropdown").dropdown();
      }, 100);

//
//      var that = this;
//      setTimeout(function () {
//        for (var i = 0; i < that.normalFields.length; i++) {
//          var field = that.normalFields[i];
//          if (field.type == "json") {
//            var editor = new JSONEditor($("#" + field.name).find(".description")[0], {});
//            console.log("Set value", field)
//            editor.set(JSON.parse(field.originalValue));
//          }
//        }
//      }, 200)

    },
    watch: {},
  }
</script>s
