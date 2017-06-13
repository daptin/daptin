<template>
  <p>Signing in...</p>
</template>

<script>
  import {setToken, checkSecret, extractInfoFromHash} from '../utils/auth'

  export default {
    mounted () {
      console.log("signed in")
      const {token, secret} = extractInfoFromHash()
      if (!checkSecret(secret) || !token) {
        this.$router.replace('/auth/signin')
        console.error('Something happened with the Sign In request')
        return
      }
      setToken(token)
      this.$router.replace('/')
    }
  }
</script>
