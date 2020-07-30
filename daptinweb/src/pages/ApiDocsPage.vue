<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <q-page>
    <div>
      <rapi-doc
        :show-header="false"
        :spec-url=" endpoint  + '/openapi.yaml' "
        render-style="read"
        style="height:100vh; width:100%"
        header-color="black"
      ></rapi-doc>

    </div>
  </q-page>
</template>

<script>
  import {mapActions, mapGetters, mapState} from 'vuex';
  import 'rapidoc';

  export default {
    name: 'ApiDocsPage',
    methods: {
      ...mapActions(['loadTables'])
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
      ...mapGetters(['endpoint']),
      ...mapState([])
    },

    watch: {}
  }
</script>
