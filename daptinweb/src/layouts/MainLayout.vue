<template>
  <q-layout view="lHh Lpr lFf">
    <!--    <q-header class="row" elevated>-->

    <!--      <q-toolbar class="col-2">-->
    <!--        <q-btn flat @click="flipDrawerLeft()" round dense icon="menu"/>-->
    <!--        <q-toolbar-title>-->
    <!--          <q-btn label="DadaDash" flat @click="$router.push('/')"></q-btn>-->
    <!--        </q-toolbar-title>-->

    <!--      </q-toolbar>-->
    <!--      <q-toolbar class="col-10">-->
    <!--        <q-separator dark vertical inset/>-->
    <!--        <q-btn flat @click="$router.push('/tables')" label="Tables"/>-->
    <!--        <q-btn flat @click="$router.push('/data')" label="Data"/>-->
    <!--        <q-space/>-->
    <!--        <q-btn class="bg-warning" icon="power" @click="logout()"></q-btn>-->
    <!--      </q-toolbar>-->
    <!--    </q-header>-->

    <q-drawer
      show-if-above
      :width="200"
      :breakpoint="700"
      elevated
    >
      <q-scroll-area class="fit">

        <q-list bordered class="rounded-borders">
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
                    <q-icon name="fas fa-list"></q-icon>
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

  export default {
    name: 'MainLayout',

    components: {},

    data() {
      return {
        ...mapGetters(['loggedIn', 'drawerLeft']),
        essentialLinks: [],
      }
    },
    mounted() {
      console.log("Mounted main layout")
      // this.load();
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
