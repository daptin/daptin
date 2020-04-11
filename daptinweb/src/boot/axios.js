import Vue from 'vue'
import axios from 'axios'

Vue.prototype.$axios = axios;
import TableSideBar from '../pages/TableSideBar'
import TableEditor from '../pages/TableEditor'

Vue.component("table-side-bar", TableSideBar);
Vue.component("table-editor", TableEditor);

