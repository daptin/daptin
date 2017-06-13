<template>


  <section class="content">
    <router-view/>
  </section>
</template>

<script>
  import {Notification} from 'element-ui';
  import worldManager from "../plugins/worldmanager"
  import jsonApi from "../plugins/jsonapi"
  import actionManager from "../plugins/actionmanager"
  import {mapGetters} from 'vuex'
  import {mapState} from 'vuex'


  export default {
    name: 'AdminView',
    props: {},
    data () {
      return {}
    },
  }
</script>
