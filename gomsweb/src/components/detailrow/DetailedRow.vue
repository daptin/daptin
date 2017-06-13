<template>


  <div class="row">
    <!-- DetailRow -->
    <template v-if="!showAll">
      <div class="col-md-12" v-if="truefalse.length > 0">
        <div class="row">
          <div class="col-md-4" v-for="tf in truefalse">
            <input disabled type="checkbox" :checked="tf.value" name="tf.name">
            <label>{{tf.label}}</label>
          </div>
        </div>
      </div>


      <div class="col-md-12">
        <table class="table">
          <tbody>
          <tr v-for="col in normalFields" :id="col.name">
            <td><b> {{col.label}} </b></td>
            <td :style="col.style"> {{col.value}} </td>
          </tr>
          </tbody>
        </table>
      </div>

    </template>


    <div class="col-md-12" v-if="showAll">
      <el-tabs>


        <el-tab-pane label="Overview">
          <table class="table">
            <tbody>
            <tr v-for="col in normalFields" :id="col.name">
              <td><b> {{col.label}} </b></td>
              <td :style="col.style"> {{col.value}} </td>
            </tr>
            </tbody>
          </table>

        </el-tab-pane>


        <el-tab-pane v-for="relation in relations" :label="relation.label">
          <list-view :json-api="jsonApi" :ref="relation.name" class="tab"
                     :data-tab="relation.name"
                     :json-api-model-name="relation.type" :json-api-relation-name="relation.name" @addRow="addRow"
                     :autoload="true"
                     :finder="relation.finder"></list-view>
        </el-tab-pane>

      </el-tabs>

    </div>
  </div>
</template>

<script>

  import worldManager from "../../plugins/worldmanager"
  import {Notification} from "element-ui"

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
        default: false
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
      getRelationByName: function (name) {
        for (var i = 0; i < this.relations.length; i++) {
          if (this.relations[i].name == name) {
            return this.relations[i];
          }
        }
        return null;
      },
      addRow: function (colName, newRow) {
        var relation = this.getRelationByName(colName);
        if (relation == null) {
          console.log("relation not found: ", colName)
          return
        }

        console.log("this meta before save row", colName, newRow, this.meta);
        var that = this;

        worldManager.getColumnKeys(newRow.type, function (newRowTypeAttributes) {

          console.log("newRowTypeAttributes for ", newRow.type, newRowTypeAttributes, newRow);

          if (newRowTypeAttributes.ColumnModel[that.jsonApiModelName + "_id"]
            && newRowTypeAttributes.ColumnModel[that.jsonApiModelName + "_id"]["jsonApi"] === "hasOne") {
            newRow.data[that.jsonApiModelName + "_id"] = {
              type: that.jsonApiModelName,
              id: that.model["id"]
            };
          }

          if (!newRow.data["id"]) {
            that.jsonApi.create(newRow.type, newRow.data).then(function (newRowResult) {
              that.$notify.success("Created new " + that.chooseTitle(newRow));

              var patchObject = {};
              patchObject[relation.name] = {"id": newRowResult["id"]};
              patchObject["id"] = that.model["id"];

              console.log("patch object", patchObject);
              that.jsonApi.update(that.jsonApiModelName, patchObject).then(function (r) {

                console.log("reference of list : ", that.$refs[relation.name])
                that.$refs[relation.name].reloadData()

                that.$notify.success("Added " + relation.type);
              }, function (err) {
                that.$notify.error(err)
              })

            })
          } else {

            var patchObject = {};
            if (newRowTypeAttributes.ColumnModel[that.jsonApiModelName + "_id"]["jsonApi"] == "hasMany"
              && that.meta["attributes"][colName]["jsonApi"] == "hasMany") {
              patchObject[relation.name] = [newRow.data];
            } else {
              patchObject[relation.name] = newRow.data;
            }


            patchObject["id"] = that.model["id"];

            console.log("patch object", patchObject);
            that.jsonApi.update(that.jsonApiModelName, patchObject).then(function (r) {
              that.$notify.success("Added " + relation.type);
              console.log("reference of list : ", that.$refs[relation.name])
              that.$refs[relation.name].reloadData()
            }, function (err) {
              that.$notify.error(err)
            })
          }


        });


      },
      titleCase: function (str) {
        return str.replace(/[-_]/g, " ").trim().split(' ')
          .map(w => w[0].toUpperCase() + w.substr(1).toLowerCase())
          .join(' ')
      },
      reloadData: function (relation) {

      },
      init: function () {
        var that = this;
        console.log("data for detailed row ", this.model)

        this.meta = this.jsonApi.modelFor(this.jsonApiModelName);

        this.attributes = this.meta["attributes"];
        this.truefalse = [];
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


              if (item.type == "user" || item.type == "usergroup") {


                that.relations.push({
                  name: columnName,
                  title: item.title,
                  finder: finder,
                  label: item.label,
                  type: item.type,
                  jsonModelAttrs: that.jsonApi.modelFor(columnName),
                });
              } else {

                that.relations.unshift({
                  name: columnName,
                  title: item.title,
                  finder: finder,
                  label: item.label,
                  type: item.type,
                  jsonModelAttrs: that.jsonApi.modelFor(columnName),
                });
              }


            })(item);

            continue;
          } else if (item.type == "truefalse") {
            this.truefalse.push(item);
            continue;
          }


          if (item.type == "datetime") {
            continue;
          }

          if (item.type == "hidden") {
            continue;
          }

          if (item.type == "json") {
            item.originalValue = item.value;
            item.value = "";
            item.style = "width: 100%; min-height: 20px;"
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

        setTimeout(function () {
          $('.menu .item').tab();
        }, 600)

      }
    }, // end: methods
    created () {
//      JSONEditor.defaults.options.theme = 'bootstrap';

      this.init();

      var that = this;
      setTimeout(function () {
        for (var i = 0; i < that.normalFields.length; i++) {
          var field = that.normalFields[i];
          if (field.type == "json") {

//            var element = document.getElementById(field.name)
//            var element = jQuery("#" + field.name).find(".description")[0];
            field.formattedValue = JSON.stringify(JSON.parse(field.originalValue), null, 4);
          }
        }
      }, 400)

    },
    watch: {
      "model": function () {
        console.log("model changed, rerender detailed view  ");
        this.init();
      }
    },
  }
</script>s
