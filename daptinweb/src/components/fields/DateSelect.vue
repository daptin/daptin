<!-- ContentField.vue -->
<template>
  <el-date-picker
    v-model="value"
    type="date"
    placeholder="Select a date">
  </el-date-picker>
</template>

<script>
  import {abstractField} from "vue-form-generator";
  import {DatePicker} from "element-ui";


  export default {
    components: {DatePicker},
    mixins: [abstractField],
    data: function () {
      return {
        editorOptions: {},
      }
    },
    mounted(){
    },
    methods: {
      formatValueToModel(d1111){
        console.log("formatValueToModel", d1111)
        return d1111;
      }
    },
    watch: {
      'value': function (newValue) {

      }
    }
  };
</script>
