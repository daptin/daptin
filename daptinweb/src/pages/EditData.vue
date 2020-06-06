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
      <q-btn @click="drawerRight = !drawerRight">New row</q-btn>
    </div>
    <div class="col-12 q-ma-md">
      <div id="spreadsheet"></div>
    </div>

    <q-drawer
      side="right"
      v-model="drawerRight"
      bordered
      :width="500"
      :breakpoint="500"
      content-class="bg-grey-3"
    >
      <q-scroll-area class="fit">
        <div class="q-pa-md" style="max-width: 400px">

          <q-form
            class="q-gutter-md"
          >

            <div v-for="column in newRowData">
              <q-input
                :label="column.meta.ColumnName"
                v-if="['label'].indexOf(column.meta.ColumnType) > -1"
                filled
                v-model="column.value"
              />
              <q-toggle
                :label="column.meta.ColumnName"
                v-if="column.meta.ColumnType === 'truefalse'"
                v-model="column.value"
              />
              <q-editor
                :label="column.meta.ColumnName"
                v-if="['content', 'json'].indexOf(column.meta.ColumnType) > -1 "
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

    <q-page-sticky position="bottom-right" :offset="[50, 50]">
      <q-fab color="primary" icon="keyboard_arrow_up" direction="up">
        <q-fab-action color="primary" icon="fas fa-file-excel"/>
        <q-fab-action color="secondary" icon="fas fa-download"/>
      </q-fab>
    </q-page-sticky>
  </div>
</template>

<script>
  import {mapActions, mapGetters, mapState} from 'vuex';

  var Tabulator = require('tabulator-tables');
  export default {
    name: "EditData",
    methods: {
      onNewRow() {
        const that = this;
        var obj = {}
        that.newRowData.map(function (e) {
          obj[e.meta.ColumnName] = e.value;
        });
        obj['tableName'] = that.$route.params.tableName;
        that.createRow(obj).then(function (res) {
          that.$q.notify({
            message: "Row created"
          });
          that.spreadsheet.setData();
          that.newRowData.map(function (e) {
            e.value = "";
          });
          that.drawerRight = false;
        }).catch(function (e) {
          that.$q.notify({
            message: e[0].title
          })
        })
      },
      onCancelNewRow() {
        this.drawerRight = false;
      },
      ...mapActions(['loadData', 'getTableSchema', 'updateRow', 'createRow']),
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
            that.newRowData.push({
                  meta: col,
                  value: col.ColumnType === "truefalse" ? false : ""
                }
            );
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
            // ajaxProgressiveLoad:"scroll",
            // ajaxProgressiveLoadDelay:200,
            // ajaxProgressiveLoadScrollMargin:300,
            index: 'reference_id',
            history: true,
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
        drawerRight: false,
        newRowData: []
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
