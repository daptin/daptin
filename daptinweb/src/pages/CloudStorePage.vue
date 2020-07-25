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
        <q-breadcrumbs-el label="Cloud stores" icon="fas fa-list"/>
      </q-breadcrumbs>
    </div>
    <q-separator></q-separator>

    <div class="row q-pa-md q-gutter-sm">

      <q-page-sticky style="z-index: 3000" position="bottom-right" :offset="[20, 20]">
        <q-btn @click="showCreateCloudStoreDrawer = true" fab icon="add" color="primary"/>
      </q-page-sticky>

      <div class="col-4 col-xl-2 col-lg-3 col-xs-12 col-sm-6 q-pa-md" v-for="store in cloudstores">
        <q-card>
          <q-card-section>
            <span class="text-h6">{{store.name}}</span>
          </q-card-section>
          <q-card-section>
            <span>Provider</span> <span class="text-bold float-right">{{store.store_provider}}</span>
          </q-card-section>
          <q-card-section>
            <span>Root path</span> <span class="text-bold float-right">{{store.root_path}}</span>
          </q-card-section>
          <q-card-section>
            <div class="row">
              <div class="col-12">
                <q-btn size="sm" label="Browse files" color="primary" class="float-right"></q-btn>
                <q-btn size="sm" label="Edit store" class="float-right"></q-btn>
              </div>
            </div>
          </q-card-section>
        </q-card>
      </div>

    </div>
  </div>
</template>

<script>
  import {mapActions, mapGetters, mapState} from 'vuex';

  export default {
    name: 'TablePage',
    methods: {
      editcloudStore(evt, cloud) {
        console.log("Edit cloud store", cloud)
      },
      createcloud() {
        const that = this;
        console.log("new cloud", this.cloud);
        this.cloud.tableName = "cloud_store";
        that.createRow(that.cloud).then(function (res) {
          that.user = {};
          that.$q.notify({
            message: "cloud created"
          });
          that.refresh();
          that.newcloudDrawer = false;
        }).catch(function (e) {
          if (e instanceof Array) {
            that.$q.notify({
              message: e[0].title
            })
          } else {
            that.$q.notify({
              message: "Failed to create cloud"
            })
          }
        });
      },
      ...mapActions(['loadData', 'getTableSchema', 'createRow']),
      refresh() {
        var tableName = "cloud_store";
        const that = this;
        this.loadData({tableName: tableName}).then(function (data) {
          console.log("Loaded data", data);
          that.cloudstores = data.data;
        })
      }
    },
    data() {
      return {
        text: '',
        showHelp: false,
        showCreateCloudStoreDrawer: false,
        cloud: {},
        filter: null,
        newcloudDrawer: false,
        cloudstores: [],
        columns: [
          {
            name: 'name',
            field: 'name',
            label: 'cloud name',
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
