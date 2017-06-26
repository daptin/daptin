<template>

  <div class="box">
    <div class="box-body">
      <el-select
        v-model="value"
        filterable
        remote
        :placeholder="'Search and add ' + jsonApiModelName"
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
    <div class="box-footer">
      <button v-if="value != null" @click.prevent="addObject"
              class="btn"> Add {{jsonApiModelName | titleCase}}
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
        console.log("emit add object event", this.value);
        this.$emit("save", {
          type: that.jsonApiModelName,
          id: this.value.id
        })
      },
      chooseTitle: function (obj) {
        var keys = Object.keys(obj);
        console.log("choose title for ", obj);
        for (var i = 0; i < keys.length; i++) {
          if (keys[i].indexOf("name") > -1 && typeof obj[keys[i]] == "string" && obj[keys[i]].length > 0) {
            return obj[keys[i]];
          }
        }


        for (var i = 0; i < keys.length; i++) {
          if (keys[i].indexOf("title") > -1 && typeof obj[keys[i]] == "string" && obj[keys[i]].length > 0) {
            return obj[keys[i]];
          }
        }


        for (var i = 0; i < keys.length; i++) {
          if (keys[i].indexOf("label") > -1 && typeof obj[keys[i]] == "string" && obj[keys[i]].length > 0) {
            return obj[keys[i]];
          }
        }
        return obj["id"].toUpperCase();

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
          delete data["links"]
          console.log("remote method response", data)
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
