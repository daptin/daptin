import Vuetable from "./components/Vuetable.vue";
import Vuecard from "./components/Vuecard.vue";
import VuetablePagination from "./components/VuetablePagination.vue";
import VuetablePaginationDropDown from "./components/VuetablePaginationDropdown.vue";
import VuetablePaginationInfo from "./components/VuetablePaginationInfo.vue";

function install(Vue) {
  Vue.component("vuetable", Vuetable);
  Vue.component("vuecard", Vuecard);
  Vue.component("vuetable-pagination", VuetablePagination);
  Vue.component("vuetable-pagination-dropdown", VuetablePaginationDropDown);
  Vue.component("vuetable-pagination-info", VuetablePaginationInfo);
}
export {
  Vuetable,
  Vuecard,
  VuetablePagination,
  VuetablePaginationDropDown,
  VuetablePaginationInfo,
  install
};

export default Vuetable;
