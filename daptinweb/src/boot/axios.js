import Vue from 'vue'
import axios from 'axios'

Vue.prototype.$axios = axios;
import TableSideBar from '../pages/TableSideBar'
import TableEditor from '../pages/TableEditor'
import TablePermissions from '../pages/Permissions'
import HelpPage from '../pages/HelpPage'
import FileBrowser from 'pages/FileBrowserComponent';
import DaptinDocumentUploader from 'pages/UserApps/Components/DaptinDocumentUploader.vue';
import UserHeaderBar from "pages/UserApps/UserHeaderBar";

const VueUploadComponent = require('vue-upload-component');
const AceEditor = require('vue2-ace-editor');
import VJstree from 'vue-jstree'
import PaginatedTableView from "pages/UserApps/PaginatedListViewTemplate/PaginatedTableView.vue";
import PaginatedCardView from "pages/UserApps/PaginatedListViewTemplate/PaginatedCardView";


// import CKEditor from '@ckeditor/ckeditor5-vue'
// Vue.use(CKEditor)

Vue.component('v-jstree', VJstree);
Vue.component('ace-editor', AceEditor);
Vue.component('user-header-bar', UserHeaderBar);
Vue.component('file-upload', VueUploadComponent);
Vue.component('daptin-document-uploader', DaptinDocumentUploader);
Vue.component('file-browser', FileBrowser);
Vue.component("table-side-bar", TableSideBar);
Vue.component("table-permissions", TablePermissions);
Vue.component("table-editor", TableEditor);
Vue.component("paginated-table-view", PaginatedTableView);
Vue.component("paginated-card-view", PaginatedCardView);
Vue.component("help-page", HelpPage);
// Vue.component("tiny-mce", Editor);


