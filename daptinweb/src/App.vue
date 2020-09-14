<template>
  <div id="q-app">
    <router-view/>

  </div>
</template>
<script>

  import {mapGetters, mapActions} from 'vuex';
  import "fontsource-source-sans-pro"

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
