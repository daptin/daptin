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
          <thead>
          <tr class="text-left">
            <th>Name</th>
            <th># Input fields</th>
            <th># Output fields</th>
            <th></th>
          </tr>
          </thead>
          <tbody>
          <tr v-for="action in filteredActions">
            <td>{{ action.action_schema.Label }} on {{ action.action_schema.OnType }}</td>
            <td>{{ action.action_schema.InFields ? action.action_schema.InFields.length : 0 }}</td>
            <td>{{ action.action_schema.OutFields ? action.action_schema.OutFields.length : 0 }}</td>
            <td class="text-right">
              <q-btn @click="showEditAction(action)" size="sm"
                     label="Edit action" class="float-right"></q-btn>

            </td>
          </tr>
          </tbody>
        </q-markup-table>
      </div>

    </div>


    <q-page-sticky style="z-index: 3000" position="bottom-right" :offset="[20, 20]">
      <q-btn @click="showCreateAction()" fab icon="add" color="primary"/>
    </q-page-sticky>

    <q-drawer overlay content-class="bg-grey-3" :width="400" side="right" v-model="showCreateActionDrawer">
      <q-scroll-area class="fit row">
        <div class="q-pa-md">
          <span class="text-h6">Create action</span>
          <q-form class="q-gutter-md">
            <textarea id="actionSchemaEditor"></textarea>
            <!--            <q-input label="Action Name" v-model="newAction.action_name"></q-input>-->
            <!--            <q-input label="Label" v-model="newAction.label"></q-input>-->
            <!--            <q-select label="On Type" :options="tables" option-value="reference_id" emit-value map-options-->
            <!--                      option-label="table_name" v-model="newAction.onType"></q-select>-->


            <q-btn class="float-right" color="primary" @click="createAction()">Create</q-btn>
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
            <q-btn color="negative" @click="deleteAction()">Delete</q-btn>
            <q-btn class="float-right" @click="showEditActionDrawer = false">Cancel</q-btn>
          </q-form>
        </div>
      </q-scroll-area>
    </q-drawer>


  </div>
</template>

<script>
import {mapActions, mapGetters, mapState} from 'vuex';
import "simplemde/dist/simplemde.min.css";
import SimpleMDE from 'simplemde';

const yaml = require('js-yaml');

export default {
  name: 'ActionPage',
  methods: {
    showCreateAction() {
      const that = this;
      that.showCreateActionDrawer = true;
      //  actionSchemaEditor
      setTimeout(function () {
        if (that.actionSchemaEditor && that.actionSchemaEditor.toTextArea) {
          that.actionSchemaEditor.toTextArea();
        }

        that.actionSchemaEditor = new SimpleMDE({
          element: document.getElementById("actionSchemaEditor"),
          toolbar: [],
        });

        that.actionSchemaEditor.value(`---
Name: my_new_action
Label: New Action Button Label
OnType: user_account
InstanceOptional: false
InFields:
OutFields:`)

      }, 400)
    },
    showEditAction(action) {
      this.selectedAction = action;
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
      console.log("new action", this.newAction);

      let actinSchema = that.actionSchemaEditor.value();
      let spec = {}
      try {
        spec = yaml.load(actinSchema);
        if (!spec) {
          that.$q.notify({
            message: "Invalid spec, not valid YAML"
          });
          return
        }
      } catch (e) {
        that.$q.notify({
          message: "Invalid spec, not valid YAML"
        });
        return
      }

      this.newAction.action_name = spec.Name;
      this.newAction.on_type = that.tables.filter(function (e) {
        return e.table_name === spec.OnType
      })[0];
      this.newAction.label = spec.Name + " on " + this.newAction.on_type.table_name;
      this.newAction.action_schema = JSON.stringify(spec);


      this.newAction.tableName = "action";
      this.newAction.world_id = {type: "world", "id": this.newAction.on_type.id};
      console.log("New action", this.newAction)
      that.createRow(that.newAction).then(function (res) {
        that.user = {};
        that.$q.notify({
          message: "action action created"
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
            message: "Failed to create action"
          })
        }
      });
    },
    ...mapActions(['loadData', 'getTableSchema', 'createRow', 'deleteRow', 'updateRow', 'executeAction']),
    refresh() {
      var tableName = "action";
      const that = this;
      this.loadData({
        tableName: tableName,
        params: {
          page: {
            size: 500
          }
        }
      }).then(function (data) {
        console.log("Loaded data", data);
        let actions = data.data.map(function (e) {
          try {
            e.action_schema = JSON.parse(e.action_schema)
          } catch (e) {
            e.action_schema = {
              InFields: [],
              OutFields: [],
              Name: e.action_name,
              Label: e.action_name,
            }
          }
          return e;
        });
        actions.sort(function (a, b) {
          return a.action_name < b.action_name;
        })
        that.actions = actions;
      })
      this.loadData({
        tableName: "world",
        params: {
          page: {
            size: 500
          }
        }
      }).then(function (data) {
        console.log("Loaded tables data", data);
        let tables = data.data.filter(function (e) {
          return e.table_name.indexOf("_has_") === -1;
        });
        tables = tables.sort(function (a, b) {
          return a.table_name > b.table_name;
        });
        that.tables = tables;
      })
    }
  },
  data() {
    return {
      text: '',
      tables: [],
      actionSchemaEditor: null,
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
        action_name: null,
        label: null,
        action_schema: '',
        world_id: null,
        instance_optional: false,
      },
      showCreateActionDrawer: false,
      showEditActionDrawer: false,
      filter: null,
      actions: [],
      columns: [
        {
          name: 'name',
          field: 'name',
          label: 'action name',
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
    ...mapGetters([]),
    ...mapState([])
  },

  watch: {}
}
</script>
