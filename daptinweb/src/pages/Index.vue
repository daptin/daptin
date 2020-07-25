<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <q-page>
    <div class="row">
      <div class="col-4 q-pa-md q-gutter-sm">
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
                <q-btn :label="signUpPublicAvailable ? 'Enabled': 'Disabled'"></q-btn>
              </div>
            </div>
            <div class="row q-pa-md">
              <div class="col-4">
                <span class="text-bold">Password Reset</span>
              </div>
              <div class="col-6 text-right">
                <q-btn :label="resetPublicAvailable ? 'Enabled': 'Disabled'"></q-btn>
              </div>
            </div>
          </q-card-section>

        </q-card>
      </div>
    </div>

  </q-page>
</template>

<script>
  import {mapActions} from 'vuex';

  export default {
    name: 'PageIndex',
    methods: {
      ...mapActions(['loadData', 'loadAggregates'])
    },

    data() {
      return {
        text: '',
        userAggregate: {},
        signUpPublicAvailable: false,
        resetPublicAvailable: false,
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
      })
    }
  }
</script>
