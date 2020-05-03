<template>
  <div class="row">
    <q-drawer
      v-model="drawerLeft"
      show-if-above
      :width="350"
      :breakpoint="700"
      elevated
      content-class=""
    >
      <q-scroll-area class="fit">
        <table-side-bar></table-side-bar>
      </q-scroll-area>
    </q-drawer>
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
      ...mapActions(['loadData', 'getTableSchema', 'setSelectedTable']),
      refreshData() {
        const that = this;

        if (!this.selectedTable) {
          this.setSelectedTable(this.$route.params.tableName);
          return
        }


        var tableName = this.selectedTable;
        console.log("loaded data editor", tableName);
        this.getTableSchema(this.selectedTable).then(function (res) {
          that.tableSchema = res;
          console.log("Schema", that.tableSchema)
        });

        this.loadData({tableName: tableName}).then(function (data) {
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
            // [
            //   {title: "Name", field: "name", sorter: "string", width: 200, editor: true},
            //   {title: "Age", field: "age", sorter: "number", hozAlign: "right", formatter: "progress"},
            //   {
            //     title: "Gender", field: "gender", sorter: "string", cellClick: function (e, cell) {
            //       console.log("cell click")
            //     },
            //   },
            //   {title: "Height", field: "height", formatter: "star", hozAlign: "center", width: 100},
            //   {title: "Favourite Color", field: "col", sorter: "string"},
            //   {title: "Date Of Birth", field: "dob", sorter: "date", hozAlign: "center"},
            //   {
            //     title: "Cheese Preference",
            //     field: "cheese",
            //     sorter: "boolean",
            //     hozAlign: "center",
            //     formatter: "tickCross"
            //   },
            // ],
          });
        })
      }
    },
    data() {
      return {
        tableSchema: {ColumnModel: []},
        rows: [],
        defaultColumns: ['permission', 'reference_id', 'created_at', 'updated_at'],
      }
    },
    computed: {
      ...mapGetters(['selectedTable', 'drawerLeft'])
    },
    mounted() {
      this.refreshData();
    },
    watch: {
      'selectedTable': function (id) {
        console.log("selected table", this.selectedTable);
        this.$router.push("/data/" + this.selectedTable);
        this.refreshData();
      }
    },
  }
</script>

<style scoped>

</style>
