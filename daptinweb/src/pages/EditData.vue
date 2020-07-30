<template>
  <div>
    <div class="q-pa-md q-gutter-sm">
      <q-breadcrumbs>
        <template v-slot:separator>
          <q-icon
            size="1.2em"
            name="arrow_forward"
            color="purple"
          />
        </template>
        <q-breadcrumbs-el label="Database" icon="fas fa-database"/>
        <q-breadcrumbs-el label="Tables" icon="fas fa-table"/>
        <q-breadcrumbs-el :label="$route.params.tableName"/>
      </q-breadcrumbs>
    </div>
    <q-separator></q-separator>

    <div class="row">
      <div class="col-12">
        <q-btn @click="showNewRowDrawer()" color="primary" flat>New row</q-btn>
        <q-btn @click="showPermissionsDrawer()" color="primary" flat>Table Permissions</q-btn>

        <q-btn v-if="selectedRows.length > 0" @click="deleteSelectedRows" flat color="red">Delete selected rows
        </q-btn>
        <q-separator></q-separator>
      </div>

      <div class="col-12">
        <div id="spreadsheet" style="height: 80vh"></div>
      </div>

      <q-page-sticky v-if="!newRowDrawer" position="bottom-right" :offset="[20, 20]">
        <q-fab color="primary" icon="keyboard_arrow_up" direction="up">
          <q-fab-action @click="downloadData('xls')" color="primary" icon="fas fa-file-excel"/>
          <q-fab-action @click="downloadData('csv')" color="primary" icon="fas fa-download"/>
        </q-fab>
      </q-page-sticky>
    </div>


    <q-drawer
      side="right"
      v-model="newRowDrawer"
      bordered
      :width="500"
      :breakpoint="500"
      content-class="bg-grey-3"
    >
      <q-scroll-area class="fit">
        <div class="q-pa-md">
          <span class="text-h6">New {{$route.params.tableName}}</span>
          <q-form class="q-gutter-md">

            <div v-for="column in newRowData">
              <q-input
                :label="column.meta.ColumnName"
                v-if="['label', 'measurement', 'value', 'email'].indexOf(column.meta.ColumnType) > -1"
                filled
                v-model="column.value"
              />

              <q-file
                filled bottom-slots v-model="column.value" :label="column.meta.ColumnName"
                v-if="column.meta.ColumnType.startsWith('file.')"
                counter>
                <template v-slot:prepend>
                  <q-icon name="cloud_upload" @click.stop/>
                </template>
                <template v-slot:append>
                  <q-icon name="close" @click.stop="column.value = null" class="cursor-pointer"/>
                </template>
              </q-file>


              <q-input
                :label="column.meta.ColumnName"
                type="password"
                v-if="['password'].indexOf(column.meta.ColumnType) > -1"
                filled
                v-model="column.value"
              />

              <q-toggle
                :label="column.meta.ColumnName"
                class="text-right"
                v-if="column.meta.ColumnType === 'truefalse'"
                v-model="column.value"
              />
              <q-editor
                :label="column.meta.ColumnName"
                v-if="['content', 'json'].indexOf(column.meta.ColumnType) > -1 "
                v-model="column.value"
              />

              <q-date
                v-if="['datetime'].indexOf(column.meta.ColumnType) > -1 "
                :subtitle="column.meta.ColumnName"
                v-model="column.value"
              />

            </div>


            <div>
              <q-btn label="Submit" @click="onNewRow" color="primary"/>
              <q-btn label="Reset" @click="onCancelNewRow" color="primary" flat class="q-ml-sm"/>
            </div>
          </q-form>

        </div>

      </q-scroll-area>
    </q-drawer>

    <q-drawer side="right" v-model="tablePermissionDrawer" bordered :width="400" :breakpoint="1400"
              content-class="bg-grey-3">
      <q-scroll-area class="fit row">
        <div class="q-pa-md">
          <div class="col-12">
            <span class="text-h5">Table permissions</span>
          </div>
        </div>
        <div>
          <div class="col-12">
            <table-permissions @close="tablePermissionDrawer = false" v-if="tableData"
                               v-bind:selectedTable="tableData"/>
          </div>
        </div>
      </q-scroll-area>
    </q-drawer>
  </div>
</template>
<style>
  .tabulator-col-title input {
    margin-left: 9px;
  }
</style>

<script>
  import {mapActions, mapGetters, mapState} from 'vuex';

  import XLSX from 'xlsx';

  window.XLSX = XLSX;
  const assetEndpoint = window.location.hostname === "site.daptin.com" ? "http://localhost:6336" : window.location.protocol + "//" + window.location.hostname + (window.location.port === "80" ? "" : ':' + window.location.port);
  var Tabulator = require('tabulator-tables');

  Tabulator.prototype.extendModule("format", "formatters", {
    image: function (cell, formatterParams) {
      console.log("format image cell", cell);
      var column = cell._cell.column;
      var row = cell._cell.row;
      var field = row.data[column.field][0];
      console.log("Image data", field);
      return "<img style='width: 300px; height: 200px' class='fileicon' src='data:" + field.type + ";base64," + field.contents + "'/>";
    },
    audio: function (cell, formatterParams) {
      console.log("format audio cell", cell);
      var column = cell._cell.column;
      var row = cell._cell.row;
      var field = row.data[column.field][0];
      return "<audio controls class='audio' src='data:" + field.type + ";base64," + field.contents + "'/>";
    },
    video: function (cell, formatterParams) {
      console.log("format video cell", cell);
      var column = cell._cell.column;
      var row = cell._cell.row;
      var field = row.data[column.field][0];
      return "<video controls style='width: 300px; height: 200px' class='video' src='data:" + field.type + ";base64," + field.contents + "'/>";
    },
    file: function (cell, formatterParams) {
      console.log("format video cell", cell);
      var column = cell._cell.column;
      var row = cell._cell.row;
      var field = row.data[column.field][0];
      return "<a href='" + assetEndpoint + "/asset/" + row.data.__type + "/" + row.data.reference_id + "/" + column.field + ".'" + field.type.split("/")[1] + "></a>";
    },
  });

  export default {
    name: "EditData",
    methods: {
      downloadData(format) {
        if (format === "csv") {
          this.spreadsheet.download("csv", this.$route.params.tableName + ".csv")
        } else if (format === "xls") {
          this.spreadsheet.download("xlsx", this.$route.params.tableName + ".xls", {
            sheetName: this.$route.params.tableName
          });
        }
      },
      showPermissionsDrawer() {
        this.newRowDrawer = false;
        this.tablePermissionDrawer = true;
      },
      showNewRowDrawer() {
        this.tablePermissionDrawer = false;
        this.newRowDrawer = true;
      },
      deleteSelectedRows() {
        const that = this;
        if (this.selectedRows.length === 0) {
          this.$q.notify({
            message: "Select rows to delete"
          });
        } else {
          Promise.all(this.selectedRows.map(function (row) {
            return that.deleteRow({
              tableName: that.$route.params.tableName,
              reference_id: row.reference_id
            })
          })).then(function () {
            that.spreadsheet.setData();
          }).catch(function (e) {
            that.$q.notify({
              message: e[0].title
            });
            that.spreadsheet.setData();
          })
        }
      },
      onNewRow() {
        const that = this;
        const obj = {};
        const promises = [];
        that.newRowData.map(function (e) {
          if (!e.meta.ColumnType.startsWith('file.')) {
            obj[e.meta.ColumnName] = e.value;
          } else {

            obj[e.meta.ColumnName] = [];
            // for (let i = 0; i < e.value.length; i++) {
            console.log("Create promise for file", e.value);
            promises.push((function (file) {
              console.log("File to read", file);
              return new Promise(function (resolve, reject) {
                const name = file.name;
                const type = file.type;
                const reader = new FileReader();
                reader.onload = function (fileResult) {
                  console.log("File loaded", fileResult);
                  obj[e.meta.ColumnName].push({
                    name: name,
                    file: fileResult.target.result,
                    type: type
                  });
                  resolve();
                };
                reader.onerror = function () {
                  console.log("Failed to load file onerror", e, arguments);
                  reject(name);
                };
                reader.readAsDataURL(file);
              })
            })(e.value));
            // }
            console.log("Asset set set column", e)
          }
        });
        console.log("Promises list", promises);
        obj['tableName'] = that.$route.params.tableName;

        Promise.all(promises).then(function () {
          that.createRow(obj).then(function (res) {
            that.$q.notify({
              message: "Row created"
            });
            that.spreadsheet.setData();
            that.newRowData.map(function (e) {
              e.value = "";
              if (e.meta.ColumnType.startsWith('file.')) {
                e.value = []
              } else if (e.meta.ColumnType === 'truefalse') {
                e.value = false
              } else {
                e.value = ""
              }
            });
            that.newRowDrawer = false;
          }).catch(function (e) {
            if (e instanceof Array) {
              that.$q.notify({
                message: e[0].title
              })
            } else {
              that.$q.notify({
                message: "Failed to save row"
              })
            }
          });
        }).catch(function (e) {
          console.log("Failed to upload file", e);
          that.$q.notify({
            message: "Failed to upload file: " + e[0]
          })
        })


      },
      onCancelNewRow() {
        this.newRowDrawer = false;
      },
      ...mapActions(['loadData', 'getTableSchema', 'updateRow', 'createRow', 'deleteRow']),
      refreshData() {
        const that = this;
        var assetColumns = [];

        var tableName = this.$route.params.tableName;
        console.log("loaded data editor", tableName);
        that.getTableSchema(tableName).then(function (res) {
          that.tableSchema = res;
          console.log("Schema", that.tableSchema);
          // that.loadData({tableName: tableName}).then(function (data) {
          //   console.log("Loaded data", data);
          //   that.rows = data.data;
          let columns = Object.keys(that.tableSchema.ColumnModel).map(function (columnName) {
            var col = that.tableSchema.ColumnModel[columnName];
            // console.log("Make column ", col);
            if (col.jsonApi || col.ColumnName === "__type" || that.defaultColumns.indexOf(col.ColumnName) > -1) {
              return null;
            }
            if (col.ColumnType.startsWith('file.')) {
              assetColumns.push(col.ColumnName)
              that.newRowData.push({
                  meta: col,
                  value: []
                }
              );
            } else if (col.ColumnType === 'truefalse') {
              that.newRowData.push({
                  meta: col,
                  value: false
                }
              );
            } else {
              that.newRowData.push({
                  meta: col,
                  value: ""
                }
              );
            }

            var tableColumn = {
              title: col.Name,
              field: col.ColumnName,
              editor: true,
              headerFilter: true,
              editable: !col.ColumnType.startsWith('file.'),
              formatter: col.ColumnType === "truefalse" ? "tickCross" : null,
              hozAlign: col.ColumnType === "truefalse" ? "center" : "left",
              sorter: col.ColumnType === "measurement" ? "number" : null,
            };

            if (col.ColumnType.startsWith("file.") && col.ColumnType.indexOf('jpg') > -1) {
              tableColumn.formatter = "image";
            }
            if (col.ColumnType.startsWith("file.") && col.ColumnType.indexOf('mp4') > -1) {
              tableColumn.formatter = "video";
            }
            if (col.ColumnType.startsWith("file.") && col.ColumnType.indexOf('mp3') > -1) {
              tableColumn.formatter = "audio";
            }

            return tableColumn;
          }).filter(e => !!e);


          console.log("Table columns", columns);
          columns.unshift({
            formatter: "rowSelection",
            titleFormatter: "rowSelection",
            align: "center",
            headerSort: false
          });
          that.spreadsheet = new Tabulator("#spreadsheet", {
            data: [],
            columns: columns,
            // pagination: "remote",
            tooltips: true,
            ajaxSorting: true,

            layout: "fitDataFill",
            ajaxFiltering: true,
            paginationSizeSelector: true,
            ajaxProgressiveLoad: "scroll",
            ajaxProgressiveLoadDelay: 200,
            ajaxProgressiveLoadScrollMargin: 600,
            index: 'reference_id',
            history: true,
            movableColumns: true,
            rowSelectionChanged: function (data, rows) {
              console.log("row selection changed", data, rows);
              //rows - array of row components for the selected rows in order of selection
              //data - array of data objects for the selected rows in order of selection
              that.selectedRows = data;
            },
            paginationSize: 100,
            cellEdited: function (cell) {
              const reference_id = cell._cell.row.data.reference_id;
              const field = cell._cell.column.field;
              const newValue = cell._cell.value;
              //cell - cell component
              console.log("cell edited", reference_id, arguments);
              const obj = {
                tableName: that.$route.params.tableName,
                id: reference_id,
              };
              obj[field] = newValue;
              that.updateRow(obj).then(function () {
                that.$q.notify({
                  message: "Saved"
                });
              }).catch(function (e) {
                that.$q.notify({
                  message: "Failed to save"
                });
                that.spreadsheet.undo();
              });
            },
            ajaxURL: that.endpoint + "/api/" + tableName, //set url for ajax request
            ajaxURLGenerator: function (url, config, params) {
              //url - the url from the ajaxURL property or setData function
              //config - the request config object from the ajaxConfig property
              //params - the params object from the ajaxParams property, this will also include any pagination, filter and sorting properties based on table setup

              //return request url
              console.log("Generate request url ", url, config, params);
              config.headers = {
                Authorization: "Bearer " + that.authToken
              };
              let requestUrl = that.endpoint + "/api/" + tableName + "?page[number]=" + params.page + "&" + "page[size]=" + params.size + "&";
              if (params.sorters) {
                var sorts = "";
                for (var i = 0; i < params.sorters.length; i++) {
                  var sortBy = params.sorters[i];
                  sorts = sorts + (sortBy.dir === "asc" ? "" : "-") + sortBy.field + ","
                }
                sorts = sorts.substring(0, sorts.length - 1);
                requestUrl = requestUrl + "sort=" + sorts + "&"
              }
              if (params.filters) {
                var queryFilters = [];
                for (var i in params.filters) {
                  var filter = params.filters[i];
                  switch (filter.type) {
                    case "like":
                      queryFilters.push({
                        "column": filter.field,
                        "operator": "like",
                        "value": filter.value,
                      })
                      break;
                    default:
                      queryFilters.push({
                        "column": filter.field,
                        "operator": filter.type,
                        "value": filter.value,
                      })
                  }
                }
                requestUrl = requestUrl + "query=" + JSON.stringify(queryFilters) + "&"

              }
              if (assetColumns.length > 0) {
                requestUrl = requestUrl + "included_relations=" + assetColumns.join(",") + "&"
              }
              console.log("Request url ", requestUrl);
              return requestUrl; //encode parameters as a json object
            },

            rowUpdated: function (row) {
              console.log("Row edited", row);
              //row - row component
            },
            ajaxResponse: function (url, params, response) {
              console.log("ajax call complete", url, params, response);
              //url - the URL of the request
              //params - the parameters passed with the request
              //response - the JSON object returned in the body of the response.

              return {
                last_page: response.links.last_page,
                data: response.data.map(function (e) {
                  return e.attributes
                })
              }; //return the response data to tabulator
            },
          });
          // })
        });

        that.loadData({
          tableName: 'world',
          params: {
            query: JSON.stringify([
              {
                column: 'table_name',
                operator: 'is',
                value: tableName
              }
            ]),
            included_relations: 'user_account',
          }
        }).then(function (res) {
          console.log("Table row", res, arguments);
          if (!res.data || res.data.length !== 1) {
            that.$q.notify({
              message: "Failed to load table metadata"
            });
            return;
          }
          that.tableData = res.data[0];
        }).catch(function (err) {
          console.log("Failed to load table metadata", err);
          that.$q.notify({
            message: "Failed to load table metadata"
          });
        });

      }
    },
    data() {
      return {
        tablePermissionDrawer: false,
        currentPage: 1,
        currentPagination: {},
        defaultColumns: ['updated_at', 'created_at', 'reference_id', 'permission'],
        tableSchema: {ColumnModel: []},
        rows: [],
        tableData: null,
        newRowDrawer: false,
        newRowData: [],
        selectedRows: [],
      }
    },
    computed: {
      ...mapGetters(['endpoint', 'authToken'])
    },
    mounted() {
      this.refreshData();
    },
    watch: {},
  }
</script>

<style scoped>

</style>
