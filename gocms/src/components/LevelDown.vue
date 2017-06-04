<template>


  <div class="ui three column grid">


    <div class="four wide column">
      <div class="ui segment top attached">
        <h2>Tables</h2>
      </div>

      <div class="ui segment attached">
        <ul class="ui relaxed list">
          <div class="item" v-for="w in world" v-if="w.is_top_level == '0' && w.is_visible">
            <div class="content">
              <!--<a class="header" href="#" style="text-transform: capitalize;" @click.prevent="setTable(w.table_name)">-->
                <router-link v-bind:to="w.table_name + '/all'">{{w.table_name | titleCase}}</router-link>
              <!--</a>-->
            </div>
          </div>
        </ul>
      </div>
    </div>


    <div class="eight wide column" v-if="selectedWorld != null">
      <div class="ui segment attached top">

        <div class="four wide column">
          <h2>
            {{selectedWorld | titleCase}}
            <!--<el-button @click="newRow()"><span class="fa fa-plus"></span></el-button>-->
          </h2>

        </div>
        <div class="ui four wide column right floated" style="text-align: right;">
          <div class="ui icon buttons">
            <el-button class="ui button" @click.prevent="viewMode = 'table'"><i class="fa fa-table"></i></el-button>
            <el-button class="ui button" @click.prevent="viewMode = 'items'"><i class="fa fa-th-large"></i></el-button>
            <el-button class="ui button" @click.prevent="showAddEdit = true"><i class="fa fa-plus"></i></el-button>
          </div>
        </div>
      </div>

      <div class="ui column segment attached bottom" v-if="showAddEdit && selectedRow != null">

        <div class="row">
          <div class="sixteen column">
            <!--{{selectedWorldColumns}}-->
            <model-form  :json-api="jsonApi"  @save="saveRow(selectedRow)" @cancel="showAddEdit = false" v-bind:model="selectedRow"
                        v-bind:meta="selectedWorldColumns" ref="modelform"></model-form>
          </div>

        </div>

      </div>
      <table-view @newRow="newRow()" @editRow="editRow" v-if="viewMode == 'table'" :finder="finder" ref="tableview"
                  :json-api="jsonApi"
                  :json-api-model-name="selectedWorld"></table-view>


    </div>
    <div class="four wide column" v-if="showAddEdit && selectedRow != null">
      content here
    </div>


  </div>
</template>

<script>
  import {Notification} from 'element-ui';

  export default {
    name: 'Home',
    filters: {
      titleCase: function (str) {
        return str.replace(/[-_]/g, " ").split(' ')
            .map(w => w[0].toUpperCase() + w.substr(1).toLowerCase())
            .join(' ')
      }
    },
    data () {
      return {
        world: [],
        viewMode: 'table',
        msg: "message",
        selectedWorld: null,
        filterText: "",
        selectedWorldColumns: [],
        showAddEdit: false,
        tableData: [],
        jsonApi: jsonApi,
        selectedRow: {},
        tableMap: {},
        modelLoader: null,
      }
    },
    methods: {
      deleteRow(row) {
        var that = this;
        console.log("delete row", this.selectedWorld);
        jsonApi.destroy(this.selectedWorld, row["reference_id"]).then(function () {
          that.setTable(that.selectedWorld);
        })
      },
      saveRow(row) {
        console.log("save row", row);
        if (row["reference_id"]) {
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
            that.$refs.tableview.reloadData(that.selectedWorld)
          });
        }
      },
      newRow() {
        console.log("new row", this.selectedWorld);
        this.selectedRow = {};
        this.showAddEdit = true;
      },
      editRow(row) {
        console.log("new row", this.selectedWorld);
        this.selectedRow = row;
        this.showAddEdit = true;
      },
      setTable(tableName) {
        var that = this;
        that.selectedWorld = tableName;
        var all = jsonApi.all(tableName);
        that.finder = all.builderStack;
        that.selectedWorldColumns = jsonApi.modelFor(tableName)["attributes"];
        all.builderStack = [];
        if (that.$refs.tableview) {
          that.$refs.tableview.reloadData(tableName)
        }

      },
      logout: function () {
        this.$parent.logout();
      }
    },
    mounted() {
      var that = this;
//      console.log("path ", this.$route)

      that.modelLoader = getColumnKeysWithErrorHandleWithThisBuilder(that);

      jsonApi.findAll('world', {
        page: {number: 1, size: 50},
        include: ['world_column']
      }).then(function (res) {


        res = res.map(function(r){
          return r.toJSON();
        });

        for (var t = 0; t < res.length; t++) {


          (function (typeName) {
            that.modelLoader(typeName, function (model) {
              console.log("Loaded model", typeName, model);
              jsonApi.define(typeName, model.ColumnModel);
            })
          })(res[t].table_name)

        }
        that.world = res.sort(function (a, b) {
          if (a.table_name < b.table_name) {
            return -1;
          } else if (a.table_name > b.table_name) {
            return 1;
          }
          return 0;
        });
        console.log("got world", res)
      });


    }
  }
</script>
