<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <div>
    <div class="q-pa-md q-gutter-sm">
      <q-breadcrumbs class="text-orange" active-color="secondary">
        <template v-slot:separator>
          <q-icon
            size="1.2em"
            name="arrow_forward"
            color="primary"
          />
        </template>

        <q-breadcrumbs-el label="Users" icon="fas fa-user"/>
        <q-breadcrumbs-el label="Groups" icon="fas fa-users"/>
      </q-breadcrumbs>
    </div>
    <q-separator></q-separator>

    <div class="row">
      <div class="col-8 q-pa-md q-gutter-sm">
        <q-markdown src="::: tip
You can create different user groups here. Different user groups can have different permissions.
E.g. Admin Group that has permissions to create, read, write and delete tables.
:::"></q-markdown>
      </div>
    </div>

    <q-page-sticky position="bottom-right" :offset="[50, 50]">
      <q-btn @click="newGroupDrawer = true" label="Add Group" fab icon="add" color="primary"/>
    </q-page-sticky>

    <div class="row">
      <div class="col-8 q-pa-md q-gutter-sm">
        <q-table
          title="User groups"
          :data="groups"
          row-key="index"
          @row-click="editGroup"
          :rows-per-page-options="[50]"
          :columns="columns"
        >
          <template v-slot:top-right>
            <q-input borderless dense debounce="300" v-model="filter" placeholder="Search">
              <template v-slot:append>
                <q-icon name="search"/>
              </template>
            </q-input>
          </template>
        </q-table>

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

  </div>
</template>

<script>
  import {mapActions, mapGetters, mapState} from 'vuex';

  export default {
    name: 'TablePage',
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
        this.loadData({tableName: tableName}).then(function (data) {
          console.log("Loaded data", data);
          that.groups = data.data;
        })
      }
    },
    data() {
      return {
        text: '',
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
