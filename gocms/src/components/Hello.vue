<template>


  <div class="row grid ui">


    <div class="five wide column">
      <div class="row">
        <h2>Tables</h2>
      </div>

      <div class="row">
        <ul class="list ui">
          <li class="item" v-for="w in world">
            <a href="#" style="font-size: 20px; text-transform: capitalize; padding: 10px;"
               @click.prevent="setTable(w.table_name)">
              <span>{{w.table_name | titleCase}}</span>
            </a>
          </li>
        </ul>
      </div>

    </div>


    <div class="eleven wide column" v-if="selectedWorld != null">
      <div class="row">
        <div class="sixteen column">
          <h3>
            {{selectedWorld | titleCase}}
            <el-button @click="newRow()"><span class="fa fa-plus"></span></el-button>
          </h3>
        </div>

        <div class="ten column">

          <div class="row" v-if="showAddEdit && selectedRow != null">
            <div class="sixteen column">
              <model-form @save="saveRow(selectedRow)" @cancel="showAddEdit = false" v-bind:model="selectedRow"
                          v-bind:meta="selectedWorldColumns"></model-form>
            </div>
          </div>

        </div>
      </div>
      <div class="sixteen column pull-left">
        <vuetable-pagination ref="pagination" @change-page="onChangePage"></vuetable-pagination>
      </div>

      <div class="sixteen column">
        <vuetable ref="vuetable"
                  :json-api="jsonApi"
                  pagination-path="links"
                  :json-api-model-name="selectedWorld"
                  @pagination-data="onPaginationData"
                  :api-mode="true"
                  :load-on-start="true">
          <template slot="actions" scope="props">
            <div class="table-button-container">
              <button class="ui button" @click="editRow(props.rowData)"><i class="fa fa-edit"></i> Edit</button>&nbsp;&nbsp;
              <button class="ui basic red button" @click="deleteRow(props.rowData)"><i class="fa fa-remove"></i>
                Delete
              </button>&nbsp;&nbsp;
            </div>
          </template>
        </vuetable>

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
    window.jsonApi.headers['Authorization'] = 'Bearer ' + localStorage.getItem('id_token');


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
        jQuery.ajax({
            url: 'http://localhost:6336/jsmodel/' + typeName + ".js",
            headers: {
                "Authorization": "Bearer " + localStorage.getItem("id_token")
            },
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
        jQuery("body").append(t);
    }


    export default {
        name: 'hello',
        filters: {
            titleCase: function (str) {
                let replace = str.replace(/_/g, " ");
                replace = replace[0].toUpperCase() + replace.substring(1)
                return replace
            }
        },
        components: {
            "model-form": {
                props: [
                    "model",
                    "meta"
                ],
                template: `

                <form class="form">
                  <div class="row">
                    <div class="form-group three column" v-for="col in meta">
                      <label :for="col">{{col}}
                        <input class="form-control" :id="col" :value="model[col]" v-model="model[col]">
                      </label>
                    </div>
                  </div>
                  <div class="row">
                    <div class="sixteen column">
                      <div class="form-group">
                        <el-button @click="saveRow(model)">
                          Save
                        </el-button>
                        <el-button @click="cancel()">Cancel</el-button>
                      </div>
                    </div>
                  </div>


                </form>

    `,
                methods: {
                    saveRow: function () {
                        console.log("save row");
                        this.$emit('save', this.model)

                    },
                    cancel: function () {
                        console.log("canel row");
                        this.$emit('cancel')
                    },
                },
                data: function () {
                    console.log("this data", this);
                    console.log(arguments);
                    console.log(this.model);
                    return {
                        currentElement: "el-input",
                    }
                },
                beforeCreate: function () {

                    console.log("model", this, arguments)
                },
                mounted: function () {
                    var that = this;

//                    setTimeout(function () {
//                        console.log("change type");
//                        that.currentElement = "textarea";
//                    }, 2000)
                }
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
                jsonApi: jsonApi,
                selectedRow: {},
                tableMap: {},
            }
        },
        methods: {
            trueFalseView (value) {
                console.log("Redner", value)
                return value === "1" ? '<span class="fa fa-check"></span>' : '<span class="fa fa-times"></span>'
            },
            onPaginationData (paginationData) {
                console.log("set pagifnation method", paginationData, this.$refs.pagination)
                this.$refs.pagination.setPaginationData(paginationData)
            },
            onChangePage (page) {
                console.log("cnage pge", page)
                this.$refs.vuetable.changePage(page)
            },
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
                that.selectedWorldColumns = [];
                that.tableData = [];


                var model = jsonApi.modelFor(tableName);
                if (!model) {
                    getColumnKeys(tableName, function (columnKeys) {
                        jsonApi.define(tableName, columnKeys);
                        console.log("mo model", jsonApi.modelFor(tableName), columnKeys);
                        that.selectedWorld = tableName;
                        that.reloadData(tableName)
                    });
                    return;
                }
                that.selectedWorld = tableName;
                this.reloadData(tableName)
            },
            reloadData(tableName) {
//                console.log("this refs", this.$refs)
                var that = this
                if (!that.$refs.vuetable) {
                    return;
                }
                setTimeout(function () {
//                    console.log("reload")
                    that.$refs.vuetable.reinit();
                }, 400)
                return;
//                var that = this;
//                jsonApi.findAll(tableName, {page: {offset: 0, limit: 50}}).then(function (res) {
//                    console.log("set columns", jsonApi.models[that.selectedWorld])
//                    var keys = Object.keys(jsonApi.models[that.selectedWorld].attributes);
//                    that.selectedWorldColumns = keys.filter(function (e) {
//                        return e != "type";
//                    }).sort();
//                    that.tableData = res;
//                })
            }
        },
        mounted() {
            var that = this;


            getColumnKeys("user", function (columnKeys) {
                jsonApi.define("user", columnKeys);
                getColumnKeys("usergroup", function (columnKeys) {
                    jsonApi.define("usergroup", columnKeys);

                    getColumnKeys("world", function (columnKeys) {
                        jsonApi.define("world", columnKeys);

                        jsonApi.findAll('world', {
                            page: {number: 1, size: 50},
                            include: ['world_column']
                        }).then(function (res) {
                            that.world = res.sort(function (a, b) {
//                    console.log("argumen", arguments)
                                if (a.table_name < b.table_name) {
                                    return -1;
                                } else if (a.table_name > b.table_name) {
                                    return 1;
                                }
                                return 0;
                            });
                            console.log("got world", res)
                        });

                    })


                });

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
