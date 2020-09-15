<template>
  <q-layout class="user-area-pattern" view="lHh Lpr lFf">

    <!--    <q-drawer :mini-to-overlay="true"-->
    <!--              content-style="color: white"-->
    <!--              :mini="!showDrawerFull"-->
    <!--              @mouseover="showDrawerFull = true"-->
    <!--              @mouseout="showDrawerFull = false"-->
    <!--              content-class="user-area-pattern"-->
    <!--              :width="250" :breakpoint="100"-->
    <!--              show-if-above side="left"-->
    <!--              v-if="isUser && userDrawer">-->


    <!--      <q-scroll-area class="fit">-->

    <!--        <q-list>-->
    <!--          <q-item clickable @click="$router.push('/apps')">-->
    <!--            <q-item-section avatar>-->
    <!--              <q-icon name="fas fa-home"></q-icon>-->
    <!--            </q-item-section>-->
    <!--            <q-item-section>-->
    <!--              Home-->
    <!--            </q-item-section>-->
    <!--          </q-item>-->
    <!--          <q-item clickable @click="$router.push('/apps/files')">-->
    <!--            <q-item-section avatar>-->
    <!--              <q-icon name="fas fa-cloud"></q-icon>-->
    <!--            </q-item-section>-->
    <!--            <q-item-section>-->
    <!--              Files-->
    <!--            </q-item-section>-->
    <!--          </q-item>-->

    <!--          <q-item disable clickable @click="$router.push('/apps/email')">-->
    <!--            <q-item-section avatar>-->
    <!--              <q-icon name="fas fa-envelope"></q-icon>-->
    <!--            </q-item-section>-->
    <!--            <q-item-section>-->
    <!--              Email-->
    <!--            </q-item-section>-->
    <!--          </q-item>-->

    <!--          <q-item disable clickable @click="$router.push('/apps/contacts')">-->
    <!--            <q-item-section avatar>-->
    <!--              <q-icon name="fas fa-users"></q-icon>-->
    <!--            </q-item-section>-->
    <!--            <q-item-section>-->
    <!--              Contacts-->
    <!--            </q-item-section>-->
    <!--          </q-item>-->

    <!--          <q-item clickable @click="$router.push('/apps/calendar')">-->
    <!--            <q-item-section avatar>-->
    <!--              <q-icon name="fas fa-calendar"></q-icon>-->
    <!--            </q-item-section>-->
    <!--            <q-item-section>-->
    <!--              Calendar-->
    <!--            </q-item-section>-->
    <!--          </q-item>-->

    <!--          <q-item  clickable v-ripple @click="logout">-->
    <!--            <q-item-section class="text-negative" avatar>-->
    <!--              <q-icon name="fas fa-power-off"></q-icon>-->
    <!--            </q-item-section>-->
    <!--            <q-item-section>-->
    <!--              <q-item-label>-->
    <!--                Logout-->
    <!--              </q-item-label>-->
    <!--            </q-item-section>-->
    <!--          </q-item>-->


    <!--        </q-list>-->


    <!--      </q-scroll-area>-->
    <!--    </q-drawer>-->

    <router-view v-if="loaded"/>

  </q-layout>
</template>
<style>
.user-background-pattern {
  background: linear-gradient(
    limegreen,
    transparent
  ),
  linear-gradient(
    90deg,
    skyblue,
    transparent
  ),
  linear-gradient(
    -90deg,
    coral,
    transparent
  );

  background-blend-mode: screen;
}

/*.user-area-pattern {*/
/*  background: url("data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHhtbG5zOnhsaW5rPSJodHRwOi8vd3d3LnczLm9yZy8xOTk5L3hsaW5rIiB3aWR0aD0iNTAwIiBoZWlnaHQ9IjUwMCI+CjxmaWx0ZXIgaWQ9Im4iPgo8ZmVUdXJidWxlbmNlIHR5cGU9ImZyYWN0YWxOb2lzZSIgYmFzZUZyZXF1ZW5jeT0iLjciIG51bU9jdGF2ZXM9IjEwIiBzdGl0Y2hUaWxlcz0ic3RpdGNoIj48L2ZlVHVyYnVsZW5jZT4KPC9maWx0ZXI+CjxyZWN0IHdpZHRoPSI1MDAiIGhlaWdodD0iNTAwIiBmaWxsPSIjMDAwIj48L3JlY3Q+CjxyZWN0IHdpZHRoPSI1MDAiIGhlaWdodD0iNTAwIiBmaWx0ZXI9InVybCgjbikiIG9wYWNpdHk9IjAuNCI+PC9yZWN0Pgo8L3N2Zz4=");*/
/*}*/

.user-area-pattern {
  background-color: #e7e8e9;
  /*background-image: linear-gradient(rgba(0, 0, 0, .5) 2px, transparent 2px),*/
  /*linear-gradient(90deg, rgba(0, 0, 0, .5) 2px, transparent 2px);*/
  /*linear-gradient(rgba(255, 255, 255, .28) 1px, transparent 1px),*/
  /*linear-gradient(90deg, rgba(255, 255, 255, .28) 1px, transparent 1px);*/
  /*background-size: 100px 100px, 100px 100px, 20px 20px, 20px 20px;*/
}
</style>
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
      showDrawerFull: false,
      showAdminDrawerMini: false,
      showAdminDrawerStick: false,
      ...mapGetters(['loggedIn', 'drawerLeft', 'authToken', 'decodedAuthToken']),
      essentialLinks: [],
      drawer: false,
      userDrawer: true,
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

    that.loadModel(["cloud_store", "user_account", "usergroup", "world", "action", 'site', 'integration', 'event', 'document']).then(async function () {
      that.loaded = true;
      that.getDefaultCloudStore();
      that.loadData({
        tableName: "user_account",
      }).then(function (res) {
        const users = res.data;
        console.log("Users: ", users);
        that.isUser = true;
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
