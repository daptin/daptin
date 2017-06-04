<template>

  <div class="ui two column grid">

    <div class="ui column">
      <h3> Search and add {{jsonApiModelName}}</h3>
      {{model}}
    </div>
    <div class="ui column">
      <el-select
          v-model="value"
          filterable
          remote
          placeholder="Search"
          :remote-method="remoteMethod"
          :loading="loading">
        <el-option
            v-for="item in options"
            :key="item.value"
            :label="item.label"
            :value="item">
        </el-option>

      </el-select>
    </div>
    <div class="ui column">
      Selected {{jsonApiModelName | titleCase}}: <b>{{value | chooseTitle}}</b>
    </div>
    <div class="ui column right floated">
      <button v-if="value != null"
              @click.prevent="addObject"
              class="el-button ui button el-button--default green">
        Click here to add {{jsonApiModelName | titleCase}}
      </button>
    </div>
  </div>

</template>

<script>
  export default {
    props: {
      jsonApi: {
        type: Object,
        required: true
      },
      model: {
        type: Object,
        required: false,
      },
      jsonApiModelName: {
        type: String,
        required: true,
      }
    },
    filters: {
      titleCase: function (str) {
        if (!str) {
          return str;
        }
        return str.replace(/[-_]/g, " ").split(' ')
            .map(w => w[0].toUpperCase() + w.substr(1).toLowerCase()).join(' ')
      },
      chooseTitle: function (obj) {
        if (!obj) {
          return ""
        }
        var keys = Object.keys(obj);
        for (var i = 0; i < keys.length; i++) {
          if (keys[i].indexOf("name") > -1 && typeof obj[keys[i]] == "string" && obj[keys[i]].length > 0) {
            return obj[keys[i]];
          }
        }
        return obj["type"] + "  #" + obj["id"];

      },

    },
    data: function () {
      return {
        formModel: null,
        loading: false,
        value: null,
        options: []
      }
    },
    methods: {

      addObject: function (value) {
        var that = this;
        console.log("emit add object event", this.value)
        this.$emit("save", {
          type: that.jsonApiModelName,
          id: this.value.id
        })
      },

      chooseTitle: function (obj) {
        var keys = Object.keys(obj);
        for (var i = 0; i < keys.length; i++) {
          if (keys[i].indexOf("name") > -1 && typeof obj[keys[i]] == "string" && obj[keys[i]].length > 0) {
            return obj[keys[i]];
          }
        }
        return obj["type"] + " #" + obj["id"];

      },
      remoteMethod: function (query) {
        console.log("remote method called", arguments);
        var that = this;
        this.loading = true;
        this.jsonApi.findAll(this.jsonApiModelName, {
          page: 1,
          size: 20,
          query: query
        }).then(function (data) {

          data = data.map(function(r){
            return r.toJSON();
          });

          for (var i = 0; i < data.length; i++) {
            data[i].label = that.chooseTitle(data[i])
            data[i].value = data[i]["id"]
          }
          console.log("final result optiopsn", data);
          that.options = data;
          that.loading = false;
        })
      }
    },
    mounted: function () {
      var that = this;

      console.log("start select one or more", this.model, this.meta)

    },
    watch: {},
  }
</script>s
