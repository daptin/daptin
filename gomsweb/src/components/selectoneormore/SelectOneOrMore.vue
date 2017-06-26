<template>

  <div class="box">
    <div class="box-title">
      <div class="box-header">
        <span class="font-size: 20px; font-weight: 400">{{schema.name | titleCase }}</span>
      </div>
    </div>
    <div class="box-body">
      <el-select
        v-model="selectedItem"
        filterable
        remote
        :multiple="schema.multiple"
        :placeholder="'Search and add ' + schema.inputType"
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
      <button v-if="selectedItem != null" @click.prevent="addObject"
              class="btn"> Add {{schema.name | titleCase}}
      </button>
    </div>
  </div>

</template>

<script>
  import {abstractField} from "vue-form-generator";
  import jsonApi from "../../plugins/jsonapi"

  export default {
    mixins: [abstractField],
    props: {
      model: {
        type: Object,
        required: false,
      }
    },
    data: function () {
      return {
        formModel: null,
        loading: false,
        options: [],
        selectedItem: null,
      }
    },
    methods: {
      formatValueToModel(obj){
        console.log("formatValueToModel", arguments)
        return {
          id: obj.id,
          type: obj.type
        };
      },
      addObject: function () {
        var that = this;
        console.log("emit add object event", this.value);
        this.$emit("save", {
          name: that.schema.name,
          id: this.selectedItem.id
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
        jsonApi.findAll(this.schema.inputType, {
          page: 1,
          size: 20,
          query: query
        }).then(function (data) {
          console.log("remote method response", data)
          delete data["links"]
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
      that.selectedItem = that.model;
      console.log("select one or more value on mounted", that.value, that.schema.value);
      if (that.schema.multiple) {
        that.value = [that.value];
      } else {

      }
      console.log("start select one or more", this.model, that.meta, that.value, this.schema)
    },
    watch: {
      'selectedItem': function(to){
        console.log("value change", to);
      }
    },
  }
</script>s
