<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <div>

    <div class="q-pa-md q-gutter-sm">
      <q-breadcrumbs>
        <template v-slot:separator>
          <q-icon
            size="1.2em"
            name="arrow_forward"
            color="black"
          />
        </template>

        <q-breadcrumbs-el label="Integrations" icon="fas fa-bolt"/>
        <q-breadcrumbs-el label="API Catalogue" icon="fas fa-plug"/>
      </q-breadcrumbs>
    </div>
    <q-separator></q-separator>

    <div class="row">
      <div class="col-xl-3 col-lg-4 col-6 col-sm-8 col-xs-12 q-pa-md">
        <q-input label="Search" v-model="filterWord"></q-input>
      </div>
    </div>
    <div class="row">

      <div class="col-4 col-xl-2 col-lg-3 col-xs-12 col-sm-6 q-pa-md" v-for="integration in filteredIntegrations">
        <q-card>
          <q-card-section>
            <span class="text-h6"
                  style="text-overflow: ellipsis; overflow: hidden; white-space: nowrap; display: -webkit-box; -webkit-line-clamp: 1; -webkit-box-orient: vertical;">{{integration.name}}</span>
          </q-card-section>
          <q-card-section>
            <span>Format</span> <span class="text-bold float-right">{{integration.specification_format}}</span>
          </q-card-section>
          <q-card-section>
            <span>Language</span> <span class="text-bold float-right">{{integration.specification_language}}</span>
          </q-card-section>
          <q-card-section>
            <div class="row">
              <div class="col-12">
                <!--                <q-btn size="sm" @click="listFiles(integration)" label="Browse files" color="primary"-->
                <!--                       class="float-right"></q-btn>-->
                <q-btn @click="showEditIntegration(integration)" size="sm"
                       label="Edit integration" class="float-right"></q-btn>
              </div>
            </div>
          </q-card-section>
        </q-card>
      </div>

    </div>


    <q-page-sticky style="z-index: 3000" position="bottom-right" :offset="[20, 20]">
      <q-btn @click="showCreateIntegrationDrawer = true" fab icon="add" color="primary"/>
    </q-page-sticky>

    <q-drawer overlay content-class="bg-grey-3" :width="400" side="right" v-model="showCreateIntegrationDrawer">
      <q-scroll-area class="fit row">
        <div class="q-pa-md">
          <span class="text-h6">Create integration</span>
          <q-form class="q-gutter-md">
            <q-input label="Name" v-model="newIntegration.name"></q-input>
            <q-file @input="fileAdded()" label="OpenAPI Spec file" v-model="specFile"></q-file>

            <q-btn color="primary" :loading="fileIsBeingLoaded" @click="createIntegration()">Create</q-btn>
            <q-btn @click="showCreateIntegrationDrawer = false">Cancel</q-btn>
          </q-form>
        </div>
      </q-scroll-area>
    </q-drawer>


    <q-drawer overlay content-class="bg-grey-3" :width="400" side="right" v-model="showEditIntegrationDrawer">
      <q-scroll-area class="fit row">
        <div class="q-pa-md">
          <span class="text-h6">Edit integration</span>
          <q-form class="q-gutter-md">
            <q-input disable label="Name" v-model="newIntegration.name"></q-input>


            <q-btn color="negative" @click="deleteIntegration()">Delete</q-btn>
            <q-btn class="float-right" @click="showEditIntegrationDrawer = false">Cancel</q-btn>
          </q-form>
        </div>
      </q-scroll-area>
    </q-drawer>


  </div>
</template>

<script>
  import {mapActions, mapGetters, mapState} from 'vuex';

  export default {
    name: 'ApiCataloguePage',
    methods: {
      fileAdded() {
        const that = this;
        this.fileIsBeingLoaded = true;
        var file = this.specFile;
        console.log("File to read", file);

        if (file.name.toLowerCase().endsWith(".yaml") || file.type.toLowerCase().endsWith("yaml")) {
          this.newIntegration.specification_format = "yaml";
        } else {
          this.newIntegration.specification_format = "json";
        }

        var obj = {};
        var filePromise = new Promise(function (resolve, reject) {
          const name = file.name;
          const type = file.type;
          const reader = new FileReader();
          reader.onload = function (fileResult) {

            resolve(fileResult);
          };
          reader.onerror = function () {
            console.log("Failed to load file onerror", e, arguments);
            reject(name);
          };
          reader.readAsDataURL(file);
        });

        filePromise.then(function (specData) {
          console.log("Spec file added", that.newIntegration, that.specFile);
          console.log("File data", specData);
          var specContentText = atob(specData.target.result.split("base64,")[1]);
          console.log("Spec content text", specContentText)

          if (specContentText.indexOf("openapi: 3") > -1) {
            that.newIntegration.specification_language = "openapiv3"
          }

          if (specContentText.indexOf("openapi: 2") > -1) {
            that.newIntegration.specification_language = "openapiv2"
          }

          if (specContentText.indexOf("\"openapi\": \"3") > -1) {
            that.newIntegration.specification_language = "openapiv3"
          }

          if (specContentText.indexOf("\"openapi\": \"2") > -1) {
            that.newIntegration.specification_language = "openapiv2"
          }

          that.newIntegration.specification = specContentText;
          that.fileIsBeingLoaded = false;
        })

      },
      // listFiles(integration) {
      //   console.log("list files in cloud integration", integration)
      //   const that = this;
      //   that.executeAction({
      //     tableName: "integration",
      //     actionName: "list_files",
      //     params: {
      //       integration_id: integration.id
      //     }
      //   }).then(function (res) {
      //     console.log("list files Response", res)
      //   }).catch(function (err) {
      //     console.log("failed to list files", err)
      //   })
      // },
      showEditIntegration(integration) {
        this.selectedIntegration = integration
        this.showEditIntegrationDrawer = true
        this.newIntegration.name = integration.name;
        this.newIntegration.root_path = integration.root_path;
      },
      deleteIntegration() {
        const that = this;
        console.log("Delete integration", this.selectedIntegration);
        this.deleteRow({
          tableName: "integration",
          reference_id: this.selectedIntegration.id
        }).then(function (res) {
          that.showEditIntegrationDrawer = false;
          that.selectedIntegration = {};
          that.$q.notify({
            title: "Success",
            message: "Integration deleted"
          });
          that.refresh()
        }).catch(function (res) {
          that.$q.notify({
            title: "Failed",
            message: JSON.stringify(res)
          })
        })
      },
      editIntegration() {
        const that = this;
        console.log("Delete integration", this.selectedIntegration);
        this.newIntegration.tableName = "integration";
        this.newIntegration.id = this.selectedIntegration.id;
        this.updateRow(this.newIntegration).then(function (res) {
          that.showEditIntegrationDrawer = false;
          that.selectedIntegration = {};
          that.$q.notify({
            title: "Success",
            message: "Integration updated"
          });
          that.refresh()
        }).catch(function (res) {
          that.$q.notify({
            title: "Failed",
            message: JSON.stringify(res)
          })
        })
      },
      createIntegration() {
        const that = this;
        console.log("new integration", this.newIntegration);
        this.newIntegration.tableName = "integration";
        that.createRow(that.newIntegration).then(function (res) {
          that.user = {};
          that.$q.notify({
            message: "cloud integration created"
          });
          that.refresh();
          that.showCreateIntegrationDrawer = false;
        }).catch(function (e) {
          if (e instanceof Array) {
            that.$q.notify({
              message: e[0].title
            })
          } else {
            that.$q.notify({
              message: "Failed to create integration"
            })
          }
        });
      },
      ...mapActions(['loadData', 'getTableSchema', 'createRow', 'deleteRow', 'updateRow', 'executeAction']),
      refresh() {
        var tableName = "integration";
        const that = this;
        this.loadData({
          tableName: tableName,
          params: {
            fields: "name,specification_language,specification_format",
            page: {
              size: 500,
            }
          }
        }).then(function (data) {
          console.log("Loaded data", data);
          that.integrations = data.data;
        })
      }
    },
    data() {
      return {
        text: '',
        fileIsBeingLoaded: false,
        filterWord: null,
        selectedIntegration: {},
        showHelp: false,
        specFile: null,
        newIntegration: {
          name: null,
          enable: true,
          specification_format: null,
          specification: null,
          authentication_type: 'token',
          authentication_specification: '{}',
          specification_language: null,
        },
        showCreateIntegrationDrawer: false,
        showEditIntegrationDrawer: false,
        filter: null,
        integrations: [],
        columns: [
          {
            name: 'name',
            field: 'name',
            label: 'cloud name',
            align: 'left',
            sortable: true,
          }
        ],
        ...mapState([])
      }
    },
    mounted() {
      this.refresh();
    },
    computed: {
      filteredIntegrations() {
        const that = this;
        console.log("filtered integragtions", that.filterWord, that.integrations)
        return !that.filterWord ? this.integrations : this.integrations.filter(function (e) {
          return e.name.toLowerCase().indexOf(that.filterWord.toLowerCase()) > -1;
        })
      },
      ...mapGetters(['selectedTable']),
      ...mapState([])
    },

    watch: {}
  }
</script>
