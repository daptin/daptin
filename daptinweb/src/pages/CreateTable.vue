<template>
  <div class="row">
    <div class="col-12 q-pa-md">
      <table-editor :table="{}" v-on:save="createTable"></table-editor>
    </div>
  </div>
</template>

<script>
  import {mapActions, mapGetters, mapState} from 'vuex';

  export default {
    name: 'CreateTable',
    methods: {
      ...mapActions(['executeAction', 'refreshTableSchema', 'loadModel']),
      createTable(table) {
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
            col.DataType = parts[1];
            if (col.ColumnType.startsWith("file.")) {
              col.IsForeignKey = true;
              col.ForeignKeyData = {
                DataSource: 'cloud_store',
                Namespace: 'localstore',
                KeyName: col.ColumnName,
              }
            }
            table.ColumnModel[i] = col;
          }

        }
        console.log("Table data", table);
        const relations = table.Relations;
        this.$q.notify("Creating table " + table.TableName);
        that.$q.loading.show();
        that.executeAction({
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
          console.log("Created table", e);
          that.$q.notify("Created table, updating schema");
          setTimeout(function () {
            that.refreshTableSchema(table.TableName).then(function () {
              that.$q.loading.hide();
              that.$q.notify("Schema refreshed");
              that.$router.push("/tables")
            }).catch(function (e) {
              that.$q.notify("Failed to refresh");
              that.$q.loading.hide();
            })
          }, 2000)
        }).catch(function (e) {
          that.$q.notify("Failed to create " + e);
          that.$q.loading.hide();
        });


      }
    },
    data() {
      return {
        text: '',

      }
    },
    mounted() {

    },
    watch: {}
  }
</script>
