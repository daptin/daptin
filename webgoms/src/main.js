// The Vue build version to load with the `import` command
// (runtime-only or standalone) has been set in webpack.base.conf with an alias.
import Bourgeon from 'bourgeon'
import systemInit from "./plugins/main";

import worldManager from "./plugins/worldmanager";
import jsonApi from "./plugins/jsonapi";
import actionManager from "./plugins/actionmanager";
import axios from "./plugins/axios";


import App from './App'
import Vue from 'vue'


import store from './store';


Vue.use(Bourgeon, {
  locales: ['fr', 'en']
});

/* eslint-disable no-new */
new Vue({
  store,
  render: h => h(App)
}).$mount('#app');
