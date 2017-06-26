<template>
  <div class="container">
    Oauth response handler
  </div>
</template>

<script>
  import configManager from '../plugins/configmanager'
  import actionManager from "../plugins/actionmanager"
  import {Notification} from "element-ui"

  export default {

    data() {
      return {
        actionManager: actionManager,
      }
    },
    methods: {
      init() {
        var that = this;
        console.log("oauth response", this.$route)
        var query = this.$route.query;
        this.actionManager.doAction("oauth_token", "oauth.login.response", this.$route.query).then(function () {

        }, function () {
          Notification.error({
            message: "Failed to validate connection"
          });
          that.$router.push({
            name: "Dashboard"
          })
        });
      },
    },
    mounted () {
      this.init();
    }
  }
</script>
