<template>
  <q-page class="q-pa-md">
    <span class="text-h4">
      Usergroup
    </span>
    <div class="row" v-if="group">
      <div class="col-12 q-pa-md q-gutter-sm">
        <span class="text-h6">{{group.name}}</span>
      </div>
      <div class="col-12 q-pa-md q-gutter-sm">
        <q-btn label="Change name" color="warning"></q-btn>
        <q-btn label="Delete" color="negative"></q-btn>
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
      ...mapActions(['loadData'])
    }
  }
</script>
