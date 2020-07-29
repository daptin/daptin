<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <q-page>
    <!--    <iframe-->
    <!--      sandbox="allow-same-origin"-->
    <!--      :src=" endpoint  + '/graphql' "-->
    <!--      style="height:100vh; width:100%"-->
    <!--    ></iframe>-->

  </q-page>
</template>

<script>
  import {mapActions, mapGetters, mapState} from 'vuex';
  import 'rapidoc';

  export default {
    name: 'GraphQLPage',
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

      window.open(this.endpoint + '/graphql', "_black")
    },
    computed: {
      ...mapGetters(['endpoint']),
      ...mapState([])
    },

    watch: {}
  }
</script>
