<template>
  <div class="content-wrapper">
    <!-- Content Header (Page header) -->
    <section class="content-header">
      <ol class="breadcrumb">
        <li>
          <a href="javascript:">
            <i class="fa fa-home"></i>New item</a>
        </li>
      </ol>
    </section>
    <section class="content">
      <div class="box">
        <div class="box-header">
          <h1 v-if="!data.TableName">New Table</h1>
          <h1 v-else>{{data.TableName | titleCase}}</h1>
        </div>
        <div class="box-body">
          <form onsubmit="return false" role="form">

            <div class="row">
              <div class="col-md-6">
                <div class="form-group">
                  <h3>Name</h3>
                  <input type="text" class="form-control" v-model="data.TableName" name="name"
                         placeholder="sale_record">
                </div>
              </div>
            </div>

            <div class="row">

              <div class="col-md-6">
                <div class="box">
                  <div class="box-header">
                    <h3 class="box-title">Columns</h3>
                  </div>

                  <div class="box-body">
                    <div class="form-group" v-for="col in data.Columns">
                      <div class="row">
                        <div class="col-md-4">
                          <input type="text" v-model="col.Name" placeholder="name" class="form-control"
                                 :disabled="col.ReadOnly">
                        </div>
                        <div class="col-md-3">
                          <select class="form-control" v-model="col.ColumnType" :disabled="col.ReadOnly">
                            <option :value="colData.Name" v-for="(colData, colTypeName) in columnTypes">
                              {{colTypeName | titleCase}}
                            </option>
                          </select>
                        </div>
                        <div class="col-md-2">
                          <input v-model="col.IsUnique" type="checkbox"/> Unique
                        </div>
                        <div class="col-md-2">
                          <input v-model="col.IsIndexed" type="checkbox"/> Indexed
                        </div>
                        <div class="col-md-1" style="padding-left: 0px;" v-if="!col.ReadOnly">
                          <button @click="removeColumn(col)" class="btn btn-danger btn-sm"><i
                            class="fa fa-minus"></i></button>
                        </div>
                      </div>
                    </div>
                  </div>

                  <div class="box-footer">
                    <div class="box-tools pull-right">
                      <button @click="data.Columns.push({})" class="btn btn-primary"><i class="fa fa-plus"></i> Column
                      </button>
                    </div>
                  </div>

                </div>
              </div>

              <div class="col-md-6">
                <div class="box">
                  <div class="box-header">
                    <h2 class="box-title">Relations</h2>

                  </div>
                  <div class="box-body">
                    <div class="form-group" v-for="relation in data.Relations">
                      <div class="row">
                        <div class="col-md-6">
                          <select class="form-control" v-model="relation.Relation" :disabled="relation.ReadOnly">
                            <option value="has_one">Has one</option>
                            <option value="belongs_to">Belongs to</option>
                            <option value="has_many">Has many</option>
                            <option value="has_many_and_belongs_to_many">Has many and belongs to many</option>
                          </select>
                        </div>
                        <div class="col-md-5">
                          <select class="form-control" v-model="relation.Object" :disabled="relation.ReadOnly">
                            <option :value="world.table_name" v-for="world in relatableWorlds">
                              {{world.table_name | titleCase}}
                            </option>
                          </select>
                        </div>
                        <div class="col-md-1" style="padding-left: 0px;" v-if="!relation.ReadOnly">
                          <button @click="removeRelation(relation)" class="btn btn-danger btn-sm"><i
                            class="fa fa-minus"></i></button>
                        </div>
                      </div>
                    </div>
                  </div>
                  <div class="box-footer">
                    <div class="box-tools pull-right">
                      <button @click="data.Relations.push({})" class="btn btn-primary"><i class="fa fa-plus"></i>
                        Relation
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div class="row">
              <div class="col-md-3">
                Row audit <input v-model="data.IsAuditEnabled" type="checkbox"/>
              </div>
              <div class="col-md-3">
                Translations enabled <input v-model="data.TranslationsEnabled" type="checkbox"/>
              </div>
            </div>

          </form>
        </div>
        <div class="box-footer">
          <div class="form-group">
            <button class="btn btn-primary btn-lg" @click="createEntity">Create</button>
          </div>
        </div>

      </div>
    </section>
  </div>
</template>
<script>

  import worldManager from '../plugins/worldmanager';
  import {mapState} from 'vuex';
  import actionManager from '../plugins/actionmanager'

  var typeMeta = [
    {
      name: "entity",
      label: "Entity"
    },
  ];

  export default {
    computed: {
      ...mapState(['worlds']),
      relatableWorlds: function () {
        return this.worlds.filter(function (e) {
          return e.table_name.indexOf("_has_") == -1 && e.table_name.indexOf("_audit") == -1;
        })
      }
    },
    methods: {
      removeColumn(colData) {
        console.log("remove columne", colData);
        let index = this.data.Columns.indexOf(colData);
        if (index > -1) {
          this.data.Columns.splice(index, 1);
        }
      },
      removeRelation(relation) {
        console.log("remove relation", relation);
        let index = this.data.Relations.indexOf(relation);

        if (index > -1) {
          this.data.Relations.splice(index, 1);
        }
      },
      setup() {
        console.log("query table name", this.$route.query)
      },
      createEntity: function () {
        var that = this;
        console.log(this.data);
        var fileContent = JSON.stringify({
          Tables: [
            {
              TableName: this.data.TableName,
              TranslationsEnabled: this.data.TranslationsEnabled,
              IsAuditEnabled: this.data.IsAuditEnabled,
              Columns: this.data.Columns.map(function (col) {
                if (!col.Name) {
                  return null;
                }
                col.ColumnName = col.Name;
                col.ColumnName = col.Name;
                col.DataType = that.columnTypes[col.ColumnType].DataTypes[0];
                return col;
              }).filter(function (e) {
                return !!e && !e.ReadOnly;
              }),
            }
          ],
          Relations: this.data.Relations.map(function (rel) {
            rel.Subject = that.data.TableName;
            return rel
          }).filter(function (e) {
            return !!e && !e.ReadOnly
          })
        });
        console.log("New table json", fileContent);

        var postData = {
          "schema_file": [{
            "name": this.data.TableName + ".json",
            "file": "data:application/json;base64," + btoa(fileContent),
            "type": "application/json"
          }]
        };
        actionManager.doAction("world", "upload_system_schema", postData)


      }
    },
    data() {
      return {
        data: {
          TableName: null,
          IsAuditEnabled: false,
          TranslationsEnabled: false,
          Columns: [
            {
              Name: 'name',
              ColumnType: "label"
            }
          ],
          Relations: [{
            Relation: "belongs_to",
            Object: "user_account"
          }, {
            Relation: "has_many",
            Object: "usergroup"
          }]
        },
        columnTypes: [],
      }
    },
    mounted() {
      console.log("Loaded new meta page");
      var that = this;
      that.columnTypes = worldManager.getColumnFieldTypes();
      let query = this.$route.query;
      if (query && query.table) {
        worldManager.getColumnKeys(query.table, function (columns) {
          var columnModel = columns.ColumnModel;
          var columnNames = Object.keys(columnModel);
          var finalColumns = [];
          var finalRelations = [];
          that.data.TableName = query.table;
          for (var i = 0; i < columnNames.length; i++) {
            var columnName = columnNames[i];
            if (columnName == "__type") {
              continue
            }
            var model = columnModel[columnName];
            if (model.IsForeignKey || model.jsonApi) {

              if (model.type.indexOf("_audit") > -1) {
                continue;
              }

              var relationType = "has_many";
              switch (model.jsonApi) {
                case "hasMany":
                  relationType = "has_many";
                  break;
                case "belongsTo":
                  relationType = "belongs_to";
                  break;
                case "hasOne":
                  relationType = "has_one";
                  break;
              }


              console.log("add table relations", model);
              finalRelations.push({
                Relation: relationType,
                Subject: query.table,
                Object: model.type,
                ReadOnly: true,
              })
            } else {
              console.log("add column", model);
              model.ReadOnly = true;
              finalColumns.push(model);
            }
          }
          finalColumns.forEach(function (e) {
            console.log("final column", e);
            e.ColumnType = e.ColumnType.split(".")[0];
          });
          that.data.Columns = finalColumns;
          that.data.Relations = finalRelations;
          console.log("selected world columns", columns)
        });
      }
      console.log("selected world", query);
      console.log("column types", that.columnTypes);
      that.setup();
    }
  }

</script>
