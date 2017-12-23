<template>


  <div class="row">
    <div class="col-md-12" style="min-height: 70px;">
      <!--<div class="pull-left">-->
      <!--</div>-->

        <vuetable-pagination :css="css.pagination" ref="pagination" @change-page="onChangePage"></vuetable-pagination>

    </div>
    <div class="col-md-12">
      <!-- TableView -->
      <vuetable v-if="viewMode == 'table'" ref="vuetable"
                :json-api="jsonApi"
                :finder="finder"
                track-by="id"
                detail-row-component="detailed-table-row"
                edit-row-component="model-form"
                @vuetable:cell-clicked="onCellClicked"
                pagination-path="links"
                data-path="data"
                :css="css.table"
                :json-api-model-name="jsonApiModelName"
                @pagination-data="onPaginationData"
                :api-mode="true"
                :query-params="{ sort: 'sort', page: 'page[number]', perPage: 'page[size]' }"
                :load-on-start="autoload">
        <template slot="actions" slot-scope="props">
          <div class="custom-actions">

            <button class="btn btn-box-tool"
                    @click="onAction('go-item', props.rowData, props.rowIndex)">
              <i class="fa fa-2x fa-expand"></i>
            </button>

            <!--<button class="btn btn-box-tool"-->
                    <!--@click="onAction('view-item', props.rowData, props.rowIndex)">-->
              <!--<i class="fa  fa-2x fa-eye"></i>-->
            <!--</button>-->

            <button class="btn btn-box-tool"
                    @click="onAction('edit-item', props.rowData, props.rowIndex)">
              <i class="fa fa-2x fa-pencil-square"></i>
            </button>

            <el-popover
              placement="top"
              trigger="click"
              width="160">
              <p>Are you sure to delete this?</p>
              <div style="text-align: right; margin: 0">
                <el-button type="primary" size="mini" @click="onAction('delete-item', props.rowData, props.rowIndex)">
                  confirm
                </el-button>
              </div>
              <button class="btn btn-box-tool" slot="reference">
                <i class="fa fa-2x fa-times red"></i>
              </button>

            </el-popover>


          </div>
        </template>
      </vuetable>

      <vuecard v-if="viewMode == 'card'" ref="vuetable"
               :json-api="jsonApi"
               :finder="finder"
               track-by="id"
               detail-row-component="detailed-table-row"
               edit-row-component="model-form"
               @vuetable:cell-clicked="onCellClicked"
               pagination-path="links"
               data-path="data"
               :css="css.table"
               :json-api-model-name="jsonApiModelName"
               @pagination-data="onPaginationData"
               :api-mode="true"
               :query-params="{ sort: 'sort', page: 'page[number]', perPage: 'page[size]' }"
               :load-on-start="autoload">
        <template slot="actions" slot-scope="props">
          <div class="custom-actions">

            <button class="btn btn-box-tool"
                    @click="onAction('go-item', props.rowData, props.rowIndex)">
              <i class="fa fa-2x fa-expand"></i>
            </button>

            <!--<button class="btn btn-box-tool"-->
                    <!--@click="onAction('view-item', props.rowData, props.rowIndex)">-->
              <!--<i class="fa  fa-2x fa-eye"></i>-->
            <!--</button>-->

            <button class="btn btn-box-tool"
                    @click="onAction('edit-item', props.rowData, props.rowIndex)">
              <i class="fa fa-2x fa-pencil-square"></i>
            </button>

            <el-popover
              placement="top"
              trigger="click"
              width="160">
              <p>Are you sure to delete this?</p>
              <div style="text-align: right; margin: 0">
                <el-button type="primary" size="mini" @click="onAction('delete-item', props.rowData, props.rowIndex)">
                  confirm
                </el-button>
              </div>
              <button class="btn btn-box-tool" slot="reference">
                <i class="fa fa-2x fa-times red"></i>
              </button>

            </el-popover>


          </div>
        </template>
      </vuecard>

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
        rquired: false,
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
        console.log("set pagifnation method", paginationData, this.$refs.pagination);
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
        console.log("selectedWorldColumns", that.selectedWorldColumns)
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
        var that = this;
        console.log("finder updated in ", newFinder, oldFinder);
        setTimeout(function(){
          that.reloadData(that.selectedWorld);
        }, 100)
      }
    }
  }
</script>
