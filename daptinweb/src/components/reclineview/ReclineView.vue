<template>
  <div class="col-md-12" style="height: 500px;">

    <div class="data-explorer-here">
      data explorer
    </div>
    <div style="clear: both;"></div>
  </div>

</template>
<style type="text/css">
  .data-explorer-here {
    height: 600px;
  }

  .recline-slickgrid {
    height: 600px;
  }

  .recline-timeline .vmm-timeline {
    height: 550px;
  }

  /*.changelog {*/
  /*display: none;*/
  /*border-bottom: 1px solid #ccc;*/
  /*margin-bottom: 10px;*/
  /*}*/

</style>
<script>

  import _ from "underscore";
  import axios from 'axios';
  import worldManager from '../../plugins/worldmanager';


  window._ = _;


  export default {
    name: 'recline-view',
    props: {
      jsonApi: {
        type: Object,
        required: true
      },
      autoload: {
        type: Boolean,
        required: false,
        default: true
      },
      jsonApiModelName: {
        type: String,
        required: true
      },
      finder: {
        type: Array,
        required: true,
      },
      viewMode: {
        type: String,
        required: false,
        default: "card"
      },
    },
    data() {
      return {
        world: [],
        selectedWorld: null,
        selectedWorldColumns: [],
        tableData: [],
        selectedRow: {},
        multiView: null,
        explorerDiv: null,
      }
    },
    methods: {
      onAction(action, data) {
        console.log("on action", action, data);
        const that = this;
        if (action === "view-item") {
          this.$refs.vuetable.toggleDetailRow(data.id)
        } else if (action === "edit-item") {
          this.$emit("editRow", data)
        } else if (action === "go-item") {


          this.$router.push({
            name: "Instance",
            params: {
              tablename: data["__type"],
              refId: data["id"]
            }
          });
        } else if (action === "delete-item") {
          this.jsonApi.destroy(this.selectedWorld, data.id).then(function () {
            that.setTable(that.selectedWorld);
          });
        }
      },
      titleCase: function (str) {
        return str.replace(/[-_]/g, " ").split(' ')
          .map(w => w[0].toUpperCase() + w.substr(1).toLowerCase())
          .join(' ')
      },
      saveRow(data) {
        let that;
        console.log("save row", data);
        if (data.id) {
          that = this;
          that.jsonApi.update(this.selectedWorld, data).then(function () {
            that.setTable(that.selectedWorld);
            that.showAddEdit = false;
          });
        } else {
          that = this;
          that.jsonApi.create(this.selectedWorld, data).then(function () {
            that.setTable(that.selectedWorld);
            that.showAddEdit = false;
          });
        }
      },
      edit(row) {
        this.$parent.emit("editRow", row)
      },
      setTable(tableName) {
        const that = this;
        console.log("Set table in tableview by [setTable] ", tableName, that.finder);
        that.selectedWorldColumns = {};
        that.tableData = [];
        that.showAddEdit = false;
        that.reloadData(tableName)
      },
      createMultiView(dataset, state) {
        var that = this;
        console.log("that selected world columns", that.selectedWorldColumns);
        var dateColumns = [];
        var columnNames = Object.keys(that.selectedWorldColumns);
        for (var i = 0; i < columnNames.length; i++) {
          let columnName = columnNames[i];

          if (columnName == "deleted_at") {
            continue
          }


          if (columnName == "updated_at") {
            continue
          }


          var columnType = that.selectedWorldColumns[columnName];
          if (columnType == "datetime" || columnType == "date" || columnType == "time" || columnType == "timestamp") {

            if (columnName == "created_at") {
              dateColumns.push(columnName)
            } else {
              dateColumns.unshift(columnName)
            }

          }
        }
        console.log('date column', dateColumns)
        // remove existing multiview if present
        var reload = false;
        if (that.multiView) {
          that.multiView.remove();
          that.multiView = null;
          reload = true;
        }

        var $el = $('<div />');
        $el.appendTo(that.explorerDiv);

        // customize the subviews for the MultiView
        let timeline = new recline.View.Timeline({
          model: dataset,
          state: {
            startField: dateColumns[0],
            endField: dateColumns[1],
          }
        });

        timeline.convertRecord = function (record, fields) {
          var attrs = record.attributes;
          var objTitle = window.chooseTitle(attrs);
          console.log("convert 1record title", record, objTitle);
//          return objTitle;
          return {
            "startDate": attrs[dateColumns[0]],
            "endDate": attrs[dateColumns[1]],
            "headline": objTitle,
            "text": attrs["description"],
            "tag": []
          };
          var out = this._convertRecord(record);
          if (out) {
            out.headline = record.get('height').toString();
          }
          console.log("out is ", out);
          return out;
        }


        var views = [
          {
            id: 'grid',
            label: 'Grid',
            view: new recline.View.SlickGrid({
              model: dataset,
              state: {
                gridOptions: {
                  editable: true,
                  // Enable support for row delete
                  enabledDelRow: true,
                  // Enable support for row ReOrder
                  enableReOrderRow: true,
                  autoEdit: false,
                  forceFitColumns: true,
                  enableCellNavigation: true,
                },
                columnsEditor: [
                  {column: 'date', editor: Slick.Editors.Date},
                  {column: 'date-time', editor: Slick.Editors.Date},
                  {column: 'title', editor: Slick.Editors.Text}
                ]
              }
            })
          },
          {
            id: 'graph',
            label: 'Graph',
            view: new recline.View.Graph({
              model: dataset

            })
          },
          {
            id: 'map',
            label: 'Map',
            view: new recline.View.Map({
              model: dataset
            })
          },
          {
            id: "timeline",
            label: "Timeline",
            view: timeline
          }
        ];

        var multiView = new recline.View.MultiView({
          model: dataset,
          el: $el,
          state: state,
          views: views
        });
        return multiView;
      },


      reloadData(tableName) {
        const that = this;
        console.log("Reload data in tableview by [reloadData]", tableName, that.finder);

        if (!tableName) {
          tableName = that.selectedWorld;
        }

        if (!tableName) {
          alert("setting selected world to null");
        }

        that.selectedWorld = tableName;
        let jsonModel = that.jsonApi.modelFor(tableName);
        if (!jsonModel) {
          console.error("Failed to find json api model for ", tableName);
        }
        console.log("selectedWorldColumns", that.selectedWorldColumns);
        that.selectedWorldColumns = jsonModel["attributes"];
        // TODO: init recline here

        that.explorerDiv = $('.data-explorer-here');
        that.explorerDiv.html("");


        var options = {
          enableColumnReorder: false
        };

        that.createDataset(function (dataset) {
          that.dataset = dataset;
          that.multiView = that.createMultiView(that.dataset);
          that.dataset.fetch();
          that.dataset.records.bind('all', function (name, obj) {
            console.log(name, obj);


            switch (name) {
              case "change":
                that.saveRow(obj.attributes);
                break;
              case "destroy":

                that.jsonApi.destroy(that.selectedWorld, obj.id).then(function () {
                });
                break;
            }

          });

        });

      },
      createDataset(callback) {
        var that = this;
        worldManager.getReclineModel(that.jsonApiModelName, function (reclineModel) {
          console.log("columns", reclineModel);


          recline.Backend = recline.Backend || {};
          recline.Backend.JsonAPI = recline.Backend.JsonAPI || {};
          (function (my) {
            my.__type__ = 'jsonapi';
            var Deferred = (typeof jQuery !== "undefined" && jQuery.Deferred) || _.Deferred;

            // Fetch data from a Google Docs spreadsheet.
            //
            // For details of config options and returned values see the README in
            // the repo at https://github.com/Recline/backend.gdocs/
            my.fetch = function (config) {
              var dfd = new Deferred();
              console.log("backend fetch ", arguments);


              that.jsonApi.builderStack = that.finder;
              that.jsonApi.get({
                page: {
                  number: 1,
                  size: 100
                }
              }).then(function (result) {
//                result = result.data;
//                console.log("recline view result data", result)
                dfd.resolve([]);

              }, function () {
                that.$notify({
                  type: "error",
                  title: "Failed to fetch data",
                  message: "Are you still logged in ?"
                });
                dfd.reject("Failed to fetch data: Are you still logged in ?");
              })


              return dfd.promise();
            };

            my.query = function (query) {
              var dfd = new Deferred();

              that.jsonApi.builderStack = that.finder;

              var sortOrder = query.sort;
              var sort = [];
              if (sortOrder && sortOrder.length > 0) {

                for (var y = 0; y < sortOrder.length; y++) {
                  var field = sortOrder[y].field;
                  var order = sortOrder[y].order;

                  if (order == "desc") {
                    sort.push("-" + field);
                  } else {
                    sort.push(field);
                  }
                }

              }

              let {data, errors, meta, links} = that.jsonApi.get({
                page: {
                  number: query.from + 1,
                  size: query.size
                },
                filter: query.q,
                sort: sort.length > 0 ? sort.join(",") : "",
              }).then(function (result) {
                console.log("here ");
                dfd.resolve({
                  total: result.links.total,
                  hits: result.data,
                });

              }, function () {
                that.$notify({
                  type: "error",
                  title: "Failed to fetch data",
                  message: "Are you still logged in ?"
                });
                dfd.reject("Failed to fetch data: Are you still logged in ?");
              });

              console.log("backend query", arguments);
              return dfd.promise();
            }


          }(recline.Backend.JsonAPI));


          var dataset = new recline.Model.Dataset({
            fields: reclineModel,
            backend: 'jsonapi'
          });
          console.log("Dataset", dataset);
          callback(dataset);
        });
      }
    },
    mounted() {
      const that = this;
      that.selectedWorld = that.jsonApiModelName;
      console.log("Mounted ReclineView for ", that.jsonApiModelName);
      let jsonModel = that.jsonApi.modelFor(that.jsonApiModelName);
      if (!jsonModel) {
        console.error("Failed to find json api model for ", that.jsonApiModelName);
        return
      }
      that.selectedWorldColumns = Object.keys(jsonModel["attributes"]);
      that.reloadData();
    },
    watch: {
      'finder': function (newFinder, oldFinder) {
        var that = this;
        console.log("finder updated in ", newFinder, oldFinder);
        setTimeout(function () {
          that.reloadData(that.selectedWorld);
        }, 100)
      }
    }
  }
</script>
