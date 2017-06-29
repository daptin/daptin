<template>
  <ul class="sidebar-menu">
    <li class="header">Items </li>

    <li class="pageLink" v-on:click="toggleMenu" v-for="w in topWorlds">
      <router-link :class="w.table_name + '-link'" :to="{name: 'Entity', params: {tablename: w.table_name}}">
        <span class="page">{{w.table_name | titleCase}}</span>
      </router-link>
    </li>


    <li class="header">Connections</li>
    <li class="pageLink">
      <router-link class="list-connections"
                   :to="{name : 'Entity', params: {tablename: 'oauth_connect'}}">
        <i class="fa fa-link"></i> Connections
      </router-link>
    </li>

    <li class="pageLink">
      <router-link class="oauth-tokens"
                   :to="{name : 'Entity', params: {tablename: 'oauth_token'}}">
        <i class="fa fa-tags"></i> Oauth Tokens
      </router-link>
    </li>

    <li class="pageLink">
      <router-link class="data-exchanges"
                   :to="{name : 'Entity', params: {tablename: 'data_exchange'}}">
        <i class="fa fa-exchange"></i> Data Exchanges
      </router-link>
    </li>


    <li class="header">System</li>
    <li class="pageLink" v-on:click="toggleMenu">
      <router-link to="/in/world">
        <i class="fa fa-th"></i>
        <span class="page">All tables</span>
      </router-link>
    </li>

    <li class="treeview system-action-list">
      <a href="#">
        <i class="fa fa-folder-o"></i>
        <span>System Actions</span>
        <span class="pull-right-container">
          <i class="fa fa-angle-left fa-fw pull-right"></i>
        </span>
      </a>
      <ul class="treeview-menu">
        <li>
          <router-link class="upload-schema"
                       :to="{name : 'Action', params: {tablename: 'world', actionname: 'upload_system_schema'}}">
            <i class="fa fa-upload"></i> Update Features using JSON
          </router-link>
        </li>
        <li>
          <router-link class="download-schema"
                       :to="{name : 'Action', params: {tablename: 'world', actionname: 'download_system_schema'}}">
            <i class="fa fa-download"></i> Download System Schema
          </router-link>
        </li>
        <li>
          <router-link class="become-admin-button"
                       :to="{name : 'Action', params: {tablename: 'world', actionname: 'invoke_become_admin'}}">
            <i class="fa fa-graduation-cap"></i> Become Admin
          </router-link>
        </li>
      </ul>
    </li>

    <li class="treeview help-support">
      <a href="#">
        <i class="fa fa-question"></i>
        <span>Help and Support</span>
        <span class="pull-right-container">
          <i class="fa fa-angle-left fa-fw pull-right"></i>
        </span>
      </a>
      <ul class="treeview-menu">
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
      toggleMenu (event) {
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

        setTimeout(function () {
          $(window).resize()
        }, 300);
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
