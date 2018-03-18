<template>
  <div class="container-fluid flat-white">
    <div ref="tabl" :id="tableId"></div>
  </div>
</template>
<style>

  .flat-white .king-table tr td {
    line-height: 15px;
    border-bottom: 1px solid #ddd;
    font-size: 13px;
    font-family: 'Open Sans', sans-serif;
  }

  .flat-white {
    min-height: 400px;
  }

  .flat-white td, .flat-white th, .flat-white span {
    font-family: 'Open Sans', sans-serif;
    max-width: 500px;
  }

  .flat-white .king-table tr td {
    padding: 0;
    height: 30px;
  }

  .input-cell {
    width: 100%;
    height: 100%;
    border: none;
    border-left: 1px solid #eee;
    padding-left: 6px;
    padding-top: 6px;
    text-overflow: ellipsis;
  }


</style>
<script>
  import {Notification} from 'element-ui';
  import KingTable from "kingtable"
  import KingTableUtils from "kingtable/utils"

  const YAML = require('json2yaml')
  require("npm-font-open-sans/open-sans.css")
  require("open-iconic/font/css/open-iconic.min.css")

  function generateID() {
    const length = 5;
    let text = "";
    const possible = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";

    for (let i = 0; i < length; i++) {
      text += possible.charAt(Math.floor(Math.random() * possible.length))
    }

    return "a" + text
  }

  export default {
    name: 'table-view',
    props: {
      jsonApi: {
        type: Object,
        required: true
      },
      autoload: {
        type: Boolean,
        required: false,
        default: true
      },
      jsonApiModelName: {
        type: String,
        required: true
      },
    },
    data() {
      return {
        world: [],
        selectedWorld: null,
        selectedWorldColumns: [],
        tableData: [],
        selectedRow: {},
        data: {},
        jsonModel: {},
        dataMap: {},
        tableId: generateID(),
        inputs: [],
      }
    },
    methods: {

      loadTable() {
        const that = this;
        that.jsonApi.findAll(that.selectedWorld).then(function (data) {
          console.log("got all data", that.jsonModel, that.selectedWorldColumns);


          const attributes = that.jsonModel.attributes;
          const tableAttribtues = {};
          that.selectedWorldColumns.map(function (e) {
            console.log(e, attributes[e], "make table attribute");
            const attribute = attributes[e];


            if (attribute instanceof Object) {
              console.log(attribute, "is an object");
              return null;
            }

            var value = null;
            console.log("choose for ", attributes[e])
            switch (attributes[e]) {
              case "hidden":
                break;
              case "json":
                value = {
                  name: titleCase(e),
                  html: function (item, value) {
//                    console.log("Return function for ", e);
                    const val = YAML.stringify(JSON.parse(value)).replace(/\n/g, "<br>").replace(/\t/g, "  ").replace(/ /g, "&nbsp;")
                    return `<div class='input-cell'>${value}</div>`
                  }
                }
                break;
              case "datetime":
                value = {
                  name: titleCase(e),
                  html: function (item, value) {
                    if (value) {
                      return `<div class='input-cell'>${value}</div>`
                    }
                    return `<div class='input-cell'></div>`
                  }
                }
                break;
              case "truefalse":
                value = {
                  name: titleCase(e),
                  html: function (item, value) {
                    console.log("choose for truefaluse vaule", value)

                    value = value.toLowerCase();
                    if (value == "true" || value == "1") {
                      value = true
                    } else if (value == "false" || value == "0") {
                      value = false
                    }

                    if (value) {
                      return `<div class='input-cell'><input type="checkbox" checked></div>`
                    }
                    return `<div class='input-cell'><input type="checkbox"></div>`
                  }
                }
                break;
              default:
                value = {
                  name: titleCase(e),
                  html: function (item, value) {
                    if (value) {
                      return `<div class='input-cell'>${value}</div>`
                    }
                    return `<div class='input-cell'></div>`
                  }
                }
            }
            if (value) {
              value.hidden = false,
                value.secret = false,
                tableAttribtues[e] = value;
            }
          })

          var keys = that.selectedWorldColumns;
          var attrs = {};
          for (var i = 0; i < keys.length; i++) {
            attrs[keys[i]] = {
              name: titleCase(keys[i])
            };
          }

          console.log("array data", attrs, tableAttribtues);
          const table = new KingTable({
            data: data.data,
//            caption: titleCase(that.selectedWorld),
            collectionName: titleCase(that.selectedWorld),
//            url: `http://api.daptin.com:6336/api/${that.selectedWorld}`,
            id: `table-${that.tableId}`,
            idProperty: 'id',
            onFetchDone: function (res) {
              console.log("on fetch done", res);
              res = res.data;
              return res;
            },
            element: document.getElementById(`${that.tableId}`),
            columns: tableAttribtues,
            columnDefault: {
              name: "",         // display name of column
              type: "text",     // type of data
              sortable: true,   // whether to allow sort by this column
              allowSearch: true,// whether to allow text search by this column
              hidden: true,    // allows to hide column (can still be displayed editing menu options)
              secret: true,    // allows to hide completely the column
              format: undefined // allows to define a formatting function for values
            },
            getTableData: function () {
              console.log("Get table data for ", arguments);
              return new Promise(function (resolve, reject) {
                // TODO: implement your logic to fetch table specific data (e.g. an AJAX request)
                // resolve promise with return object (reject in case of AJAX error)
                that.jsonApi.findAll(that.selectedWorld).then(function (data) {
                  console.log("resolving promise of table data", data);
                  resolve(data.data)
                }, function (err) {
                  reject(err)
                });

              });
            },
            fields: [
//              {
//                name: "delete-btn",
//                displayName: "Delete",
//                html: function () {
//                  return "<button class='btn btn-xs btn-danger delete'><i class='fas fa-times'></i></button>";
//                }
//              }
            ],
            events: {
              "click .delete": function (e, item) {
                // event handler is called in the context of the table builder (access to table instance)
                // first parameter is the click Event; second parameter is the clicked item
              },
              "click td": function (e, item) {
                console.log("item clicked", item);
                // event handler is called in the context of the table builder (access to table instance)
                // first parameter is the click Event; second parameter is the clicked item
//                e.onblur = function (e1) {
//                  console.log("turn off event", x);
//                  console.log("lost focus from item", e, e1, item)
//                }
              },
              "blur td input": function (e, item) {
                console.log("item blur", item);
              },
              "click .pagination-button.pagination-bar-refresh.oi": function (e) {
                console.log("refresh clicked", arguments);
              }
            }
          });
          table.on("hard:refresh", function (filters) {
            console.log("call for hard refresh ")
            that.jsonApi.findAll(that.selectedWorld).then(function (data) {
              console.log("call for hard refresh completed")
              table.data = data.data;
            });
          });


          table.render();
        });


      },
      onAction(action, data) {
        console.log("on action", action, data);
        const that = this;
        if (action === "view-item") {
          this.$refs.vuetable.toggleDetailRow(data.id)
        } else if (action === "edit-item") {
          this.$emit("editRow", data)
        } else if (action === "go-item") {


          this.$router.push({
            name: "Instance",
            params: {
              tablename: data["__type"],
              refId: data["id"]
            }
          });
        } else if (action === "delete-item") {
          this.jsonApi.destroy(this.selectedWorld, data.id).then(function () {
            that.setTable(that.selectedWorld);
          });
        }
      },
      titleCase: function (str) {
        return str.replace(/[-_]/g, " ").split(' ')
          .map(w => w[0].toUpperCase() + w.substr(1).toLowerCase())
          .join(' ')
      },
      onCellClicked(data, field, event) {
        console.log('cellClicked 1: ', data, this.selectedWorld);
//        this.$refs.vuetable.toggleDetailRow(data.id);
        console.log("this router", data["id"])
      },
      trueFalseView(value) {
        console.log("Render", value);
        return value === "1" ? '<span class="fa fa-check"></span>' : '<span class="fa fa-times"></span>'
      },
      onPaginationData(paginationData) {
        console.log("set pagifnation method", paginationData, this.$refs.pagination);
        this.$refs.pagination.setPaginationData(paginationData)
      },
      onChangePage(page) {
        console.log("cnage pge", page, typeof this.$refs.vuetable);
        if (typeof this.$refs.vuetable !== "undefined") {
          this.$refs.vuetable.changePage(page)
        }
      },
      saveRow(row) {
        let that;
        console.log("save row", row);
        if (data.id) {
          that = this;
          that.jsonApi.update(this.selectedWorld, row).then(function () {
            that.setTable(that.selectedWorld);
            that.showAddEdit = false;
          });
        } else {
          that = this;
          that.jsonApi.create(this.selectedWorld, row).then(function () {
            that.setTable(that.selectedWorld);
            that.showAddEdit = false;
          });
        }
      },
      edit(row) {
        this.$parent.emit("editRow", row)
      },
      setTable(tableName) {
        const that = this;
        that.selectedWorldColumns = {};
        that.tableData = [];
        that.showAddEdit = false;
        that.reloadData(tableName)
      },


      reloadData(tableName) {
        const that = this;

        if (!tableName) {
          tableName = that.selectedWorld;
        }

        if (!tableName) {
          alert("setting selected world to null");
        }

        that.selectedWorld = tableName;
        let jsonModel = that.jsonApi.modelFor(tableName);
        if (!jsonModel) {
          console.error("Failed to find json api model for ", tableName);
          that.$notify({
            type: "error",
            message: "This is out of reach.",
            title: "Unauthorized"
          });
          return
        }
        console.log("selectedWorldColumns", that.selectedWorldColumns);
        that.selectedWorldColumns = jsonModel["attributes"];

        setTimeout(function () {
          try {
            that.$refs.vuetable.changePage(1);
            that.$refs.vuetable.reinit();
          } catch (e) {
            console.log("probably table doesnt exist yet", e)
          }
        }, 16);
      }
    },
    mounted() {
      const that = this;
      that.selectedWorld = that.jsonApiModelName;
      let jsonModel = that.jsonApi.modelFor(that.jsonApiModelName);
      console.log("Mounted TableView for ", that.jsonApiModelName, jsonModel);
      that.jsonModel = jsonModel;
      if (!jsonModel) {
        console.error("Failed to find json api model for ", that.jsonApiModelName);
        return
      }
      that.selectedWorldColumns = Object.keys(jsonModel["attributes"]);
      that.loadTable();
    },
    watch: {}

  }
</script>
