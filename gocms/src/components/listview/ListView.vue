<template>


  <div class="ui">

    <div v-if="!tableData || tableData.length == 0" class="ui column segment">
      <h4> No {{jsonApiModelName}} </h4>
      <el-button @click="reloadData()">Reload</el-button>
    </div>

    <div class="ui column" ng-if="tableData && tableData.length > 0">

      <detailed-table-row :show-all="false" class="ui segment" :model="item" :json-api="jsonApi" :json-api-model-name="jsonApiModelName"
                          v-for="item in tableData">
      </detailed-table-row>


    </div>

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
        selectedRow: {},
        displayData: [],
      }
    },
    methods: {
      chooseTitle: function (obj) {

        console.log("this, meta ", this.meta);
        return obj;

      },
      titleCase: function (str) {
        return str.replace(/[-_]/g, " ").split(' ')
            .map(w => w[0].toUpperCase() + w.substr(1).toLowerCase())
            .join(' ')
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
    reloadData() {
      var that = this;
    },
    mounted() {
      var that = this;
      that.meta = that.jsonApi.modelFor(that.jsonApiModelName)["attributes"];
      console.log("mounted list vuew", that.meta);
      var cols = Object.keys(that.meta);


      that.selectedWorld = that.jsonApiModelName;
      that.selectedWorldColumns = Object.keys(that.jsonApi.modelFor(that.jsonApiModelName)["attributes"])
    }
  }
</script>
