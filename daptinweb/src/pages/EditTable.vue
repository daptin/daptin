<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <div class="row">
    <div class="col-2">
      <div class="row">
        <table-side-bar></table-side-bar>
      </div>
    </div>
    <div class="col-10">
      <div v-if="tableSchema" class="col-10 q-pa-md">
        <table-editor v-bind:table="tableSchema" v-on:save="saveTable"></table-editor>
      </div>
    </div>
  </div>
</template>

<script>
  import {mapActions, mapGetters, mapState} from 'vuex';
  import TableSideBar from './TableSideBar';

  export default {
    name: 'CreateTable',
    methods: {
      saveTable(table) {

        const that = this;
        if (table.ColumnModel.length === 0) {
          this.$q.notify("Please add columns");
          return
        }

        for (var i = 0; i < table.ColumnModel.length; i++) {
          var col = table.ColumnModel[i];
          if (col.ColumnType.indexOf(" - ") > -1) {
            var parts = col.ColumnType.split(" - ");
            col.ColumnType = parts[0];
            col.DataType = parts[1]
            table.ColumnModel[i] = col;
          }
        }


        console.log("Table data", table);
        const relations = table.Relations;
        this.$q.notify("Updating table structure " + table.TableName);
        that.$q.loading.show();
        this.executeAction({
          tableName: 'world',
          actionName: 'upload_system_schema',
          params: {
            schema_file: [{
              "name": "empty.json", "file": "data:application/json;base64," + btoa(JSON.stringify({
                Tables: [{
                  TableName: table.TableName,
                  Columns: table.ColumnModel,
                }],
                Relations: relations,
              })), "type": "application/json"
            }]
          }
        }).then(function (e) {
          console.log("Update table", e);
          setTimeout(function () {
            that.$q.notify("Updated table structure, refreshing schema");
            that.refreshTableSchema(table.TableName).then(function () {
              that.$q.notify("Schema refreshed");
              that.$q.loading.hide();
            }).catch(function (e) {
              that.$q.notify("Failed to refresh schema " + e);
              that.$q.loading.hide();
            });
          }, 2000)
        }).catch(function (e) {
          that.$q.notify("Failed to create " + e);
          that.$q.loading.hide();
        });


      },
      loadTable() {
        const that = this;
        that.tableSchema = null;
        console.log("Edit table", this.$route.params.tableName);
        this.getTableSchema(this.$route.params.tableName).then(function (res) {
          that.tableSchema = res;
          console.log("Schema", that.tableSchema)
        })
      },
      ...mapActions(['getTableSchema', 'executeAction', 'refreshTableSchema'])
    },
    data() {
      return {
        text: '',
        tableSchema: null,
      }
    },
    mounted() {
      this.loadTable()
    },
    watch: {
      '$route.params.tableName': function (id) {
        this.loadTable()
      }
    },
    computed: {
      ...mapGetters([])
    }
  }
</script>
