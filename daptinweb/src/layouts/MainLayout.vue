<template>
  <q-layout view="lHh Lpr lFf">

    <q-drawer
      show-if-above
      :width="250"
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
            </q-list>

          </q-expansion-item>
          <q-expansion-item
            expand-icon-class="text-white"
            expand-separator
            icon="fas fa-user"
            label="Users">

            <q-list>

              <q-item :inset-level="1" clickable v-ripple @click="$router.push('/users')">
                <q-item-section>
                  <q-item-label>
                    <q-icon name="fas fa-address-book"></q-icon>
                    Accounts
                  </q-item-label>
                </q-item-section>
              </q-item>
              <q-item :inset-level="1" clickable v-ripple @click="$router.push('/groups')">
                <q-item-section>
                  <q-item-label>
                    <q-icon name="fas fa-users"></q-icon>
                    Groups
                  </q-item-label>
                </q-item-section>
              </q-item>

            </q-list>

          </q-expansion-item>


        </q-list>
        <q-list>
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


    <q-page-container v-if="loggedIn() && loaded">
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
        loaded: false,
        miniState: true,
      }
    },
    mounted() {
      const that = this;
      console.log("Mounted main layout");

      that.loadModel(["cloud_store", "user_account", "usergroup", "world"]).then(function () {
        that.loaded = true;
        that.getDefaultCloudStore();
        var decoded = jwt.decode(that.authToken());
        console.log("Token decoded", that.authToken(), decoded);
        if (!decoded || decoded.exp < new Date().getTime() / 1000) {
          // console.log("Token expired");
          that.$q.notify({
            message: "Token expired, please login again"
          });
          that.logout();
          return
        }

        that.executeAction({
          tableName: 'world',
          actionName: "become_an_administrator"
        }).then(function (res) {
          that.$q.notify({
            message: "You have become the administrator of this instance"
          })
        }).catch(function (err) {
          console.log("Failed to become admin", err);
        })
      }).catch(function (err) {
        console.log("Failed to load model for cloud store", err);
        that.$q.notify({
          message: "Failed to load model for cloud store"
        })
      })
    },
    methods: {
      ...mapActions(['getDefaultCloudStore', 'loadModel', 'executeAction']),
      logout() {
        localStorage.removeItem("token");
        localStorage.removeItem("user ");
        this.$router.push("/login")
      }
    }
  }
</script>
