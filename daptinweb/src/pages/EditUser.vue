<template>
  <q-page class="q-pa-md">
    <span class="text-h4">
      User
    </span>
    <div class="row" v-if="user">
      <div class="col-12">
        <q-form class="q-pa-md q-gutter-sm">
          <q-input label="Name" v-model="user.name"></q-input>
          <q-input label="Email" v-model="user.email"></q-input>
          <q-btn class="float-left" color="negative" label="Delete user"></q-btn>
          <q-btn class="float-left" label="Reset password"></q-btn>
          <q-btn class="float-right" color="primary" label="Save"></q-btn>
        </q-form>

      </div>
    </div>
  </q-page>
</template>
<script>
  import {mapActions} from "vuex";

  export default {
    name: 'EditUser',
    data: function () {
      return {
        user: null,
      }
    },
    mounted() {
      const that = this;
      that.loadData({
        tableName: "user_account",
        params: {
          query: JSON.stringify([
            {
              column: 'email',
              operator: 'is',
              value: this.$route.params.emailId
            }
          ])
        }
      }).then(function (res) {
        console.log("Loaded user", res);
        if (!res.data || res.data.length == 0) {
          that.$q.notify({
            message: "User not found"
          });
          that.$router.back();
          return
        }
        that.user = res.data[0];
      })
    },
    methods: {
      ...mapActions(['loadData'])
    }
  }
</script>
