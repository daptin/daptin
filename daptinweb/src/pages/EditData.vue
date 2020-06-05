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
      ...mapActions(['loadData', 'getTableSchema']),
      refreshData() {
        const that = this;

        var tableName = this.$route.params.tableName;
        console.log("loaded data editor", tableName);
        that.getTableSchema(tableName).then(function (res) {
          that.tableSchema = res;
          console.log("Schema", that.tableSchema);
          that.loadData({tableName: tableName}).then(function (data) {
            console.log("Loaded data", data);
            that.rows = data.data;
            let columns = Object.keys(that.tableSchema.ColumnModel).map(function (columnName) {
              var col = that.tableSchema.ColumnModel[columnName];
              console.log("Make column ", col);
              if (col.jsonApi || col.ColumnName == "__type" || that.defaultColumns.indexOf(col.ColumnName) > -1) {
                return null;
              }
              return {
                title: col.Name,
                field: col.ColumnName,
              }
            }).filter(e => !!e);
            console.log("Table columns", columns);
            that.spreadsheet = new Tabulator("#spreadsheet", {
              data: that.rows,
              columns: columns
            });
          })
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
      ...mapGetters(['drawerLeft'])
    },
    mounted() {
      this.refreshData();
    },
    watch: {},
  }
</script>

<style scoped>

</style>
