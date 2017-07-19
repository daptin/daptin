<template>


  <div class="box">
    <!-- ListView -->

    <div class="box-header">
      <div class="box-title">
        <span style="font-weight: 600; font-size: 35px;"> {{jsonApiModelName | titleCase}} </span>
      </div>
      <div class="box-tools">
        <div class="ui icon buttons">

          <button type="button" class="btn btn-box-tool" @click="reloadData()">
            <span>
              <i class="fa fa-2x fa-refresh yellow"></i>
            </span>
          </button>

          <button type="button" class="btn btn-box-tool" @click="showAddEdit = true">
            <span>
              <i class="fa fa-2x fa-plus green"></i>
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
        <detailed-table-row :show-all="false" :model="item" :json-api="jsonApi"
                            :json-api-model-name="jsonApiModelName"
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
        rquired: false,
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
    data () {
      return {
        selectedWorld: null,
        selectedWorldColumns: [],
        tableData: [],
        meta: null,
        showSelect: true,
        selectedRow: {},
        displayData: [],
        showAddEdit: false,
      }
    },
    methods: {

      saveRow(obj) {
        var that = this;
        var res = {data: obj, type: this.jsonApiModelName};
        this.$emit("addRow", this.jsonApiRelationName, res)
        this.showAddEdit = false;
        setTimeout(function () {
          console.log("reload data")
          that.reloadData();
        }, 1000);
      },

      cancel() {
        this.showAddEdit = false;
      },
      onPaginationData (paginationData) {
        // console.log("set pagifnation method", paginationData, this.$refs.pagination)
        this.$refs.pagination.setPaginationData(paginationData)
      },
      onChangePage (page) {
        // console.log("cnage pge", page);
        this.$refs.vuetable.changePage(page)
      },
      reloadData() {
        var that = this;
        // console.log("reload data", that.selectedWorld, that.finder)

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
        var that = this;
        // console.log("data loaded", arguments)
        that.tableData = data;
      },
      failed() {
        this.tableData = [];
        // console.log("data load failed", arguments)
      }
    },
    mounted() {
      var that = this;
      // console.log("this json api name ", that.jsonApiModelName)
      worldManager.getColumnKeys(that.jsonApiModelName, function (cols) {
        // console.log("mounted list vuew", cols);
        that.meta = cols.ColumnModel;
        var cols = Object.keys(that.meta);


        that.selectedWorld = that.jsonApiModelName;
        that.selectedWorldColumns = Object.keys(that.meta)

        if (that.autoload) {
          that.reloadData()
        }

      })

    }
  }
</script>
