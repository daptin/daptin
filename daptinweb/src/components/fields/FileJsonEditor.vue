<!-- FileUpload.vue -->
<template>
  <div class="col-md-12">
    <div class="ui icon buttons">
      <button @click="mode = 'ace'" class="btn btn-box-tool"><i class="fas fa-align-justify fa-2x grey"></i></button>
      <button @click="mode = 'je'" class="btn btn-box-tool"><i class="fas fa-edit fa-2x grey"></i></button>
    </div>
    <div id="jsonEditor" style="width: 100%; height: 600px;" v-if="mode == 'je'"></div>
    <editor ref="aceEditor" :options="options" :content="initValue" v-if="mode == 'ace'" :lang="'markdown'"
            :sync="true"></editor>
  </div>
</template>

<script>
  import {abstractField} from "vue-form-generator";
  import editor from 'vue2-ace'
  import 'brace/theme/chrome'
  import 'brace/mode/markdown'
  import jsonApi from '../../plugins/jsonapi';
  import Jsoneditor from 'jsoneditor';

  require("jsoneditor/dist/jsoneditor.min.css");

  export default {
    mixins: [abstractField],
    data: function () {
      return {
        fileList: [],
        useAce: false,
        mode: 'none',
        initValue: null,
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
//        console.log("ace wanted mode: ", mode);
        return false;
      };
      var that = this;
      setTimeout(function () {
        var startVal = that.value;
//        if (!startVal) {
//          that.value = "";
//        }
//        console.log("start value", startVal);

        try {
//          console.log("try parse file json", startVal);
          var t = JSON.parse(startVal);
          startVal = JSON.stringify(t, null, 2);
          that.value = startVal;
        } catch (e) {

        }


        let schema;

        console.log("field json schema", that.schema);

        jsonApi.findAll("json_schema", {
          filter: that.schema.inputType
        }).then(function (e) {
          e = e.data;
          if (e.length > 0) {
            var schema = {};
            try {
              schema = JSON.parse(e[0].json_schema);

            } catch (e) {
              console.log("Failed to parse json schema", e);
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
        });


        console.log("this is new");
        if (false) {
          try {
            var json = JSON.parse(startVal);
            if (json instanceof Object) {
              var container = document.getElementById("jsonEditor");
              var editor = new Jsoneditor(container, {
                onChange: function () {
                  that.value = JSON.stringify(editor.get());
                }
              });
              editor.set(json);
              return;
            }
          } catch (e) {
            console.log("Failed to init json editor", e)
          }


        }

        if (false) {

        } else {
          if (!that.value) {
            that.value = "";
          }
          schema = {};
          that.useAce = true;
          that.initValue = that.value;
          that.$on('editor-update', function (newValue) {
            console.log("Value  updated", newValue);
            that.value = newValue;
          });
        }

      }, 500)
    },
    methods: {
      updated() {
        console.log("editor adsflkj asdf", arguments);
      }
    },
    watch: {
      mode: function (newMode) {
        var that = this;
        var startVal = this.value;
        switch (newMode) {
          case "ace":
            break;
          case "je":
            var json = JSON.parse(startVal);
            if (json instanceof Object) {
              setTimeout(function () {
                var container = document.getElementById("jsonEditor");
                var editor = new Jsoneditor(container, {
                  onChange: function () {
                    that.value = JSON.stringify(editor.get());
                  }
                });
                editor.set(json);
              }, 500)
            }
            break;
        }
        console.log("mode changes", arguments)
      },
      initValue: function(newValue) {
        this.value = newValue;
      }
    }
  };
</script>
