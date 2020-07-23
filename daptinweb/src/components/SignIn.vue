<template>
  <div class="container">
    <div class="row vertical-10p">
      <div class="container">
        <div class="register-logo">
          <a href="javascript:;"><b>Daptin</b></a>
        </div>
        <div class="col-md-4 col-sm-offset-4">
          <!-- login form -->
          <action-view :model="{}" :hide-cancel="true" v-if="signInAction" :actionManager="actionManager"
                       :action="signInAction"></action-view>

          <!-- errors -->
          <div v-if=response class="text-red"><p>{{response}}</p></div>
        </div>
        <div class="col-md-3">
          <div class="row" v-for="connect in oauthConnections">
            <div class="col-md-12">
              <el-button style="margin: 5px;" @click="oauthLogin(connect)">Login via {{ connect | chooseTitle }}</el-button>
            </div>
          </div>
        </div>
        <div class="col-md-4 col-sm-offset-4">
          <div class="box">
            <div class="box-body">
              <router-link :class="'btn bg-blue'" :to="{name: 'SignIn2FA'}">Sign In 2FA</router-link>
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
  import worldManager from "../plugins/worldmanager"
  import jsonApi from "../plugins/jsonapi"

  export default {

    data() {
      return {
        response: null,
        signInAction: null,
        actionManager: actionManager,
        oauthConnections: [],
      }
    },
    methods: {
      oauthLogin(oauthConnect) {

        console.log("action initiate oauth login being for ", oauthConnect);
        actionManager.doAction("oauth_connect", "oauth.login.begin", {
          "oauth_connect_id": oauthConnect.id
        }).then(function (actionResponse) {
          console.log("action response", actionResponse);
        })

      },
      init() {
        const that = this;
        console.log("sign in loaded");


        actionManager.getGuestActions().then(function (guestActions) {
          console.log("guest actions", guestActions, guestActions["user:signin"]);
          that.signInAction = guestActions["user:signin"];
        });

        worldManager.loadModel("oauth_connect", {
          include: ""
        }).then(function () {

          jsonApi.findAll('oauth_connect', {
            page: {number: 1, size: 500},
            query: btoa(JSON.stringify([{
              "column": "allow_login",
              "operator": "is",
              "value": "1"
            }]))
          }).then(function (res) {
            res = res.data;
            console.log("visible oauth connections: ", res);
            that.oauthConnections = res;
          });


        })


      },
    },
//    updated () {
//      this.init();
//    },
    mounted() {
      this.init();
    }
  }
</script>
