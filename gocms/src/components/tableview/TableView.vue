<template>


  <div class="ui segment attached ">
    <vuetable-pagination ref="pagination" @change-page="onChangePage"></vuetable-pagination>

    <vuetable ref="vuetable"
              :json-api="jsonApi"
              :finder="finder"
              track-by="id"
              detail-row-component="detailed-table-row"
              edit-row-component="model-form"
              @vuetable:cell-clicked="onCellClicked"
              pagination-path="links"
              :json-api-model-name="jsonApiModelName"
              @pagination-data="onPaginationData"
              :api-mode="true"
              :query-params="{ sort: 'sort', page: 'page[number]', perPage: 'page[size]' }"
              :load-on-start="true">
      <template slot="actions" scope="props">
        <div class="custom-actions">
          <button class="ui basic button"
                  @click="onAction('view-item', props.rowData, props.rowIndex)">
            <i class="zoom icon"></i>
          </button>
          <button class="ui basic button"
                  @click="onAction('edit-item', props.rowData, props.rowIndex)">
            <i class="edit icon"></i>
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
            <button class="ui basic button" slot="reference">
              <i class="delete icon"></i>
            </button>

          </el-popover>


        </div>
      </template>
    </vuetable>
  </div>

</template>

<script>
  import {Notification} from 'element-ui';
  import ElementUI from 'element-ui'

  export default {
    name: 'table-view',
    props: {
      jsonApi: {
        type: Object,
        required: true
      },
      jsonApiModelName: {
        type: String,
        required: true
      },
      finder: {
        type: Array,
        required: true,
      }
    },
    data () {
      return {
        world: [],
        selectedWorld: null,
        selectedWorldColumns: [],
        tableData: [],
        selectedRow: {},
      }
    },
    methods: {
      onAction (action, data){
        console.log("on action", action, data)
        var that = this;
        if (action == "view-item") {
          this.$refs.vuetable.toggleDetailRow(data.id)
        } else if (action == "edit-item") {
          this.selectedRow = data;
        } else if (action == "delete-item") {
          this.jsonApi.destroy(this.selectedWorld, data.id).then(function(){
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
        console.log('cellClicked 1: ', data)
//        this.$refs.vuetable.toggleDetailRow(data.id);
        this.$router.push({
          name: 'Instance',
          params: {
            tablename: this.selectedWorld,
            refId: data.id,
          }
        })
      },
      trueFalseView (value) {
        console.log("Render", value)
        return value === "1" ? '<span class="fa fa-check"></span>' : '<span class="fa fa-times"></span>'
      },
      onPaginationData (paginationData) {
        console.log("set pagifnation method", paginationData, this.$refs.pagination)
        this.$refs.pagination.setPaginationData(paginationData)
      },
      onChangePage (page) {
        console.log("cnage pge", page)
        this.$refs.vuetable.changePage(page)
      },
      saveRow(row) {
        console.log("save row", row);
        if (data.id) {
          var that = this;
          jsonApi.update(this.selectedWorld, row).then(function () {
            that.setTable(that.selectedWorld);
            that.showAddEdit = false;
          });
        } else {
          var that = this;
          jsonApi.create(this.selectedWorld, row).then(function () {
            that.setTable(that.selectedWorld);
            that.showAddEdit = false;
          });
        }
      },
      edit(row) {
        this.$parent.emit("editRow", row)
      },
      setTable(tableName) {
        var that = this;
        console.log("choose table", tableName, that.tableMap, that.finder);
        that.selectedWorldColumns = {};
        that.tableData = [];
        that.showAddEdit = false;
        that.reloadData(tableName)
      },
      reloadData(tableName) {
        var that = this;

        if (!tableName) {
          tableName = that.selectedWorld;
        }

        that.selectedWorld = tableName;
        that.selectedWorldColumns = jsonApi.modelFor(tableName)["attributes"];
        if (!that.$refs.vuetable) {
          return;
        }
        setTimeout(function () {
          that.$refs.vuetable.changePage(1);
          that.$refs.vuetable.reinit();
        }, 100);
      }
    },
    reloadData(tableName) {
      var that = this;
      that.selectedWorld = tableName;
      that.selectedWorldColumns = jsonApi.modelFor(tableName)["attributes"];
      if (!that.$refs.vuetable) {
        return;
      }
      setTimeout(function () {
        that.$refs.vuetable.changePage(1);
        that.$refs.vuetable.reinit();
      }, 100);
    },
    mounted() {
      var that = this;
      that.selectedWorld = that.jsonApiModelName;
      that.selectedWorldColumns = Object.keys(that.jsonApi.modelFor(that.jsonApiModelName)["attributes"])
    }
  }
</script>
