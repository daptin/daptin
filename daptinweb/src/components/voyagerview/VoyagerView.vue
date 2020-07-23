<template>
  <div class="col-md-12" style="height: 500px;">

    <div class="data-explorer-here" id="data-explorer-here">

    </div>

  </div>

</template>
<style type="text/css">


</style>
<script>

  import _ from "underscore";
  import axios from 'axios';
  import worldManager from '../../plugins/worldmanager';

  //  const libVoyager = require('../../../static/js/plugins/voyager/js/lib-voyager');
  const container = document.getElementById("data-explorer-here");


  export default {
    name: 'voyager-view',
    props: {
      jsonApi: {
        type: Object,
        required: true
      },
      autoload: {
        type: Boolean,
        required: false,
        default: true
      },
      jsonApiModelName: {
        type: String,
        required: true
      },
      finder: {
        type: Array,
        required: true,
      },
      viewMode: {
        type: String,
        required: false,
        default: "card"
      },
    },
    data() {
      return {
        world: [],
        selectedWorld: null,
        selectedWorldColumns: [],
        tableData: [],
        selectedRow: {},
        multiView: null,
        explorerDiv: null,
      }
    },
    methods: {
      onAction(action, data) {
        console.log("on action", action, data);
        const that = this;
        if (action === "view-item") {
          this.$refs.vuetable.toggleDetailRow(data.id)
        } else if (action === "edit-item") {
          this.$emit("editRow", data)
        } else if (action === "go-item") {


          this.$router.push({
            name: "Instance",
            params: {
              tablename: data["__type"],
              refId: data["id"]
            }
          });
        } else if (action === "delete-item") {
          this.jsonApi.destroy(this.selectedWorld, data.id).then(function () {
            that.setTable(that.selectedWorld);
          });
        }
      },
      titleCase: function (str) {
        return str.replace(/[-_]/g, " ").split(' ')
          .map(w => w[0].toUpperCase() + w.substr(1).toLowerCase())
          .join(' ')
      },
      saveRow(data) {
        let that;
        console.log("save row", data);
        if (data.id) {
          that = this;
          that.jsonApi.update(this.selectedWorld, data).then(function () {
            that.setTable(that.selectedWorld);
            that.showAddEdit = false;
          });
        } else {
          that = this;
          that.jsonApi.create(this.selectedWorld, data).then(function () {
            that.setTable(that.selectedWorld);
            that.showAddEdit = false;
          });
        }
      },
      edit(row) {
        this.$parent.emit("editRow", row)
      },
      setTable(tableName) {
        const that = this;
        console.log("Set table in tableview by [setTable] ", tableName, that.finder);
        that.selectedWorldColumns = {};
        that.tableData = [];
        that.showAddEdit = false;
        that.reloadData(tableName)
      },
      createMultiView(dataset, state) {
        var that = this;
        console.log("that selected world columns", that.selectedWorldColumns);
        var dateColumns = [];
        var columnNames = Object.keys(that.selectedWorldColumns);
        for (var i = 0; i < columnNames.length; i++) {
          let columnName = columnNames[i];

          if (columnName == "deleted_at") {
            continue
          }


          if (columnName == "created_at") {
            continue
          }


          if (columnName == "updated_at") {
            continue
          }


          var columnType = that.selectedWorldColumns[columnName];
          if (columnType == "datetime" || columnType == "date" || columnType == "time" || columnType == "timestamp") {
            dateColumns.push(columnName)
          }
        }
        console.log('date column', dateColumns)
        // remove existing multiview if present
        var reload = false;
        if (that.multiView) {
          that.multiView.remove();
          that.multiView = null;
          reload = true;
        }

      },


      reloadData(tableName) {
        const that = this;
        console.log("Reload data in tableview by [reloadData]", tableName, that.finder);

        if (!tableName) {
          tableName = that.selectedWorld;
        }

        if (!tableName) {
          alert("setting selected world to null");
        }

        that.selectedWorld = tableName;
        let jsonModel = that.jsonApi.modelFor(tableName);
        if (!jsonModel) {
          console.error("Failed to find json api model for ", tableName);
        }
        console.log("selectedWorldColumns", that.selectedWorldColumns);
        that.selectedWorldColumns = jsonModel["attributes"];
        // TODO: init recline here

        that.explorerDiv = $('.data-explorer-here');
        that.explorerDiv.html("");


        var options = {
          enableColumnReorder: false
        };


        that.jsonApi.builderStack = that.finder;
        that.jsonApi.get({
          page: {
            number: 1,
            size: 100
          }
        }).then(function (result) {
//          console.log("result for voyager ")
          result =  result.data;
          var container = document.getElementById("data-explorer-here");
          var config = {};
          let data = {values: result};
          console.log("results", data)
//          const voyagerInstance = libVoyager.CreateVoyager(container, undefined, undefined)


//          voyagerInstance.updateData(data);
        });

      }
    },
    mounted() {
      const that = this;
      that.selectedWorld = that.jsonApiModelName;
      console.log("Mounted VoyagerView for ", that.jsonApiModelName);
      let jsonModel = that.jsonApi.modelFor(that.jsonApiModelName);
      if (!jsonModel) {
        console.error("Failed to find json api model for ", that.jsonApiModelName);
        return
      }
      that.selectedWorldColumns = Object.keys(jsonModel["attributes"]);
      that.reloadData();
    },
    watch: {
      'finder': function (newFinder, oldFinder) {
        var that = this;
        console.log("finder updated in ", newFinder, oldFinder);
        setTimeout(function () {
          that.reloadData(that.selectedWorld);
        }, 100)
      }
    }
  }
</script>
