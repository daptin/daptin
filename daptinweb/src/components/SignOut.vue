<template>
  <p>Signing off...</p>
</template>

<script>
  import {unsetToken} from '../utils/auth'

  export default {
    mounted () {
      unsetToken();
      console.log("logged out");
      this.$router.push({
        name: 'SignIn'
      })
    }
  }
</script>
