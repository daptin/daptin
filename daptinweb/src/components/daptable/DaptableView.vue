<template>
  <div class="gridContainer">
    <div ref="tabl" :id="tableId"></div>
  </div>
</template>
<style>

  .gridContainer {

    position: relative;
    width: 100%;
    height: 250px;
  }

  .gridContainer > div {

    width: 100%;
    height: 100%;
  }

</style>
<script>
  import {Notification} from 'element-ui';

  function generateID() {
    const length = 5;
    let text = "";
    const possible = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";

    for (let i = 0; i < length; i++) {
      text += possible.charAt(Math.floor(Math.random() * possible.length))
    }

    return "a" + text
  }

  export default {
    name: 'table-view',
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
    },
    data() {
      return {
        world: [],
        selectedWorld: null,
        selectedWorldColumns: [],
        tableData: [],
        selectedRow: {},
        data: {},
        jsonModel: {},
        dataMap: {},
        tableId: generateID(),
        inputs: [],
      }
    },
    methods: {

      loadTable() {
        const that = this;
        that.jsonApi.findAll(that.selectedWorld).then(function (data) {
          console.log("got all data", data);

          const grid = $(`#${that.tableId}`).ip_Grid({
            rows: 10,
            cols: 26,
//            rowData: [[0]],
//            colData: [6,7,8,9]
          });
          console.log("hello grid", grid);
          window.grid = grid;

        });


      },
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
      onCellClicked(data, field, event) {
        console.log('cellClicked 1: ', data, this.selectedWorld);
//        this.$refs.vuetable.toggleDetailRow(data.id);
        console.log("this router", data["id"])
      },
      trueFalseView(value) {
        console.log("Render", value);
        return value === "1" ? '<span class="fa fa-check"></span>' : '<span class="fa fa-times"></span>'
      },
      onPaginationData(paginationData) {
        console.log("set pagifnation method", paginationData, this.$refs.pagination);
        this.$refs.pagination.setPaginationData(paginationData)
      },
      onChangePage(page) {
        console.log("cnage pge", page, typeof this.$refs.vuetable);
        if (typeof this.$refs.vuetable !== "undefined") {
          this.$refs.vuetable.changePage(page)
        }
      },
      saveRow(row) {
        let that;
        console.log("save row", row);
        if (data.id) {
          that = this;
          that.jsonApi.update(this.selectedWorld, row).then(function () {
            that.setTable(that.selectedWorld);
            that.showAddEdit = false;
          });
        } else {
          that = this;
          that.jsonApi.create(this.selectedWorld, row).then(function () {
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
        that.selectedWorldColumns = {};
        that.tableData = [];
        that.showAddEdit = false;
        that.reloadData(tableName)
      },


      reloadData(tableName) {
        const that = this;

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
          that.$notify({
            type: "error",
            message: "This is out of reach.",
            title: "Unauthorized"
          });
          return
        }
        console.log("selectedWorldColumns", that.selectedWorldColumns);
        that.selectedWorldColumns = jsonModel["attributes"];

        setTimeout(function () {
          try {
            that.$refs.vuetable.changePage(1);
            that.$refs.vuetable.reinit();
          } catch (e) {
            console.log("probably table doesnt exist yet", e)
          }
        }, 16);
      }
    },
    mounted() {
      const that = this;
      that.selectedWorld = that.jsonApiModelName;
      let jsonModel = that.jsonApi.modelFor(that.jsonApiModelName);
      console.log("Mounted TableView for ", that.jsonApiModelName, jsonModel);
      that.jsonModel = jsonModel;
      if (!jsonModel) {
        console.error("Failed to find json api model for ", that.jsonApiModelName);
        return
      }
      that.selectedWorldColumns = Object.keys(jsonModel["attributes"]);
      that.loadTable();
    },
    watch: {}

  }
</script>
