<template>
  <div id="app">
    <div class="row">

      <div class="col-md-12">
        <el-button @click="login()" v-show="!authenticated">Login</el-button>
        <el-button @click="logout()" v-show="authenticated">Logout</el-button>
      </div>
    </div>
    <router-view></router-view>

    <link href="./static/bower_components/bootstrap/css/bootstrap.min.css" rel="stylesheet">
    <link href="./static/bower_components/bootstrap/css/bootstrap-theme.min.css" rel="stylesheet">
    <link href="./static/bower_components/font-awesome/css/font-awesome.min.css" rel="stylesheet">
    <!--<link href="./static/bower_components/elementui/element.css" rel="stylesheet">-->

    <script src="./static/bower_components/jquery/jquery-2.1.4.min.js" type="application/javascript"></script>
    <!--<script src="./static/bower_components/elementui/element.js" type="application/javascript"></script>-->
    <script src="./static/bower_components/bootstrap/js/bootstrap.min.js" type="application/javascript"></script>

  </div>
</template>

<script>
    export default {
        name: 'app',
        data: function () {
            return {
                authenticated: false,
                secretThing: '',
                lock: new Auth0Lock('edsjFX3nR9fqqpUi4kRXkaKJefzfRaf_', 'gocms.auth0.com', {
                    auth: {
                        redirectUrl: 'http://localhost:8080/#/',
                        responseType: 'token',
                        params: {
                            scope: 'openid email' // Learn about scopes: https://auth0.com/docs/scopes
                        }
                    }
                }),
            }
        },
        mounted() {
            var self = this;

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
