<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <q-page>
    <div class="row" :style="{height: '90vh'}">
      <div class="col-12" style="height: 100%" v-if="helpPath">
        <slot name="help-content">
          <iframe style="width: 100%; height: 100%; border: none;" :src=" (hostname === 'site.daptin.com' ? 'http://localhost:8000' : 'https://daptin.github.io/daptin') + ( helpPath[0] === '/' ? '' : '/' ) + helpPath"></iframe>
        </slot>
      </div>
      <q-page-sticky v-if="showHelp = true" position="bottom-right" :offset="[10, 10]">
        <q-btn flat size="sm" @click="$emit('closeHelp')" fab icon="fas fa-times"/>
      </q-page-sticky>

    </div>



  </q-page>
</template>

<script>
  import {mapActions} from 'vuex';

  export default {
    name: 'HelpPage',
    methods: {
      ...mapActions([])
    },
    computed: {
      iframeSrcPath() {
        let s = window.location.hostname === 'site.daptin.com' && window.location.port == "8080" ? 'http://localhost:8000' : 'https://daptin.github.com/daptin/'+ this.helpPath;
        console.log("iframe path", s)
        return s;
      }
    },
    data() {
      return {
        text: '',
        helpPath: null,
        hostname: null,
      }
    },
    mounted() {
      this.helpPath = window.location.href.split('#')[1];
      this.hostname = window.location.hostname;
      console.log("Window router", this.$router, window.location.href, this.helpPath);
    }
  }
</script>
