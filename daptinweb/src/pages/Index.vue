<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <q-page>

    <div class="q-pa-md q-gutter-sm">
      <q-breadcrumbs>
        <template v-slot:separator>
          <q-icon
            size="1.2em"
            name="arrow_forward"
          />
        </template>

        <q-breadcrumbs-el :label="serverConfig.hostname" icon="fas fa-home"/>
      </q-breadcrumbs>
    </div>
    <q-separator></q-separator>


    <div class="row">


      <div class="col-4 col-md-6 col-lg-4 col-xl-3 col-xs-12 col-sm-12 q-pa-md q-gutter-sm">
        <q-card>


          <q-card-section>
            <q-item>
              <q-item-section avatar>
                <q-avatar>
                  <q-icon size="lg" name="fas fa-user"></q-icon>
                </q-avatar>
              </q-item-section>
              <q-item-section>
                <span class="text-h4" v-if="!showHostnameEdit">Users</span>
                <span class="text-bold" v-if="!showHostnameEdit">@ {{serverConfig.hostname}}</span>
                <q-input @keypress.enter="saveHostname()" v-if="showHostnameEdit" :value="serverConfig.hostname"
                         v-model="serverConfig.hostname"
                         label="Hostname"></q-input>
              </q-item-section>
              <q-item-section avatar>
                <q-icon v-if="!showHostnameEdit" @click="changeHostname()" style="cursor: pointer" name="fas fa-edit"
                        size="xs"></q-icon>
                <q-icon v-if="showHostnameEdit" @click="saveHostname()" style="cursor: pointer" name="fas fa-save"
                        size="xs"></q-icon>
              </q-item-section>
            </q-item>
          </q-card-section>


          <q-card-section>
            <div class="row q-pa-md">
              <div class="col-4">
                <span class="text-bold">Total</span>
              </div>
              <div class="col-6 text-right">
                {{userAggregate.count}}
              </div>
            </div>
            <div class="row q-pa-md">
              <div class="col-4">
                <span class="text-bold">User registrations</span>
              </div>
              <div class="col-6 text-right">
                <q-btn-toggle size="sm" rounded color="white" toggle-color="primary" toggle-text-color="white"
                              text-color="black"
                              :options="[
          {label: signUpPublicAvailable ? 'Enabled' : 'Enable', value: true},
          {label: !signUpPublicAvailable ? 'Disabled' : 'Disable', value: false},
        ]" v-model="signUpPublicAvailable"></q-btn-toggle>
              </div>
            </div>
            <div class="row q-pa-md">
              <div class="col-4">
                <span class="text-bold">Password Reset</span>
              </div>
              <div class="col-6 text-right">
                Disabled
                <!--                <q-btn-toggle size="sm" rounded color="white" toggle-color="primary" toggle-text-color="white"-->
                <!--                              text-color="black"-->
                <!--                              :options="[-->
                <!--        {label: resetPublicAvailable ? 'Enabled' : 'Enable', value: true},-->
                <!--          {label: !resetPublicAvailable ? 'Disabled' : 'Disable', value: false},-->
                <!--        ]" v-model="resetPublicAvailable"></q-btn-toggle>-->
              </div>
            </div>
          </q-card-section>

        </q-card>
      </div>


      <div class="col-4  col-md-6 col-lg-4 col-xl-3 col-xs-12 col-sm-12 q-pa-md q-gutter-sm">
        <q-card>

          <q-card-section>
            <q-item>
              <q-item-section avatar>
                <q-avatar>
                  <q-icon size="lg" name="fas fa-database"></q-icon>
                </q-avatar>
              </q-item-section>
              <q-item-section>
                <span class="text-h4">Data tables</span>
              </q-item-section>
            </q-item>
          </q-card-section>


          <q-card-section>
            <div class="row q-pa-md">
              <div class="col-4">
                <span class="text-bold">Total</span>
              </div>
              <div class="col-6 text-right">
                {{tables().length}}
              </div>
            </div>

          </q-card-section>
          <q-card-section>
            <div class="row ">
              <div class="col-12 q-pa-md q-gutter-sm">
                <q-btn class="float-right" @click="$router.push('/tables')" icon="list" round></q-btn>
                <q-btn class="float-right" @click="$router.push('/tables/create')" round icon="add"></q-btn>
              </div>
            </div>
          </q-card-section>

        </q-card>
      </div>

      <div class="col-4 col-md-6 col-lg-4 col-xl-3 col-xs-12 col-sm-12 q-pa-md q-gutter-sm">
        <q-card>


          <q-card-section>
            <q-item>
              <q-item-section avatar>
                <q-avatar>
                  <q-icon size="lg" name="fas fa-archive"></q-icon>
                </q-avatar>
              </q-item-section>
              <q-item-section>
                <span class="text-h4">Storage</span>
              </q-item-section>
            </q-item>
          </q-card-section>


          <q-card-section>
            <div class="row q-pa-md">
              <div class="col-4">
                <span class="text-bold">Cloud stores</span>
              </div>
              <div class="col-6 text-right">
                {{cloudStoreAggregate.count}}
              </div>
            </div>
          </q-card-section>
          <q-card-section>
            <div class="row ">
              <div class="col-12 q-pa-md q-gutter-sm">
                <q-btn class="float-right" @click="$router.push('/cloudstore')" icon="list" round></q-btn>
                <q-btn class="float-right" @click="$router.push('/cloudstore?create=true')" round icon="add"></q-btn>
              </div>
            </div>
          </q-card-section>

        </q-card>
      </div>

      <div class="col-4 col-md-6 col-lg-4 col-xl-3 col-xs-12 col-sm-12 q-pa-md q-gutter-sm">
        <q-card>

          <q-card-section>
            <q-item>
              <q-item-section avatar>
                <q-avatar>
                  <q-icon size="lg" name="fas fa-film"></q-icon>
                </q-avatar>
              </q-item-section>
              <q-item-section>
                <span class="text-h4">Sites</span>
              </q-item-section>
            </q-item>
          </q-card-section>


          <q-card-section>
            <div class="row q-pa-md">
              <div class="col-4">
                <span class="text-bold">Active</span>
              </div>
              <div class="col-6 text-right">
                {{siteAggregate.active}}
              </div>
            </div>
            <div class="row q-pa-md">
              <div class="col-4">
                <span class="text-bold">Total</span>
              </div>
              <div class="col-6 text-right">
                {{siteAggregate.total}}
              </div>
            </div>
          </q-card-section>
          <q-card-section>
            <div class="row ">
              <div class="col-12 q-pa-md q-gutter-sm">
                <q-btn @click="$router.push('/cloudstore/sites')" class="float-right" round icon="list"></q-btn>
              </div>
            </div>
          </q-card-section>

        </q-card>
      </div>

      <div class="col-4  col-md-6 col-lg-4 col-xl-3 col-xs-12 col-sm-12 q-pa-md q-gutter-sm">
        <q-card>
          <q-card-section>
            <q-item>
              <q-item-section avatar>
                <q-avatar>
                  <q-icon size="lg" name="fas fa-bolt"></q-icon>
                </q-avatar>
              </q-item-section>
              <q-item-section>
                <span class="text-h4">Integrations</span>
              </q-item-section>
            </q-item>
          </q-card-section>


          <q-card-section>
            <div class="row q-pa-md">
              <div class="col-4">
                <span class="text-bold">API Specs</span>
              </div>
              <div class="col-6 text-right">
                {{integrationAggregate.count}}
              </div>
            </div>
            <div class="row q-pa-md">
              <div class="col-4">
                <span class="text-bold">Actions</span>
              </div>
              <div class="col-6 text-right">
                {{actionAggregate.count}}
              </div>
            </div>
          </q-card-section>

          <q-card-section>
            <div class="row ">
              <div class="col-12 q-pa-md q-gutter-sm">
                <q-btn class="float-right" label="Add API Spec"></q-btn>
                <q-btn class="float-right" label="Create an action"></q-btn>
              </div>
            </div>
          </q-card-section>

        </q-card>
      </div>

      <div class="col-4  col-md-6 col-lg-4 col-xl-3 col-xs-12 col-sm-12 q-pa-md q-gutter-sm">
        <q-card>
          <q-card-section>
            <q-item>
              <q-item-section avatar>
                <q-avatar>
                  <q-icon size="lg" name="fas fa-plug"></q-icon>
                </q-avatar>
              </q-item-section>
              <q-item-section>
                <span class="text-h4">Services</span>
              </q-item-section>
              <q-item-section avatar>
                <q-btn rounded color="primary" @click="reloadServer()" flat size="md" icon="fas fa-sync"></q-btn>
              </q-item-section>
            </q-item>
          </q-card-section>


          <q-card-section>
            <div class="row q-pa-md">
              <div class="col-4">
                <span class="text-bold">JSON API endpoint</span>
              </div>
              <div class="col-6 text-right">
                Yes
              </div>
            </div>
            <div class="row q-pa-md">
              <div class="col-4">
                <span class="text-bold">FTP service</span>
              </div>
              <div class="col-6 text-right">
                <!--                <q-checkbox v-model="serverConfig['ftp.enable']"/>-->
                {{serverConfig['ftp.enable'] === 'true' ? 'Yes' : 'No'}}
              </div>
            </div>
            <div class="row q-pa-md">
              <div class="col-4">
                <span class="text-bold">GraphQL endpoint</span>
              </div>
              <div class="col-6 text-right">
                <q-btn-toggle size="sm" rounded color="white" toggle-color="primary" toggle-text-color="white"
                              text-color="black" @click="updateGraphqlEndpoint()"
                              :options="[
          {label: serverConfig['graphql.enable'] ? 'Enabled' : 'Enable', value: true},
          {label: !serverConfig['graphql.enable'] ? 'Disabled' : 'Disable', value: false},
        ]" v-model="serverConfig['graphql.enable']"></q-btn-toggle>

              </div>
            </div>
            <div class="row q-pa-md">
              <div class="col-4">
                <span class="text-bold">IMAP endpoint</span>
              </div>
              <div class="col-6 text-right">
                {{serverConfig['imap.enabled'] === 'true' ? 'Yes' : 'No'}}
              </div>
            </div>
            <div class="row q-pa-md">
              <div class="col-4">
                <span class="text-bold">Connection throttle</span>
              </div>
              <div class="col-6 text-right">
                {{serverConfig['limit.max_connections']}}
              </div>
            </div>
            <div class="row q-pa-md">
              <div class="col-4">
                <span class="text-bold">Allowed rate limit</span>
              </div>
              <div class="col-6 text-right">
                {{serverConfig['limit.rate']}}
              </div>
            </div>
          </q-card-section>

        </q-card>
      </div>


    </div>


    <q-page-sticky v-if="!showHelp" position="top-right" :offset="[0, 0]">
      <q-btn flat @click="showHelp = true" fab icon="fas fa-question"/>
    </q-page-sticky>

    <q-drawer overlay :width="400" side="right" v-model="showHelp">
      <q-scroll-area class="fit" v-if="showHelp">
        <help-page @closeHelp="showHelp = false">
        </help-page>
      </q-scroll-area>
    </q-drawer>


  </q-page>
</template>

<script>
  import {mapActions, mapGetters} from 'vuex';

  export default {
    name: 'PageIndex',
    methods: {
      updateGraphqlEndpoint() {
        const that = this;
        console.log("Update graphql endpoint", this.serverConfig['graphql.enable'])

        this.saveConfig({name: "graphql.enable", value: this.serverConfig['graphql.enable']}).then(function (res) {
          if (that.serverConfig['graphql.enable']) {
            that.$q.notify({
              message: "GraphQL endpoint enabled"
            });
          } else {
            that.$q.notify({
              message: "GraphQL endpoint disabled"
            });
            that.reloadServer();

          }
          that.showHostnameEdit = false;
        }).catch(function (res) {
          console.log("Failed to update graphql endpoint", res);
          that.$q.notify({
            message: "Failed to update endpoint status"
          })
        })

      },
      saveHostname() {
        const that = this;
        this.saveConfig({name: "hostname", value: this.serverConfig.hostname}).then(function (res) {
          that.$q.notify({
            message: "Hostname updated"
          });
          that.reloadServer();
          that.showHostnameEdit = false;
        }).catch(function (res) {
          console.log("failed to upate hostname", res)
          that.$q.notify({
            message: "Failed to update hostname"
          })
        })
      },
      changeHostname() {
        this.showHostnameEdit = true;
      },
      reloadServer() {
        console.log("Reload server");
        const that = this;
        that.executeAction({
          tableName: "world",
          actionName: "restart_daptin"
        }).then(function (res) {
          that.$q.notify({
            message: "Server restarted"
          })
        }).catch(function (err) {
          that.$q.notify({
            message: "Failed to restart"
          });
          console.log("Failed to restart daptin", err)
        })
      },
      ...mapActions(['loadData', 'loadAggregates', 'loadServerConfig', 'executeAction', 'saveConfig'])
    },

    data() {
      return {
        text: '',
        showHelp: false,
        showHostnameEdit: false,
        actionMap: {},
        userAggregate: {},
        cloudStoreAggregate: {},
        serverConfig: {},
        siteAggregate: {},
        integrationAggregate: {},
        actionAggregate: {},
        signUpPublicAvailable: false,
        resetPublicAvailable: false,
        ...mapGetters(['tables'])
      }
    },
    mounted() {
      const that = this;
      that.loadData({
        tableName: 'action',
        params: {
          page: {
            size: 500
          }
        }
      }).then(function (res) {
        console.log("Actions", res);
        var data = res.data;
        var actionMap = {};
        var signUpAction = data.filter(function (e) {
          actionMap[e.action_name] = e
          return e.action_name === 'signup'
        })[0];
        // console.log("Sign up action", signUpAction);
        if (signUpAction && signUpAction.permission && 1) {
          that.signUpPublicAvailable = true;
        }
        var resetAction = data.filter(function (e) {
          return e.action_name === 'resetpassword'
        })[0];
        // console.log("Reset action", resetAction);
        if (resetAction && resetAction.permission && 1) {
          that.resetPublicAvailable = true;
        }
        that.actionMap = actionMap;
        console.log("Action map" , actionMap)

      }).catch(function (res) {
        console.log("Failed to load actions", res);
      });


      that.loadAggregates({
        tableName: 'user_account',
        column: 'count'
      }).then(function (res) {
        console.log("User account aggregates", res);
        that.userAggregate = res.data[0];
      });


      that.loadAggregates({
        tableName: 'cloud_store',
        column: 'count'
      }).then(function (res) {
        console.log("cloud store aggregates", res);
        that.cloudStoreAggregate = res.data[0];
      });


      that.loadAggregates({
        tableName: 'site',
        column: 'count',
        group: 'enable'
      }).then(function (res) {
        console.log("Site aggregates", res);
        var enableStat = null;
        var disableStat = null;
        for (var i in res.data){
          var stat = res.data[i];
          if (stat.enable === true ) {
            enableStat = stat;
          } else if (stat.enable === false) {
            disableStat = stat;
          }
        }

        that.siteAggregate = {
          active: 0,
          total: 0,
        };
        if (enableStat) {
          that.siteAggregate.active = enableStat.count;
          that.siteAggregate.total += enableStat.count;
        }
        if (disableStat) {
          that.siteAggregate.total += disableStat.count;
        }
      });
      that.loadAggregates({
        tableName: 'action',
        column: 'count',
      }).then(function (res) {
        console.log("Action aggregates", res);
        that.actionAggregate = res.data[0];
      });
      that.loadAggregates({
        tableName: 'integration',
        column: 'count',
      }).then(function (res) {
        console.log("Site aggregates", res);
        that.integrationAggregate = res.data[0];
      });

      that.loadServerConfig().then(function (res) {
        for (var key in res) {
          if (res[key] === "true") {
            res[key] = true
          } else if (res[key] === "false") {
            res[key] = false
          }
        }
        console.log("Server config", res)

        that.serverConfig = res;
      }).catch(function (err) {
        console.log("Failed to load server config", err)
      });


    }
  }
</script>
