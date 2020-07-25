<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <div class="row q-pa-md q-gutter-sm">


    <div class="col-12">
      <span class="text-h4">Site</span>
    </div>

    <div class="col-3 q-pa-md q-gutter-sm">

      <q-card v-for="site in sites">
        <q-card-section>
          {{site.hostname}}
        </q-card-section>
      </q-card>
    </div>

  </div>
</template>

<script>
  import {mapActions, mapGetters, mapState} from 'vuex';

  export default {
    name: 'TablePage',
    methods: {
      editSite(evt, site) {
        console.log("Edit site", site)
      },
      createSloud() {
        const that = this;
        console.log("new site", this.site);
        this.site.tableName = "site";
        that.createRow(that.site).then(function (res) {
          that.user = {};
          that.$q.notify({
            message: "site created"
          });
          that.refresh();
          that.newsiteDrawer = false;
        }).catch(function (e) {
          if (e instanceof Array) {
            that.$q.notify({
              message: e[0].title
            })
          } else {
            that.$q.notify({
              message: "Failed to create site"
            })
          }
        });
      },
      ...mapActions(['loadData', 'getTableSchema', 'createRow']),
      refresh() {
        var tableName = "site";
        const that = this;
        this.loadData({tableName: tableName}).then(function (data) {
          console.log("Loaded data", data);
          that.sites = data.data;
        })
      }
    },
    data() {
      return {
        text: '',
        showHelp: false,
        site: {},
        filter: null,
        newsiteDrawer: false,
        sites: [],
        columns: [
          {
            name: 'name',
            field: 'name',
            label: 'site name',
            align: 'left',
            sortable: true,
          }
        ],
        ...mapState([])
      }
    },
    mounted() {
      this.refresh();
    },
    computed: {
      ...mapGetters(['selectedTable']),
      ...mapState([])
    },

    watch: {}
  }
</script>
