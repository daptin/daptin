<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <q-page>
    <div class="row">
      <div class="col-12">
        <div class="q-pa-md">
          <span class="text-h4">Help</span>
        </div>
      </div>
      <div class="col-12 q-pa-md">
        <slot name="help-content"></slot>
      </div>
    </div>

    <q-page-sticky v-if="showHelp = true" position="top-right" :offset="[10, 10]">
      <q-btn flat size="sm" @click="$emit('closeHelp')" fab icon="fas fa-times"/>
    </q-page-sticky>


  </q-page>
</template>

<script>
  import {mapActions} from 'vuex';

  export default {
    name: 'PageIndex',
    methods: {
      ...mapActions([])
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
