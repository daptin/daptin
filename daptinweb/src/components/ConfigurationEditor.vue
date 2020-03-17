<template>
  <!-- Content Wrapper. Contains page content -->
  <div class="content-wrapper">
    <!-- Content Header (Page header) -->
    <section class="content-header">
      <h1>
        Configuration
      </h1>
      <ol class="breadcrumb">
        <button v-on:click="saveChanges" class="btn btn-warning" v-if="showSave">Save changes</button>
        <li>
          <a href="javascript:;">
            <i class="fa fa-home"></i>Home</a>
        </li>
        <li v-for="crumb in $route.meta.breadcrumb">
          <template v-if="crumb.to">
            <router-link :to="crumb.to">{{crumb.label}}</router-link>
          </template>
          <template v-else>
            {{crumb.label}}
          </template>
        </li>
      </ol>
    </section>
    <section class="content">
      <div class="row">
        <form>
          <div class="col-md-4" style="height: 150px;" v-if="!name.startsWith('encryption')"
               v-for="(item, name, idx) in settings"
               :key="idx">
            <div class="card" style="margin: 10px">
              <div class="card-body">
                <label>{{settingsHelp[name]}}</label>
                <input v-once v-on:change="showSaveButton(name, $event.target.value)" type="text" class="form-control"
                       :value="item" v-if="item != 'true' && item != 'false'">
                <div class="checkbox" v-if="item == 'true' || item == 'false'">
                  <label>
                    <input v-once v-bind:checked="item === 'true'" v-on:change="showSaveButton(name, $event.target.checked)"
                           type="checkbox">
                  </label>
                </div>
                <small class="form-text text-muted">{{name}}</small>
              </div>
            </div>
          </div>
        </form>
      </div>
    </section>
  </div>
</template>

<script>
  import ConfigManager from '../plugins/configmanager'

  export default {
    name: "ConfigurationEditor",
    data: function () {
      return {
        settings: {},
        showSave: false,
        settingsHelp: {
          "language.default": "Default system language",
          "jwt.secret": "Secret for encrypting JWT tokens",
          "logs.enable": "/_logs endpoint enabled",
          "jwt.token.issuer": "JWT issuer authority name",
          "rclone.retries": "Retry count for failures for rclone",
          "jwt.token.life.hours": "JWT token life time in hours",
          "totp.secret": "Secret used to generate TOTP tokens",
          "hostname": "Hostname",
          "imap.enabled": "Enable IMAP",
          "imap.listen_interface": "IMAP listening interface",
          "graphql.enable": "Enable Graphql",
          "ftp.listen_interface": "FTP listening interface",
          "ftp.enable": "Enable FTP service",
        },
        changed: {},
      }
    },
    methods: {
      saveChanges() {
        const that = this;
        // for (var key in this.changed) {
        //   var value = this.changed[key];
        //   console.log("save ", key, value)
        //   ConfigManager.setConfig(key, value).then(function (res) {
        //     console.log("saved ", key, value)
        //   })
        // }
        Promise.all(Object.keys(this.changed).map(function (item) {
              return ConfigManager.setConfig(item, that.changed[item]);
            })
        ).then(function (res) {
          console.log("saved all settings ");
          that.showSave = false;
          ConfigManager.getAllConfig().then(function (data) {
            that.settings = data;
          })
        }).catch(function (err) {
          console.log("failed to save setting", err)
        })
      },
      showSaveButton(itemName, value) {
        if (value === true) {
          value = "true"
        } else if (value === false) {
          value = "false"
        }
        this.showSave = true;
        console.log("set ", itemName, value);
        this.changed[itemName] = value;
      }
    },
    mounted() {
      const that = this;
      ConfigManager.getAllConfig().then(function (data) {
        that.settings = data;
      })
    }
  }
</script>

<style scoped>

</style>
