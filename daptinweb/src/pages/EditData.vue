<template>
  <div class="row">
    <div class="col-12">
      <div id="spreadsheet"></div>
    </div>
    <div class="col-md-12" style="overflow-y:scroll">
      <table>
        <thead>
        <tr>
          <th v-if="col.Name" v-for="col in tableSchema.ColumnModel">
            {{col.Name}}
          </th>
        </tr>
        </thead>
        <tbody>
        <tr v-for="row in rows">
          <td v-if="col.Name" v-for="col in tableSchema.ColumnModel">{{row[col.Name]}}</td>
        </tr>
        </tbody>
      </table>

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
        this.getTableSchema(this.$route.params.tableName).then(function (res) {
          that.tableSchema = res;
          console.log("Schema", that.tableSchema)
        });

        this.loadData({tableName: tableName}).then(function (data) {
          console.log("Loaded data", data);
          that.rows = data.data;
          // that.spreadsheet = new Tabulator("#spreadsheet", {
          //   data: that.rows,
          //   columns: [
          //     {title: "Name", field: "name", sorter: "string", width: 200, editor: true},
          //     {title: "Age", field: "age", sorter: "number", hozAlign: "right", formatter: "progress"},
          //     {
          //       title: "Gender", field: "gender", sorter: "string", cellClick: function (e, cell) {
          //         console.log("cell click")
          //       },
          //     },
          //     {title: "Height", field: "height", formatter: "star", hozAlign: "center", width: 100},
          //     {title: "Favourite Color", field: "col", sorter: "string"},
          //     {title: "Date Of Birth", field: "dob", sorter: "date", hozAlign: "center"},
          //     {
          //       title: "Cheese Preference",
          //       field: "cheese",
          //       sorter: "boolean",
          //       hozAlign: "center",
          //       formatter: "tickCross"
          //     },
          //   ],
          // });
        })
      }
    },
    data() {
      return {
        tableSchema: {ColumnModel: []},
        rows: [],
      }
    },
    mounted() {
      this.refreshData();
    },
    watch: {
      '$route.params.tableName': function () {
        console.log("Table changed", arguments);
        this.refreshData();
      }
    }
  }
</script>

<style scoped>

</style>
