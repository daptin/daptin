<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <div>

    <file-browser v-if="site" v-on:close="$router.back()"
                  :site="site"></file-browser>


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
        var siteId = that.$route.params.siteId;
        this.loadData({
          tableName: 'site',
          params: {
            query: JSON.stringify([
              {
                column: 'reference_id',
                operator: 'is',
                value: siteId
              }
            ]),
            included_relations: "cloud_store_id"
          },
        }).then(function (data) {
          console.log("Site data loaded", data);
          that.site = data.data[0];
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
