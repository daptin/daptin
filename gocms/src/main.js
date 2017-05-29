// The Vue build version to load with the `import` command
// (runtime-only or standalone) has been set in webpack.base.conf with an alias.
import Vue from 'vue'
import App from './App'
import router from './router'
import ElementUI from 'element-ui'
import Vuetable from './components/vuetable'
import DetailedRow from './components/detailrow/DetailedRow.vue'
import ModelForm from './components/modelform/ModelForm.vue'
import VuetablePagination from './components/vuetable/components/VuetablePagination.vue'
import CustomActions from './components/detailrow/CustomActions.vue'
import TableView from './components/tableview/TableView.vue'
import {Notification} from 'element-ui';

global.jQuery = require('jquery');

Vue.config.productionTip = false;

Vue.use(ElementUI);
Vue.use(Vuetable);
Vue.use(VuetablePagination);
Vue.use(DetailedRow);

import 'element-ui/lib/theme-default/index.css'
import './components/vuetable/vuetable.css'

Vue.component('custom-actions', CustomActions);
Vue.component('table-view', TableView);
Vue.component('model-form', ModelForm);
Vue.component("vuetable", Vuetable);
Vue.component("detailed-table-row", DetailedRow);
Vue.component("vuetable-pagination", VuetablePagination);

// Vue.component("vuetable-pagination-dropdown", Vuetable.VueTablePaginationDropDown);
// Vue.component("vuetable-pagination-info", Vuetable.VueTablePaginationInfo);


/* eslint-disable no-new */
new Vue({
  el: '#app',
  router,
  template: '<App/>',
  components: {App},
});
