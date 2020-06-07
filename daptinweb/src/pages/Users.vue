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
      <q-breadcrumbs-el label="Accounts" icon="fas fa-address-book" />
    </q-breadcrumbs>
    </div>

    <q-markdown class= "q-pa-md" src="::: tip
You can add users to your instance here. You can also send the sign up link where users can signup themselves. You can also bulk upload users from an excel sheet.
You need following fields as headers: email, name, and password 
:::"></q-markdown>

    <q-page-sticky position="bottom-right" :offset="[50, 50]">
      <q-btn @click="$router.push('/users/create')" label="Add User" fab icon="add" color="primary"/>
    </q-page-sticky>
    <div class="row">
      <div class="col-8 q-pa-md q-gutter-sm">
      <q-table
        search
        title="User Accounts"
        :data="users"
        :rows-per-page-options="[50]"
        :columns="columns"
        :filter="filter"
        row-key="index"
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
      ...mapActions(['load', 'loadData', 'getTableSchema']),
      refresh() {
        var tableName = "user_account";
        const that = this;
        this.getTableSchema(tableName).then(function (res) {
          that.tableSchema = res;
          console.log("Schema", that.tableSchema)
        });

        this.loadData({tableName: tableName}).then(function (data) {
          console.log("Loaded data", data);
          that.users = data.data;
        })
      }
    },
    data() {
      return {
        text: '',
        users: [],
        columns: [
          {
            name: 'email',
            field: 'email',
            label: 'Email',
            align: 'left',
            sortable: true,
          }, {
            name: 'name',
            field: 'name',
            label: 'Name',
            align: 'left',
          },
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
