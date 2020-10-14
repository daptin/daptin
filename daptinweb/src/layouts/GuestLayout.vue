<template>
  <q-layout view="lHh Lpr lFf">
    <q-page-container class="bg-white">
      <router-view/>
    </q-page-container>
  </q-layout>
</template>

<script>
  export default {
    name: 'GuestLayout',

    components: {},
    mounted() {
      console.log("mounted guest layout")
    },

    data() {
      return {
        leftDrawerOpen: false,
        essentialLinks: []
      }
    }
  }
</script>
