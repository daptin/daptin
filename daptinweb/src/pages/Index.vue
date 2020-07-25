<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <q-page>
    <div class="row">


      <div class="col-4 col-md-6 col-lg-4 col-xl-3 col-xs-12 col-sm-12 q-pa-md q-gutter-sm">
        <q-card>
          <q-card-section>
            <span class="text-h4">Users</span>
          </q-card-section>
          <q-card-section>
            <div class="row q-pa-md">
              <div class="col-4">
                <span class="text-bold">Total</span>
              </div>
              <div class="col-6 text-right">
                {{userAggregate.count}}
              </div>
            </div>
            <div class="row q-pa-md">
              <div class="col-4">
                <span class="text-bold">User registrations</span>
              </div>
              <div class="col-6 text-right">
                <q-btn-toggle size="sm" rounded color="white" toggle-color="primary" toggle-text-color="white"
                              text-color="black"
                              :options="[
          {label: 'Enabled', value: true},
          {label: 'Disabled', value: false},
        ]" v-model="signUpPublicAvailable"></q-btn-toggle>
              </div>
            </div>
            <div class="row q-pa-md">
              <div class="col-4">
                <span class="text-bold">Password Reset</span>
              </div>
              <div class="col-6 text-right">
                <q-btn-toggle size="sm" rounded color="white" toggle-color="primary" toggle-text-color="white"
                              text-color="black"
                              :options="[
          {label: 'Enabled', value: true},
          {label: 'Disabled', value: false},
        ]" v-model="resetPublicAvailable"></q-btn-toggle>
              </div>
            </div>
          </q-card-section>

        </q-card>
      </div>


      <div class="col-4  col-md-6 col-lg-4 col-xl-3 col-xs-12 col-sm-12 q-pa-md q-gutter-sm">
        <q-card>
          <q-card-section>
            <span class="text-h4">Data tables</span>
          </q-card-section>
          <q-card-section>
            <div class="row q-pa-md">
              <div class="col-4">
                <span class="text-bold">Total</span>
              </div>
              <div class="col-6 text-right">
                {{tables().length}}
              </div>
            </div>

          </q-card-section>
          <q-card-section>
            <div class="row ">
              <div class="col-12 q-pa-md q-gutter-sm">
                <q-btn class="float-right" @click="$router.push('/tables')" icon="list" round></q-btn>
                <q-btn class="float-right" @click="$router.push('/tables/create')" round icon="add"></q-btn>
              </div>
            </div>
          </q-card-section>

        </q-card>
      </div>

      <div class="col-4 col-md-6 col-lg-4 col-xl-3 col-xs-12 col-sm-12 q-pa-md q-gutter-sm">
        <q-card>
          <q-card-section>
            <span class="text-h4">Storage</span>
          </q-card-section>
          <q-card-section>
            <div class="row q-pa-md">
              <div class="col-4">
                <span class="text-bold">Cloud stores</span>
              </div>
              <div class="col-6 text-right">
                {{cloudStoreAggregate.count}}
              </div>
            </div>
          </q-card-section>
          <q-card-section>
            <div class="row ">
              <div class="col-12 q-pa-md q-gutter-sm">
                <q-btn class="float-right" @click="$router.push('/cloudstore')" icon="list" round></q-btn>
                <q-btn class="float-right" @click="$router.push('/cloudstore?create=true')" round icon="add"></q-btn>
              </div>
            </div>
          </q-card-section>

        </q-card>
      </div>

      <div class="col-4 col-md-6 col-lg-4 col-xl-3 col-xs-12 col-sm-12 q-pa-md q-gutter-sm">
        <q-card>
          <q-card-section>
            <span class="text-h4">Sites</span>
          </q-card-section>
          <q-card-section>
            <div class="row q-pa-md">
              <div class="col-4">
                <span class="text-bold">Active</span>
              </div>
              <div class="col-6 text-right">
                {{siteAggregate.active}}
              </div>
            </div>
            <div class="row q-pa-md">
              <div class="col-4">
                <span class="text-bold">Total</span>
              </div>
              <div class="col-6 text-right">
                {{siteAggregate.total}}
              </div>
            </div>
          </q-card-section>

        </q-card>
      </div>

      <div class="col-4  col-md-6 col-lg-4 col-xl-3 col-xs-12 col-sm-12 q-pa-md q-gutter-sm">
        <q-card>
          <q-card-section>
            <span class="text-h4">Integrations</span>
          </q-card-section>
          <q-card-section>
            <div class="row q-pa-md">
              <div class="col-4">
                <span class="text-bold">API Specs</span>
              </div>
              <div class="col-6 text-right">
                {{integrationAggregate.count}}
              </div>
            </div>
            <div class="row q-pa-md">
              <div class="col-4">
                <span class="text-bold">Actions</span>
              </div>
              <div class="col-6 text-right">
                {{actionAggregate.count}}
              </div>
            </div>
          </q-card-section>

        </q-card>
      </div>


    </div>

  </q-page>
</template>

<script>
  import {mapActions, mapGetters} from 'vuex';

  export default {
    name: 'PageIndex',
    methods: {
      ...mapActions(['loadData', 'loadAggregates'])
    },

    data() {
      return {
        text: '',
        userAggregate: {},
        cloudStoreAggregate: {},
        siteAggregate: {},
        integrationAggregate: {},
        actionAggregate: {},
        signUpPublicAvailable: false,
        resetPublicAvailable: false,
        ...mapGetters(['tables'])
      }
    },
    mounted() {
      const that = this;
      that.loadData({
        tableName: 'action',
        params: {
          page: {
            size: 500
          }
        }
      }).then(function (res) {
        console.log("Actions", res);
        var data = res.data;
        var signUpAction = data.filter(function (e) {
          return e.action_name === 'signup'
        })[0];
        console.log("Sign up action", signUpAction);
        if (signUpAction && signUpAction.permission && 1) {
          that.signUpPublicAvailable = true;
        }
        var resetAction = data.filter(function (e) {
          return e.action_name === 'resetpassword'
        })[0];
        console.log("Reset action", resetAction);
        if (resetAction && resetAction.permission && 1) {
          that.resetPublicAvailable = true;
        }

      }).catch(function (res) {
        console.log("Failed to load actions", res);
      });


      that.loadAggregates({
        tableName: 'user_account',
        column: 'count'
      }).then(function (res) {
        console.log("User account aggregates", res);
        that.userAggregate = res.data[0];
      });


      that.loadAggregates({
        tableName: 'cloud_store',
        column: 'count'
      }).then(function (res) {
        console.log("cloud store aggregates", res);
        that.cloudStoreAggregate = res.data[0];
      });


      that.loadAggregates({
        tableName: 'site',
        column: 'count',
        group: 'enable'
      }).then(function (res) {
        console.log("Site aggregates", res);
        that.siteAggregate = {
          active: 0,
          total: 0,
        };
      });
      that.loadAggregates({
        tableName: 'action',
        column: 'count',
      }).then(function (res) {
        console.log("Site aggregates", res);
        that.actionAggregate = res.data[0];
      });
      that.loadAggregates({
        tableName: 'integration',
        column: 'count',
      }).then(function (res) {
        console.log("Site aggregates", res);
        that.integrationAggregate = res.data[0];
      });


    }
  }
</script>
