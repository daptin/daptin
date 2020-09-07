<template>
  <q-page-container>
    <q-header>
      <q-toolbar class="bg-black">
        <q-toolbar-title>Title</q-toolbar-title>
      </q-toolbar>
    </q-header>
    <div class="row q-pa-md">
      <div class="col-12 text-white">
        User apps 1
      </div>
    </div>
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
