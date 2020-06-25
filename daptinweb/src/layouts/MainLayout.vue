<template>
  <q-layout view="lHh Lpr lFf">

    <q-drawer
      show-if-above
      :width="250"
      @click.capture="drawerClick"
      :breakpoint="700"
      content-class="bg-primary text-white"
      elevated>
      <q-scroll-area class="fit">

        <q-icon name="fab fa-pied-piper-alt" size="30px" class="q-ma-md"></q-icon>

        <q-list>
          <q-expansion-item
            expand-separator
            label="Database"
            expand-icon-class="text-white"
            icon="fas fa-database">

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
            expand-icon-class="text-white"
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
        drawer: false,
        miniState: true,
      }
    },
    mounted() {
      const that = this;
      console.log("Mounted main layout");
      var decoded = jwt.decode(this.authToken());
      // console.log("Token decoded", this.authToken(), decoded);
      if (decoded.exp < new Date().getTime() / 1000) {
        // console.log("Token expired");
        this.$q.notify({
          message: "Token expired, please login again"
        });
        this.logout();
        return
      }
      that.loadModel("cloud_store").then(function () {
        that.getDefaultCloudStore();
      }).catch(function (err) {
        console.log("Failed to load model for cloud store", err);
        that.$q.notify({
          message: "Failed to load model for cloud store"
        })
      })
    },
    methods: {
      drawerClick(e) {
        // if in "mini" state and user
        // click on drawer, we switch it to "normal" mode
        if (this.miniState) {
          this.miniState = false;

          // notice we have registered an event with capture flag;
          // we need to stop further propagation as this click is
          // intended for switching drawer to "normal" mode only
          e.stopPropagation()
        }
      },
      ...mapActions(['getDefaultCloudStore', 'loadModel']),
      logout() {
        localStorage.removeItem("token");
        localStorage.removeItem("user ");
        this.$router.push("/login")
      }
    }
  }
</script>
