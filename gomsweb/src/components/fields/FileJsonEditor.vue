<!-- FileUpload.vue -->
<template>
  <div class="col-md-12">
    <div id="jsonEditor" v-if="!useAce"></div>
    <editor ref="aceEditor" :content="value" v-if="useAce" :lang="'json'"
            :sync="true"></editor>
  </div>
</template>

<script>
  import {abstractField} from "vue-form-generator";
  import editor from 'vue2-ace'
  import 'brace/mode/json'
  import 'brace/theme/chrome'


  var schemas = {
    "world": {
      schema: {
        type: "object",
        headerTemplate: "{{self.TableName}}",
        title: "Entity Description",
        properties: {
          IsHidden: {
            type: "boolean",
            title: "Hidden table"
          },
          DefaultPermission: {
            type: "integer",
            title: "Default permission",
            propertyOrder: 100
          },
          TableName: {
            type: "string",
            title: "Table Name",
            propertyOrder: 1
          },
          Columns: {
            type: "array",
            title: "Columns",
            propertyOrder: 10,
            items: {
              title: "Column",
              type: "object",
              headerTemplate: "{{self.Name}}",
              properties: {
                Name: {
                  type: "string",
                  title: "Name",
                  propertyOrder: 1
                },
                ColumnName: {
                  type: "string",
                  title: "Column Name",
                  propertyOrder: 1
                },
                ColumnType: {
                  type: "string",
                  title: "Column Type",
                  propertyOrder: 1,
                  enum: [
                    "id",
                    "alias",
                    "date",
                    "time",
                    "day",
                    "month",
                    "year",
                    "minute",
                    "hour",
                    "datetime",
                    "email",
                    "name",
                    "value",
                    "truefalse",
                    "timestamp",
                    "location.latitude",
                    "location.longitude",
                    "location.altitude",
                    "color",
                    "measurement",
                    "label",
                    "content",
                    "file",
                    "url",
                    "image"
                  ]
                },
                IsPrimaryKey: {
                  type: "boolean",
                  "format": "checkbox",
                  propertyOrder: 10,
                  title: "Is Primary Key"
                },
                IsAutoIncrement: {
                  type: "boolean",
                  "format": "checkbox",
                  propertyOrder: 10,
                  title: "Is Auto Increment"
                },
                IsIndexed: {
                  type: "boolean",
                  "format": "checkbox",
                  propertyOrder: 10,
                  title: "Is Indexed"
                },
                IsUnique: {
                  type: "boolean",
                  "format": "checkbox",
                  propertyOrder: 10,
                  title: "Is Unique"
                },
                IsNullable: {
                  type: "boolean",
                  "format": "checkbox",
                  propertyOrder: 10,
                  title: "Is Nullable"
                },
                Permission: {
                  type: "string",
                  propertyOrder: 1,
                  title: "Permission",
                  default: "755",
                  enum: [
                    '644',
                    '655',
                    '666',
                    '755',
                    '766',
                    '777',
                    '444',
                    '222',
                    '111',
                    '700',
                    '070',
                    '007',
                    '770',
                    '707',
                    '077',
                    '000'
                  ]
                },
                IsForeignKey: {
                  type: "boolean",
                  "format": "checkbox",
                  title: "Is Foreign Key"
                },
                ExcludeFromApi: {
                  type: "boolean",
                  "format": "checkbox",
                  propertyOrder: 10,
                  title: "Exclude from API"
                },
                ForeignKeyData: {
                  type: "object",
                  title: "Foreign Key Data",
                  propertyOrder: 5,
                  properties: {
                    DataSource: {
                      type: "string",
                      title: "Data Source",
                      enum: [
                        "self",
                        "rest"
                      ]
                    },
                    TableName: {
                      type: "string",
                      title: "Table Name",
                    },
                    ColumnName: {
                      type: "string",
                      title: "Column Name",
                    },
                  }
                },
                DataType: {
                  type: "string",
                  propertyOrder: 12,
                  title: "Data type",
                  enum: [
                    "int(11)",
                    "varchar(10)",
                    "varchar(50)",
                    "varchar(100)",
                    "varchar(200)",
                    "varchar(500)",
                    "varchar(1000)",
                    "text",
                    "timestamp",
                    "text",
                  ],
                },
                DefaultValue: {
                  type: "string",
                  propertyOrder: 3,
                  title: "Default Value",

                },
              },
            },
          },
          Relations: {
            type: "array",
            items: {
              headerTemplate: "{{self.SubjectName}} {{self.Relation}} {{self.ObjectName}} ",
              properties: {
                Subject: {
                  type: "string",
                  title: "Subject"
                },
                SubjectName: {
                  type: "string",
                  title: "Subject Name"
                },
                Relation: {
                  type: "string",
                  title: "Relation",
                  enum: [
                    "has_one",
                    "has_many",
                    "belongs_to",
                    "has_many_and_belongs_to_many"
                  ]
                },
                Object: {
                  type: "string",
                  title: "Object"
                },
                ObjectName: {
                  type: "string",
                  title: "ObjectName"
                },

              },
            },
          },
          IsStateTrackingEnabled: {
            type: 'boolean',
            "format": "checkbox",
            title: "Is State Tracking Enabled"
          },
          IsJoinTable: {
            type: 'boolean',
            "format": "checkbox",
            title: "Is Join Table"
          },
          IsTopLevel: {
            type: 'boolean',
            "format": "checkbox",
            title: "Show table on side bar"
          },
          Permission: {
            type: 'integer',
            title: "Permission",
            enum: [
              '644',
              '655',
              '666',
              '755',
              '766',
              '777',
              '444',
              '222',
              '111',
              '700',
              '070',
              '007',
              '770',
              '707',
              '077',
              '000'
            ]
          }
        },
        required_by_default: true,
        defaultProperties: ["table_name"]
      }
    },
    data_exchange_options: {
      schema: {
        type: "object",
        title: "Data exchange options",
        properties: {
          hasHeaders: {
            title: "Has Headers",
            type: "boolean",
            "format": "checkbox",
          }
        }
      },
    },
    data_exchange_attributes: {
      schema: {
        type: "array",
        title: "Data exchange attributes",
        items: {
          properties: {
            sourceColumn: {
              type: "string",
              title: "Source column"
            },
            sourceColumnType: {
              type: "string",
              title: "Source column type"

            },
            targetColumn: {
              type: "string",
              title: "Target column"
            },
            targetColumnType: {
              type: "string",
              title: "Target column type"
            }
          }
        }
      },
    }
  };

  export default {
    mixins: [abstractField],
    data: function () {
      return {
        fileList: [],
        useAce: false,
      }
    },
    components: {
      editor
    },
    updated() {

    },
    mounted() {
      var that = this;
      setTimeout(function () {
        var startVal = that.value;
        if (!startVal) {
          startVal = {};
        } else {
          if (typeof startVal != "string") {
            startVal = startVal
          } else {
            try {
              var startValNew = JSON.parse(startVal);
              startVal = startValNew;
            } catch (e) {

            }
          }
        }
        console.log("start value", startVal);
        var element = document.getElementById('jsonEditor');

        let schema;
        if (schemas[that.schema.inputType]) {
          schema = schemas[that.schema.inputType].schema;

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
          schema = {};
          that.value = JSON.stringify(startVal, null, 2);
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
