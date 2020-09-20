<template>
  <q-layout view="lHh Lpr lFf">

    <q-drawer
      v-if="isAdmin"
      v-model="showAdminDrawer"
      :mini-to-overlay="!showAdminDrawerStick"
      :mini="!showAdminDrawerStick && showAdminDrawerMini"
      @mouseover="showAdminDrawerMini = false"
      @mouseout="showAdminDrawerMini = true"

      :mini-width="50"
      :width="250"
      :breakpoint="1400"
      content-class=""
      elevated>
      <q-scroll-area class="fit">

        <q-list class="bg-black">
          <q-item clickable>
            <q-item-section @click="$router.push('/')" style="text-transform: capitalize;
font-weight: bold;
font-size: 22px;
text-align: center;
" class="text-white">
              DASHBOARD
            </q-item-section>
            <q-item-section avatar>
              <q-btn flat size="xs" @click="showAdminDrawerStick = !showAdminDrawerStick" color="white" icon="menu"/>
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

          <q-item clickable
            @click="$router.push('/apps/files')"
          >
            <q-item-section avatar>
              <q-icon name="fas fa-briefcase"></q-icon>
            </q-item-section>
            <q-item-section>
              Apps
            </q-item-section>

          </q-item>


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

              <q-item :inset-level="1" clickable v-ripple @click="$router.push('/cloudstore/sites')">
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
            icon="fas fa-book"
            label="Documentation">

            <q-list>


              <q-item :inset-level="1" clickable v-ripple @click="$router.push('/tables/apidocs')">
                <q-item-section avatar>
                  <q-icon name="fas fa-book"></q-icon>
                </q-item-section>
                <q-item-section>
                  <q-item-label>
                    JSON API docs
                  </q-item-label>
                </q-item-section>
              </q-item>


              <q-item :inset-level="1" clickable v-ripple @click="$router.push('/tables/graphql')">
                <q-item-section avatar>
                  <q-icon name="fas fa-book"></q-icon>
                </q-item-section>
                <q-item-section>
                  <q-item-label>
                    GraphQL docs
                  </q-item-label>
                </q-item-section>
              </q-item>


            </q-list>

          </q-expansion-item>

        </q-list>


        <q-list>
          <q-item clickable @click="logout()">
            <q-item-section avatar>
              <q-icon color="negative" name="fas fa-power-off"></q-icon>

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

      <q-page-sticky v-if="isAdmin || isUser && (!showHelp)" position="top-right" :offset="[0, 0]">
        <q-btn flat @click="showHelp = true" fab icon="fas fa-question"/>
      </q-page-sticky>

      <q-drawer :width="fileDrawerWidth > 800 ? 800 : fileDrawerWidth" overlay :breakpoint="400" side="right"
                v-model="showHelp">
        <q-scroll-area class="fit" v-if="showHelp">
          <help-page @closeHelp="showHelp = false">
          </help-page>
        </q-scroll-area>
      </q-drawer>

    </q-page-container>

  </q-layout>
</template>

<script>
import {mapGetters, mapActions} from 'vuex';

var jwt = require('jsonwebtoken');

export default {
  name: 'MainLayout',

  computed: {
    fileDrawerWidth() {
      return window.screen.availWidth;
    },
  },
  components: {},

  data() {
    return {
      showHelp: false,
      showAdminDrawer: false,
      showAdminDrawerMini: true,
      showAdminDrawerStick: false,
      ...mapGetters(['loggedIn', 'drawerLeft', 'authToken', 'decodedAuthToken']),
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
    if (that.decodedAuthToken()) {
      let decodedAuthToken = that.decodedAuthToken();
      let isLoggedOut = decodedAuthToken.exp * 1000 < new Date().getTime();
      console.log("Decoded auth token", isLoggedOut, decodedAuthToken);
      if (isLoggedOut) {
        that.$q.notify({
          message: "Authentication has expired, please login again"
        });
        that.setDecodedAuthToken(null);
        that.logout();
      }
    }

    that.loadModel(["cloud_store", "user_account", "usergroup", "world", "action", 'site', 'integration']).then(async function () {
      that.loaded = true;
      that.getDefaultCloudStore();

      that.loadData({
        tableName: "user_account",
      }).then(function (res) {
        const users = res.data;
        console.log("Users: ", users);

        if (users.length == 2) {
          that.isAdmin = true;
          that.showAdminDrawer = true;
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
          that.showAdminDrawer = true;
          that.isUser = false;
        } else {
          that.isUser = true;
          that.$router.push('/apps/files')
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
    ...mapActions(['getDefaultCloudStore', 'loadModel', 'executeAction', 'loadData', 'setDecodedAuthToken']),
    logout() {
      localStorage.removeItem("token");
      localStorage.removeItem("user");
      this.$router.push("/login");
      window.location = window.location;
    }
  }
}
</script>
