<template>
  <div>
    <div id="app" class="ui">


      <div class="ui two column grid inverted fixed menu navbar">

        <div class="ui column right floated" style="text-align: right">


          <div class="ui icon button">
            <i class="setting power" @click="login()"  v-show="!authenticated"> Login </i>
            <i class="setting power" @click="logout()" v-show="authenticated">Logout </i>
          </div>
        </div>


      </div>
      <nuxt v-if="loaded"/>
    </div>
  </div>

</template>
<script>

  import { getToken } from '~/utils/auth'
  import Notification from 'element-ui'
  import worldManager from "~/plugins/worldmanager"

  export default {
    name: 'app',
    data: function () {

      return {
        loaded: false,
        authenticated: !!getToken(),
        secretThing: '',
      }
    },
    mounted() {
      var that = this;
      var self = this;
      console.log("default layout loaded, waiting for world load")
      var promise = worldManager.loadModels();
      promise.then(function(){
        console.log("World loaded, start view")
        that.loaded = true;
      })
//            console.log("Auth0Lock 11", Auth0Lock)

    },
    methods: {
      init() {
        if (!this.authenticated) {
          this.login();
        }
      },
      login() {
        Notification.success({
          title: "message",
          message: "nody"
        })
        console.log("login called")
      },
      logout() {
        console.log("logout called")
        // To log out, we just need to remove the token and profile
        // from local storage
        this.authenticated = false;
      },
    },
  }
</script>
<style>
  html {
    font-family: "Source Sans Pro", -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
    font-size: 16px;
    word-spacing: 1px;
    -ms-text-size-adjust: 100%;
    -webkit-text-size-adjust: 100%;
    -moz-osx-font-smoothing: grayscale;
    -webkit-font-smoothing: antialiased;
    box-sizing: border-box;
  }

  *, *:before, *:after {
    box-sizing: border-box;
    margin: 0;
  }

  .button--green {
    display: inline-block;
    border-radius: 4px;
    border: 1px solid #3b8070;
    color: #3b8070;
    text-decoration: none;
    padding: 10px 30px;
  }

  .button--green:hover {
    color: #fff;
    background-color: #3b8070;
  }

  .button--grey {
    display: inline-block;
    border-radius: 4px;
    border: 1px solid #35495e;
    color: #35495e;
    text-decoration: none;
    padding: 10px 30px;
    margin-left: 15px;
  }

  .button--grey:hover {
    color: #fff;
    background-color: #35495e;
  }
</style>
