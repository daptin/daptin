<template>


  <div class="row">
    <div class="col-md-12" v-if="!showAll">
      <div class="box">
        <div class="box-header">
          <div class="box-title">
            {{model | chooseTitle | titleCase}}
          </div>
          <div class="box-tools pull-right">

            <div class="ui icon buttons">
              <button @click="initiateDelete" type="button" class="btn btn-box-tool">
                <span class="fa fa-2x fa-times red"></span>
              </button>
              <button @click="editPermission" v-if="jsonApiModelName == 'usergroup'" type="button"
                      class="btn btn-box-tool">
                <span class="fas fa-edit fa-2x grey"></span>
              </button>

              <router-link type="button" class="btn btn-box-tool"
                           :to="{name: 'Instance', params: {tablename: jsonApiModelName, refId: model.reference_id}}">
                <span class="fa fa-2x fa-expand"></span>
              </router-link>
            </div>

          </div>

        </div>

        <div class="box-body">
          <div class="col-md-4" v-for="tf in truefalse">
            <input disabled type="checkbox" :checked="tf.value" name="tf.name">
            <label>{{tf.label}}</label>
          </div>

          <div class="col-md-6">
            <table class="table">
              <tbody>
              <tr v-for="col in normalFields" :id="col.name" v-if="col.value != ''">
                <td style="width: 50%"><b> {{col.label}} </b></td>
                <td :style="col.style"> {{col.value}}</td>
              </tr>
              </tbody>
            </table>
          </div>

          <div class="col-md-6" v-if="rowBeingEdited && showAddEdit">
            <model-form :hideTitle="true" @save="saveRow(rowBeingEdited)" :json-api="jsonApi"
                        @cancel="showAddEdit = false"
                        v-bind:model="rowBeingEdited"
                        v-bind:meta="selectedTableColumns" ref="modelform"></model-form>
          </div>
        </div>

      </div>

    </div>

    <template v-if="showAll">
      <div class="col-md-12">
        <el-tabs>
          <el-tab-pane label="Overview">
            <div class="col-md-6">
              <div class="box-invisible">
                <div class="box-header">
                  <div class="box-title">
                    Details
                  </div>
                </div>
                <div class="box-body">
                  <table class="table">
                    <tbody>
                    <tr v-for="col in normalFields" :id="col.name">
                      <td><b> {{col.label}} </b></td>
                      <td :style="col.style" v-html="col.value"></td>
                    </tr>
                    </tbody>
                  </table>

                </div>
              </div>
            </div>
            <div class="col-md-6">
              <div class="row" v-for="imageField in imageFields">
                <h3>{{imageField.name | titleCase}}</h3>
                <div class="col-md-6" v-for="image in imageField.value">
                  <img style="height: 200px; width: 100%" :src="'data:image/jpeg;base64,'  + image.contents">
                </div>
              </div>
            </div>

            <div class="col-md-6" v-if="truefalse != null && truefalse.length > 0">

              <div class="box">
                <div class="box-header">
                  <div class="box-title">
                    Options
                  </div>
                </div>
                <div class="box-body">
                  <table class="table">
                    <tbody>
                    <tr v-for="tf in truefalse">
                      <td><input disabled type="checkbox" :checked="tf.value" name="tf.name"></td>
                      <td><label>{{tf.label}}</label>
                      </td>
                    </tr>
                    </tbody>
                  </table>
                </div>
              </div>


            </div>

          </el-tab-pane>


          <el-tab-pane v-for="relation in relations" v-if="!relation.failed" :key="relation.name"
                       :label="relation.label">
            <list-view :json-api="jsonApi" :ref="relation.name" class="tab"
                       :data-tab="relation.name" @onDeleteRow="initiateDelete" @saveRow="saveRow"
                       :json-api-model-name="relation.type" :json-api-relation-name="relation.name" @addRow="addRow"
                       :autoload="true" @onLoadFailure="loadFailed(relation)"
                       :finder="relation.finder"></list-view>
          </el-tab-pane>

        </el-tabs>

      </div>


    </template>
  </div>
</template>

<script>

  import worldManager from "../../plugins/worldmanager";
  var markdown_renderer = require('markdown-it')();
  import {Notification} from "element-ui";

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
    data() {
      return {
        meta: {},
        metaMap: {},
        activeTabName: "first",
        editData: null,
        attributes: null,
        visible2: false,
        normalFields: [],
        imageFields: [],
        relatedData: {},
        selectedTableColumns: null,
        rowBeingEdited: null,
        relations: [],
        showAddEdit: false,
        imageMap: {},
        relationFinder: {},
        truefalse: []
      }
    },
    created() {
    },
    computed: {},
    methods: {
      saveRow: function (relatedRow) {
        var that = this;
        console.log("Save from row being edited", relatedRow)
        if (!this.showAll) {
          console.log("not the parent");
          this.$emit("saveRelatedRow", relatedRow)
        } else {
          console.log("start to save this row", that.jsonApiModelName, that.relations);

          var typeName = that.jsonApiModelName + "_" + that.jsonApiModelName + "_id_has_" + relatedRow["type"] + "_" + relatedRow["type"] + "_id",
            relatedRow;
          console.log("typename is", typeName);
          that.jsonApi.update(typeName, relatedRow).then(function (r) {
            that.$notify.success("Added " + relation.type);
            // console.log("reference of list : ", that.$refs[relation.name])
            that.$refs[relation.name].reloadData()
          }, function (err) {
            that.$notify.error(err)
          })

        }

      },
      editPermission: function () {
        this.showAddEdit = true;
        this.selectedTableColumns = {
          "permission": {
            "Name": "permission",
            "ColumnName": "permission",
            "ColumnType": "value",
            "DataType": "int(11)",
          }
        };
        this.rowBeingEdited = this.model;
      },
      initiateDelete: function () {

        if (!this.showAll) {
          console.log("not the parent", this.model);
          this.$emit("deleteRow", this.model)
        } else {
          console.log("start to delete this row", this.model, this.showAll)
        }
      },
      loadFailed: function (relation) {
        console.log("relation not loaded", relation);
        relation.failed = true;
      },
      getRelationByName: function (name) {
        for (var i = 0; i < this.relations.length; i++) {
          if (this.relations[i].name == name) {
            return this.relations[i];
          }
        }
        return null;
      },
      deleteRow: function (colName, rowToDelete) {
        console.log("call to delete row", arguments);
      },
      addRow: function (colName, newRow) {
        var relation = this.getRelationByName(colName);
        if (relation == null) {
          console.log("relation not found: ", colName)
          return
        }

        // console.log("this meta before save row", colName, newRow, this.meta);
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
              that.patchObjectAddRelation(colName, relation, newRowResult.id);
            })
          } else {
            that.patchObjectAddRelation(colName, relation, newRow.data.id);
          }


        });


      },

      patchObjectAddRelation: function (colName, relation, newRowId) {
        var that = this;
        console.log("add to existing object", newRowId)
        var patchObject = {};


        if (that.meta["attributes"][colName]["jsonApi"] == "hasMany") {
          patchObject[relation.name] = [{
            id: newRowId,
            type: relation.type,
          }];
        } else {
          patchObject[relation.name] = {
            id: newRowId,
            type: relation.type,
          };
        }


        patchObject["id"] = that.model["id"];

        console.log("patch object", patchObject);
        that.jsonApi.update(that.jsonApiModelName, patchObject).then(function (r) {
          that.$notify.success("Added " + relation.type);
          // console.log("reference of list : ", that.$refs[relation.name])
          that.$refs[relation.name].reloadData()
        }, function (err) {
          that.$notify.error(err)
        })
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
        console.log("data for detailed row ", this.model);

        this.meta = this.jsonApi.modelFor(this.jsonApiModelName);

        this.attributes = this.meta["attributes"];
        this.truefalse = [];
        this.imageFields = [];
        var attributes = this.meta["attributes"];

        var normalFields = [];
        that.relations = [];

        var columnKeys = Object.keys(attributes);
        // console.log("keys ", columnKeys, attributes);
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
//          console.log("Column information: ", item)

          if (item.valueType == "entity") {


            (function (item) {

              var columnName = item.name;
              columnNameTitleCase = item.name

//              console.log("relation", item, that.jsonApiModelName, that.model);

              var builderStack = that.jsonApi.one(that.jsonApiModelName, that.model["id"]).all(item.name);
              var finder = builderStack.builderStack;
              builderStack.builderStack = [];
              // console.log("finder: ", finder)

              try {
                let relationJsonApiModel = that.jsonApi.modelFor(item.type);


                if (item.type == "user_account" || item.type == "usergroup") {


                  that.relations.push({
                    name: columnName,
                    title: item.title,
                    finder: finder,
                    label: item.label,
                    type: item.type,
                    failed: false,
                    jsonModelAttrs: relationJsonApiModel,
                  });
                } else {

                  that.relations.unshift({
                    name: columnName,
                    title: item.title,
                    finder: finder,
                    label: item.label,
                    failed: false,
                    type: item.type,
                    jsonModelAttrs: relationJsonApiModel,
                  });

                }
              } catch (e) {
                console.log("Model for ", item.type, "not found")
              }


            })(item);

            continue;
          } else if (item.type == "truefalse") {
            this.truefalse.push(item);
            continue;
          }

          if (item.type.indexOf("image.") == 0) {
            this.imageFields.push(item)
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

          if (item.type == "markdown") {
            item.originalValue = item.value;
            item.value = markdown_renderer.render(item.originalValue);
          }

          if (item.name == "reference_id") {
            continue
          }

          if (item.name == "password") {
            continue
          }
          if (item.name == "created_at") {
            continue
          }
          if (item.name == "updated_at") {
            continue
          }

          if (item.name == "status") {
            continue
          }


          // console.log("row ", item);

          if (item.type == "label") {
            normalFields.unshift(item)
          } else {
            normalFields.push(item);
          }

        }


        this.normalFields = normalFields;


        // console.log("Created detailed row", this.jsonApiModelName, this.model, this.meta)

        setTimeout(function () {
          $('.menu .item').tab();
        }, 600)

      }
    }, // end: methods
    created() {
//      JSONEditor.defaults.options.theme = 'bootstrap';

      this.init();

      var that = this;
      setTimeout(function () {
        for (var i = 0; i < that.normalFields.length; i++) {
          var field = that.normalFields[i];
          if (field.type == "json") {
            try {
              field.formattedValue = JSON.stringify(JSON.parse(field.originalValue), null, 4);
            } catch (e) {
              console.log("Value is not proper json")
            }
          }
        }
      }, 400)

    },
    watch: {
      "model": function () {
        // console.log("model changed, rerender detailed view  ");
        this.init();
      }
    },
  }
</script>
