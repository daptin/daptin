<template>


  <div class="row">
    <div class="col-md-12">

      <h1>Grid view</h1>

    </div>
  </div>

</template>

<script>
  import {Notification} from 'element-ui';

  export default  {
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
    data () {
      return {
        world: [],
        selectedWorld: null,
        selectedWorldColumns: [],
        tableData: [],
        selectedRow: {},
        css: {
          table: {
            tableClass: 'table table-striped table-bordered',
            ascendingIcon: 'fa fa-sort-alpha-desc',
            descendingIcon: 'fa fa-sort-alpha-asc',
            handleIcon: 'fa fa-wrench'
          },
          pagination: {
            wrapperClass: "pagination pull-right",
            activeClass: "btn-primary",
            disabledClass: "disabled",
            pageClass: "btn btn-border",
            linkClass: "btn btn-border",
            icons: {
              first: "fa fa-backward",
              prev: "fa fa-chevron-left",
              next: "fa fa-chevron-right",
              last: "fa fa-forward"
            }
          }
        }
      }
    },
    methods: {
      onAction (action, data){
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
      onCellClicked (data, field, event){
        console.log('cellClicked 1: ', data, this.selectedWorld);
//        this.$refs.vuetable.toggleDetailRow(data.id);
        console.log("this router", data["id"])

//        this.$router.push({
//          name: "tablename-refId",
//          params: {
//            tablename: data["type"],
//            refId: data["id"]
//          }
//        })
      },
      trueFalseView (value) {
        console.log("Render", value);
        return value === "1" ? '<span class="fa fa-check"></span>' : '<span class="fa fa-times"></span>'
      },
      onPaginationData (paginationData) {
//        console.log("set pagifnation method", paginationData, this.$refs.pagination);
        this.$refs.pagination.setPaginationData(paginationData)
      },
      onChangePage (page) {
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
        console.log("Set table in tableview by [setTable] ", tableName, that.finder);
        that.selectedWorldColumns = {};
        that.tableData = [];
        that.showAddEdit = false;
        that.reloadData(tableName)
      },


      reloadData(tableName) {
        const that = this;
        console.log("Reload data in tableview by [reloadData]", tableName, that.finder)

        if (!tableName) {
          tableName = that.selectedWorld;
        }

        if (!tableName) {
          alert("setting selected world to null")
        }

        that.selectedWorld = tableName;
        let jsonModel = that.jsonApi.modelFor(tableName);
        if (!jsonModel) {
          console.error("Failed to find json api model for ", tableName);
        }
        console.log("selectedWorldColumns", that.selectedWorldColumns)
        that.selectedWorldColumns = jsonModel["attributes"];

        setTimeout(function () {
          try {
            that.$refs.vuetable.changePage(1);
            that.$refs.vuetable.reinit();
          } catch (e) {
            console.log("probably table doesnt exist yet", e)
          }
        }, 300);
      }
    },
    mounted() {
      const that = this;
      that.selectedWorld = that.jsonApiModelName;
      console.log("Mounted TableView for ", that.jsonApiModelName);
      let jsonModel = that.jsonApi.modelFor(that.jsonApiModelName);
      if (!jsonModel) {
        console.error("Failed to find json api model for ", that.jsonApiModelName);
        return
      }
      that.selectedWorldColumns = Object.keys(jsonModel["attributes"])
    },
    watch: {
      'finder': function (newFinder, oldFinder) {
        console.log("finder updated in ", newFinder, oldFinder)
      }
    }
  }
</script>
