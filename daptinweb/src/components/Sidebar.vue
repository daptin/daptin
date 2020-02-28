<template>
  <aside class="main-sidebar">

    <!-- sidebar: style can be found in sidebar.less -->
    <section class="sidebar">
      <!-- Sidebar user panel (optional) -->
      <div class="user-panel" v-if="user">
        <div class="pull-left image">
          <img :src="user.picture"/>
        </div>
        <div class="pull-left info">
          <div>
            <p class="black">{{ user.name }}</p>
          </div>
        </div>
      </div> <!-- Sidebar user panel (optional) -->
      <div class="user-panel" v-if="!user">
        <div class="pull-left">
          <a href="/auth/signin">
            <i class="fas fa-circle text-success"></i> Login
          </a>
        </div>
      </div>

      <!-- Sidebar Menu -->
      <sidebar-menu :filter="filter"/>
      <!-- /.sidebar-menu -->
    </section>
    <!-- /.sidebar -->
  </aside>
</template>
<script>
  import SidebarMenu from './SidebarMenu'

  export default {
    name: 'Sidebar',
    props: ['user'],
    data: function() {
      return  {
        filter: '',
      }
    },
    components: {SidebarMenu}
  }
</script>
<style>
  .user-panel .image img {
    border-radius: 50%;
  }
</style>
