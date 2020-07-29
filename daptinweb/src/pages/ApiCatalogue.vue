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

        <q-breadcrumbs-el label="Storage" icon="fas fa-amazon"/>
        <q-breadcrumbs-el label="Integrations" icon="fas fa-list"/>
      </q-breadcrumbs>
    </div>
    <q-separator></q-separator>

    <div class="row q-pa-md q-gutter-sm">

      <div class="col-4 col-xl-2 col-lg-3 col-xs-12 col-sm-6 q-pa-md" v-for="integration in integrations">
        <q-card>
          <q-card-section>
            <span class="text-h6">{{integration.name}}</span>
          </q-card-section>
          <q-card-section>
            <span>Provider</span> <span class="text-bold float-right">{{integration.integration_provider}}</span>
          </q-card-section>
          <q-card-section>
            <span>Root path</span> <span class="text-bold float-right">{{integration.root_path}}</span>
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
      <q-btn @click="showCreateintegrationDrawer = true" fab icon="add" color="primary"/>
    </q-page-sticky>

    <q-drawer overlay content-class="bg-grey-3" :width="400" side="right" v-model="showCreateintegrationDrawer">
      <q-scroll-area class="fit row">
        <div class="q-pa-md">
          <span class="text-h6">Create integration</span>
          <q-form class="q-gutter-md">
            <q-input label="Name" v-model="newIntegration.name"></q-input>

            <!--            <q-input readonly label="Integration type" v-model="newIntegration.integration_type"></q-input>-->


            <!--            <q-input label="Integration provider" v-model="newIntegration.integration_provider"></q-input>-->

            <!--            <q-select-->
            <!--              filled-->
            <!--              v-model="newIntegration.integration_provider"-->
            <!--              :options="integrationProviderOptions"-->
            <!--              label="Provider"-->
            <!--              color="black"-->
            <!--              options-selected-class="text-deep-orange"-->
            <!--            >-->
            <!--              <template v-slot:option="scope">-->
            <!--                <q-item-->
            <!--                  v-bind="scope.itemProps"-->
            <!--                  v-on="scope.itemEvents"-->
            <!--                >-->
            <!--                  <q-item-section avatar>-->
            <!--                    <q-icon :name="scope.opt.icon"/>-->
            <!--                  </q-item-section>-->
            <!--                  <q-item-section>-->
            <!--                    <q-item-label v-html="scope.opt.label"/>-->
            <!--                    <q-item-label caption>{{ scope.opt.description }}</q-item-label>-->
            <!--                  </q-item-section>-->
            <!--                </q-item>-->
            <!--              </template>-->
            <!--            </q-select>-->


            <q-input label="Root path" v-model="newIntegration.root_path"></q-input>

            <!--            <q-editor-->
            <!--              :toolbar="[]"-->
            <!--              style="font-family: 'JetBrains Mono'"-->
            <!--              label="Integration parameters"-->
            <!--              v-model="newIntegration.integration_parameters"-->
            <!--            />-->


            <q-btn color="primary" @click="createIntegration()">Create</q-btn>
            <q-btn @click="showCreateintegrationDrawer = false">Cancel</q-btn>
          </q-form>
        </div>
      </q-scroll-area>
    </q-drawer>


    <q-drawer overlay content-class="bg-grey-3" :width="400" side="right" v-model="showEditintegrationDrawer">
      <q-scroll-area class="fit row">
        <div class="q-pa-md">
          <span class="text-h6">Edit integration</span>
          <q-form class="q-gutter-md">
            <q-input label="Name" v-model="newIntegration.name"></q-input>


            <q-input label="Root path" v-model="newIntegration.root_path"></q-input>

            <q-btn color="negative" @click="deleteIntegration()">Delete</q-btn>
            <q-btn class="float-right" color="primary" @click="editIntegration()">Save</q-btn>
            <q-btn class="float-right" @click="showEditintegrationDrawer = false">Cancel</q-btn>
          </q-form>
        </div>
      </q-scroll-area>
    </q-drawer>


  </div>
</template>

<script>
  import {mapActions, mapGetters, mapState} from 'vuex';

  export default {
    name: 'TablePage',
    methods: {
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
        this.showEditintegrationDrawer = true
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
          that.showEditintegrationDrawer = false;
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
          that.showEditintegrationDrawer = false;
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
        console.log("new cloud", this.newIntegration);
        this.newIntegration.tableName = "integration";
        that.createRow(that.newIntegration).then(function (res) {
          that.user = {};
          that.$q.notify({
            message: "cloud integration created"
          });
          that.refresh();
          that.showCreateintegrationDrawer = false;
        }).catch(function (e) {
          if (e instanceof Array) {
            that.$q.notify({
              message: e[0].title
            })
          } else {
            that.$q.notify({
              message: "Failed to create cloud"
            })
          }
        });
      },
      ...mapActions(['loadData', 'getTableSchema', 'createRow', 'deleteRow', 'updateRow', 'executeAction']),
      refresh() {
        var tableName = "integration";
        const that = this;
        this.loadData({tableName: tableName}).then(function (data) {
          console.log("Loaded data", data);
          that.integrations = data.data;
        })
      }
    },
    data() {
      return {
        text: '',
        selectedIntegration: {},
        integrationProviderOptions: [
          {
            icon: 'fas fa-aws',
            label: 'Amazon Drive',
            description: 'OAuth token based'
          },
          {
            icon: 'fas fa-aws',
            label: 'Amazon S3',
            description: 'OAuth token based'
          },
          {
            icon: 'fas fa-aws',
            label: 'Backblaze B2',
            description: 'OAuth token based'
          },
          {
            icon: 'fas fa-aws',
            label: 'Dropbox',
            description: 'OAuth token based'
          },
          {
            icon: 'fas fa-aws',
            label: 'FTP',
            description: 'OAuth token based'
          },
          {
            icon: 'fas fa-aws',
            label: 'Google Drive',
            description: 'OAuth token based'
          },
          {
            icon: 'fas fa-aws',
            label: 'local',
            description: 'The local filesystem'
          },
        ],
        showHelp: false,
        newIntegration: {
          name: null,
          integration_provider: 'local',
          integration_type: 'local',
          root_path: null,
          integration_parameters: '{}',
        },
        showCreateintegrationDrawer: false,
        showEditintegrationDrawer: false,
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
      ...mapGetters(['selectedTable']),
      ...mapState([])
    },

    watch: {}
  }
</script>
