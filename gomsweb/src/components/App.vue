<template>
  <div id="app">
    <template v-if="loaded">
      <router-view></router-view>
    </template>
  </div>
</template>

<script>
  import {setToken, checkSecret, extractInfoFromHash} from '../utils/auth'
  import worldManager from "../plugins/worldmanager"
  export default {
    name: 'App',
    data () {
      return {
        section: 'Head',
        loaded: false,
      }
    },
    mounted: function () {
      var that = this;
      if (!this.$store.getters.isAuthenticated) {

        const {token, secret} = extractInfoFromHash();
        console.log("check token", token);
        if (token && checkSecret(secret)) {
          setToken(token);
          this.$router.go('/');
          window.location = "/";
          return;
        } else {
          console.log(" is not authenticated ");
          if (this.$route.path == "/auth/signin" || this.$route.path == "/auth/signed") {
          } else {
            this.$router.push({name: 'SignIn'});
          }
        }
        that.loaded = true;

      } else {
        console.log("begin load models")
        var promise = worldManager.loadModels();
        promise.then(function () {
          console.log("World loaded, start view");
          that.loaded = true;
        });

      }
    },
    methods: {
      logout () {
        this.$store.commit('SET_USER', null);
        this.$store.commit('SET_TOKEN', null);

        if (window.localStorage) {
          window.localStorage.setItem('user', null);
          window.localStorage.setItem('token', null)
        }

        this.$router.push("/auth/signin")
      }
    }
  }
</script>
