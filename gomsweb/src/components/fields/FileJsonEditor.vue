<!-- FileUpload.vue -->
<template>
  <div class="col-md-12">
    <div id="jsonEditor" v-if="!useAce"></div>
    <editor ref="aceEditor" :options="options" :content="value" v-if="useAce" :lang="'markdown'"
            :sync="true"></editor>
  </div>
</template>

<script>
  import {abstractField} from "vue-form-generator";
  import editor from 'vue2-ace'
  import 'brace/theme/chrome'
  import 'brace/mode/markdown'
  import jsonApi from '../../plugins/jsonapi';

  export default {
    mixins: [abstractField],
    data: function () {
      return {
        fileList: [],
        useAce: false,
        options: {
          fontSize: 18,
          wrap: true,
        },
      }
    },
    components: {
      editor
    },
    updated() {

    },
    mounted() {
      window.ace.require = function (mode) {
        console.log("ace wanted mode: ", mode)
        return false;
      };
      var that = this;
      setTimeout(function () {
        var startVal = that.value;
//        if (!startVal) {
//          that.value = "";
//        }
        console.log("start value", startVal);

        try {
          console.log("try parse file json", startVal)
          var t = JSON.parse(startVal);
          startVal = JSON.stringify(t, null, 2);
          that.value = startVal;
        } catch(e) {

        }


        let schema;

        console.log("field json schema", that.schema)

        jsonApi.findAll("json_schema", {
          filter: that.schema.inputType
        }).then(function (e) {
          if (e.length > 0) {
            var schema = {};
            try {
              schema = JSON.parse(e[0].json_schema);

            } catch (e) {
              console.log("Failed to parse json schema", e)
              return;
            }
            that.useAce = false;
            setTimeout(function () {
              var element = document.getElementById('jsonEditor');
              console.log("schema", schema, element);

              try {
                var startValNew = JSON.parse(startVal);
                startVal = startValNew;
              } catch (e) {

              }

              var editor = new JSONEditor(element, {
                startval: startVal,
                schema: schema,
                theme: 'bootstrap3',
              });
              editor.on('change', function () {
                // Do something
                console.log("Json data updated", editor.getValue());
                var val = editor.getValue();
                if (!val) {
                  that.value = null;
                } else {
                  that.value = JSON.stringify(editor.getValue());
                }
              });
            }, 1000)
          }
          console.log("got json schema", e)
        })


        if (false) {
          schema = schemas[that.schema.inputType].schema;

          try {
            var startValNew = JSON.parse(startVal);
            startVal = startValNew;
          } catch (e) {

          }

          var editor = new JSONEditor(element, {
            startval: startVal,
            schema: schema,
            theme: 'bootstrap3'
          });
          editor.on('change', function () {
            // Do something
            console.log("Json data updated", editor.getValue());
            var val = editor.getValue();
            if (!val) {
              that.value = null;
            } else {
              that.value = JSON.stringify(editor.getValue());
            }
          });
        } else {
          if (!that.value) {
            that.value = "";
          }
          schema = {};
          that.useAce = true;
          that.$on('editor-update', function (newValue) {
            that.value = newValue;
          });
        }

      }, 500)
    },
    methods: {
      updated() {
        console.log("editor adsflkj asdf", arguments);
      }
    }
  };
</script>
