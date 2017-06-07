<template>
  <div id="auth0-lock"/>
</template>

<script>
  import {show} from '~/utils/lock'

  export default {
    middleware: 'anonymous',
    mounted () {
      show('auth0-lock')
    }
  }
</script>
