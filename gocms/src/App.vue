<template>
  <div id="app" class="ui wide">


    <div class="ui segment">
      <el-button @click="login()" v-show="!authenticated">Login</el-button>
      <el-button @click="logout()" v-show="authenticated">Logout</el-button>
    </div>
    <div class="ui column">
      <router-view></router-view>
    </div>

    <link href="./static/bower_components/font-awesome/css/font-awesome.min.css" rel="stylesheet">
    <link href="./static/bower_components/semantic/dist/semantic.css" rel="stylesheet">
    <link href="./static/bower_components/jsoneditor/dist/jsoneditor.min.css" rel="stylesheet">

    <script src="./static/bower_components/semantic/dist/semantic.js" type="application/javascript"></script>
    <script src="./static/bower_components/semantic/dist/components/dropdown.min.js"
            type="application/javascript"></script>
    <script src="./static/bower_components/jquery/dist/jquery.js" type="application/javascript"></script>
    <script src="./static/bower_components/jsoneditor/dist/jsoneditor.min.js" type="application/javascript"></script>

  </div>
</template>

<script>
  export default {
    name: 'app',
    data: function () {

      return {
        authenticated: true,
        secretThing: '',
        lock: lock,
      }
    },
    mounted() {
      var self = this;
//            console.log("Auth0Lock 11", Auth0Lock)

    },
    methods: {
      init() {

      },
      login() {
        window.lock.show();
      },
      logout() {
        console.log("logout called")
        // To log out, we just need to remove the token and profile
        // from local storage
        localStorage.removeItem('id_token');
        localStorage.removeItem('profile');
        this.authenticated = false;
      },
    }
  }
</script>

<style>
  #app {
    font-family: 'Avenir', Helvetica, Arial, sans-serif;
    -webkit-font-smoothing: antialiased;
    color: #2c3e50;
    margin: 10px;
    padding: 5px;
  }
</style>
