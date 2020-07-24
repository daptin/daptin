<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <div>
    <div class="q-pa-md q-gutter-sm">
      <q-breadcrumbs  >
        <template v-slot:separator>
          <q-icon
            size="1.2em"
            name="arrow_forward"
          />
        </template>

        <q-breadcrumbs-el label="Storage" icon="fas fa-archive"/>
        <q-breadcrumbs-el label="Cloud stores" icon="fas fa-bars"/>
      </q-breadcrumbs>
    </div>
    <q-separator></q-separator>

    <q-page-sticky position="bottom-right" :offset="[50, 50]">
      <q-btn @click="newcloudDrawer = true" label="Add cloud store" fab icon="add" color="primary"/>
    </q-page-sticky>

    <div class="row q-pa-md q-gutter-sm">
      <div class="col-2 col-sm-3 col-xs-12" v-for="cloud1 in cloudstores">
        <q-card>
          <q-card-section>
            <span class="text-h4">{{cloud1.name}}</span>
          </q-card-section>
        </q-card>

      </div>
    </div>

    <q-drawer :width="500" content-class="bg-grey-3" side="right" v-model="newcloudDrawer">
      <q-scroll-area class="fit row">
        <div class="q-pa-md">
          <span class="text-h6">Create cloud</span>
          <q-form class="q-gutter-md">
            <q-input label="Name" v-model="cloud.name"></q-input>
            <q-btn color="primary" @click="createcloud()">Create</q-btn>
            <q-btn @click="newcloudDrawer = false">Cancel</q-btn>
          </q-form>
        </div>
      </q-scroll-area>
    </q-drawer>

    <q-page-sticky v-if="!showHelp && !newcloudDrawer" position="top-right" :offset="[0, 0]">
      <q-btn flat @click="showHelp = true" fab icon="fas fa-question"/>
    </q-page-sticky>

    <q-drawer overlay :width="400" side="right" v-model="showHelp && !newcloudDrawer">
      <q-scroll-area class="fit">
        <help-page @closeHelp="showHelp = false">
          <template v-slot:help-content>
            <q-markdown src="::: tip
You can create different user clouds here. Different user clouds can have different permissions.
E.g. Admin cloud that has permissions to create, read, write and delete tables.
:::"></q-markdown>
          </template>
        </help-page>
      </q-scroll-area>
    </q-drawer>


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
