<template>
  <div id="q-app">
    <router-view/>

  </div>
</template>
<style>
  @import '~tabulator-tables/dist/css/tabulator_simple.css';
</style>
<script>

  import {mapGetters, mapActions} from 'vuex';

  export default {
    data() {
      return {
        showLogin: false,
        ...mapGetters(['loggedIn'])
      }
    },
    methods: {
      ...mapActions([])
    },
    name: 'App',
    mounted() {
      console.log("Loaded app user is ", this.loggedIn());
      if (!this.loggedIn()) {
        this.$router.push("/login")
      }

    }
  }
</script>
