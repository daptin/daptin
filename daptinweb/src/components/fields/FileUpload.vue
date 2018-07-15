<!-- FileUpload.vue -->
<template>
  <el-upload
    action="https://jsonplaceholder.typicode.com/posts/"
    :on-preview="handlePreview"
    :on-remove="handleRemove"
    :auto-upload="false"
    :on-change="processFile"
    :before-upload="handlePreview"
    :file-list="fileList">

    <el-button size="small" type="primary">Add file</el-button>
    <div slot="tip" class="el-upload__tip">File type: {{schema.inputType.split("|").join(" or ")}}</div>

  </el-upload>
</template>

<script>
  import {abstractField} from "vue-form-generator";
  import {Upload} from "element-ui";

  export default {
    components: {Upload},
    mixins: [abstractField],
    data: function () {
      return {
        fileList: []
      }
    },
    mounted() {
      console.log("File upload initial value: ", this.value)
      setTimeout(function () {
        let $input = $("input[type=file]");
        if ($input && $input.length > 0) {
          $input.css("display", "none")
        }
      }, 100)
    },
    methods: {
      handlePreview: function () {
        console.log("handle preview", arguments)
      },
      handleRemove: function (file, filelist) {
        console.log("handle remove", file, filelist);
        var fileNameToRemove = file.name;
        var indexToRemove = -1;

        if (!this.value) {
          this.value = [];
        }

        for (var i = 0; i < this.value.length; i++) {
          if (this.value[i].name == fileNameToRemove) {
            var indexToRemove = i;
          }
        }
        if (indexToRemove > -1) {
          this.value.splice(indexToRemove, 1);
        }
      },
      processFile: function (file, filelist) {
        console.log("provided schema", this.schema, file.raw);

        let expectedFileType = this.schema.inputType;
        if (expectedFileType !== "*") {
          var allTypes = expectedFileType.split("|");

          var fileName = file.raw.name;
          var fileNameParts = fileName.split(".");
          var fileExtension = "";
          if (fileNameParts.length > 1) {
            fileExtension = fileNameParts[fileNameParts.length - 1];
          }

          const isFileTypeOkay = allTypes.indexOf(fileExtension) > -1;

          if (!isFileTypeOkay) {

            for (var i = 0; i < filelist.length; i++) {
              if (filelist[i].uid == file.uid) {
                filelist.splice(i, 1);
                break;
              }
            }

            this.$message.error('Please select a ' + expectedFileType + ' file. You are uploading: ' + file.raw.type);
            return isFileTypeOkay;
          }

        }

        var that = this;
        console.log("process file arguments", arguments, file, filelist);
        that.value = [];
        for (var i = 0; i < filelist.length; i++) {
          var name = filelist[i].name;
          var type = filelist[i].raw.type;
          var reader = new FileReader();
          reader.onload = (function (theFile, type) {
            return function (e) {
              // Render thumbnail.
              that.value.push({
                name: theFile,
                file: e.target.result,
                type: type
              });
            };
          })(name, type);
          reader.readAsDataURL(filelist[i].raw);
        }
      }
    }
  };
</script>
