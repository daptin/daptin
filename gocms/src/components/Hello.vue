<template>

  <div class="container-fluid">
    <div class="row">

      <div class="col-md-2">
        <h2>Tables</h2>
        <div class="row" v-for="w in world">
          <a style="font-size: 20px; text-transform: capitalize; padding: 10px;"
             @click.prevent="setTable(w.table_name)">{{w.table_name | titleCase}}</a>
        </div>

      </div>


      <div class="col-md-10" v-if="selectedWorld != null">
        <div class="row">
          <div class="col-md-3">
            <h3>
              {{selectedWorld | titleCase}}
              <el-button @click="newRow()"><span class="fa fa-plus"></span></el-button>
            </h3>
          </div>
          <div class="col-md-4">
            <el-input :model="filterText"></el-input>
          </div>

          <div class="col-md-12 pull-left">

            <div class="row" v-if="showAddEdit && selectedRow != null">
              <div class="col-md-12">

                <form class="form">
                  <div class="row">
                    <div class="form-group col-md-3" v-for="col in selectedWorldColumns">
                      <label :for="col">{{col}}
                        <input class="form-control" :id="col" :value="selectedRow[col]" v-model="selectedRow[col]">
                      </label>
                    </div>
                  </div>
                  <div class="row">
                    <div class="col-md-12">
                      <div class="form-group">
                        <el-button @click="saveRow(selectedRow)">
                          Save
                        </el-button>
                        <el-button @click="showAddEdit = false">Cancel</el-button>
                      </div>
                    </div>
                  </div>


                </form>

              </div>
            </div>


          </div>
        </div>
        <div class="row">

          <div class="col-md-12">
            <table class="table">
              <thead>
              <tr>
                <th></th>
                <th v-for="col in selectedWorldColumns">
                  {{col}}
                </th>
              </tr>
              </thead>

              <tbody>
              <tr v-for="row in tableData">
                <td>
                  <el-button @click="edit(row)"><span class="fa fa-pencil"></span></el-button>
                  <el-button @click="deleteRow(row)"><span class="fa fa-times"></span></el-button>
                </td>
                <td v-for="col in selectedWorldColumns">
                  {{row[col]}}
                </td>
              </tr>
              </tbody>

              <tfoot>
              <tr>
                <th></th>
                <th v-for="col in selectedWorldColumns">
                  {{col}}
                </th>
              </tr>
              </tfoot>
            </table>
          </div>

        </div>
      </div>
    </div>


  </div>
</template>

<script>
    import JsonApi from 'devour-client'
    window.jsonApi = new JsonApi({
        apiUrl: 'http://localhost:6336/api',
        pluralize: false,
    });

    var types = {
        "alias": {
            "group": "relations"
        },
        "color": {
            "group": "properties"
        },
        "content": {
            "group": "main",
            "inputType": "textarea"
        },
        "date": {
            "group": "time",
            "inputType": "datePicker"
        },
        "datetime": {
            "group": "time",
            "inputType": "dateTimePicker"
        },
        "day": {
            "group": "time"
        },
        "email": {
            "group": "main",
            "inputType": "email"
        },
        "file": {
            "inputType": "file"
        },
        "hour": {},
        "id": {},
        "image": {
            "inputType": "file"
        },
        "label": {},
        "location.latitude": {},
        "location.longitude": {},
        "measurement": {},
        "minute": {},
        "month": {},
        "name": {},
        "time": {},
        "truefalse": {},
        "url": {},
        "value": {},
        "year": {}
    };

    var columnProperties = {
        "id": {
            "hidden": true,
            "readonly": true,
        },
    };

    function getColumnKeys(typeName, callback) {
        $.ajax({
            url: 'http://localhost:6336/jsmodel/' + typeName + ".js",
            success: function (r) {
                callback(r);
            }
        })
    }

    function loadModel(typeName, callback) {
        var t = document.createElement("script");
        t.onloaddata = function () {
            console.log("load complete");
        };
        t.src = "http://localhost:6336/jsmodel/" + typeName + ".js";
        $("body").append(t);
    }


    export default {
        name: 'hello',
        filters: {
            titleCase: function (str) {
                return str.replace(/_/g, " ")
            }
        },
        data () {
            return {
                world: [],
                msg: "message",
                selectedWorld: null,
                filterText: "",
                selectedWorldColumns: [],
                showAddEdit: false,
                tableData: [],
                selectedRow: {},
                tableMap: {
                    world: jsonApi.define('world', {
                        "created_at": new Date(),
                        "deleted_at": new Date(),
                        "id": 0,
                        "permission": 0,
                        "reference_id": "",
                        "status": "pending",
                        "updated_at": new Date(),
                        "user_id": "",
                        "usergroup_id": "",
                        "table_name": "",
                        "schema_json": "",
                        "default_permission": "",
                    })
                },
            }
        },
        methods: {
            deleteRow(row) {
                var that = this;
                console.log("delete row", this.selectedWorld);
                jsonApi.destroy(this.selectedWorld, row["reference_id"]).then(function () {
                    that.setTable(that.selectedWorld);
                })
            },
            saveRow(row) {
                console.log("save row", row);
                if (row["reference_id"]) {
                    var that = this;
                    jsonApi.update(this.selectedWorld, row).then(function () {
                        that.setTable(that.selectedWorld);
                        that.showAddEdit = false;
                    });
                } else {
                    var that = this;
                    jsonApi.create(this.selectedWorld, row).then(function () {
                        that.setTable(that.selectedWorld);
                        that.showAddEdit = false;
                    });
                }
            },
            newRow() {
                console.log("new row", this.selectedWorld);
                this.selectedRow = {};
                this.showAddEdit = true;
            },
            edit(row) {
                console.log("new row", this.selectedWorld);
                this.selectedRow = row;
                this.showAddEdit = true;
            },
            setTable(tableName) {
                var that = this;

                console.log("choose table", tableName, that.tableMap);
                that.selectedWorld = tableName;

                that.selectedWorldColumns = [];
                that.tableData = [];


                var model = jsonApi.modelFor(tableName);
                if (!model) {
                    getColumnKeys(tableName, function (columnKeys) {
                        jsonApi.define(tableName, columnKeys);
                        console.log("mo model", jsonApi.modelFor(tableName), columnKeys);
                        that.reloadData(tableName)
                    });
                    return;
                }
                this.reloadData(tableName)
            },
            reloadData(tableName) {
                var that = this;
                jsonApi.findAll(tableName).then(function (res) {
                    console.log("set columns", jsonApi.models[that.selectedWorld])
                    var keys = Object.keys(jsonApi.models[that.selectedWorld].attributes);
                    that.selectedWorldColumns = keys.filter(function (e) {
                        return e != "type";
                    }).sort();
                    that.tableData = res;
                })
            }
        },
        mounted() {
            var that = this;
            jsonApi.findAll('world').then(function (res) {
                that.world = res.sort(function (a, b) {
                    console.log("argumen", arguments)
                    if (a.table_name < b.table_name) {
                        return -1;
                    } else if (a.table_name > b.table_name) {
                        return 1;
                    }
                    return 0;
                });
                console.log("got world", res)
            });

        }
    }
</script>

<!-- Add "scoped" attribute to limit CSS to this component only -->
<style scoped>
  h1, h2 {
    font-weight: normal;
  }

  ul {
    list-style-type: none;
    padding: 0;
  }

  li {
    display: inline-block;
    margin: 0 10px;
  }

  a {
    color: #42b983;
  }
</style>
