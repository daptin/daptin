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
        <q-breadcrumbs-el label="Actions" icon="fas fa-wrench"/>
      </q-breadcrumbs>
    </div>
    <q-separator></q-separator>

    <div class="row q-pa-md q-gutter-sm">

      <div class="col-12">
        <q-input clear-icon="fas fa-times" label="search" v-model="actionFilter"></q-input>
      </div>
      <div class="col-12">
        <q-markup-table flat>
          <tbody>
          <tr v-for="action in filteredActions">
            <td>{{action.action_schema.Label}}</td>
            <td>{{action.action_schema.OnType}}</td>
            <td>{{action.action_schema.InFields ? action.action_schema.InFields.length: 0}}</td>
            <td>{{action.action_schema.OutFields.length}}</td>
            <td class="text-right">
              <q-btn @click="showEditAction(action)" size="sm"
                     label="Edit action" class="float-right"></q-btn>

            </td>
          </tr>
          </tbody>
        </q-markup-table>
      </div>
      <div class="col-4 col-xl-2 col-lg-3 col-xs-12 col-sm-6 q-pa-md" v-for="action in filteredActions">

        <q-card>
          <q-card-section>
            <span class="text-h6">{{action.action_schema.Label}}</span>
          </q-card-section>
          <q-card-section>
            <span>On</span> <span class="text-bold float-right">{{action.action_schema.OnType}}</span>
          </q-card-section>
          <q-card-section>
            <span>Input fields</span> <span class="text-bold float-right">{{action.action_schema.InFields ? action.action_schema.InFields.length: 0}}</span>
          </q-card-section>
          <q-card-section>
            <span>Output actions</span> <span
            class="text-bold float-right">{{action.action_schema.OutFields.length}}</span>
          </q-card-section>
          <q-card-section>
            <div class="row">
              <div class="col-12">
                <!--                <q-btn size="sm" @click="listFiles(action)" label="Browse files" color="primary"-->
                <!--                       class="float-right"></q-btn>-->
                <q-btn @click="showEditAction(action)" size="sm"
                       label="Edit action" class="float-right"></q-btn>
              </div>
            </div>
          </q-card-section>
        </q-card>
      </div>

    </div>


    <q-page-sticky style="z-index: 3000" position="bottom-right" :offset="[20, 20]">
      <q-btn @click="showCreateActionDrawer = true" fab icon="add" color="primary"/>
    </q-page-sticky>

    <q-drawer overlay content-class="bg-grey-3" :width="400" side="right" v-model="showCreateActionDrawer">
      <q-scroll-area class="fit row">
        <div class="q-pa-md">
          <span class="text-h6">Create action</span>
          <q-form class="q-gutter-md">
            <q-input label="Name" v-model="newAction.name"></q-input>


            <q-input label="Root path" v-model="newAction.root_path"></q-input>


            <q-btn color="primary" @click="createAction()">Create</q-btn>
            <q-btn @click="showCreateActionDrawer = false">Cancel</q-btn>
          </q-form>
        </div>
      </q-scroll-area>
    </q-drawer>


    <q-drawer overlay content-class="bg-grey-3" :width="400" side="right" v-model="showEditActionDrawer">
      <q-scroll-area class="fit row">
        <div class="q-pa-md">
          <span class="text-h6">Edit action</span>
          <q-form class="q-gutter-md">
            <q-input label="Name" v-model="newAction.name"></q-input>


            <q-input label="Root path" v-model="newAction.root_path"></q-input>

            <q-btn color="negative" @click="deleteAction()">Delete</q-btn>
            <q-btn class="float-right" color="primary" @click="editAction()">Save</q-btn>
            <q-btn class="float-right" @click="showEditActionDrawer = false">Cancel</q-btn>
          </q-form>
        </div>
      </q-scroll-area>
    </q-drawer>


  </div>
</template>

<script>
  import {mapActions, mapGetters, mapState} from 'vuex';

  export default {
    name: 'ActionPage',
    methods: {
      // listFiles(action) {
      //   console.log("list files in cloud action", action)
      //   const that = this;
      //   that.executeAction({
      //     tableName: "action",
      //     actionName: "list_files",
      //     params: {
      //       action_id: action.id
      //     }
      //   }).then(function (res) {
      //     console.log("list files Response", res)
      //   }).catch(function (err) {
      //     console.log("failed to list files", err)
      //   })
      // },
      showEditAction(action) {
        this.selectedAction = action
        this.showEditActionDrawer = true
        this.newAction.name = action.name;
        this.newAction.root_path = action.root_path;
      },
      deleteAction() {
        const that = this;
        console.log("Delete action", this.selectedAction);
        this.deleteRow({
          tableName: "action",
          reference_id: this.selectedAction.id
        }).then(function (res) {
          that.showEditActionDrawer = false;
          that.selectedAction = {};
          that.$q.notify({
            title: "Success",
            message: "Action deleted"
          });
          that.refresh()
        }).catch(function (res) {
          that.$q.notify({
            title: "Failed",
            message: JSON.stringify(res)
          })
        })
      },
      editAction() {
        const that = this;
        console.log("Delete action", this.selectedAction);
        this.newAction.tableName = "action";
        this.newAction.id = this.selectedAction.id;
        this.updateRow(this.newAction).then(function (res) {
          that.showEditActionDrawer = false;
          that.selectedAction = {};
          that.$q.notify({
            title: "Success",
            message: "Action updated"
          });
          that.refresh()
        }).catch(function (res) {
          that.$q.notify({
            title: "Failed",
            message: JSON.stringify(res)
          })
        })
      },
      createAction() {
        const that = this;
        console.log("new cloud", this.newAction);
        this.newAction.tableName = "action";
        that.createRow(that.newAction).then(function (res) {
          that.user = {};
          that.$q.notify({
            message: "cloud action created"
          });
          that.refresh();
          that.showCreateActionDrawer = false;
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
        var tableName = "action";
        const that = this;
        this.loadData({
          tableName: tableName, params: {
            page: {
              size: 500
            }
          }
        }).then(function (data) {
          console.log("Loaded data", data);
          that.actions = data.data.map(function (e) {
            e.action_schema = JSON.parse(e.action_schema);
            return e;
          });
        })
      }
    },
    data() {
      return {
        text: '',
        actionFilter: null,
        selectedAction: {},
        actionProviderOptions: [
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
        newAction: {
          name: null,
          action_provider: 'local',
          action_type: 'local',
          root_path: null,
          action_parameters: '{}',
        },
        showCreateActionDrawer: false,
        showEditActionDrawer: false,
        filter: null,
        actions: [],
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
      filteredActions() {
        const that = this;
        return that.actions.filter(function (e) {
          return !that.actionFilter || (
              e.action_name.indexOf(that.actionFilter) > -1 ||
              e.action_schema.OnType.indexOf(that.actionFilter) > -1 ||
              e.action_schema.Label.indexOf(that.actionFilter) > -1
          )
        })
      },
      ...mapGetters(['selectedTable']),
      ...mapState([])
    },

    watch: {}
  }
</script>
