<!-- ContentField.vue -->
<template>
  <div class="checkbox">
    <label>
      <input type="checkbox"
             v-model="value"/>
    </label>
  </div>

</template>

<script>
  import {abstractField} from "vue-form-generator";

  export default {
    mixins: [abstractField],
    data: function () {
      return {
        editorOptions: {},
      }
    },
    mounted() {
    },
    methods: {
      formatValueToModel(d1111) {
        console.log("formatValueToModel", d1111);
        return d1111;
      }
    },
    watch: {
      'value': function (newValue) {

      }
    }
  };
</script>
