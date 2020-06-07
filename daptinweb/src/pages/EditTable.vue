<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <q-page>
    <div class="q-pa-md q-gutter-sm">
      <q-breadcrumbs class="text-orange" active-color="secondary">
      <template v-slot:separator>
        <q-icon
          size="1.2em"
          name="arrow_forward"
          color="primary"
        />
      </template>

      <q-breadcrumbs-el label="Database" icon="fas fa-database" />
      <q-breadcrumbs-el label="Tables" icon="fas fa-table" />
      <q-breadcrumbs-el :label="$route.params.tableName"/>
    </q-breadcrumbs>
    </div>

     <div class="row">
      <div class="col-8 q-pa-md q-gutter-sm">
        <q-markdown src=":::warning
When you add a new column to the table, either a set default value or the set the column as nullable
:::"></q-markdown>
      </div>
     </div>   

     <div class="row">
        <div class="col-10 q-pa-md q-gutter-sm">
          <div v-if="tableSchema" class="col-10 q-pa-sm">
          <table-editor v-on:deleteRelation="deleteTableRelation"
                        v-on:deleteColumn="deleteTableColumn"
                        v-bind:table="tableSchema" v-on:save="saveTable"></table-editor>
        </div>
        </div> 
     </div>  

  </q-page>  
</template>

<script>
  import {mapActions, mapGetters, mapState} from 'vuex';
  import TableSideBar from './TableSideBar';

  export default {
    name: 'CreateTable',
    methods: {
      deleteTableRelation(relation) {
        console.log("Delete relation", relation);

      },
      deleteTableColumn(column) {
        console.log("Delete column", column);
        const that = this;

        this.executeAction({
          tableName: 'world',
          actionName: 'remove_column',
          params: {
            "world_id": "",
            "column_level": "",
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
        let tableName = this.$route.params.tableName;
        console.log("Edit table", tableName);
        if (!tableName) {
          this.setSelectedTable(this.$route.params.tableName);
          return
        }
        this.getTableSchema(tableName).then(function (res) {
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
    watch: {},
    computed: {
      ...mapGetters(['drawerLeft']),
      ...mapState([])
    }
  }
</script>
