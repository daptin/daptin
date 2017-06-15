<template>
  <ul class="sidebar-menu">
    <li class="header">Items </li>

    <li class="pageLink" v-on:click="toggleMenu" v-for="w in topWorlds">
      <router-link :class="w.table_name + '-link'" :to="{name: 'Entity', params: {tablename: w.table_name}}">
        <span class="page">{{w.table_name | titleCase}}</span>
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


    <!--<li class="pageLink" v-on:click="toggleMenu">-->
    <!--<router-link to="/setting">-->
    <!--<i class="fa fa-cog"></i>-->
    <!--<span class="page">Settings</span>-->
    <!--</router-link>-->
    <!--</li>-->

  </ul>
</template>
<script>
  import {mapGetters} from "vuex"
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
      ...mapGetters([
        'topWorlds'
      ])
    },
    mounted() {
      console.log("sidebarmenu visible worlds: ", this.topWorlds)
      setTimeout(function () {
        $(window).resize()
      }, 300);
    },
    watch: {
      'topWorlds': function () {
        console.log("got worlds")
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
