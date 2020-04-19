<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <q-page>
    <div class="row">
      <div class="col-10">
        <router-view/>
      </div>
    </div>

  </q-page>
</template>

<script>
  import {mapActions, mapGetters, mapState} from 'vuex';

  export default {
    name: 'TablePage',
    methods: {
      ...mapActions(['load'])
    },
    data() {
      return {
        text: '',
      }
    },
    mounted() {
    },
    computed: {
      ...mapGetters([])
    }
  }
</script>
