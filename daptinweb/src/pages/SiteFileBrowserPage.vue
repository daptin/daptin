<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <div>

    <div class="q-pa-md q-gutter-sm">
      <q-breadcrumbs>
        <template v-slot:separator>
          <q-icon
            size="1.2em"
            name="arrow_forward"
            color="black"
          />
        </template>

        <q-breadcrumbs-el label="Storage" icon="fas fa-archive"/>
        <q-breadcrumbs-el label="Site" icon="fas fa-list"/>
      </q-breadcrumbs>
    </div>
    <q-separator></q-separator>

    <div class="row q-pa-md q-gutter-sm">
      <file-browser v-if="site" v-on:close="$router.back()"
                    :site="site"></file-browser>
    </div>


    <q-drawer :breakpoint="1400" :width="fileDrawerWidth > 800 ? 800 : fileDrawerWidth" side="right" overlay
              v-model="showFileBrowser">
      <q-scroll-area class="fit">

      </q-scroll-area>
    </q-drawer>

  </div>
</template>

<script>
  import {mapActions, mapGetters, mapState} from 'vuex';

  export default {
    name: 'SiteFileBrowserPage',
    methods: {
      ...mapActions(['loadData', 'getTableSchema', 'createRow', 'deleteRow', 'updateRow', 'executeAction', 'loadOneData']),
      refresh() {
        var tableName = "site";
        const that = this;
        var siteId = that.$route.params.site_id;
        this.loadOneData({
          tableName: 'site',
          referenceId: siteId,
          params: {
            included_relations: "cloud_store_id"
          }
        }).then(function (data) {
          console.log("Site data loaded", data);
          that.site = data.data;
        });
      }
    },
    data() {
      return {
        text: '',
        site: null,
        ...mapState([])
      }
    },
    mounted() {
      console.log("Site page scope", this, window.screen.availWidth);
      this.refresh();
    },
    computed: {
      fileDrawerWidth() {
        return window.screen.availWidth;
      },
      ...mapGetters(['selectedTable']),
      ...mapState([])
    },

    watch: {}
  }
</script>
