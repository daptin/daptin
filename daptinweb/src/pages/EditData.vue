<template>
  <div class="row">
    <div class="q-pa-md q-gutter-sm">
      <q-breadcrumbs separator="---" class="text-orange" active-color="secondary">
        <q-breadcrumbs-el label="Database" icon="fas fa-database"/>
        <q-breadcrumbs-el label="Tables" icon="fas fa-table"/>
        <q-breadcrumbs-el :label="$route.params.tableName"/>
      </q-breadcrumbs>
    </div>
    <div class="col-12 q-ma-md">
      <div id="spreadsheet"></div>
    </div>
  </div>
</template>

<script>
  import {mapActions, mapGetters, mapState} from 'vuex';

  var Tabulator = require('tabulator-tables');
  export default {
    name: "EditData",
    methods: {
      ...mapActions(['loadData', 'getTableSchema', 'updateRow']),
      refreshData() {
        const that = this;

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
            return {
              title: col.Name,
              field: col.ColumnName,
              editor: true,
              formatter: col.ColumnType === "truefalse" ? "tickCross" : null,
              hozAlign: col.ColumnType === "truefalse" ? "center" : "left",
              sorter: col.ColumnType === "measurement" ? "number" : null,
            }
          }).filter(e => !!e);
          console.log("Table columns", columns);
          that.spreadsheet = new Tabulator("#spreadsheet", {
            data: [],
            columns: columns,
            pagination: "remote",
            tooltips: true,
            ajaxSorting: true,
            ajaxFiltering: true,
            paginationSizeSelector: true,
            index: 'reference_id',
            history:true,
            movableColumns: true,
            paginationSize: 10,
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


      }
    },
    data() {
      return {
        defaultColumns: ['updated_at', 'created_at', 'reference_id', 'permission'],
        tableSchema: {ColumnModel: []},
        rows: [],
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
