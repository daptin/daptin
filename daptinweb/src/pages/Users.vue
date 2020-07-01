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
        <q-breadcrumbs-el label="Accounts" icon="fas fa-address-book"/>
      </q-breadcrumbs>
    </div>
    <q-separator></q-separator>

    <div class="row">
      <div class="col-8 q-pa-md q-gutter-sm">
        <q-markdown src="::: tip
You can add users to your instance here. You can also send the sign up link where users can signup themselves.
:::"></q-markdown>
      </div>
    </div>

    <q-page-sticky position="bottom-right" :offset="[50, 50]">
      <q-btn @click="newUserDrawer = true" label="Add User" fab icon="add" color="primary"/>
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
                <q-icon name="search"/>
              </template>
            </q-input>
          </template>
        </q-table>

      </div>
    </div>

    <q-drawer content-class="bg-grey-3" :width="500" side="right" v-model="newUserDrawer">
      <q-scroll-area class="fit row">
        <div class="q-pa-md">
          <span class="text-h6">Create user</span>
          <q-form class="q-gutter-md">
            <q-input label="Name" v-model="user.name"></q-input>
            <q-input label="Email" v-model="user.email"></q-input>
            <q-input label="Password" type="password" v-model="user.password"></q-input>
            <q-btn color="primary" @click="createUser()">Create</q-btn>
            <q-btn @click="newUserDrawer = false">Cancel</q-btn>
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
      createUser() {
        const that = this;
        console.log("new user", this.user);
        this.user.tableName = "user_account";
        that.createRow(that.user).then(function (res) {
          that.user = {};
          that.$q.notify({
            message: "User created"
          });
          that.refresh();
          that.newUserDrawer = false;
        }).catch(function (e) {
          if (e instanceof Array) {
            that.$q.notify({
              message: e[0].title
            })
          } else {
            that.$q.notify({
              message: "Failed to create user"
            })
          }
        });
      },
      ...mapActions(['loadData', 'getTableSchema', 'createRow']),
      refresh() {
        const that = this;
        var tableName = "user_account";
        this.loadData({
          tableName: tableName, params: {
            page: {
              size: 500,
            }
          }
        }).then(function (data) {
          console.log("Loaded data", data);
          that.users = data.data;
        })
      },
    },
    data() {
      return {
        text: '',
        user: {},
        newUserDrawer: false,
        users: [],
        filter: null,
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
