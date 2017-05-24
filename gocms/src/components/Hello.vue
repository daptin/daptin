<template>

  <div class="container">
    <div class="row">
      <h1>Hello</h1>
    </div>
    <div class="row">
      <div class="col-md-2" v-for="w in world">
        <h3><a @click.prevent="setTable(w.table_name)"> {{w.table_name}} </a></h3>
      </div>
    </div>

  </div>
</template>

<script>
  import JsonApi from 'devour-client'
  const jsonApi = new JsonApi({
    apiUrl: 'http://localhost:6336/api',
    pluralize: false,
  });


  //  // Define Model
  //  jsonApi.define('world', {
  //    "created_at": "",
  //    "deleted_at": "",
  //    "id": 0,
  //    "permission": 0,
  //    "reference_id": "",
  //    "status": "pending",
  //    "updated_at": "",
  //    "user_id": "",
  //    "usergroup_id": "",
  //    "table_name": "",
  //    "schema_json": "",
  //    "default_permission": "",
  //  });

  export default {
    name: 'hello',
    data () {
      return {
        world: [],
        msg: "message",
        selectedWorld: null,
        selectedWorldColumns: [],
        tableData: [],
        tableMap: {
          world: jsonApi.define('world', {
            "created_at": new Date(),
            "deleted_at": new Date(),
            "id": 0,
            "permission": 0,
            "reference_id": "",
            "status": "pending",
            "updated_at": new Date(),
            "user_id": "",
            "usergroup_id": "",
            "table_name": "",
            "schema_json": "",
            "default_permission": "",
          })
        },
      }
    },
    methods: {
      setTable(tableName) {
        console.log("choose table", tableName);
        this.selectedWorld = tableName;
        var model;
        model = this.tableMap[tableName];
        var firstTry = false;
        if (!model) {
          firstTry = true;
          model = jsonApi.define(tableName, {
            "created_at": "",
            "deleted_at": "",
            "id": 0,
            "permission": 0,
            "reference_id": "",
            "status": "pending",
            "updated_at": "",
            "user_id": "",
            "usergroup_id": ""
          });
          this.tableMap[tableName] = model;
        }
        var that = this;
        jsonApi.findAll(tableName).then(function (res) {

          if (!res || res.length < 1) {
            console.log("no data for ", tableName);
            return
          }

          var keys = Object.keys(res[0]);
          that.selectedWorldColumns = keys;
          that.tableData = res;
          if (firstTry) {
              console.log("redefine", tableName, res[0]);
            jsonApi.define(tableName, res[0]);
            jsonApi.findAll(tableName).then(function (res) {
                console.log("new daata", res);
              that.tableData = res;
            });
          }
        })
      },
    },
    mounted() {
      var that = this;
      jsonApi.findAll('world').then(function (res) {
        that.world = res;
        console.log("got world", res)
      });

    }
  }
</script>

<!-- Add "scoped" attribute to limit CSS to this component only -->
<style scoped>
  h1, h2 {
    font-weight: normal;
  }

  ul {
    list-style-type: none;
    padding: 0;
  }

  li {
    display: inline-block;
    margin: 0 10px;
  }

  a {
    color: #42b983;
  }
</style>
