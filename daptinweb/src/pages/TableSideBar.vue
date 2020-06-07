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
      <q-breadcrumbs-el label="Database" icon="fas fa-database"/>
      <q-breadcrumbs-el label="Tables" icon="fas fa-table"/>
    </q-breadcrumbs>

    <q-page-sticky position="bottom-right" :offset="[50, 50]">
      <q-btn @click="$router.push('/tables/create')" label="Create Table" fab icon="add"/>
    </q-page-sticky>

    <div class="row">

      <div class="col-6 q-pa-md">
        <q-markdown src="::: tip
Daptin creates **user_account** table automatically. You can create new tables and edit existing tables, or view table data.
:::"></q-markdown>
        <div class="q-pa-lg">

          <div class="col q-pa-sm">
            <q-markup-table flat>
              <thead>
              <tr>
                <th align="left">Table</th>
                <th align="right"></th>
                <th></th>
              </tr>
              </thead>  
              <tbody>
              <tr v-for="table in tablesFiltered">
                <td>{{table.table_name}}</td>
                <td align="right">
                  <q-btn @click="$router.push('/tables/edit/' + table.table_name)" flat icon="fas fa-wrench"></q-btn>
                </td>
                <td align="left">
                  <q-btn @click="$router.push('/tables/data/' + table.table_name)" flat icon="fas fa-list"></q-btn>
                </td>
              </tr>
              </tbody>
            </q-markup-table>
          </div>
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
        console.log(this.tablesFiltered);
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
