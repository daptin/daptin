<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <q-page>
    <q-drawer
      v-model="drawerLeft"
      show-if-above
      :width="350"
      :breakpoint="700"
      elevated
      content-class=""
    >
      <q-scroll-area class="fit">
        <table-side-bar></table-side-bar>
      </q-scroll-area>
    </q-drawer>
    <div class="row">
      <div class="col-10">
        <h4>Select a table to edit</h4> {{drawerLeft}}
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
        ...mapState(['drawerLeft'])
      }
    },
    mounted() {
    },
    computed: {
      ...mapGetters(['selectedTable']),
      ...mapState([])
    },

    watch: {
      'selectedTable': function () {
        console.log("selected table", this.selectedTable);
        this.$router.push("/tables/edit/" + this.selectedTable)
      }
    }
  }
</script>
