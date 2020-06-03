<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
<div class="col-12 q-ma-md">
    <q-breadcrumbs class="text-orange" active-color="secondary">
      <template v-slot:separator>
        <q-icon
          size="1.2em"
          name="arrow_forward"
          color="purple"
        />
      </template>
      <q-breadcrumbs-el label="Database" icon="fas fa-database" />
      <q-breadcrumbs-el label="Tables" icon="fas fa-table" />
    </q-breadcrumbs>
    
    <q-page-sticky position="bottom-right" :offset="[50, 50]">
            <q-btn label = "Create Table" fab icon="add" color="cyan" />
    </q-page-sticky>

    <div class="row">
      
      <div class="col-6 q-pa-md">
        <q-markdown src="::: tip
Create and edit tables, or view table data. Daptin creates **user_account** table automatically.
:::"></q-markdown>
        <div class="q-pa-lg">
              <q-option-group
                v-model="selectedTable"
                :options="tableOptions"
                color="primary"
              >
              </q-option-group>
              
              <q-pagination
              v-model="current"
              color="black"
              :max="10"
              :max-pages="6"
              :boundary-numbers="false"
              >

              </q-pagination>

              <q-list bordered>
                <q-item clickable v-ripple>
                  <q-item-section>User Accounts</q-item-section>
                  <q-icon name="fas fa-edit"></q-icon>  
                </q-item>
                <q-item clickable v-ripple>
                  <q-item-section>Emupdates</q-item-section>
                  <q-icon name="fas fa-edit"></q-icon>
                </q-item>
              </q-list>


        </div>

        <div class="q-pa-md q-gutter-sm">
          <q-btn color="primary" icon="edit" label="Edit" @click="$router.push('/tables/edit/' + table.table_name)"/>
          <q-btn color="secondary" icon="add" label="Add Data" @click="$router.push('/tables/data/' + table.table_name)"/>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
  import {mapActions, mapGetters, mapState} from 'vuex';

  export default {
    name: 'TableSideBar',
    methods: {
      setTable(tableName) {
        console.log("set table", tableName);
        this.setSelectedTable(tableName)
      },
      ...mapActions(['load', 'setSelectedTable'])
    },
    data() {
      return {
        text: '',
        selectedTable: null
      }
    },
    mounted() {
      const that = this;
      this.$q.loadingBar.start();
      that.load().then(function () {
        that.$q.loadingBar.stop()
      });
    },
    computed: {
      tableOptions() {
        console.log(this.tablesFiltered)
        return this.tablesFiltered.map(function (e) {
          return {
            label: e.table_name,
            value: e.table_name
          }
        })
      },
      tablesFiltered() {
        const that = this;
        console.log("Get tables filtered", that.tables);
        if (that.text && that.text.length > 0) {
          return that.tables.filter(function (e) {
            return e.table_name.indexOf(that.text) > -1 && !e.is_hidden;
          })
        } else {
          return that.tables.filter(function (e) {
            return !e.is_hidden;
          });
        }
      },
      ...mapGetters(['tables'])
    }
  }
</script>
