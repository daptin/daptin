<template>
  <q-page-container>
    <user-header-bar :buttons="{}"></user-header-bar>
  </q-page-container>


</template>
<script>
export default {
  name: "UsersAppsList",
  mounted() {
    console.log("Mounted user apps")
  }
}
</script>
