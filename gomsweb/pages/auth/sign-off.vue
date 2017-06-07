<template>
  <p>Signing off...</p>
</template>

<script>
import { unsetToken } from '~/utils/auth'
import { logout } from '~/utils/lock'

export default {
  mounted () {
    unsetToken()
    logout()
  }
}
</script>
