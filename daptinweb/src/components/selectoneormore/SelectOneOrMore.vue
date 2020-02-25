<template>

  <div class="box">
    <div class="box-title">
      <div class="box-header">
        <span class="font-size: 20px; font-weight: 400">{{schema.name | titleCase }}</span>
      </div>
    </div>
    <div class="box-body">


      <div class="ui section">
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
      <div class="ui section" v-if="selectedItem">
        <p> Selected: {{selectedItem | chooseTitle | titleCase }}</p>
      </div>


    </div>
    <div class="box-footer">
      <button v-if="selectedItem != null" @click.prevent="addObject"
              class="btn btn-primary"> Add {{schema.name | titleCase}}
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
      },
      schema: {
        type: Object,
        required: true,
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
      formatValueToModel(obj) {
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
      remoteMethod: function (query) {
        console.log("remote method called", arguments);
        var that = this;
        this.loading = true;
        jsonApi.findAll(this.schema.inputType, {
          page: 1,
          size: 20,
          filter: query
        }).then(function (data) {
          data = data.data;
          console.log("remote method response", data)
          delete data["links"]
          for (var i = 0; i < data.length; i++) {
            data[i].label = window.chooseTitle(data[i])
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
        if (!(that.value instanceof Array)) {
          that.value = [that.value];
        }
      } else {

      }
      console.log("start select one or more", this.model, that.meta, that.value, this.schema)
    },
    watch: {
      'selectedItem': function (to) {
        console.log("value change", to);
      }
    },
  }
</script>s
