<template>
  <q-layout view="lHh Lpr lFf">

    <q-drawer
      show-if-above
      :width="250"
      :breakpoint="700"
      content-class="bg-cyan"
      elevated
    >
      <q-scroll-area class="fit">

        <q-icon name="fab fa-pied-piper-alt" size="50px" class="q-ma-md"></q-icon>

        <q-list>
          <q-expansion-item
            expand-separator
            icon="fas fa-database"
            label="Database">

            <q-list>
              <q-item :inset-level="1" clickable v-ripple @click="$router.push('/tables')">
                <q-item-section>
                  <q-item-label>
                    <q-icon name="fas fa-table"></q-icon>
                    Tables
                  </q-item-label>
                </q-item-section>
              </q-item>
              <q-item :inset-level="1" clickable v-ripple @click="$router.push('/user/permissions')">
                <q-item-section>
                  <q-item-label>
                    <q-icon name="fas fa-lock"></q-icon>
                    Permissions
                  </q-item-label>
                </q-item-section>
              </q-item>
            </q-list>

          </q-expansion-item>
          <q-expansion-item
            expand-separator
            icon="fas fa-user"
            label="Users">

            <q-list>

              <q-item :inset-level="1" clickable v-ripple @click="$router.push('/user/accounts')">
                <q-item-section>
                  <q-item-label>
                    <q-icon name="fas fa-address-book"></q-icon>
                    Accounts
                  </q-item-label>
                </q-item-section>
              </q-item>
              <q-item :inset-level="1" clickable v-ripple @click="$router.push('/user/groups')">
                <q-item-section>
                  <q-item-label>
                    <q-icon name="fas fa-users"></q-icon>
                    Groups
                  </q-item-label>
                </q-item-section>
              </q-item>

            </q-list>

          </q-expansion-item>

          <q-space/>
          <q-separator/>

          <q-item clickable @click="logout()">
            <q-item-section>
              <q-item-label>
                <q-icon name="fas fa-power-off"></q-icon>
                Logout
              </q-item-label>
            </q-item-section>
          </q-item>

        </q-list>

      </q-scroll-area>
    </q-drawer>


    <q-page-container v-if="loggedIn()">
      <router-view/>
    </q-page-container>
  </q-layout>
</template>

<script>
  import {mapGetters, mapActions} from 'vuex';

  var jwt = require('jsonwebtoken');

  export default {
    name: 'MainLayout',

    components: {},

    data() {
      return {
        ...mapGetters(['loggedIn', 'drawerLeft', 'authToken']),
        essentialLinks: [],
      }
    },
    mounted() {
      console.log("Mounted main layout");
      var decoded = jwt.decode(this.authToken());
      console.log("Token decoded", this.authToken(), decoded);
      if (decoded.exp < new Date().getTime() / 1000) {
        console.log("Token expired");
        this.$q.notify({
          message: "Token expired, please login again"
        });
        this.logout();
      }
    },
    methods: {
      ...mapActions(['load']),
      logout() {
        localStorage.removeItem("token");
        localStorage.removeItem("user ");
        this.$router.push("/login")
      }
    }
  }
</script>
