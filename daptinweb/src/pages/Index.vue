<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <q-page>
    <div class="row">
      <div class="col-2">
        <div class="q-pa-md">
          <h2>Get Started</h2>
          <router-link to="/tables"></router-link>
        </div>
      </div>
      <div class="col-10">

      </div>
    </div>

  </q-page>
</template>

<script>
  import {mapActions} from 'vuex';

  export default {
    name: 'PageIndex',
    methods: {
      // ...mapActions(['loadTables'])
    },
    data() {
      return {
        text: '',
      }
    },
    mounted() {
    }
  }
</script>
