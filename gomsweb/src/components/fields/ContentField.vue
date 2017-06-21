<!-- ContentField.vue -->
<template>
  <div class="col-md-12">

    <!-- Or manually control the data synchronization（或手动控制数据流） -->
    <quill-editor :content="value"
                  :options="editorOption"
                  @change="onEditorChange($event)">
    </quill-editor>


    <div slot="tip" class="el-upload__tip">{{schema.placeholder}}</div>


  </div>
</template>

<script>
  import {abstractField} from "vue-form-generator";
  import VueQuillEditor from 'vue-quill-editor';

  export default {
    components: {VueQuillEditor},
    mixins: [abstractField],
    data: function () {
      return {
        value: "",
        editorOptions: {},
      }
    },
    mounted(){
      setTimeout(function () {
        let $input = $("input[type=file]");
        if ($input && $input.length > 0) {
          $input.css("display", "none")
        }
      }, 100)
    },
    methods: {
      onEditorChange() {
        console.log("on editor change", arguments, this.value);
      }
    }
  };
</script>
