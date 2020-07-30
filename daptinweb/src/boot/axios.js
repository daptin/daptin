import Vue from 'vue'
import axios from 'axios'

Vue.prototype.$axios = axios;
import TableSideBar from '../pages/TableSideBar'
import TableEditor from '../pages/TableEditor'
import TablePermissions from '../pages/Permissions'
import HelpPage from '../pages/HelpPage'
import FileBrowser from 'pages/FileBrowserComponent';
const VueUploadComponent = require('vue-upload-component');
var VueMarkdown = require('vue-markdown');

Vue.component('file-upload', VueUploadComponent);
Vue.component('vue-markdown', VueMarkdown);
Vue.component('file-browser', FileBrowser);
Vue.component("table-side-bar", TableSideBar);
Vue.component("table-permissions", TablePermissions);
Vue.component("table-editor", TableEditor);
Vue.component("help-page", HelpPage);

