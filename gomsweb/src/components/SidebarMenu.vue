<template>
  <ul class="sidebar-menu">
    <li class="header">Dashboard </li>
    <li class="pageLink" v-on:click="toggleMenu">
      <router-link :to="{name: 'Dashboard', params: {}}">
        <span class="page">Dashboard</span>
      </router-link>
    </li>

    <li class="header">Items </li>

    <li class="pageLink" v-on:click="toggleMenu" v-for="w in topWorlds"
        v-if="w.table_name != 'user' && w.table_name != 'usergroup'">
      <router-link :class="w.table_name + '-link'" :to="{name: 'Entity', params: {tablename: w.table_name}}">
        <span class="page">{{w.table_name | titleCase}}</span>
      </router-link>
    </li>

    <li class="header">People </li>
    <li class="pageLink" v-on:click="toggleMenu">
      <router-link :class="'user-link'" :to="{name: 'Entity', params: {tablename: 'user'}}">
        <span class="page">User</span>
      </router-link>
    </li>
    <li class="pageLink" v-on:click="toggleMenu">
      <router-link :class="'user-link'" :to="{name: 'Entity', params: {tablename: 'usergroup'}}">
        <span class="page">User Group</span>
      </router-link>
    </li>


    <li class="treeview help-support">
      <a href="#">
        <span>Adminstration</span>
        <span class="pull-right-container">
          <i class="fa fa-angle-left fa-fw pull-right"></i>
        </span>
      </a>
      <ul class="treeview-menu">

        <li class="pageLink">
          <router-link class="upload-schema"
                       :to="{name : 'Action', params: {tablename: 'world', actionname: 'upload_system_schema'}}">
            <i class="fa fa-upload"></i> Update Features using JSON
          </router-link>
        </li>

        <li class="pageLink">
          <router-link class="upload-schema"
                       :to="{name : 'Action', params: {tablename: 'world', actionname: 'upload_xls_to_system_schema'}}">
            <i class="fa fa-upload"></i> Upload Xls to create entity
          </router-link>
        </li>

        <li class="pageLink" v-on:click="toggleMenu">
          <router-link :to="{name: 'Entity', params: {tablename: 'world'}}">
            <i class="fa fa-th"></i>
            <span class="page">All tables</span>
          </router-link>
        </li>
        <li class="pageLink">
          <router-link class="data-exchanges"
                       :to="{name : 'Entity', params: {tablename: 'data_exchange'}}">
            <i class="fa fa-exchange"></i> Data Exchanges
          </router-link>
        </li>
        <li class="pageLink">
          <router-link class="oauth-tokens"
                       :to="{name : 'Entity', params: {tablename: 'oauth_token'}}">
            <i class="fa fa-tags"></i> Oauth Tokens
          </router-link>
        </li>
        <li class="pageLink">
          <router-link class="list-connections"
                       :to="{name : 'Entity', params: {tablename: 'oauth_connect'}}">
            <i class="fa fa-link"></i> Connections
          </router-link>
        </li>
        <li class="pageLink">
          <router-link class="list-external-storage"
                       :to="{name : 'Entity', params: {tablename: 'cloud_store'}}">
            <i class="fa fa-folder"></i> External storage
          </router-link>
        </li>
        <li class="pageLink">
          <router-link class="list-site"
                       :to="{name : 'Entity', params: {tablename: 'site'}}">
            <i class="fa fa-sitemap"></i> Sub sites
          </router-link>
        </li>
        <li class="pageLink">
          <router-link class="download-schema"
                       :to="{name : 'Action', params: {tablename: 'world', actionname: 'download_system_schema'}}">
            <i class="fa fa-download"></i> Download System Schema
          </router-link>
        </li>

        <li class="pageLink">
          <router-link class="become-admin-button"
                       :to="{name : 'Action', params: {tablename: 'world', actionname: 'invoke_become_admin'}}">
            <i class="fa fa-graduation-cap"></i> Become Admin
          </router-link>
        </li>


        <li><a href="https://github.com/artpar/goms/wiki" target="_blank"><span class="fa fa-files-o"></span>
          Dev help</a></li>


        <li><a href="https://github.com/artpar/goms/issues/new" target="_blank"><span class="fa fa-cogs"></span>
          File an issue/bug</a></li>


        <li><a href="mailto:artpar@gmail.com?subject=GoMS&body=Hi Parth,\n"><span class="fa fa-envelope-o"></span>
          Email support</a></li>

      </ul>
    </li>

  </ul>
</template>
<script>
  import {mapState} from "vuex"

  export default {
    name: 'SidebarName',
    methods: {
      toggleMenu(event) {
        // remove active from li
        var active = document.querySelector('li.pageLink.active')

        if (active) {
          active.classList.remove('active')
        }
        // window.$('li.pageLink.active').removeClass('active')
        // Add it to the item that was clicked
        event.toElement.parentElement.className = 'pageLink active'
      }
    },
    props: {
      filter: {
        type: String,
        required: true,
        default: ''
      }
    },
    computed: {
      ...mapState([
        'worlds'
      ])
    },
    data: function () {
      return {
        topWorlds: [],
      }
    },
    mounted() {
      var that = this;
      console.log("sidebarmenu visible worlds: ", this.topWorlds)

      that.topWorlds = that.worlds.filter(function (w, r) {
        return w.is_top_level == '1' && w.is_hidden == '0';
      });

      setTimeout(function () {
        $(window).resize()
        console.log("this sidebar again", that.topWorlds)
      }, 300);
    },
    watch: {
      'worlds': function () {
        console.log("got worlds")
        that.topWorlds = that.worlds.filter(function (w, r) {
          return w.is_top_level == '1' && w.is_hidden == '0';
        });
      }
    }
  }
</script>
<style>
  /* override default */
  .sidebar-menu > li > a {
    padding: 12px 15px 12px 15px;
  }

  .sidebar-menu li.active > a > .fa-angle-left, .sidebar-menu li.active > a > .pull-right-container > .fa-angle-left {
    animation-name: rotate;
    animation-duration: .2s;
    animation-fill-mode: forwards;
  }

  @keyframes rotate {
    0% {
      transform: rotate(0deg);
    }

    100% {
      transform: rotate(-90deg);
    }
  }
</style>
