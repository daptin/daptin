<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <q-page>
    <div class="flex flex-center">
      <div style="min-width: 30%">
        <div class="q-pa-md">
          <h3>Register</h3>

          <q-form
            @submit="onSubmit"
            class="q-gutter-md"
          >
            <q-input
              filled
              v-model="email"
              label="Email"
              lazy-rules
              :rules="[ val => val && val.length > 0 || 'Please type something']"
            />

            <q-input
              filled
              type="password"
              v-model="password"
              label="Password"
              lazy-rules
            />
            <q-input
              filled
              type="password"
              v-model="passwordConfirm"
              label="Confirm Password"
              lazy-rules
            />


            <div>
              <q-btn class="float-left" label="Register" type="submit" color="primary"/>
              <q-btn class="float-right" label="Login" @click="$router.push('/login')" type="reset" color="secondary"
                     flat/>
            </div>
          </q-form>

        </div>
      </div>
      <div class="col-10">

      </div>
    </div>

  </q-page>
</template>

<script>
  import {mapActions} from 'vuex';

  export default {
    name: 'PageSignup',
    methods: {
      ...mapActions(['executeAction']),
      onSubmit() {
        var that = this;
        that.executeAction({
          tableName: 'user_account',
          actionName: 'signup',
          params: {
            email: this.email,
            password: this.password,
            name: this.email,
            passwordConfirm: this.passwordConfirm,
          }
        }).then(function (responses) {
          for (var i = 0; i < responses.length; i++) {
            var response = responses[i];
            if (response.ResponseType == "client.notify") {
              if (response.Attributes.type == "success") {
                that.$q.notify(response.Attributes);
                break;
              } else {
                that.$q.notify(response.Attributes);
                return;
              }
            }
          }

          that.executeAction({
            tableName: 'user_account',
            actionName: 'signin',
            params: {
              email: that.email,
              password: that.password,
            }
          }).then(function (e) {
            console.log("Sign in successful", arguments);
            that.$router.push('/tables')
          }).catch(function (e) {
            console.log("Failed to sign in", arguments);
            that.$q.notify("Error", "Failed to login");
            that.$router.push('/login');
          })
        }).catch(function (responses) {
          that.$q.notify("Error", "Failed to signup");
          console.log("Failed to register", responses)
        })
      },
    },
    data() {
      return {
        email: null,
        password: null,
        passwordConfirm: null,
      }
    },
    mounted() {
    }
  }
</script>
