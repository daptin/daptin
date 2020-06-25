<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <q-page>
    <div class="q-pa-md q-gutter-sm">
      <q-breadcrumbs class="text-orange" active-color="secondary">
      <template v-slot:separator>
        <q-icon
          size="1.2em"
          name="arrow_forward"
          color="primary"
        />
      </template>

      <q-breadcrumbs-el label="Users" icon="fas fa-user" />
      <q-breadcrumbs-el label="Groups" icon="fas fa-users" />
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
      <q-btn @click="$router.push('/users/group')" label="Add Group" fab icon="add" color="primary"/>
    </q-page-sticky>

    <div class="row">
      <div class="col-8 q-pa-md q-gutter-sm">
      <q-table
        title="User groups"
        :data="groups"
        row-key="index"
        :rows-per-page-options="[50]"
        :columns="columns"
      >
      <template v-slot:top-right>
        <q-input borderless dense debounce="300" v-model="filter" placeholder="Search">
          <template v-slot:append>
            <q-icon name="search" />
          </template>
        </q-input>
      </template>
      </q-table>

      </div>
    </div>

  </q-page>
</template>

<script>
  import {mapActions, mapGetters, mapState} from 'vuex';

  export default {
    name: 'TablePage',
    methods: {
      ...mapActions(['loadData', 'getTableSchema']),
      refresh() {
        var tableName = "usergroup";
        const that = this;
        this.getTableSchema(tableName).then(function (res) {
          that.tableSchema = res;
          console.log("Schema", that.tableSchema)
        });

        this.loadData({tableName: tableName}).then(function (data) {
          console.log("Loaded data", data);
          that.groups = data.data;
        })
      }
    },
    data() {
      return {
        text: '',
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
