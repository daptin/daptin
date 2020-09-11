<template>
  <div id="q-app">
    <router-view/>

  </div>
</template>
<style>
  @import '~tabulator-tables/dist/css/tabulator_simple.css';
  @import '../node_modules/@syncfusion/ej2-buttons/styles/material.css';
  @import '../node_modules/@syncfusion/ej2-base/styles/material.css';
  @import '../node_modules/@syncfusion/ej2-inputs/styles/material.css';
  @import '../node_modules/@syncfusion/ej2-popups/styles/material.css';
  @import '../node_modules/@syncfusion/ej2-lists/styles/material.css';
  @import '../node_modules/@syncfusion/ej2-navigations/styles/material.css';
  @import '../node_modules/@syncfusion/ej2-splitbuttons/styles/material.css';
  @import '../node_modules/@syncfusion/ej2-dropdowns/styles/material.css';
  @import "../node_modules/@syncfusion/ej2-vue-documenteditor/styles/material.css";

</style>
<script>

  import {mapGetters, mapActions} from 'vuex';
  import "fontsource-source-sans-pro"

  export default {
    data() {
      return {
        showLogin: false,
        ...mapGetters(['loggedIn'])
      }
    },
    methods: {
      ...mapActions([])
    },
    name: 'App',
    mounted() {
      console.log("Loaded app user is ", this.loggedIn());
      if (!this.loggedIn()) {
        this.$router.push("/login")
      }

    }
  }
</script>
