<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <div class="col-12">
    <div class="row">
      <div class="q-pa-md col-4">
        <q-input color="teal" filled v-model="text" label="search table">
          <template v-slot:prepend>
            <q-icon name="search"/>
          </template>
        </q-input>
      </div>
    </div>
    <div class="row q-pa-md">
      <div class="col-6 ">
        <h4>Tables ({{tablesFiltered.length}})</h4>
      </div>
      <div class="col-3">
        <q-btn style="float: right" @click="$router.push('/tables/create')" class="btn btn-sm bg-primary text-white"
               label="Create new table"></q-btn>
      </div>
      <div class="col-6">
        <q-list padding class="rounded-borders">
          <q-item v-for="table in tablesFiltered" :key="table.table_name">
            <q-item-section>
              {{table.table_name}}
            </q-item-section>
            <q-item-section>
              <q-btn @click="$router.push('/tables/edit/' + table.table_name)">Modify table</q-btn>
            </q-item-section>
            <q-item-section>
              <q-btn @click="$router.push('/tables/data/' + table.table_name)">Edit data</q-btn>
            </q-item-section>
          </q-item>
        </q-list>
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
