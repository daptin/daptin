<template>
  <div class="container">
    <div class="row vertical-10p">
      <div class="container">
        <div class="register-logo">
          <a href="javascript:;"><b>Daptin</b></a>
        </div>

        <div class="col-md-4 col-sm-offset-4">
          <!-- login form -->
          <action-view @action-complete="signupComplete" :model="{}" :hide-cancel="true" v-if="signInAction"
                       :actionManager="actionManager"
                       :action="signInAction"></action-view>

          <!-- errors -->
          <div v-if=response class="text-red"><p>{{response}}</p></div>
        </div>
        <div class="col-md-4 col-sm-offset-4">
          <div class="box">
            <div class="box-body">
              <router-link :class="'btn bg-blue'" :to="{name: 'SignIn'}">Sign In</router-link>
              <router-link :class="'btn bg-blue'" :to="{name: 'SignIn2FA'}">Sign In 2FA</router-link>
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

  import {Notification} from "element-ui"

  export default {

    data() {
      return {
        response: null,
        signInAction: null,
        actionManager: actionManager,
        loading: "",
      }
    },
    methods: {
      signupComplete(){
//        Notification({
//          title: "Registration successful",
//          type: 'success',
//          message: "redirecting to sign in page",
//        });
//        this.$router.push({
//          name: "SignIn"
//        })
      },
      init() {
        var that = this;
        console.log("sign in loaded");
        actionManager.getGuestActions().then(function (guestActions) {
          console.log("guest actions", guestActions, guestActions["user:signup"]);
          that.signInAction = guestActions["user:signup"];
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
