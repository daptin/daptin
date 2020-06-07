<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <q-page>    
      <table-side-bar></table-side-bar>
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
        ...mapState([])
      }
    },
    mounted() {
    },
    computed: {
      ...mapGetters([]),
      ...mapState([])
    },

    watch: {}
  }
</script>
