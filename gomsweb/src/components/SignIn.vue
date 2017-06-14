<template>
  <div id="auth0-lock"/>
</template>

<script>
  import {show} from '../utils/lock'
  import configManager from '../plugins/configmanager'

  export default {


    methods: {
      init() {
        console.log("sign in loaded");
        show('auth0-lock');
        configManager.getAllConfig().then(function (configs) {
          console.log("configs", configs)
        }, function () {
          console.log("Failed to get configs", arguments)
        })
      },
    },
    updated () {
      this.init();
    },
    mounted () {
      this.init();
    }
  }
</script>
