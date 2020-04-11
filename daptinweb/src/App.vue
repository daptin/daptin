<template>
  <div id="q-app">
    <router-view/>

  </div>
</template>

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
      ...mapActions(['load'])
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
