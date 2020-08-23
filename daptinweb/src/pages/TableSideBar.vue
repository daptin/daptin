<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <div>
    <div class="q-pa-md q-gutter-sm">
      <q-breadcrumbs >
        <template v-slot:separator>
          <q-icon
            size="1.2em"
            name="arrow_forward"
          />
        </template>

        <q-breadcrumbs-el label="Database" icon="fas fa-database"/>
        <q-breadcrumbs-el label="Tables" icon="fas fa-table"/>
      </q-breadcrumbs>
    </div>
    <q-separator></q-separator>


    <q-page-sticky style="z-index: 3000" position="bottom-right" :offset="[20, 20]">
      <q-btn size="md" @click="$router.push('/tables/create')" fab icon="add" color="primary"/>
    </q-page-sticky>

    <div class="row">
      <div class="col-12 q-gutter-sm">
        <q-markup-table flat>
          <tbody>
          <tr style="cursor: pointer" @click="$router.push('/tables/data/' + table.table_name)" v-for="table in tablesFiltered">
            <td>{{table.table_name}}</td>
            <td align="right">
              <q-btn @click.stop="$router.push('/tables/edit/' + table.table_name)" flat icon="fas fa-wrench"></q-btn>
            </td>
            <td align="left">
              <q-btn @click.stop="$router.push('/tables/data/' + table.table_name)" flat icon="fas fa-list"></q-btn>
            </td>
          </tr>
          </tbody>
        </q-markup-table>
      </div>
    </div>

    <q-page-sticky v-if="!showHelp" position="top-right" :offset="[0, 0]">
      <q-btn flat @click="showHelp = true" fab icon="fas fa-question"/>
    </q-page-sticky>

    <q-drawer overlay :width="400" side="right" v-model="showHelp">
      <q-scroll-area class="fit">
        <help-page @closeHelp="showHelp = false">
          <template v-slot:help-content>
                <q-markdown src="::: tip
Daptin creates **user_account** table automatically. You can create new tables and edit existing tables, or view table data.
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
    name: 'TableSideBar',
    methods: {
      setTable(tableName) {
        console.log("set table", tableName);
        this.setSelectedTable(tableName)
      },
      ...mapActions(['setSelectedTable'])
    },
    data() {
      return {
        text: '',
        showHelp: false,
        selectedTable: null
      }
    },
    mounted() {
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
    },
    watch: {
      tables() {
        console.log("updated tables  in watch ", this.tables, this.tablesFiltered)
      }
    }
  }
</script>
