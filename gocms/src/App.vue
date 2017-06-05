<template>
  <div id="app" class="ui">


    <div class="ui two column grid inverted fixed menu navbar">

      <div class="ui column right floated" style="text-align: right">


        <div class="ui icon button">
          <i class="setting power"
             @click="login()" v-show="!authenticated">
            Login
          </i>
          <i class="setting power"
             @click="logout()" v-show="authenticated">Logout
          </i>
        </div>
      </div>


    </div>
    <router-view v-if="authenticated"></router-view>

    <link href="./static/bower_components/font-awesome/css/font-awesome.min.css" rel="stylesheet">
    <link href="./static/bower_components/semantic/dist/semantic.min.css" rel="stylesheet">
    <script src="./static/bower_components/semantic/dist/semantic.min.js" type="application/javascript"></script>

  </div>
</template>
<style>
  /* Backgrounds */
  .bg-navy {
    background-color: #001F3F;
  }

  .bg-blue {
    background-color: #0074D9;
  }

  .bg-aqua {
    background-color: #7FDBFF;
  }

  .bg-teal {
    background-color: #39CCCC;
  }

  .bg-olive {
    background-color: #3D9970;
  }

  .bg-green {
    background-color: #2ECC40;
  }

  .bg-lime {
    background-color: #01FF70;
  }

  .bg-yellow {
    background-color: #FFDC00;
  }

  .bg-orange {
    background-color: #FF851B;
  }

  .bg-red {
    background-color: #FF4136;
  }

  .bg-fuchsia {
    background-color: #F012BE;
  }

  .bg-purple {
    background-color: #B10DC9;
  }

  .bg-maroon {
    background-color: #85144B;
  }

  .bg-white {
    background-color: #FFFFFF;
  }

  .bg-gray {
    background-color: #AAAAAA;
  }

  .bg-silver {
    background-color: #DDDDDD;
  }

  .bg-black {
    background-color: #111111;
  }

  /* Colors */
  .navy {
    color: #001F3F;
  }

  .blue {
    color: #0074D9;
  }

  .aqua {
    color: #7FDBFF;
  }

  .teal {
    color: #39CCCC;
  }

  .olive {
    color: #3D9970;
  }

  .green {
    color: #2ECC40;
  }

  .lime {
    color: #01FF70;
  }

  .yellow {
    color: #FFDC00;
  }

  .orange {
    color: #FF851B;
  }

  .red {
    color: #FF4136;
  }

  .fuchsia {
    color: #F012BE;
  }

  .purple {
    color: #B10DC9;
  }

  .maroon {
    color: #85144B;
  }

  .white {
    color: #FFFFFF;
  }

  .silver {
    color: #DDDDDD;
  }

  .gray {
    color: #AAAAAA;
  }

  .black {
    color: #111111;
  }

  /* Border colors
     Use with another border utility that sets border-width and style
     i.e .border { border-width: 1px); border-style: solid); }
  */
  .border--navy {
    border-color: #001F3F;
  }

  .border--blue {
    border-color: #0074D9;
  }

  .border--aqua {
    border-color: #7FDBFF;
  }

  .border--teal {
    border-color: #39CCCC;
  }

  .border--olive {
    border-color: #3D9970;
  }

  .border--green {
    border-color: #2ECC40;
  }

  .border--lime {
    border-color: #01FF70;
  }

  .border--yellow {
    border-color: #FFDC00;
  }

  .border--orange {
    border-color: #FF851B;
  }

  .border--red {
    border-color: #FF4136;
  }

  .border--fuchsia {
    border-color: #F012BE;
  }

  .border--purple {
    border-color: #B10DC9;
  }

  .border--maroon {
    border-color: #85144B;
  }

  .border--white {
    border-color: #FFFFFF;
  }

  .border--gray {
    border-color: #AAAAAA;
  }

  .border--silver {
    border-color: #DDDDDD;
  }

  .border--black {
    border-color: #111111;
  }
</style>

<script>
  export default {
    name: 'app',
    data: function () {

      return {
        authenticated: !!localStorage.getItem("id_token"),
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
        if (!this.authenticated) {
          this.login();
        }
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
