<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <div>
    <div class="q-pa-md q-gutter-sm">
      <q-breadcrumbs  >
        <template v-slot:separator>
          <q-icon
            size="1.2em"
            name="arrow_forward"
          />
        </template>

        <q-breadcrumbs-el label="User" icon="fas fa-user"/>
        <q-breadcrumbs-el label="Profile" icon="fas fa-id-card"/>
      </q-breadcrumbs>
    </div>
    <q-separator></q-separator>


    <div class="row q-pa-md q-gutter-sm" v-if="user">
      <div class="col-8">
        <span class="text-h5">{{user.email}}</span>
      </div>
      <div class="col-12">
        <q-btn label="Reset password"></q-btn>
      </div>
    </div>

  </div>
</template>

<script>
  import {mapActions, mapGetters, mapState} from 'vuex';

  export default {
    name: 'UserProfile',
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
      ...mapActions(['loadData', 'getTableSchema', 'createRow', 'loadOneData']),
      refresh() {
        const that = this;
        var tableName = "user_account";
        this.loadOneData({
          tableName: tableName,
          referenceId: 'mine'
        }).then(function (data) {
          console.log("Loaded data", data);
          that.user = data.data;
        })
      },
    },
    data() {
      return {
        text: '',
        user: null,
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
