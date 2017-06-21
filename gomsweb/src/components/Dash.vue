<template>
  <div :class="['wrapper', classes]">
    <header class="main-header">
	<span class="logo-mini">
		<a href="/"><img src="/static/img/copilot-logo-white.svg" alt="Logo" class="img-responsive center-block logo"></a>
	</span>


      <!-- Header Navbar -->
      <nav class="navbar navbar-static-top" role="navigation">
        <!-- Sidebar toggle button-->
        <a href="javascript:;" class="sidebar-toggle" data-toggle="offcanvas" role="button">
          <span class="sr-only">Toggle navigation</span>
        </a>


        <div class="navbar-custom-menu">
          <ul class="nav navbar-nav">

            <li><a href="https://github.com/artpar/goms/wiki" target="_blank"><span class="fa fa-files-o"></span>
              Dev help</a></li>


            <li><a href="https://github.com/artpar/goms/issues/new" target="_blank"><span class="fa fa-cogs"></span>
              File an issue/bug</a></li>


            <li><a href="mailto:artpar@gmail.com?subject=GoMS&body=Hi Parth,\n"><span class="fa fa-envelope-o"></span>
              Email support</a></li>

            <!-- Notifications: style can be found in dropdown.less -->
            <li class="dropdown notifications-menu">
              <a href="#" class="dropdown-toggle" data-toggle="dropdown">
                <i class="fa fa-film"></i>
                <span class="label label-warning">7</span> Tours
              </a>
              <ul class="dropdown-menu">
                <li class="header">You have 7 tours available</li>
                <li>
                  <!-- inner menu: contains the actual data -->
                  <ul class="menu">
                    <li @click="startTour(1)">
                      <router-link :to="{name: 'Dashboard'}">
                        <i class="fa fa-th-list teal"></i> #1 Sidebar and actions
                      </router-link>
                    </li>
                    <li @click="startTour(2)">
                      <router-link :to="{name: 'Entity', params: {tablename: 'user'}}">
                        <i class="fa fa-users blue"></i> #2 Users and table view
                      </router-link>
                    </li>
                    <li @click="startTour(3)">
                      <router-link :to="{name: 'Dashboard'}">
                        <i class="fa fa-graduation-cap green"></i> #3 Become admin
                      </router-link>
                    </li>
                    <li @click="startTour(4)">
                      <router-link :to="{name: 'Dashboard'}">
                        <i class="fa fa-cubes orange"></i> #4 Add features using JSON
                      </router-link>
                    </li>
                    <li @click="startTour(5)">
                      <router-link :to="{name: 'Dashboard'}">
                        <i class="fa fa-star fuchsia"></i> #5 What's new after the JSON feature upload ?
                      </router-link>
                    </li>
                    <li @click="startTour(6)">
                      <router-link :to="{name: 'Dashboard'}">
                        <i class="fa fa-cubes grey"></i> #6 Actions and chains
                      </router-link>
                    </li>
                    <li @click="startTour(7)">
                      <router-link :to="{name: 'Dashboard'}">
                        <i class="fa fa-road maroon"></i> #7 What now ?
                      </router-link>
                    </li>
                  </ul>
                </li>
                <li class="footer"><a href="#">View all</a></li>
              </ul>
            </li>

            <!-- Messages-->
            <!--<li class="dropdown messages-menu">-->
            <!--<a href="javascript:;" class="dropdown-toggle" data-toggle="dropdown">-->
            <!--<i class="fa fa-envelope-o"></i>-->
            <!--<span class="label label-success">{{ userInfo.messages | count }}</span>-->
            <!--</a>-->
            <!--<ul class="dropdown-menu">-->
            <!--<li class="header">You have {{ userInfo.messages | count }} message(s)</li>-->
            <!--<li v-if="userInfo.messages.length > 0">-->
            <!--&lt;!&ndash; inner menu: contains the messages &ndash;&gt;-->
            <!--<ul class="menu">-->
            <!--<li>-->
            <!--&lt;!&ndash; start message &ndash;&gt;-->
            <!--<a href="javascript:;">-->
            <!--&lt;!&ndash; Message title and timestamp &ndash;&gt;-->
            <!--<h4>-->
            <!--Support Team-->
            <!--<small>-->
            <!--<i class="fa fa-clock-o"></i> 5 mins-->
            <!--</small>-->
            <!--</h4>-->
            <!--&lt;!&ndash; The message &ndash;&gt;-->
            <!--<p>Why not consider this a test message?</p>-->
            <!--</a>-->
            <!--</li>-->
            <!--&lt;!&ndash; end message &ndash;&gt;-->
            <!--</ul>-->
            <!--&lt;!&ndash; /.menu &ndash;&gt;-->
            <!--</li>-->
            <!--<li class="footer" v-if="userInfo.messages.length > 0">-->
            <!--<a href="javascript:;">See All Messages</a>-->
            <!--</li>-->
            <!--</ul>-->
            <!--</li>-->
            <!-- /.messages-menu -->

            <!-- Notifications Menu -->
            <!--<li class="dropdown notifications-menu">-->
            <!--<a href="javascript:;" class="dropdown-toggle" data-toggle="dropdown">-->
            <!--<i class="fa fa-bell-o"></i>-->
            <!--<span class="label label-warning">{{ userInfo.notifications | count }}</span>-->
            <!--</a>-->
            <!--<ul class="dropdown-menu">-->
            <!--<li class="header">You have {{ userInfo.notifications | count }} notification(s)</li>-->
            <!--<li v-if="userInfo.notifications.length > 0">-->
            <!--&lt;!&ndash; Inner Menu: contains the notifications &ndash;&gt;-->
            <!--<ul class="menu">-->
            <!--<li>-->
            <!--&lt;!&ndash; start notification &ndash;&gt;-->
            <!--<a href="javascript:;">-->
            <!--<i class="fa fa-users text-aqua"></i> 5 new members joined today-->
            <!--</a>-->
            <!--</li>-->
            <!--&lt;!&ndash; end notification &ndash;&gt;-->
            <!--</ul>-->
            <!--</li>-->
            <!--<li class="footer" v-if="userInfo.notifications.length > 0">-->
            <!--<a href="javascript:;">View all</a>-->
            <!--</li>-->
            <!--</ul>-->
            <!--</li>-->

            <!-- Tasks Menu -->
            <!--<li class="dropdown tasks-menu">-->
            <!--<a href="javascript:;" class="dropdown-toggle" data-toggle="dropdown">-->
            <!--<i class="fa fa-flag-o"></i>-->
            <!--<span class="label label-danger">{{ userInfo.tasks | count }} </span>-->
            <!--</a>-->
            <!--<ul class="dropdown-menu">-->
            <!--<li class="header">You have {{ userInfo.tasks | count }} task(s)</li>-->
            <!--<li v-if="userInfo.tasks.length > 0">-->
            <!--&lt;!&ndash; Inner menu: contains the tasks &ndash;&gt;-->
            <!--<ul class="menu">-->
            <!--<li>-->
            <!--&lt;!&ndash; Task item &ndash;&gt;-->
            <!--<a href="javascript:;">-->
            <!--&lt;!&ndash; Task title and progress text &ndash;&gt;-->
            <!--<h3>-->
            <!--Design some buttons-->
            <!--<small class="pull-right">20%</small>-->
            <!--</h3>-->
            <!--&lt;!&ndash; The progress bar &ndash;&gt;-->
            <!--<div class="progress xs">-->
            <!--&lt;!&ndash; Change the css width attribute to simulate progress &ndash;&gt;-->
            <!--<div class="progress-bar progress-bar-aqua" style="width: 20%" role="progressbar"-->
            <!--aria-valuenow="20" aria-valuemin="0" aria-valuemax="100">-->
            <!--<span class="sr-only">20% Complete</span>-->
            <!--</div>-->
            <!--</div>-->
            <!--</a>-->
            <!--</li>-->
            <!--&lt;!&ndash; end task item &ndash;&gt;-->
            <!--</ul>-->
            <!--</li>-->
            <!--<li class="footer" v-if="userInfo.tasks.length > 0">-->
            <!--<a href="javascript:;">View all tasks</a>-->
            <!--</li>-->
            <!--</ul>-->
            <!--</li>-->

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
                <!-- Menu Body -->
                <!--<li class="user-body">-->
                <!--<div class="row">-->
                <!--<div class="col-xs-4 text-center">-->
                <!--<a href="#">Followers</a>-->
                <!--</div>-->
                <!--<div class="col-xs-4 text-center">-->
                <!--<a href="#">Sales</a>-->
                <!--</div>-->
                <!--<div class="col-xs-4 text-center">-->
                <!--<a href="#">Friends</a>-->
                <!--</div>-->
                <!--</div>-->
                <!--&lt;!&ndash; /.row &ndash;&gt;-->
                <!--</li>-->
                <!-- Menu Footer-->
                <li class="user-footer">
                  <!--<div class="pull-left">-->
                  <!--<a href="#" class="btn btn-default btn-flat">Profile</a>-->
                  <!--</div>-->
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
      </nav>
    </header>
    <!-- Left side column. contains the logo and sidebar -->
    <sidebar :user="user"/>

    <!-- Content Wrapper. Contains page content -->
    <div class="content-wrapper">
      <!-- Content Header (Page header) -->
      <section class="content-header">
        <!--<h1>-->

        <!--{{$route.name.toUpperCase() }}-->

        <!--<small>{{ $route.meta.description }}</small>-->
        <!--</h1>-->
        <!--<ol class="breadcrumb">-->
        <!--<li>-->
        <!--<a href="javascript:;">-->
        <!--<i class="fa fa-home"></i>Home</a>-->
        <!--</li>-->
        <!--<li class="active">{{$route.name.toUpperCase()}}</li>-->
        <!--</ol>-->
      </section>

      <router-view></router-view>
    </div>
    <!-- /.content-wrapper -->

    <!-- Main Footer -->
    <footer class="main-footer">
      <strong><a href="javascript:;">GoMS</a>.</strong> All rights reserved.
    </footer>
  </div>
  <!-- ./wrapper -->
</template>

<script>
  import faker from 'faker'
  import {mapState} from 'vuex'
  import config from '../config'
  import Sidebar from './Sidebar'
  import 'hideseek'

  import {getToken} from '../utils/auth'
  import {logout} from '../utils/lock'
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
        'user'
      ]),
      demo () {
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
      if (!this.isAuthenticated) {
        const {token, secret} = extractInfoFromHash();

        if (!checkSecret(secret) || !token) {
          console.info('Something happened with the Sign In request');
//          that.$router.go({name: "sigini"})
          this.$router.push("/auth/signin");
          return
        } else {
          console.log("got token from url", token)
          setToken(token);
          window.location.hash = "";
          window.location.reload();
          return;
        }
      }
    },
    methods: {
      startTour(tourId) {

        if (Shepherd.activeTour) {
          Shepherd.activeTour.hide();
        }


        var tour;

        tour = new Shepherd.Tour({
          defaults: {
            classes: 'shepherd-theme-dark',
            scrollTo: true
          }
        });


        if (tourId == 1) {


          tour.addStep('hello', {
            text: 'Hi. Let us take a quick view of all the things on this page',
            buttons: [
              {
                text: 'Next',
                action: tour.next
              }
            ]
          });


          tour.addStep('sidebar', {
            text: 'This sidebar will help us to go to different part of the system.',
            attachTo: '.sidebar-menu right',
            buttons: [
              {
                text: 'Back',
                action: tour.back
              },
              {
                text: 'Next',
                action: tour.next
              }
            ]
          });


          tour.addStep('sidebar', {
            text: 'This also has links to some functional actions which we are going to use soon. But before that, lets checkout the User and Usergroup links there.',
            attachTo: '.treeview right',
            buttons: [
              {
                text: 'Back',
                action: tour.back
              },
              {
                text: 'Next',
                action: tour.next
              }
            ]
          });


          tour.addStep('sidebar', {
            text: 'Clicking on the "User" link will take us to the users page. This is the end of this tour. You can start the next tour.',
            attachTo: '.user-link right',
            advanceOn: '.user-link click',
            buttons: [
              {
                text: 'Back',
                action: tour.back
              },
            ]
          });

        }

        if (tourId == 2) {


          tour.addStep('sidebar', {
            text: 'This is the list of users which you currently have access to. I will explain the concept of authorization much later in another tour.',
            attachTo: '.vuetable top',
            buttons: [
              {
                text: 'Next',
                action: tour.next
              }
            ]
          });


          tour.addStep('sidebar', {
            text: 'The expand button takes us to particular item, which shows all its related actions and items.',
            attachTo: '.fa-expand top',
            buttons: [
              {
                text: 'Back',
                action: tour.back
              },
              {
                text: 'Next',
                action: tour.next
              }
            ]
          });

          tour.addStep('sidebar', {
            text: 'The eye button shows details of the item here itself. Go ahead and click it. Click it again to close the detailed view.',
            attachTo: '.fa-eye top',
            buttons: [
              {
                text: 'Back',
                action: tour.back
              },
              {
                text: 'Next',
                action: tour.next
              }
            ]
          });


          tour.addStep('sidebar', {
            text: 'You can use the "edit" button to edit the item (in this case, user), but this rarely how you will be interacting with the system. Most of your work flow interactions will happen via "Actions" which we will go through later.',
            attachTo: '.fa-pencil-square top',
            buttons: [
              {
                text: 'Back',
                action: tour.back
              },
              {
                text: 'Next',
                action: tour.next
              }
            ]
          });

          tour.addStep('sidebar', {
            text: 'The cross of course deletes the item.',
            attachTo: '.fa-times top',
            buttons: [
              {
                text: 'Back',
                action: tour.back
              },
              {
                text: 'Next',
                action: tour.next
              }
            ]
          });

          tour.addStep('sidebar', {
            text: 'You can add a new item by clicking here. The orange button will reload the data from the database.',
            attachTo: '.fa-plus left',
            buttons: [
              {
                text: 'Back',
                action: tour.back
              },
              {
                text: 'Next',
                action: tour.next
              }
            ]
          });

          tour.addStep('sidebar', {
            text: 'These two grey buttons don\'t do anything for now, but they will give access to a different kind of view (card layout/other layouts)',
            attachTo: '.fa-table left',
            buttons: [
              {
                text: 'Back',
                action: tour.back
              },
              {
                text: 'Next',
                action: tour.next
              }
            ]
          });

          tour.addStep('sidebar', {
            text: 'That was all about this page. If this is a fresh installation of GoMS, there would probably be only "User" and "Usergroup" available in the sidebar, because these two form the basis for everything else. In another tour we will see how to begin customising GoMS for your needs.',
            buttons: [
              {
                text: 'Back',
                action: tour.back
              },
              {
                text: 'Next',
                action: tour.next
              }
            ]
          });


        }
        if (tourId == 3) {

          tour.addStep('sidebar', {
            text: 'Let us visit the actions again, first we need to take ownership of this GoMS instance by becoming admin. This can only be done when there is exactly one user in the system. Until someone takes ownership of GoMS, GoMS is open to everyone.',
            attachTo: '.system-action-list right',
            advanceOn: '.system-action-list click',
            buttons: [
              {
                text: 'Close',
                action: tour.hide
              }
            ]
          });

          tour.addStep('sidebar', {
            text: 'The first thing you would probably do with any GoMS installation is to Become Administrator.  You will see a quick reload of your page. <br><br>Click "Become Admin" to take ownership. ',
            attachTo: '.become-admin-button right',
            buttons: [
              {
                text: 'Back',
                action: tour.back
              }
            ]
          });

        }

        if (tourId == 4) {


          tour.addStep('sidebar', {
            text: 'The main purpose of GoMS is to get modified to suit your needs. You can add "New Features" to GoMS using JSON files, which will act like plugins in near future. <br><br>Let us <a class="download-json btn btn-success" href="https://raw.githubusercontent.com/artpar/goms/master/gomsweb/static/samples/blog.json" target="_blank">Download a sample JSON file</a> that I have created for playing around, based on what a "basic blogging system" would look like.',
            advanceOn: ".download-json click",
            buttons: []
          });


          tour.addStep('sidebar', {
            text: 'We will now "Add New Features" by uploading the JSON, which will take you through another refresh and you will be able to see your new entities in this Sidebar.',
            attachTo: '.system-action-list right',
            advanceOn: '.upload-schema click',
            buttons: [
              {
                text: 'Next',
                action: tour.next
              }
            ]
          });


        }

        if (tourId == 5) {


          tour.addStep('sidebar', {
            text: 'Welcome back after uploading the JSON. If you uploaded the [blog.json] from the earlier tour, you will see a new sidebar entry "Blog"',
            advanceOn: ".download-json click",
            buttons: [
              {
                text: 'Next',
                action: tour.next
              }
            ]
          });


          tour.addStep('sidebar', {
            text: 'Clicking on this will take us to the table view of blogs in the system. This will be the same view we visited in the earlier tour of users.<br> <br>Click on "Blog" to continue',
            advanceOn: ".blog-link click",
            attachTo: ".blog-link right",
            buttons: []
          });


          tour.addStep('sidebar', {
            text: 'The table is empty, as it should be, because we just added a "Blog" feature to GoMS, but have not used it yet.',
            attachTo: ".vuetable top",
            buttons: [
              {
                text: 'Next',
                action: tour.next
              }
            ]
          });

          tour.addStep('sidebar', {
            text: 'The table is empty, as it should be, because we just added a "Blog" feature to GoMS, but have not used it yet.',
            attachTo: ".vuetable top",
            buttons: [
              {
                text: 'Next',
                action: tour.next
              }
            ]
          });


          tour.addStep('sidebar', {
            text: 'Just to be clear and avoid confusion at a later stage, here is a description of what was in the JSON schema <br><br> <ul><li>Defines "blog" as a collection of "post", a  "viewcount" and a title. </li> <li>Each "post" has a body and a title and may have any number of "comment".</li><li>Comment has a field for "author name" and another for "comment text" itself </li></ul>',
            buttons: [
              {
                text: 'Next',
                action: tour.next
              }
            ]
          });


          tour.addStep('sidebar', {
            text: 'Now lets create a new "blog"',
            attachTo: ".fa-plus bottom",
            advanceOn: ".fa-plus click",
            buttons: []
          });

          tour.addStep('sidebar', {
            text: 'You can fill in 755 for the permission field and ignore it for now. We will go over authentication and authorization in another tour.<br><br>Choose a title, and 0 view count to begin with.<br><br>Submit to continue',
            attachTo: ".vue-form-generator top",
            advanceOn: ".el-button click",
            buttons: [
              {
                text: 'Next',
                action: tour.next
              }
            ]
          });


          tour.addStep('sidebar', {
            text: 'If everything went well, we will see a new entry in the "blog". Which brings us to the end of this long tour. <br><br>You can go in expanded more for this blog and start the next tour.',
            attachTo: ".fa-expand left",
            buttons: [
              {
                text: 'End',
                action: tour.hide
              }
            ]
          });
        }

        if (tourId == 6) {

          tour.addStep('sidebar', {
            text: 'This tour is not ready yet. <br><br>Checkout the next one',
            buttons: [
              {
                text: 'End',
                action: tour.hide
              }
            ]
          });

        }

        if (tourId == 7) {

          tour.addStep('sidebar', {
            text: 'If you plan to use GoMS at this stage (pre-alpha release), please do drop me a line so we can build better.',
            buttons: [
              {
                text: 'Next',
                action: tour.next
              }
            ]
          });

          tour.addStep('sidebar', {
            text: 'You can easily deploy GoMS on any hosting service, or using docker <pre>docker run goms/goms</pre> or run locally. It has no dependency on the internet.',
            buttons: [
              {
                text: 'Next',
                action: tour.next
              }
            ]
          });
          tour.addStep('sidebar', {
            text: 'We are also targeting simple data management services for smaller devices (will begin testing with raspberry pi)',
            buttons: [
              {
                text: 'End',
                action: tour.hide
              }
            ]
          });

        }

        tour.start();

        this.tour = tour;


      },
      changeloading () {
        this.$store.commit('TOGGLE_SEARCHING')
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
      padding-top: 50px;
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
