<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <q-page>
    <div class="row" :style="{height: '90vh'}">
      <div class="col-12" style="height: 100%">
        <slot name="help-content">
          <iframe v-if="helpPath" style="width: 100%; height: 100%; border: none;" :src="'https://daptin.github.io/daptin' + helpPath "></iframe>
        </slot>
      </div>
    </div>

    <q-page-sticky v-if="showHelp = true" position="top-right" :offset="[5, 5]">
      <q-btn flat size="sm" @click="$emit('closeHelp')" fab icon="fas fa-times"/>
    </q-page-sticky>


  </q-page>
</template>

<script>
  import {mapActions} from 'vuex';

  export default {
    name: 'HelpPage',
    methods: {
      ...mapActions([])
    },
    data() {
      return {
        text: '',
        helpPath: null,
      }
    },
    mounted() {
      console.log("Window router", this.$router, window.location.href);
      this.helpPath = window.location.href.split('#')[1];
    }
  }
</script>
