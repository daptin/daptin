// The Vue build version to load with the `import` command
// (runtime-only or standalone) has been set in webpack.base.conf with an alias.
import Vue from "vue";
import ElementUI, {Notification} from "element-ui";

import Vuetable from "../components/vuetable";
import Daptable from "../components/daptable/DaptableView.vue";
import Vuecard from "../components/vuetable/components/Vuecard.vue";
import DetailedRow from "../components/detailrow/DetailedRow.vue";
import ModelForm from "../components/modelform/ModelForm.vue";
import VuetablePagination from "../components/vuetable/components/VuetablePagination.vue";
import CustomActions from "../components/detailrow/CustomActions.vue";
import TableView from "../components/tableview/TableView.vue";
import SelectOneOrMore from "../components/selectoneormore/SelectOneOrMore.vue";
import ListView from "../components/listview/ListView.vue";
import ActionView from "../components/actionview/ActionView.vue";
import ReclineView from "../components/reclineview/ReclineView.vue";
import locale from 'element-ui/lib/locale/lang/en'

import "element-ui/lib/theme-default/index.css";
import "tether-shepherd/dist/css/shepherd-theme-dark.css";
import "../components/vuetable/vuetable.css";
// Register my awesome field
import fileUpload from "../components/fields/FileUpload.vue";
import permissionField from "../components/fields/PermissionField.vue";
import jsonEditor from "../components/fields/FileJsonEditor.vue";
import fancyCheckBox from "../components/fields/FancyCheckBox.vue";
import dateSelect from "../components/fields/DateSelect.vue";
// import VoyagerView from "../components/voyagerview/VoyagerView.vue";
Vue.component("fieldFileUpload", fileUpload);
Vue.component('fieldPermissionInput', permissionField);
Vue.component("fieldSelectOneOrMore", SelectOneOrMore);
Vue.component("fieldDateSelect", dateSelect);
Vue.component("fieldJsonEditor", jsonEditor);
Vue.component("fieldFancyCheckBox", fancyCheckBox);


Vue.use(ElementUI, {locale});
Vue.use(Vuetable);
Vue.use(Daptable);
Vue.use(Vuecard);
Vue.use(VuetablePagination);
Vue.use(DetailedRow);

Vue.component('custom-actions', CustomActions);
Vue.component('table-view', TableView);
// Vue.component('voyager-view', VoyagerView);
Vue.component('recline-view', ReclineView);
Vue.component('action-view', ActionView);
Vue.component('list-view', ListView);
Vue.component('model-form', ModelForm);
Vue.component("vuetable", Vuetable);
Vue.component("daptable", Daptable);
Vue.component("vuecard", Vuecard);
Vue.component("select-one-or-more", SelectOneOrMore);
Vue.component("detailed-table-row", DetailedRow);
Vue.component("vuetable-pagination", VuetablePagination);
