<template>

  <q-header class="bg-white text-black">
    <q-bar v-if="decodedAuthToken() !== null">
      <form @submit="emitSearch">
        <input @focusin="searchFocused" @focusout="searchUnFocused" id="searchInput"
               placeholder="Type '/' to focus here"
               type="text" v-model="searchQuery"/>
      </form>
      <q-btn :key="btn.icon" v-for="btn in buttons.before" flat @click="buttonClicked(btn)" :icon="btn.icon"></q-btn>
      <q-btn :key="btn.icon" v-for="btn in buttons.after" flat @click="buttonClicked(btn)" :label="btn.label"
             :icon="btn.icon"></q-btn>
      <q-space/>
      <q-btn flat icon="fas fa-th">
        <q-menu>
          <div class="row no-wrap q-pa-md">
            <q-list>

              <q-item :disable="!item.enable" :key="item.name" v-for="item in menuItems"
                      @click="$router.push(item.path)" clickable>
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
      <q-btn size="0.8em" class="profile-image" flat :icon="'img:' + decodedAuthToken().picture">
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
                rounded
                @click="logout()"
                size="sm"
                v-close-popup
              />
            </div>
          </div>
        </q-menu>
      </q-btn>
      <!--      <q-img :src="decodedAuthToken().picture" width="40px" ></q-img>-->
    </q-bar>
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
    emitSearch(event) {
      this.$emit('search', this.searchQuery)
      event.stopPropagation();
      event.preventDefault();
    },
    searchFocused() {
      this.isTypingSearchQuery = true;
    },
    searchUnFocused() {
      this.isTypingSearchQuery = false;
    },
    buttonClicked(btn) {
      console.log("Button clicked", btn, this.searchQuery)
      if (btn.click) {
        btn.click();
        return;
      }
      this.$emit(btn.event, this.searchQuery);
    },
    logout() {
      localStorage.removeItem("token");
      localStorage.removeItem("user");
      this.setDecodedAuthToken(null);
      this.$router.push("/login");
      window.location = window.location;
    },
    ...mapActions(['setDecodedAuthToken'])
  },
  beforeDestroy() {
    document.onkeypress = null;
  },
  mounted() {
    const that = this;
    document.onkeypress = function (keyEvent) {
      if (that.isTypingSearchQuery) {
        return;
      }
      console.log("Key pressed", keyEvent)
      if (keyEvent.key === '/') {
        document.getElementById("searchInput").focus();
        keyEvent.stopPropagation();
        keyEvent.preventDefault();
      }
    }
  },
  data() {
    return {
      ...mapGetters(['decodedAuthToken']),
      searchQuery: null,
      isTypingSearchQuery: false,
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
        // {
        //   name: "Documents",
        //   enable: true,
        //   path: '/apps/document/new',
        //   icon: 'fas fa-file-alt'
        // },
        // {
        //   name: "Spreadsheet",
        //   enable: true,
        //   path: '/apps/spreadsheet/new',
        //   icon: 'fas fa-file-csv'
        // },
        {
          name: "Calendar",
          enable: true,
          path: '/apps/calendar',
          icon: 'fas fa-calendar'
        },
        {
          name: "Drag",
          enable: true,
          path: '/apps/drageditor',
          icon: 'fas fa-hand-rock'
        },

      ]
    }
  },
  props: ['title', 'buttons']
}
</script>
