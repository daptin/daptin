<template>
  <q-page>

    <div class="q-pa-md q-gutter-sm" v-if="group">
      <q-breadcrumbs>
        <template v-slot:separator>
          <q-icon
            size="1.2em"
            name="arrow_forward"
          />
        </template>

        <q-breadcrumbs-el label="User" icon="fas fa-user"/>
        <q-breadcrumbs-el label="Groups" icon="fas fa-users"/>
        <q-breadcrumbs-el :label="group.name"/>
      </q-breadcrumbs>
    </div>
    <q-separator></q-separator>


    <div class="row" v-if="group">
      <div class="col-12 q-pa-md q-gutter-sm">
        <span class="text-h6">{{group.name}}</span>
      </div>
      <div class="col-12 q-pa-md q-gutter-sm">
        <q-btn @click="deleteGroup()" label="Delete" color="negative"></q-btn>
      </div>
    </div>
  </q-page>
</template>
<script>
  import {mapActions} from "vuex";

  export default {
    name: "EditGroup",
    data: function () {
      return {
        group: null,
      }
    },
    mounted() {
      const that = this;
      that.loadData({
        tableName: "usergroup",
        params: {
          query: JSON.stringify([
            {
              column: 'reference_id',
              operator: 'is',
              value: this.$route.params.groupId
            }
          ])
        }
      }).then(function (res) {
        console.log("Loaded group", res);
        if (!res.data || res.data.length === 0) {
          that.$q.notify({
            message: "Group not found"
          });
          that.$router.back();
          return
        }
        that.group = res.data[0];
      })
    },
    methods: {
      deleteGroup(){
        const that = this;
        that.deleteRow({
          tableName: "usergroup",
          reference_id: that.group.id
        }).then(function (res) {
          console.log("Deleted group", res);
          that.$q.notify({
            message: "Deleted group"
          });
          that.$router.back();
        }).catch(function (error) {
          that.$q.notify({
            message: JSON.stringify(error)
          })
        })
      },
      ...mapActions(['loadData', 'deleteRow'])
    }
  }
</script>
