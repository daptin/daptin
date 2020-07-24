<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <q-page>
    <div class="row">
      <div class="col-12">
        <div class="q-pa-md">
          <span class="text-h4">Home</span>
        </div>
      </div>
      <div class="col-6">
        <q-card flat>
          <q-card-section>
            <table width="400">
              <tbody>
              <tr>
                <td class="text-h6">User Registration</td>
                <td class="float-right">
                  <label :label="signUpPublicAvailable ? 'Enabled': 'Disabled'"></label>
                </td>
              </tr>
              <tr>
                <td class="text-h6">Password Reset</td>
                <td class="float-right">
                  <label :label="resetPublicAvailable ? 'Enabled': 'Disabled'"></label>
                </td>
              </tr>
              </tbody>
            </table>
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
      ...mapActions(['loadData'])
    },

    data() {
      return {
        text: '',
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

      })
    }
  }
</script>
