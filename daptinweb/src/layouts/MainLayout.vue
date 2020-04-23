<template>
  <q-layout view="lHh Lpr lFf">
    <q-header class="row" elevated>

      <q-toolbar class="col-2">
        <q-btn flat @click="flipDrawerLeft()" round dense icon="menu"/>
        <q-toolbar-title>
          <q-btn label="DadaDash" flat @click="$router.push('/')"></q-btn>
        </q-toolbar-title>

      </q-toolbar>
      <q-toolbar class="col-10">
        <q-separator dark vertical inset/>
        <q-btn flat @click="$router.push('/tables')" label="Tables"/>
        <q-btn flat @click="$router.push('/data')" label="Data"/>
        <q-space/>
        <q-btn class="bg-warning" icon="power" @click="logout()"></q-btn>
      </q-toolbar>
    </q-header>

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
      flipDrawerLeft() {
        console.log("Flip drawer left", this.drawerLeft())
        if (this.drawerLeft()) {
          this.hideDrawerLeft()
        } else {
          this.showDrawerLeft()
        }
      },
      ...mapActions(['load', 'showDrawerLeft', 'hideDrawerLeft']),
      logout() {
        localStorage.removeItem("token");
        localStorage.removeItem("user ");
        this.$router.push("/login")
      }
    }
  }
</script>
