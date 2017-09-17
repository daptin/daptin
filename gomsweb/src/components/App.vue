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
    data() {
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
            this.$store.commit('SET_LAST_URL', this.$route);
            this.$router.push({name: 'SignIn'});
          }
        }
        that.loaded = true;

      } else {
        var that = this;
        console.log("begin load models")
        var promise = worldManager.loadModels();
        promise.then(function () {
          console.log("World loaded, start view");


          if (window.localStorage) {
            var lastRoute = window.localStorage.getItem("last_route");
            if (lastRoute) {
              that.$store.commit('SET_LAST_URL', null);
              console.log("last route is present");
              that.$router.push(JSON.parse(lastRoute));
            } else {
              console.log("no last route present")
            }
          }


          that.loaded = true;
        });

      }
    },
    methods: {
      logout() {
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
