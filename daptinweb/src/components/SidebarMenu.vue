<template>
  <ul class="sidebar-menu">
    <li class="pageLink" v-on:click="toggleMenu">
      <router-link :to="{name: 'Dashboard', params: {}}">
        <i class="fas fa-tv"></i>
        <span class="page">Dashboard</span>
      </router-link>
    </li>

    <li class="treeview">
      <a href="#">
        <i class="fa fa-users"></i>
        <span>People</span>
        <span class="pull-right-container">
          <i class="fa fa-angle-left fa-fw pull-right"></i>
        </span>
      </a>
      <ul class="treeview-menu">
        <li class="pageLink" v-on:click="toggleMenu">
          <router-link :class="'user-link'" :to="{name: 'Entity', params: {tablename: 'user_account'}}">
            <i class="fa fa-user"></i>
            <span class="page">User account</span>
          </router-link>
        </li>
        <li class="pageLink" v-on:click="toggleMenu">
          <router-link :class="'user-link'" :to="{name: 'Entity', params: {tablename: 'usergroup'}}">
            <i class="fa fa-users"></i>
            <span class="page">User Group</span>
          </router-link>
        </li>
      </ul>
    </li>


    <li class="treeview data-model">
      <a href="#">
        <i class="fas fa-keyboard"></i>
        <span>Data Model</span>
        <span class="pull-right-container">
          <i class="fas fa-angle-left fa-fw pull-right"></i>
        </span>
      </a>
      <ul class="treeview-menu">
        <li class="pageLink" v-on:click="toggleMenu">
          <router-link :to="{name : 'Action', params: {tablename: 'world', actionname: 'upload_system_schema'}}"
                       class="">
            <i class="fas fa-plus "></i> Upload Schema JSON
          </router-link>


          <router-link
            :to="{name : 'Action', params: {tablename: 'world', actionname: 'upload_xls_to_system_schema'}}"
            class="">
            <i class="fas fa-file-excel"></i> Upload XLSX
          </router-link>

          <router-link
            :to="{name : 'Action', params: {tablename: 'world', actionname: 'upload_csv_to_system_schema'}}"
            class="">
            <i class="fas fa-file-alt"></i> Upload CSV
          </router-link>

          <router-link :to="{name : 'Action', params: {tablename: 'world', actionname: 'import_data'}}"
                       class="">
            <i class="fab fa-js"></i> Upload Data JSON
          </router-link>

          <router-link :to="{name : 'NewItem'}"
                       class="">
            <i class="fas fa-pencil-alt"></i> Online designer
          </router-link>
        </li>
      </ul>
    </li>

    <li class="treeview administration">
      <a href="#">
        <i class="fas fa-keyboard"></i>
        <span>Administration</span>
        <span class="pull-right-container">
          <i class="fas fa-angle-left fa-fw pull-right"></i>
        </span>
      </a>
      <ul class="treeview-menu">

        <li class="pageLink" v-on:click="toggleMenu">
          <router-link :to="{name: 'Entity', params: {tablename: 'world'}}">
            <i class="fa fa-th"></i>
            <span class="page">All tables</span>
          </router-link>
        </li>

      </ul>
    </li>

    <li class="treeview help-support">
      <a href="#">
        <i class="fa fa-comment"></i>
        <span>Support</span>
        <span class="pull-right-container">
          <i class="fa fa-angle-left fa-fw pull-right"></i>
        </span>
      </a>
      <ul class="treeview-menu">

        <li><a href="https://docs.dapt.in" target="_blank"><span class="fa fa-files-o"></span>
          Documentation</a></li>


        <li><a href="https://github.com/artpar/daptin/issues/new" target="_blank"><span class="fa fa-cogs"></span>
          File an issue/bug</a></li>


        <li><a href="mailto:artpar@gmail.com?subject=Daptin&body=Hi Parth,\n"><span class="fa fa-envelope-o"></span>
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
        var active = document.querySelector('li.pageLink.active');

        if (active) {
          active.classList.remove('active')
        }
        // window.$('li.pageLink.active').removeClass('active')
        // Add it to the item that was clicked
        // event.toElement.parentElement.className = 'pageLink active'
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
      let that = this;
      console.log("sidebarmenu visible worlds: ", this.topWorlds);

      that.topWorlds = this.worlds.filter(function (w, r) {
        return w.is_top_level && !w.is_hidden;
      });

      setTimeout(function () {
        $(window).resize();
        console.log("this sidebar again", that.topWorlds)
      }, 300);
    },
    watch: {
      'worlds': function () {
        console.log("got worlds");
        var that = this;
        that.topWorlds = that.worlds.filter(function (w, r) {
          return w.is_top_level && !w.is_hidden;
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
