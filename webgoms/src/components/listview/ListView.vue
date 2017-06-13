<template>


  <div class="ui">
    <!-- ListView -->

    <div class="ui two column grid attached top segment">
      <div class="one column wide left floated"><h4> {{jsonApiModelName | titleCase}} </h4></div>
      <div class="one column wide right floated">

        <div class="ui icon buttons">

          <button type="button" class="right floated el-button ui button el-button--default" @click="reloadData()">
            <span>
              <i class="fa fa-refresh yellow"></i>
            </span>
          </button>

          <button type="button" class="right floated el-button ui button el-button--default"
                  @click="showAddEdit = true">
            <span>
              <i class="fa fa-plus green"></i>
            </span>
          </button>
        </div>
      </div>
    </div>

    <div class="ui column segment attached" v-if="showAddEdit && meta">

      <select-one-or-more
          :json-api="jsonApi" v-if="showSelect"
          @save="saveRow"
          :json-api-model-name="jsonApiModelName">
      </select-one-or-more>

      <model-form
          :json-api="jsonApi"
          @save="saveRow"
          @cancel="cancel()"
          :meta="meta" v-if="!showSelect">
      </model-form>


    </div>
    <div class="ui column segment attached bottom" v-if="showAddEdit">
      <button class="el-button ui button el-button--default orange" v-if="showSelect" @click="showSelect = false">
        Create new {{jsonApiModelName | titleCase}}
      </button>
      <button class="el-button ui button el-button--default orange" v-if="!showSelect" @click="showSelect = true">
        Search and add {{jsonApiModelName | titleCase}}
      </button>

    </div>

    <detailed-table-row class="ui column segment attached bottom" :show-all="false" :model="item" :json-api="jsonApi"
                        :json-api-model-name="jsonApiModelName"
                        v-for="item in tableData"  :key="item.id">
    </detailed-table-row>

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
    filters: {
      chooseTitle: function (obj) {

        console.log("this, meta ", this.meta);
        return obj;

      },
      titleCase: function (str) {
        return str.replace(/[-_]/g, " ").split(' ')
            .map(w => w[0].toUpperCase() + w.substr(1).toLowerCase())
            .join(' ')
      },
    },
    methods: {

      saveRow(obj) {
        var res = {data: obj, type: this.jsonApiModelName};
        this.$emit("addRow", this.jsonApiRelationName, res)
        this.showAddEdit = false;
      },

      cancel() {
        this.showAddEdit = false;
      },
      onPaginationData (paginationData) {
        console.log("set pagifnation method", paginationData, this.$refs.pagination)
        this.$refs.pagination.setPaginationData(paginationData)
      },
      onChangePage (page) {
        console.log("cnage pge", page);
        this.$refs.vuetable.changePage(page)
      },
      reloadData() {
        var that = this;
        console.log("reload data", that.selectedWorld, that.finder)

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
        console.log("data loaded", arguments)
        that.tableData = data;
      },
      failed() {
        this.tableData = [];
        console.log("data load failed", arguments)
      }
    },
    mounted() {
      var that = this;
      console.log("this json api name ", that.jsonApiModelName)
      worldManager.getColumnKeys(that.jsonApiModelName, function (cols) {
        console.log("mounted list vuew", cols);
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
