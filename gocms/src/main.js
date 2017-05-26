// The Vue build version to load with the `import` command
// (runtime-only or standalone) has been set in webpack.base.conf with an alias.
import Vue from 'vue'
import App from './App'
import router from './router'
import ElementUI from 'element-ui'
import Vuetable from './components/vuetable'
import VuetablePagination from './components/vuetable/components/VuetablePagination.vue'
global.jQuery = require('jquery');

Vue.config.productionTip = false;

Vue.use(ElementUI);
Vue.use(Vuetable);
Vue.use(VuetablePagination);

import 'element-ui/lib/theme-default/index.css'
import './components/vuetable/vuetable.css'


Vue.component("vuetable", Vuetable);
Vue.component("vuetable-pagination", VuetablePagination);
// Vue.component("vuetable-pagination-dropdown", Vuetable.VueTablePaginationDropDown);
// Vue.component("vuetable-pagination-info", Vuetable.VueTablePaginationInfo);

// Utility to check auth status

// Vue.component("model-form", {
//     props: [
//         "model",
//         "meta"
//     ],
//     template: `
//
//      <div v-bind:is="currentElement"></div>
//
//     `,
//     components: {
//         text: {
//             template: '<el-input></el-input>'
//         },
//         number: {
//             template: '<el-select></el-select>'
//         }
//     },
//     data: function () {
//         return {
//             currentElement: "input",
//         }
//     },
//     beforeCreate: function () {
//         console.log("model", this, arguments)
//     },
//     mounted: function () {
//         var that = this;
//         setTimeout(function(){
//             console.log("change type");
//             that.currentElement = "el-button";
//         }, 2000)
//     }
// });

/* eslint-disable no-new */
new Vue({
    el: '#app',
    router,
    template: '<App/>',
    components: {App},
});