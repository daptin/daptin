<template>
  <div id="auth0-lock"/>
</template>

<script>
  import {show} from '../utils/lock'

  export default {
    updated () {
      console.log("sign in loaded")
      show('auth0-lock')
    },
    mounted () {
      console.log("sign in loaded")
      show('auth0-lock')
    }
  }
</script>
