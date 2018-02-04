<template>
  <el-menu>
    <el-menu-item index="1" @click="goto({name: 'Dashboard'})">
      <i class="fa fa-map-o"></i>
      <span class="page">Dashboard</span>
    </el-menu-item>

    <el-submenu index="2">
      <template slot="title">
        <i class="fa fa-book"></i>
        <span>Items</span>
      </template>
      <el-menu-item :index="'2-' + index" @click="goto({name: 'Entity', params: {tablename: w.table_name}})"
                    v-for="(w, index) in topWorlds"
                    v-if="w.table_name != 'user' && w.table_name != 'usergroup'">
        <span class="page">{{w.table_name | titleCase}}</span>
      </el-menu-item>
    </el-submenu>

    <el-submenu index="3">
      <template slot="title">
        <i class="fa fa-users"></i>
        <span>People</span>
      </template>
      <el-menu-item index="3-1" @click="goto({name: 'Entity', params: {tablename: 'user'}})">
        <i class="fa fa-user"></i>
        <span class="page">User</span>
      </el-menu-item>
      <el-menu-item index="3-2" @click="goto({name: 'Entity', params: {tablename: 'usergroup'}})">
        <i class="fa fa-users"></i>
        <span class="page">User Group</span>
      </el-menu-item>
    </el-submenu>

    <el-submenu index="4">
      <template slot="title">
        <i class="fa fa-keyboard-o"></i>
        <span>Administration</span>
      </template>

      <el-menu-item index="4-1" @click="goto({name: 'Entity', params: {tablename: 'world'}})">
        <i class="fa fa-th"></i>
        <span class="page">All tables</span>
      </el-menu-item>

    </el-submenu>

    <el-submenu index="5">
      <template slot="title">
        <i class="fa fa-comment"></i>
        <span>Support</span>
      </template>

      <el-menu-item index="5-1">
        <a href="https://github.com/artpar/daptin/wiki" target="_blank">
          <span class="fa fa-files-o"></span>
          Dev help
        </a>
      </el-menu-item>


      <el-menu-item index="5-2">
        <a href="https://github.com/artpar/daptin/issues/new" target="_blank">
          <span class="fa fa-cogs"></span>
          File an issue/bug
        </a>
      </el-menu-item>


      <el-menu-item index="5-3">
        <a href="mailto:artpar@gmail.com?subject=Daptin&body=Hi Parth,\n">
          <span class="fa fa-envelope-o"></span>
          Email support
        </a>
      </el-menu-item>

    </el-submenu>

  </el-menu>
</template>
<script>
  import {mapState} from "vuex"

  export default {
    name: 'SidebarName',
    methods: {
      goto(params) {
        this.$router.push(params);
      },
      toggleMenu(event) {
        // remove active from li
        var active = document.querySelector('li.pageLink.active');

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
