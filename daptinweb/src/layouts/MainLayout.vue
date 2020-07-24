<template>
  <q-layout view="lHh Lpr lFf">

    <q-drawer
      v-if="isAdmin"
      show-if-above
      :width="250"
      :breakpoint="1400"
      content-class=""
      elevated>
      <q-scroll-area class="fit">

        <q-list class="bg-black">
          <q-item clickable @click="$router.push('/')">
            <q-item-section style="text-transform: capitalize;
font-weight: bold;
font-size: 22px;
text-align: center;
" class="text-white">
              DASHBOARD
            </q-item-section>
          </q-item>
        </q-list>

        <q-list>
          <q-expansion-item
            expand-separator
            label="Database"
            :value="true"
            expand-icon-class="text-white"
            icon="fas fa-database">

            <q-list>
              <q-item :inset-level="1" clickable v-ripple @click="$router.push('/tables')">
                <q-item-section avatar>
                  <q-icon name="fas fa-table"></q-icon>
                </q-item-section>
                <q-item-section>
                  <q-item-label>
                    Tables
                  </q-item-label>
                </q-item-section>
              </q-item>
            </q-list>

          </q-expansion-item>
          <q-expansion-item
            expand-icon-class="text-white"
            :value="true"
            expand-separator
            icon="fas fa-user"
            label="Users">

            <q-list>

              <q-item :inset-level="1" clickable v-ripple @click="$router.push('/user/profile')">
                <q-item-section avatar>
                  <q-icon name="fas fa-id-card"></q-icon>

                </q-item-section>
                <q-item-section>
                  <q-item-label>
                    Profile
                  </q-item-label>
                </q-item-section>
              </q-item>

              <q-item :inset-level="1" clickable v-ripple @click="$router.push('/users')">
                <q-item-section avatar>
                  <q-icon name="fas fa-address-book"></q-icon>

                </q-item-section>
                <q-item-section>
                  <q-item-label>
                    Accounts
                  </q-item-label>
                </q-item-section>
              </q-item>

              <q-item :inset-level="1" clickable v-ripple @click="$router.push('/groups')">
                <q-item-section avatar>
                  <q-icon name="fas fa-users"></q-icon>

                </q-item-section>
                <q-item-section>
                  <q-item-label>
                    Groups
                  </q-item-label>
                </q-item-section>
              </q-item>

            </q-list>

          </q-expansion-item>


          <q-expansion-item
            expand-icon-class="text-white"
            :value="true"
            expand-separator
            icon="fas fa-archive"
            label="Storage">

            <q-list>

              <q-item :inset-level="1" clickable v-ripple @click="$router.push('/cloudstore')">
                <q-item-section avatar>
                  <q-icon name="fas fa-bars"></q-icon>

                </q-item-section>
                <q-item-section>
                  <q-item-label>
                    Cloud stores
                  </q-item-label>
                </q-item-section>
              </q-item>

              <q-item :inset-level="1" clickable v-ripple @click="$router.push('/sites')">
                <q-item-section avatar>
                  <q-icon name="fas fa-desktop"></q-icon>

                </q-item-section>
                <q-item-section>
                  <q-item-label>
                    Sites
                  </q-item-label>
                </q-item-section>
              </q-item>

            </q-list>

          </q-expansion-item>



          <q-expansion-item
            expand-icon-class="text-white"
            :value="true"
            expand-separator
            icon="fas fa-bolt"
            label="Integrations">

            <q-list>

              <q-item :inset-level="1" clickable v-ripple @click="$router.push('/integrations/spec')">
                <q-item-section avatar>
                  <q-icon name="fas fa-plug"></q-icon>

                </q-item-section>
                <q-item-section>
                  <q-item-label>
                    API Catalogue
                  </q-item-label>
                </q-item-section>
              </q-item>
              <q-item :inset-level="1" clickable v-ripple @click="$router.push('/integrations/actions')">
                <q-item-section avatar>
                  <q-icon name="fas fa-wrench"></q-icon>

                </q-item-section>
                <q-item-section>
                  <q-item-label>
                    Actions
                  </q-item-label>
                </q-item-section>
              </q-item>

            </q-list>

          </q-expansion-item>

        </q-list>
        <q-list>
          <q-item clickable @click="logout()">
            <q-item-section avatar>
              <q-icon name="fas fa-power-off"></q-icon>

            </q-item-section>
            <q-item-section>
              <q-item-label>
                Logout
              </q-item-label>
            </q-item-section>
          </q-item>
        </q-list>

      </q-scroll-area>
    </q-drawer>


    <q-page-sticky v-if="isUser" position="bottom-right" :offset="[50, 50]">
      <q-btn @click="userDrawer = !userDrawer" fab icon="menu" color="primary"/>
    </q-page-sticky>

    <q-drawer content-class="bg-grey-3" overlay :width="300" :breakpoint="100" show-if-above side="right"
              v-if="isUser && userDrawer">
      <q-scroll-area class="fit">

        <q-list>
          <q-item :inset-level="1" clickable v-ripple @click="logout">
            <q-item-section avatar>
              <q-icon name="fas fa-lock"></q-icon>
            </q-item-section>
            <q-item-section>
              <q-item-label>
                Logout
              </q-item-label>
            </q-item-section>
          </q-item>
        </q-list>
        <q-list>
          <q-item>
            <q-item-section>
              <q-btn label="Close" @click="userDrawer = false"></q-btn>
            </q-item-section>
          </q-item>
        </q-list>

      </q-scroll-area>
    </q-drawer>


    <q-page-container>
      <router-view v-if="isAdmin || isUser"/>
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
        showHelp: false,
        ...mapGetters(['loggedIn', 'drawerLeft', 'authToken']),
        essentialLinks: [],
        drawer: false,
        userDrawer: false,
        loaded: false,
        miniState: true,
        isAdmin: false,
        isUser: false,
      }
    },
    mounted() {
      const that = this;
      console.log("Mounted main layout");

      that.loadModel(["cloud_store", "user_account", "usergroup", "world"]).then(async function () {
        that.loaded = true;
        that.getDefaultCloudStore();

        that.loadData({
          tableName: "user_account",
        }).then(function (res) {
          const users = res.data;
          console.log("Users: ", users);

          if (users.length == 2) {
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
          } else if (users.length > 2) {
            that.isAdmin = true;
            that.isUser = false;
          } else {
            that.isUser = true;
            that.$router.push('/user/profile')
          }
        });

      }).catch(function (err) {
        console.log("Failed to load model for cloud store", err);
        that.$q.notify({
          message: "Failed to load model for cloud store"
        })
      })

    },
    methods: {
      ...mapActions(['getDefaultCloudStore', 'loadModel', 'executeAction', 'loadData']),
      logout() {
        localStorage.removeItem("token");
        localStorage.removeItem("user ");
        this.$router.push("/login")
      }
    }
  }
</script>
