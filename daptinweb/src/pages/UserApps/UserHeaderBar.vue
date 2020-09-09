<template>

  <q-header style="background: transparent">
    <q-toolbar v-if="decodedAuthToken() !== null">
      <q-btn v-for="btn in buttons.before" flat @click="btn.click" :icon="btn.icon"></q-btn>
      <q-toolbar-title shrink>{{ title }}</q-toolbar-title>
      <q-btn style="border: 1px solid black" v-for="btn in buttons.after" flat @click="btn.click" :label="btn.label"
             :icon="btn.icon"></q-btn>
      <q-space/>
      <q-btn flat icon="fas fa-th">
        <q-menu>
          <div class="row no-wrap q-pa-md">
            <q-list>

              <q-item :disable="!item.enable" v-for="item in menuItems" @click="$router.push(item.path)" clickable>
                <q-item-section avatar>
                  <q-icon
                    :name="item.icon"
                  />
                </q-item-section>
                <q-item-section>
                  {{ item.name }}
                </q-item-section>
              </q-item>
            </q-list>
          </div>
        </q-menu>

      </q-btn>
      <q-btn size="1.2em" class="profile-image" flat :icon="'img:' + decodedAuthToken().picture">
        <q-menu>
          <div class="row no-wrap q-pa-md">

            <div class="column items-center">
              <q-avatar size="72px">
                <img :src="decodedAuthToken().picture">
              </q-avatar>

              <div class="text-subtitle1 q-mt-md q-mb-xs">{{ decodedAuthToken().name }}</div>

              <q-btn
                color="black"
                label="Logout"
                push
                @click="logout()"
                size="sm"
                v-close-popup
              />
            </div>
          </div>
        </q-menu>
      </q-btn>
      <!--      <q-img :src="decodedAuthToken().picture" width="40px" ></q-img>-->
    </q-toolbar>
  </q-header>

</template>
<style>
.profile-image img {
  border-radius: 10px;
}
</style>
<script>
import {mapActions, mapGetters} from "vuex";

export default {
  name: "UserHeaderBar",
  methods: {
    logout() {
      localStorage.removeItem("token");
      localStorage.removeItem("user");
      this.setDecodedAuthToken(null);
      this.$router.push("/login");
      window.location = window.location;
    },
    ...mapActions(['setDecodedAuthToken'])
  },
  data() {
    return {
      ...mapGetters(['decodedAuthToken']),
      menuItems: [
        {
          name: "Email",
          enable: false,
          path: '/apps/email',
          icon: 'fas fa-envelope'
        },
        {
          name: "Files",
          path: '/apps/files',
          enable: true,
          icon: 'fas fa-archive'
        },
        {
          name: "Contacts",
          enable: false,
          path: '/apps/contacts',
          icon: 'fas fa-users'
        },
        {
          name: "Calendar",
          enable: true,
          path: '/apps/calendar',
          icon: 'fas fa-calendar'
        },
      ]
    }
  },
  props: ['title', 'buttons']
}
</script>
