<template>
  <div id="app" class="ui container wide">
    <div class="grid ui wide">

      <div class="row">
        <div class="two wide">
          <el-button @click="login()" v-show="!authenticated">Login</el-button>
          <el-button @click="logout()" v-show="authenticated">Logout</el-button>
        </div>
      </div>
      <router-view></router-view>


    </div>
    <link href="./static/bower_components/font-awesome/css/font-awesome.min.css" rel="stylesheet">
    <link href="./static/bower_components/semantic/dist/semantic.css" rel="stylesheet">

    <script src="./static/bower_components/semantic/dist/semantic.js" type="application/javascript"></script>
    <script src="./static/bower_components/jquery/dist/jquery.js" type="application/javascript"></script>

  </div>
</template>

<script>
    export default {
        name: 'app',
        data: function () {


            var lock = {};

            let v1 = typeof Auth0Lock;
            let v2 = typeof v1;
            console.log("type of", v1, v2);

            if (v1 != "undefined") {
                console.log("it is not undefined");
                lock = new Auth0Lock('edsjFX3nR9fqqpUi4kRXkaKJefzfRaf_', 'gocms.auth0.com', {
                    auth: {
                        redirectUrl: 'http://localhost:8080/#/',
                        responseType: 'token',
                        params: {
                            scope: 'openid email' // Learn about scopes: https://auth0.com/docs/scopes
                        }
                    }
                });
            } else {
                lock = {
                    checkAuth: function () {
                        return !localStorage.getItem("id_token");
                    },
                    on: function(vev){
                        console.log("nobody is listening to ", vev);
                    }
                }
            }

            return {
                authenticated: false,
                secretThing: '',
                lock: lock,
            }
        },
        mounted() {
            var self = this;
//            console.log("Auth0Lock 11", Auth0Lock)
            this.authenticated = this.checkAuth();

            this.lock.on('authenticated', (authResult) => {
                console.log('authenticated');
                localStorage.setItem('id_token', authResult.idToken);
                this.lock.getProfile(authResult.idToken, (error, profile) => {
                    if (error) {
                        // Handle error
                        return;
                    }
                    // Set the token and user profile in local storage
                    localStorage.setItem('profile', JSON.stringify(profile));

                    this.authenticated = true;
                    this.$router.push("/")
                });
            });

            this.lock.on('authorization_error', (error) => {
                // handle error when authorizaton fails
            });
        },
        methods: {
            checkAuth() {
                return !!localStorage.getItem('id_token');
            },
            login() {
                this.lock.show();
            },
            logout() {
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
