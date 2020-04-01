<template>
  <div :class="['wrapper', classes]">
    <header class="main-header">
	<span class="logo-mini">
		<a href="/"><img src="/static/img/copilot-logo-white.svg" alt="Logo" class="img-responsive center-block logo"></a>
	</span>


      <!-- Header Navbar -->
      <nav class="navbar navbar-static-top" role="navigation">
        <!-- Sidebar toggle button-->
        <a href="javascript:" class="sidebar-toggle fa" data-toggle="offcanvas" role="button">
          <span class="sr-only"> Toggle navigation</span>
        </a>
        <button type="button" class="navbar-toggle collapsed" data-toggle="collapse" data-target="#navbar-collapse-1">
          <span class="sr-only">Toggle navigation</span>
          <span class="icon-bar"></span>
          <span class="icon-bar"></span>
          <span class="icon-bar"></span>
        </button>

        <!-- Collect the nav links, forms, and other content for toggling -->
        <div class="navbar-collapse collapse" id="navbar-collapse-1" style="height: 1px;">
          <div class="col-sm-3 col-md-3">
            <form class="navbar-form" role="search" @submit.prevent="setQueryString">
              <div class="input-group">
                <input style="font-size: 16px; color: white; background-color: #0000005e" id="navbar-search-input"
                       type="text" class="form-control" placeholder="Search" name="q">
                <div class="input-group-btn">
                  <button class="btn btn-default" type="submit"><i class="fa fa-search"></i></button>
                  <button class="btn btn-default" @click.prevent="clearSearch" type="clear"><i class="fa fa-times"></i>
                  </button>
                </div>
              </div>
            </form>
          </div>


          <div class="navbar-custom-menu">"
            <ul class="nav navbar-nav">

              <!-- User Account Menu -->

              <li class="dropdown user user-menu">
                <a href="#" class="dropdown-toggle" data-toggle="dropdown" aria-expanded="false">
                  <img :src="user.picture" class="user-image" alt="User Image">
                  <span class="hidden-xs">{{user.name}}</span>
                </a>
                <ul class="dropdown-menu">
                  <!-- User image -->
                  <li class="user-header">
                    <img :src="user.picture" class="img-circle" alt="User Image">

                    <p>
                      {{user.name}}
                      <small></small>
                    </p>
                  </li>

                  <form class="navbar-form">
                    <div class="input-group">
                      <el-select filterable @change="setPreferredLanguage" v-model="preferredLanguageLocal"
                                 style="font-size: 16px; color: white; background-color: #0000005e"
                                 placeholder="Search" name="q">
                        <el-option v-for="language in languages" :key="language.id" :label="language.label" :value="language.id"
                                   :selected="preferredLanguage === language.id"></el-option>
                      </el-select>
                    </div>
                  </form>


                  <li class="user-footer">

                    <div class="pull-right">
                      <router-link :to="{name: 'SignOut'}" class="btn btn-default btn-flat">Sign out</router-link>
                    </div>
                  </li>
                </ul>
              </li>


              <!--<li class="user user-menu">-->
              <!--<router-link :to="{name: 'SignOut'}" class="dropdown-toggle" data-toggle="dropdown">-->
              <!--<span class="fa fa-2x fa-sign-out red"></span>-->
              <!--</router-link>-->
              <!--</li>-->
            </ul>
          </div>
        </div><!-- /.navbar-collapse -->

        <!--<form class="navbar-form navbar-left" @submit.prevent="setQueryString" role="search">-->
        <!--<div class="form-group">-->
        <!--<input type="text" class="form-control"  id="navbar-search-input" placeholder="Search">-->
        <!--</div>-->
        <!--</form>-->

      </nav>
    </header>
    <!-- Left side column. contains the logo and sidebar -->
    <sidebar :user="user"/>

    <router-view></router-view>
    <!-- /.content-wrapper -->

    <!-- Main Footer -->
    <!--<footer class="main-footer">-->
    <!--<strong><a href="javascript:">Daptin</a>.</strong> All rights reserved.-->
    <!--</footer>-->
  </div>
  <!-- ./wrapper -->
</template>

<script>
    import {mapState} from 'vuex'
    import {mapActions} from 'vuex'
    import config from '../config'
    import Sidebar from './Sidebar'

    import {getToken} from '../utils/auth'
    import {Notification} from 'element-ui';
    import worldManager from "../plugins/worldmanager"
    import {mapGetters} from 'vuex'
    import {setToken, checkSecret, extractInfoFromHash} from '../utils/auth'
    import Shepherd from "tether-shepherd"


    export default {
        name: 'Dash',
        components: {
            Sidebar
        },
        data: function () {
            return {
                query: "",
                preferredLanguageLocal: null,
                // section: 'Dash',
                year: new Date().getFullYear(),
                classes: {
                    fixed_layout: config.fixedLayout,
                    hide_logo: config.hideLogoOnMobile
                },
                error: '',
            }
        },
        computed: {
            ...mapGetters([
                "visibleWorlds",
                'isAuthenticated',
                'user',
                "languages",
                "preferredLanguage"
            ]),
            demo() {
                return {
                    displayName: faker.name.findName(),
                    avatar: faker.image.avatar(),
                    email: faker.internet.email(),
                    tour: null,
                    randomCard: faker.helpers.createCard()
                }
            }
        },
        mounted() {
            var that = this;
            that.preferredLanguageLocal = that.preferredLanguage;
            if (!this.isAuthenticated) {
                const {token, secret} = extractInfoFromHash();

                if (!checkSecret(secret) || !token) {
                    console.info('Something happened with the Sign In request');
//          that.$router.go({name: "sigini"})
                    this.$router.push("/auth/signin");
                } else {
                    console.log("got token from url", token);
                    setToken(token);
                    window.location.hash = "";
                    window.location.reload();
                }
            }

            document.body.className = document.body.className + " sidebar-collapse"
        },
        methods: {
            clearSearch(e) {
                $("#navbar-search-input").val("");
                this.setQueryString(null);
            },
            ...mapActions(["setQuery", "setLanguage"]),
            setQueryString(query) {
                console.log("set query", query);
                this.setQuery(query);
                return false;
            },
            setPreferredLanguage() {
                console.log("set language", this.preferredLanguageLocal);
                this.setLanguage(this.preferredLanguageLocal);
                this.$notify({
                    title: 'Success',
                    message: 'Don\'t forget to refresh after setting a new language',
                    type: 'success'
                });
                return false;
            },
            changeloading() {
                this.$store.commit('TOGGLE_SEARCHING')
            }
        },
        watch: {
            '$route': function () {
                setTimeout(function () {
                    $(window).resize();
                }, 100);
            }
        }
    }
</script>

<style lang="scss">
  .wrapper.fixed_layout {

  .main-header {
    position: fixed;
    width: 100%;
  }

  .content-wrapper {
    /*padding-top: 50px;*/
  }

  .main-sidebar {
    position: fixed;
    height: 100vh;
  }

  }

  .wrapper.hide_logo {

  @media (max-width: 767px) {
    .main-header .logo {
      display: none;
    }
  }

  }

  .logo-mini,
  .logo-lg {
    text-align: left;

  img {
    padding: .4em !important;
  }

  }

  .logo-lg {

  img {
    display: -webkit-inline-box;
    width: 25%;
  }

  }

  .user-panel {
    height: 4em;
  }

  hr.visible-xs-block {
    width: 100%;
    background-color: rgba(0, 0, 0, 0.17);
    height: 1px;
    border-color: transparent;
  }
</style>
