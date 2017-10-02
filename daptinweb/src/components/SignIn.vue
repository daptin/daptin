<template>
  <div class="container">
    <div class="row vertical-10p">
      <div class="container">
        <div class="register-logo">
          <a href="javascript:;"><b>Goms</b></a>
        </div>
        <div class="col-md-4 col-sm-offset-4">
          <!-- login form -->
          <action-view :model="{}" :hide-cancel="true" v-if="signInAction" :actionManager="actionManager" :action="signInAction"></action-view>

          <!-- errors -->
          <div v-if=response class="text-red"><p>{{response}}</p></div>
        </div>
        <div class="col-md-4 col-sm-offset-4">
          <div class="box">
            <div class="box-body">
              <router-link class="btn bg-blue" :to="{name: 'SignUp'}">Sign Up</router-link>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
  import configManager from '../plugins/configmanager'
  import actionManager from "../plugins/actionmanager"

  export default {

    data() {
      return {
        response: null,
        signInAction: null,
        actionManager: actionManager,
      }
    },
    methods: {
      init() {
        var that = this;
        console.log("sign in loaded");
        actionManager.getGuestActions().then(function (guestActions) {
          console.log("guest actions", guestActions, guestActions["user:signin"]);
          that.signInAction = guestActions["user:signin"];
        })
      },
    },
//    updated () {
//      this.init();
//    },
    mounted () {
      this.init();
    }
  }
</script>
