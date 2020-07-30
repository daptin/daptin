<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <div>
    <div class="q-pa-md q-gutter-sm">
      <q-breadcrumbs>
        <template v-slot:separator>
          <q-icon
            size="1.2em"
            name="arrow_forward"
          />
        </template>

        <q-breadcrumbs-el label="Users" icon="fas fa-user"/>
        <q-breadcrumbs-el label="Groups" icon="fas fa-users"/>
      </q-breadcrumbs>
    </div>
    <q-separator></q-separator>

    <q-page-sticky position="bottom-right" :offset="[50, 50]">
      <q-btn @click="newGroupDrawer = true" label="Add Group" fab icon="add" color="primary"/>
    </q-page-sticky>

    <div class="row">
      <div class="col-12">
        <q-markup-table>
          <tbody>
          <tr style="cursor: pointer" @click="$router.push('/groups/' + group.reference_id)" v-for="group in groups">
            <td>{{group.name}}</td>
          </tr>
          </tbody>
        </q-markup-table>

      </div>
    </div>

    <q-drawer :width="500" content-class="bg-grey-3" side="right" v-model="newGroupDrawer">
      <q-scroll-area class="fit row">
        <div class="q-pa-md">
          <span class="text-h6">Create group</span>
          <q-form class="q-gutter-md">
            <q-input label="Name" v-model="group.name"></q-input>
            <q-btn color="primary" @click="createGroup()">Create</q-btn>
            <q-btn @click="newGroupDrawer = false">Cancel</q-btn>
          </q-form>
        </div>
      </q-scroll-area>
    </q-drawer>

    <q-page-sticky v-if="!showHelp" position="top-right" :offset="[0, 0]">
      <q-btn flat @click="showHelp = true" fab icon="fas fa-question"/>
    </q-page-sticky>

    <q-drawer overlay :width="400" side="right" v-model="showHelp">
      <q-scroll-area class="fit">
        <help-page @closeHelp="showHelp = false">
          <template v-slot:help-content>
            <q-markdown src="::: tip
You can create different user groups here. Different user groups can have different permissions.
E.g. Admin Group that has permissions to create, read, write and delete tables.
:::"></q-markdown>
          </template>
        </help-page>
      </q-scroll-area>
    </q-drawer>


  </div>
</template>

<script>
  import {mapActions, mapGetters, mapState} from 'vuex';

  export default {
    name: 'UserGroupsPage',
    methods: {
      editGroup(evt, group) {
        console.log("Edit group", group)
      },
      createGroup() {
        const that = this;
        console.log("new group", this.group);
        this.group.tableName = "usergroup";
        that.createRow(that.group).then(function (res) {
          that.user = {};
          that.$q.notify({
            message: "Group created"
          });
          that.refresh();
          that.newGroupDrawer = false;
        }).catch(function (e) {
          if (e instanceof Array) {
            that.$q.notify({
              message: e[0].title
            })
          } else {
            that.$q.notify({
              message: "Failed to create group"
            })
          }
        });
      },
      ...mapActions(['loadData', 'getTableSchema', 'createRow']),
      refresh() {
        var tableName = "usergroup";
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
          that.groups = data.data;
        })
      }
    },
    data() {
      return {
        text: '',
        showHelp: false,
        group: {},
        filter: null,
        newGroupDrawer: false,
        groups: [],
        columns: [
          {
            name: 'name',
            field: 'name',
            label: 'Group name',
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
