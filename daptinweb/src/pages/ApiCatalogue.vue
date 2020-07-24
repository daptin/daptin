<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <div>
    <div class="q-pa-md q-gutter-sm">
      <q-breadcrumbs   >
        <template v-slot:separator>
          <q-icon
            size="1.2em"
            name="arrow_forward"
          />
        </template>

        <q-breadcrumbs-el label="Integration" icon="fas fa-bolt"/>
        <q-breadcrumbs-el label="API Catalogue" icon="fas fa-plug"/>
      </q-breadcrumbs>
    </div>
    <q-separator></q-separator>

    <q-page-sticky position="bottom-right" :offset="[50, 50]">
      <q-btn @click="newUserDrawer = true" label="Add User" fab icon="add" color="primary"/>
    </q-page-sticky>
    <div class="row">
      <div class="col-8 q-pa-md q-gutter-sm">
        <span class="text-h4">API Catalogue</span>
      </div>
    </div>


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
