<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <q-page>
    <div class="flex flex-center ">
      <div style="min-width: 30%">
        <div class="q-pa-md ">
          <h3>Login</h3>

          <q-form autofocus
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


            <div>
              <q-btn class="float-left" label="Login" type="submit" color="primary"/>
              <q-btn class="float-right" label="Register" @click="$router.push('/register')" type="reset"
                     color="secondary" flat/>
            </div>
          </q-form>

        </div>
      </div>
    </div>

  </q-page>
</template>

<script>
  import {mapActions} from 'vuex';

  export default {
    name: 'PageLogin',
    methods: {
      ...mapActions(['executeAction', 'setToken']),
      onSubmit() {
        const that = this;
        that.executeAction({
          tableName: 'user_account',
          actionName: 'signin',
          params: {
            email: this.email,
            password: this.password,
          }
        }).then(function (e) {
          for (var i = 0; i < e.length; i++) {
            if (e[i].ResponseType === "client.notify") {
              that.$q.notify(e[i].Attributes);
            }
          }
          that.setToken();
          that.$router.push("/apps/files");
        }).catch(function (e) {
          that.$q.notify("Failed to sign in");
          console.log("error ", arguments)
        })
      },
    },
    data() {
      return {
        email: null,
        password: null,
      }
    },
    mounted() {
      console.log("mounted login")
    }
  }
</script>
