<template>
  <div class="row">

    <div class="col-12">
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
        this.getTableSchema(tableName).then(function (res) {
          that.tableSchema = res;
          console.log("Schema", that.tableSchema)
        });

        this.loadData({tableName: tableName}).then(function (data) {
          console.log("Loaded data", data);
          that.rows = data.data;
          let columns = Object.keys(that.tableSchema.ColumnModel).map(function (columnName) {
            var col = that.tableSchema.ColumnModel[columnName];
            console.log("Make column ", col);
            if (col.jsonApi || col.ColumnName == "__type") {
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
      }
    },
    data() {
      return {
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
