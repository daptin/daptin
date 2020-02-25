<template>


  <div class="box">
    <!-- ListView -->

    <div class="box-header">
      <div class="box-title">
        <!--<span style="font-weight: 600; font-size: 35px;"> {{jsonApiModelName | titleCase}} </span>-->
      </div>
      <div class="box-tools">

        <div class="ui icon buttons">

          <vuetable-pagination style="margin: 0px" :css="css.pagination" ref="pagination" @change-page="onChangePage"></vuetable-pagination>
          <button type="button" class="btn btn-box-tool" @click="reloadData()">
            <span>
              <i class="fas fa-sync fa-2x  yellow"></i>
            </span>
          </button>

          <button type="button" class="btn btn-box-tool" @click="showAddEdit = true">
            <span>
              <i class="fas fa-plus fa-2x green"></i>
            </span>
          </button>
        </div>
      </div>
    </div>

    <div class="box-body">
      <div class="col-md-12" v-if="showAddEdit">
        <button class="btn btn-success" v-if="showSelect" @click="showSelect = false">
          Create new {{jsonApiModelName | titleCase}}
        </button>
        <button class="btn btn-primary" v-if="!showSelect" @click="showSelect = true">
          Search and add {{jsonApiModelName | titleCase}}
        </button>
      </div>


      <template v-if="showAddEdit && meta">

        <div class="col-md-6 pull-right" v-if="showSelect">
          <select-one-or-more
            @save="saveRow" :schema="{inputType: jsonApiModelName}">
          </select-one-or-more>

        </div>
        <div class="col-md-12" v-if="!showSelect">
          <model-form
            :json-api="jsonApi" @save="saveRow"
            @cancel="cancel()" :meta="meta">
          </model-form>
        </div>


      </template>


      <div class="col-md-12" v-for="item in tableData">
        <detailed-table-row :show-all="false" :model="item" :json-api="jsonApi" ref="vuetable"
                            :json-api-model-name="jsonApiModelName" @saveRelatedRow="saveRelatedRow" @deleteRow="deleteRow"
                            :key="item.id">
        </detailed-table-row>
      </div>
    </div>

  </div>

</template>

<script>
  import {Notification} from 'element-ui';
  import worldManager from "../../plugins/worldmanager"

  export default {
    name: 'table-view',
    filters: {
      titleCase: function (str) {
        return str.replace(/[-_]/g, " ").split(' ')
          .map(w => w[0].toUpperCase() + w.substr(1).toLowerCase())
          .join(' ')
      },
    },
    props: {
      jsonApi: {
        type: Object,
        required: true
      },
      jsonApiRelationName: {
        type: String,
        required: false
      },
      autoload: {
        type: Boolean,
        required: false,
        default: false
      },
      jsonApiModelName: {
        type: String,
        required: true
      },
      finder: {
        type: Array,
        required: true,
      },
      model: {
        type: Object,
        required: false,
      }
    },
    data() {
      return {
        selectedWorld: null,
        selectedWorldColumns: [],
        tableData: [],
        meta: null,
        showSelect: true,
        selectedRow: {},
        displayData: [],
        showAddEdit: false,
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
      saveRelatedRow(relatedRow){
        var that = this;
        console.log("save row from list view", relatedRow);

        that.$emit("saveRow", relatedRow);

//        if (relatedRow.type == "usergroup") {
//
//          that.jsonApi.builderStack = [];
//
//          for (var i = 0; i < that.finder.length - 1; i++) {
//            that.jsonApi.builderStack.push(that.finder[i])
//          }
//          var top = that.finder[that.finder.length - 1];
//
//          that.jsonApi.relationships().all(top.model).update(relatedRow["type"], {
//            "type": relatedRow["type"],
//            "id": relatedRow["id"],
//            "permission":  relatedRow.permission
//          }).then(function (e) {
//            that.reloadData();
//          }, function(){
//            that.reloadData();
//            that.failed();
//          });
//
//        }

      },
      deleteRow(rowToDelete) {
        var that = this;
        console.log("now delete row from list view", rowToDelete, that.finder);

        that.jsonApi.builderStack = [];

        for (var i = 0; i < that.finder.length - 1; i++) {
          that.jsonApi.builderStack.push(that.finder[i])
        }

        var top = that.finder[that.finder.length - 1];

        let rowToDeleteElement = rowToDelete["id"];
        if (false && rowToDelete["__type"] == "usergroup") {
          rowToDeleteElement = rowToDelete["relation_reference_id"]
        }

        that.jsonApi.relationships().all(top.model).destroy([{
          "type": rowToDelete["__type"],
          "id": rowToDeleteElement
        }]).then(function (e) {
            that.reloadData();
          }, function(){
          that.reloadData();
          that.failed();
        });

      },
      saveRow(obj) {
        const that = this;
        const res = {data: obj, type: this.jsonApiModelName};
        this.$emit("addRow", this.jsonApiRelationName, res);
        this.showAddEdit = false;
        setTimeout(function () {
          console.log("reload data");
          that.reloadData();
        }, 1000);
      },

      cancel() {
        this.showAddEdit = false;
      },
      onPaginationData(paginationData) {
        console.log("set pagifnation method", paginationData, this.$refs.pagination);
        this.$refs.pagination.setPaginationData(paginationData)
      },
      onChangePage(page) {
        var that = this;
        console.log("change pge", page);
        that.jsonApi.builderStack = that.finder;
        that.jsonApi.get({
          page: {
            number: page,
            size: 10,
          }
        }).then(
          that.success,
          that.failed
        )
      },
      reloadData() {
        const that = this;
        console.log("reload data", that.selectedWorld, that.finder);

        that.jsonApi.builderStack = that.finder;
        that.jsonApi.get({
          page: {
            number: 1,
            size: 10,
          }
        }).then(
          that.success,
          that.failed
        )
      },
      success(data) {
        console.log("data loaded", data.links, data.data);
        this.onPaginationData(data.links);
        data = data.data;
        const that = this;
        that.tableData = data;
        that.$emit("onLoadSuccess", this.jsonApiRelationName, data)
      },
      failed() {
        this.tableData = [];
        console.log("data load failed", arguments);
        this.$emit("onLoadFailure")
      }
    },
    mounted: function () {
      const that = this;
      // console.log("this json api name ", that.jsonApiModelName)
      worldManager.getColumnKeys(that.jsonApiModelName, function (cols) {
        // console.log("mounted list vuew", cols);
        that.meta = cols.ColumnModel;
        var cols = Object.keys(that.meta);


        that.selectedWorld = that.jsonApiModelName;
        that.selectedWorldColumns = Object.keys(that.meta);

        if (that.autoload) {
          that.reloadData()
        }

      })

    }
  }
</script>
