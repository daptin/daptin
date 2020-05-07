<template>
  <div :class="['vuecard', 'row', css.tableClass]">

    <div v-cloak class="vuecard-body">
      <div class="col-md-4" v-for="(item, index) in tableData">
        <div @dblclick="onRowDoubleClicked(item, $event)" :item-index="index" @click="onRowClicked(item, $event)"
             :render="onRowChanged(item)" :class="[onRowClass(item, index), 'box']" style="min-height: 250px">
          <div class="box-header">
            <div class="box-title">
              <span class="bold">{{item | chooseTitle | titleCase }}</span>
            </div>
            <div class="box-tools pull-right">
              <slot name="actions" :row-data="item" :row-index="index"></slot>
            </div>

          </div>
          <div class="box-body">

            <template v-for="field in tableFields">
              <dl v-if="field.visible">
                <dt v-if="!isSpecialField(field.name)">{{field.name | titleCase}}</dt>
                <template v-if="isSpecialField(field.name)">
                  <dd v-if="apiMode && extractName(field.name) == '__sequence'"
                      :class="['vuecard-sequence', field.dataClass]"
                      v-html="tablePagination.from + index">
                  </dd>
                  <dd v-if="extractName(field.name) == '__handle'" :class="['vuecard-handle', field.dataClass]"
                      v-html="renderIconTag(['handle-icon', css.handleIcon])"></dd>
                  <dd v-if="extractName(field.name) == '__checkbox'" :class="['vuecard-checkboxes', field.dataClass]">
                    <input type="checkbox"
                           @change="toggleCheckbox(item, field.name, $event)"
                           :checked="rowSelected(item, field.name)">
                  </dd>
                  <dd v-if="extractName(field.name) === '__component'" :class="['vuecard-component', field.dataClass]">
                    <component :is="extractArgs(field.name)"
                               :row-data="item" :row-index="index" :row-field="field.sortField"
                    ></component>
                  </dd>
                  <dd v-if="extractName(field.name) === '__slot'" :class="['vuecard-slot', field.dataClass]">
                    <slot :name="extractArgs(field.name)"
                          :row-data="item" :row-index="index" :row-field="field.sortField"
                    ></slot>
                  </dd>
                </template>
                <template v-else>
                  <dd v-if="hasCallback(field)" :class="field.dataClass"
                      @click="onCellClicked(item, field, $event)"
                      @dblclick="onCellDoubleClicked(item, field, $event)"
                      v-html="callCallback(field, item)"
                  >
                  </dd>
                  <dd v-else :class="field.dataClass"
                      @click="onCellClicked(item, field, $event)"
                      @dblclick="onCellDoubleClicked(item, field, $event)"
                      v-html="getObjectValue(item, field.name, '')"
                  >
                  </dd>
                </template>
              </dl>
            </template>
          </div>
        </div>
        <template v-if="useDetailRow">
          <tr v-if="isVisibleDetailRow(item[trackBy])"
              @click="onDetailRowClick(item, $event)"
              :class="[css.detailRowClass]">
            <transition :name="detailRowTransition">
              <td :colspan="countVisibleFields">
                <component :is="detailRowComponent" :model="item" :json-api="jsonApi"
                           :json-api-model-name="jsonApiModelName" :row-index="index"></component>
              </td>
            </transition>
          </tr>
        </template>
      </div>
      <template v-if="lessThanMinRows">
        <tr v-for="i in blankRows" class="blank-row">
          <template v-for="field in tableFields">
            <td v-if="field.visible">&nbsp;</td>
          </template>
        </tr>
      </template>
    </div>
  </div>
</template>

<script>
  var markdown_renderer = require('markdown-it')();

  export default {
    props: {
      loadOnStart: {
        type: Boolean,
        default: true
      },
      apiUrl: {
        type: String,
        default: ''
      },
      apiMode: {
        type: Boolean,
        default: true
      },
      data: {
        type: Array,
        default: function () {
          return null
        }
      },
      dataPath: {
        type: String,
        default: ''
      },
      paginationPath: {
        type: [String],
        default: 'links.pagination'
      },
      queryParams: {
        type: Object,
        default() {
          return {
            sort: 'sort',
            page: 'page',
            perPage: 'per_page'
          }
        }
      },
      appendParams: {
        type: Object,
        default() {
          return {}
        }
      },
      httpOptions: {
        type: Object,
        default() {
          return {}
        }
      },
      perPage: {
        type: Number,
        default() {
          return 10
        }
      },
      sortOrder: {
        type: Array,
        default() {
          return []
        }
      },
      multiSort: {
        type: Boolean,
        default() {
          return false
        }
      },
      /*
       * physical key that will trigger multi-sort option
       * possible values: 'alt', 'ctrl', 'meta', 'shift'
       * 'ctrl' might not work as expected on Mac
       */
      multiSortKey: {
        type: String,
        default: 'alt'
      },
      /* deprecated */
      rowClassCallback: {
        type: [String, Function],
        default: ''
      },
      rowClass: {
        type: [String, Function],
        default: ''
      },
      detailRowComponent: {
        type: String,
        default: ''
      },
      detailRowTransition: {
        type: String,
        default: ''
      },
      trackBy: {
        type: String,
        default: 'id'
      },
      renderIcon: {
        type: Function,
        default: null
      },
      css: {
        type: Object,
        default() {
          return {
            tableClass: 'ui blue selectable celled stackable attached table',
            loadingClass: 'loading',
            ascendingIcon: 'blue chevron up icon',
            descendingIcon: 'blue chevron down icon',
            detailRowClass: 'vuecard-detail-row',
            handleIcon: 'grey sidebar icon',
          }
        }
      },
      minRows: {
        type: Number,
        default: 0
      },
      silent: {
        type: Boolean,
        default: false
      },
      jsonApi: {
        type: Object,
        default: null
      },
      finder: {
        type: Array,
        default: null
      },
      jsonApiModelName: {
        type: String,
        default: null
      }
    },
    data() {
      return {
        eventPrefix: 'vuecard:',
        tableFields: [],
        tableData: null,
        tablePagination: null,
        actionSlot: null,
        currentPage: 1,
        selectedTo: [],
        visibleDetailRows: [],
      }
    },
    created() {
      this.normalizeFields();
      this.$nextTick(function () {
        this.emit1('initialized', this.tableFields)
      });

      if (this.apiMode && this.loadOnStart) {
        this.loadData()
      }
      if (this.apiMode == false && this.data.length > 0) {
        this.setData(this.data)
      }
    },
    computed: {
      useDetailRow() {
        if (this.tableData && this.tableData[0] && this.detailRowComponent !== '' && typeof this.tableData[0][this.trackBy] === 'undefined') {
          this.warn('You need to define unique row identifier in order for detail-row feature to work. Use `track-by` prop to define one!');
          return false
        }

        return this.detailRowComponent !== ''
      },
      countVisibleFields() {
        return this.tableFields.filter(function (field) {
          return field.visible
        }).length
      },
      lessThanMinRows: function () {
        if (this.tableData === null || this.tableData.length === 0) {
          return true
        }
        return this.tableData.length < this.minRows
      },
      blankRows: function () {
        if (this.tableData === null || this.tableData.length === 0) {
          return this.minRows
        }
        if (this.tableData.length >= this.minRows) {
          return 0
        }

        return this.minRows - this.tableData.length
      }
    },
    methods: {
      normalizeFields() {
        var that = this;
//        console.log("vuecard for ", this.jsonApiModelName)
        let modelFor = this.jsonApi.modelFor(this.jsonApiModelName);
//        console.log("json model for ", this.jsonApiModelName, " is ", modelFor)

        if (!modelFor) {
          return
        }
        this.fieldsData = modelFor["attributes"];
        this.fields = Object.keys(this.fieldsData);
//        console.log("vuecard fields for ", this.jsonApiModelName, this.fields);

        this.tableFields = [];
        let self = this;
        let obj;
        this.fields.forEach(function (field, i) {
          var fieldType = that.fieldsData[field];
//          console.log("field type", field, fieldType, that.fieldsData);
          field = {
            name: field,
            title: self.setTitle(field),
            callback: undefined,
            sortField: field
          };

          if (fieldType == "hidden") {
            field.visible = false;
          }

          if (fieldType == "encrypted") {
            field.visible = false;
          }
          if (fieldType.indexOf && fieldType.indexOf("image.") == 0) {
            field.callback = function (val, row) {
              console.log("render image on card", val);
              return "Image preview not available"
            }
          }

          if (typeof fieldType == "object") {
            field.visible = false;
          }

          if (fieldType === "truefalse") {
            field.callback = 'trueFalseView';
          }

          if (field.name == "updated_at") {
            field.visible = false;
          }

          if (field.name == "reference_id") {
//                        field.visible = false;
          }

          if (field.name == "permission") {
            field.visible = false;
          }

          if (field.name == "status") {
            field.visible = false;
          }


          if (fieldType == "alias") {
            field.visible = false;
          }

          if (fieldType == "json") {
            field.visible = false;
          }

          if (fieldType == "truefalse") {
            field.visible = false;
          }

          if (fieldType == "content") {
            field.visible = false;
          }

          if (fieldType == "markdown") {
            field.callback = function (val, row) {
              return markdown_renderer.render(val)
            }
          }

          if (fieldType == "label") {
            field.callback = function (val, row) {
//              console.log("callback for label field", val, arguments);
              return val
            }
          }

          obj = {
            name: field.name,
            title: (field.title === undefined) ? self.setTitle(field.name) : field.title,
            sortField: field.sortField,
            titleClass: (field.titleClass === undefined) ? '' : field.titleClass,
            dataClass: (field.dataClass === undefined) ? '' : field.dataClass,
            callback: (field.callback === undefined) ? '' : field.callback,
            visible: (field.visible === undefined) ? true : field.visible,
          };

          self.tableFields.push(obj)
        });
        self.actionSlot = {
          name: '__slot:actions',
//          title: '<button class="ui button" @click="newRow()"><i class="fa fa-plus"></i> Add '+ this.jsonApiModelName +'</button>',
          title: '',
          visible: true,
          titleClass: 'center aligned',
          dataClass: 'center aligned',
        };
      },
      setData(data) {
        this.apiMode = false;
        this.tableData = data
      },
      titleCase(str) {
        return this.$parent.titleCase(str);
      },
      setTitle(str) {
        if (this.isSpecialField(str)) {
          return ''
        }

        return this.titleCase(str)
      },
      renderTitle(field) {
        let title = (typeof field.title === 'undefined') ? field.name.replace(/\.\_/g, ' ') : field.title;

        if (title.length > 0 && this.isInCurrentSortGroup(field)) {
          let style = `opacity:${this.sortIconOpacity(field)};position:relative;float:right`;
          return title + ' ' + this.renderIconTag(['sort-icon', this.sortIcon(field)], `style="${style}"`)
        }

        return title
      },
      isSpecialField(fieldName) {
        return fieldName.slice(0, 2) === '__'
      },
      titleCase: function (str) {
        return str.replace(/[-_]/g, " ").split(' ')
          .map(w => w[0] ? (w[0].toUpperCase() + w.substr(1).toLowerCase()) : w)
          .join(' ')
      },
      camelCase(str, delimiter = '_') {
        let self = this;
        return str.split(delimiter).map(function (item) {
          return self.titleCase(item)
        }).join('')
      },
      notIn(str, arr) {
        return arr.indexOf(str) === -1
      },
      loadData(success = this.loadSuccess, failed = this.loadFailed) {
        var that = this;
        if (!this.apiMode) return;

        this.emit1('loading');

        this.httpOptions['params'] = this.getAllQueryParams();


        that.jsonApi.builderStack = this.finder;
        that.jsonApi.get(this.httpOptions["params"]).then(
          success,
          failed
        )
      },
      loadSuccess(response) {
//        console.log("load success", response);
        this.emit1('load-success', response);

        let body = this.transform(response);

        this.tableData = this.getObjectValue(body, this.dataPath, null);
        this.tablePagination = this.getObjectValue(body, this.paginationPath, null);


        if (this.tablePagination === null) {
          this.warn('vuecard: pagination-path "' + this.paginationPath + '" not found. '
            + 'It looks like the data returned from the sever does not have pagination information '
            + "or you may have set it incorrectly.\n"
            + 'You can explicitly suppress this warning by setting pagination-path="".'
          )
        }

        var that = this;
        this.$nextTick(function () {
          that.emit1('pagination-data', this.tablePagination);
          that.emit1('loaded')
        })
      },
      loadFailed(response) {
        console.error('load-error', response);
        this.emit1('load-error', response);
        this.emit1('loaded')
      },
      transform(data) {
        let func = 'transform';

        if (this.parentFunctionExists(func)) {
          return this.$parent[func].call(this.$parent, data)
        }

        return data
      },
      parentFunctionExists(func) {
        return (func !== '' && typeof this.$parent[func] === 'function')
      },
      callParentFunction(func, args, defaultValue = null) {
        if (this.parentFunctionExists(func)) {
          return this.$parent[func].call(this.$parent, args)
        }

        return defaultValue
      },
      emit1(eventName, args) {
        this.$emit(eventName, args)
      },
      warn(msg) {
        if (!this.silent) {
          console.warn(msg)
        }
      },
      getAllQueryParams() {
        let params = {};
        params[this.queryParams.sort] = this.getSortParam();
        params[this.queryParams.page] = this.currentPage;
        params[this.queryParams.perPage] = this.perPage;

        for (let x in this.appendParams) {
          params[x] = this.appendParams[x]
        }

        return params
      },
      getSortParam: function (sortOrder) {

        if (!this.sortOrder || this.sortOrder.field == '') {
          return ''
        }


        return this.sortOrder.map(function (sort) {
          return (sort.direction === 'desc' ? '' : '-') + sort.field
        }).join(',')
      },
      getDefaultSortParam() {
        let result = '';

        for (let i = 0; i < this.sortOrder.length; i++) {
          let fieldName = (typeof this.sortOrder[i].sortField === 'undefined')
            ? this.sortOrder[i].field
            : this.sortOrder[i].sortField;

          result += fieldName + '|' + this.sortOrder[i].direction + ((i + 1) < this.sortOrder.length ? ',' : '');
        }

        return result;
      },
      extractName(string) {
        return string.split(':')[0].trim()
      },
      extractArgs(string) {
        return string.split(':')[1]
      },
      isSortable(field) {
        return !(typeof field.sortField === 'undefined')
      },
      isInCurrentSortGroup(field) {
        return this.currentSortOrderPosition(field) !== false;
      },
      currentSortOrderPosition(field) {
        if (!this.isSortable(field)) {
          return false
        }

        for (let i = 0; i < this.sortOrder.length; i++) {
          if (this.fieldIsInSortOrderPosition(field, i)) {
            return i;
          }
        }

        return false;
      },
      fieldIsInSortOrderPosition(field, i) {
        return this.sortOrder[i].field === field.name && this.sortOrder[i].sortField === field.sortField
      },
      orderBy(field, event) {
        if (!this.isSortable(field) || !this.apiMode) return;

        let key = this.multiSortKey.toLowerCase() + 'Key';

        if (this.multiSort && event[key]) { //adding column to multisort
          this.multiColumnSort(field)
        } else {
          //no multisort, or resetting sort
          this.singleColumnSort(field)
        }

        this.currentPage = 1;    // reset page index
        this.loadData()
      },
      multiColumnSort(field) {
        let i = this.currentSortOrderPosition(field);

        if (i === false) { //this field is not in the sort array yet
          this.sortOrder.push({
            field: field.name,
            sortField: field.sortField,
            direction: 'asc'
          });
        } else { //this field is in the sort array, now we change its state
          if (this.sortOrder[i].direction === 'asc') {
            // switch direction
            this.sortOrder[i].direction = 'desc'
          } else {
            //remove sort condition
            this.sortOrder.splice(i, 1);
          }
        }
      },
      singleColumnSort(field) {
        if (this.sortOrder.length === 0) {
          this.clearSortOrder()
        }

        this.sortOrder.splice(1); //removes additional columns

        if (this.fieldIsInSortOrderPosition(field, 0)) {
          // change sort direction
          this.sortOrder[0].direction = this.sortOrder[0].direction === 'asc' ? 'desc' : 'asc'
        } else {
          // reset sort direction
          this.sortOrder[0].direction = 'asc'
        }
        this.sortOrder[0].field = field.name;
        this.sortOrder[0].sortField = field.sortField
      },
      clearSortOrder() {
        this.sortOrder.push({
          field: '',
          sortField: '',
          direction: 'asc'
        });
      },
      sortIcon(field) {
        let cls = '';
        let i = this.currentSortOrderPosition(field);

        if (i !== false) {
          cls = (this.sortOrder[i].direction == 'asc') ? this.css.ascendingIcon : this.css.descendingIcon
        }

        return cls;
      },
      sortIconOpacity(field) {
        /*
         * fields with stronger precedence have darker color
         *
         * if there are few fields, we go down by 0.3
         * ex. 2 fields are selected: 1.0, 0.7
         *
         * if there are more we go down evenly on the given spectrum
         * ex. 6 fields are selected: 1.0, 0.86, 0.72, 0.58, 0.44, 0.3
         */
        let max = 1.0,
          min = 0.3,
          step = 0.3;

        let count = this.sortOrder.length;
        let current = this.currentSortOrderPosition(field);


        if (max - count * step < min) {
          step = (max - min) / (count - 1)
        }

        let opacity = max - current * step;

        return opacity
      },
      hasCallback(item) {
        return item.callback ? true : false
      },
      callCallback(field, item) {
        if (!this.hasCallback(field)) return;

        if (typeof(field.callback) == 'function') {
          return field.callback(this.getObjectValue(item, field.name))
        }

        let args = field.callback.split('|');
        let func = args.shift();

        if (typeof this.$parent[func] === 'function') {
          let value = this.getObjectValue(item, field.name);

          return (args.length > 0)
            ? this.$parent[func].apply(this.$parent, [value].concat(args))
            : this.$parent[func].call(this.$parent, value)
        }

        return null
      },
      getObjectValue(object, path, defaultValue) {
        defaultValue = (typeof defaultValue === 'undefined') ? null : defaultValue;

        let obj = object;
        if (path.trim() != '') {
          let keys = path.split('.');
          keys.forEach(function (key) {
            if (obj !== null && typeof obj[key] !== 'undefined' && obj[key] !== null) {
              obj = obj[key]
            } else {
              obj = defaultValue;

            }
          })
        }
        return obj
      },
      toggleCheckbox(dataItem, fieldName, event) {
        let isChecked = event.target.checked;
        let idColumn = this.trackBy;

        if (dataItem[idColumn] === undefined) {
          this.warn('__checkbox field: The "' + this.trackBy + '" field does not exist! Make sure the field you specify in "track-by" prop does exist.');
          return
        }

        let key = dataItem[idColumn];
        if (isChecked) {
          this.selectId(key)
        } else {
          this.unselectId(key)
        }
        this.emit1('vuecard:checkbox-toggled', isChecked, dataItem)
      },
      selectId(key) {
        if (!this.isSelectedRow(key)) {
          this.selectedTo.push(key)
        }
      },
      unselectId(key) {
        this.selectedTo = this.selectedTo.filter(function (item) {
          return item !== key
        })
      },
      isSelectedRow(key) {
        return this.selectedTo.indexOf(key) >= 0
      },
      rowSelected(dataItem, fieldName) {
        let idColumn = this.trackBy;
        let key = dataItem[idColumn];

        return this.isSelectedRow(key)
      },
      checkCheckboxesState(fieldName) {
        if (!this.tableData) return;

        let self = this;
        let idColumn = this.trackBy;
        let selector = 'th.vuecard-th-checkbox-' + idColumn + ' input[type=checkbox]';
        let els = document.querySelectorAll(selector);

        //fixed:document.querySelectorAll return the typeof nodeList not array
        if (els.forEach === undefined)
          els.forEach = function (cb) {
            [].forEach.call(els, cb);
          };

        // count how many checkbox row in the current page has been checked
        let selected = this.tableData.filter(function (item) {
          return self.selectedTo.indexOf(item[idColumn]) >= 0
        });

        // count == 0, clear the checkbox
        if (selected.length <= 0) {
          els.forEach(function (el) {
            el.indeterminate = false
          });
          return false
        }
        // count > 0 and count < perPage, set checkbox state to 'indeterminate'
        else if (selected.length < this.perPage) {
          els.forEach(function (el) {
            el.indeterminate = true
          });
          return true
        }
        // count == perPage, set checkbox state to 'checked'
        else {
          els.forEach(function (el) {
            el.indeterminate = false
          });
          return true
        }
      },
      toggleAllCheckboxes(fieldName, event) {
        let self = this;
        let isChecked = event.target.checked;
        let idColumn = this.trackBy;

        if (isChecked) {
          this.tableData.forEach(function (dataItem) {
            self.selectId(dataItem[idColumn])
          })
        } else {
          this.tableData.forEach(function (dataItem) {
            self.unselectId(dataItem[idColumn])
          })
        }
        this.emit1('vuecard:checkbox-toggled-all', isChecked)
      },
      gotoPreviousPage() {
        if (this.currentPage > 1) {
          this.currentPage--;
          this.loadData()
        }
      },
      gotoNextPage() {
        if (this.currentPage < this.tablePagination.last_page) {
          this.currentPage++;
          this.loadData()
        }
      },
      gotoPage(page) {
        if (page != this.currentPage && (page > 0 && page <= this.tablePagination.last_page)) {
          this.currentPage = page;
          this.loadData()
        }
      },
      isVisibleDetailRow(rowId) {
        return this.visibleDetailRows.indexOf(rowId) >= 0
      },
      showDetailRow(rowId) {
        if (!this.isVisibleDetailRow(rowId)) {
          this.visibleDetailRows.push(rowId)
        }
      },
      hideDetailRow(rowId) {
        if (this.isVisibleDetailRow(rowId)) {
          this.visibleDetailRows.splice(
            this.visibleDetailRows.indexOf(rowId),
            1
          )
        }
      },
      toggleDetailRow(rowId) {
        if (this.isVisibleDetailRow(rowId)) {
          this.hideDetailRow(rowId)
        } else {
          this.showDetailRow(rowId)
        }
      },
      showField(index) {
        if (index < 0 || index > this.tableFields.length) return;

        this.tableFields[index].visible = true
      },
      hideField(index) {
        if (index < 0 || index > this.tableFields.length) return;

        this.tableFields[index].visible = false
      },
      toggleField(index) {
        if (index < 0 || index > this.tableFields.length) return;

        this.tableFields[index].visible = !this.tableFields[index].visible
      },
      renderIconTag(classes, options = '') {
        return this.renderIcon === null
          ? `<i class="${classes.join(' ')}" ${options}></i>`
          : this.renderIcon(classes, options)
      },
      onRowClass(dataItem, index) {
        if (this.rowClassCallback !== '') {
          this.warn('"row-class-callback" prop is deprecated, please use "row-class" prop instead.');
          return
        }

        if (typeof(this.rowClass) === 'function') {
          return this.rowClass(dataItem, index)
        }

        return this.rowClass
      },
      onRowChanged(dataItem) {
        this.emit1('row-changed', dataItem);
        return true
      },
      onRowClicked(dataItem, event) {
        this.emit1(this.eventPrefix + 'row-clicked', dataItem, event);
        return true
      },
      onRowDoubleClicked(dataItem, event) {
        this.emit1(this.eventPrefix + 'row-dblclicked', dataItem, event)
      },
      onDetailRowClick(dataItem, event) {
        this.emit1(this.eventPrefix + 'detail-row-clicked', dataItem, event)
      },
      onCellClicked(dataItem, field, event) {
        this.emit1(this.eventPrefix + 'cell-clicked', dataItem, field, event)
      },
      onCellDoubleClicked(dataItem, field, event) {
        this.emit1(this.eventPrefix + 'cell-dblclicked', dataItem, field, event)
      },
      /*
       * API for externals
       */
      changePage(page) {
//        console.log("set page", page);
        if (page === 'prev') {
          this.gotoPreviousPage()
        } else if (page === 'next') {
          this.gotoNextPage()
        } else {
          this.gotoPage(page)
        }
      },
      reload() {

        this.loadData()
      },
      refresh() {
        this.currentPage = 1;
        this.loadData()
      },
      resetData() {
        this.tableData = null;
        this.tablePagination = null;
        this.emit1('data-reset')
      },
      reinit() {
        this.normalizeFields();
        this.$nextTick(function () {
          this.emit1('initialized', this.tableFields)
        });

        if (this.apiMode && this.loadOnStart) {
          this.loadData()
        }
        if (this.apiMode == false && this.data.length > 0) {
          this.setData(this.data)
        }
      },
    }, // end: methods
    watch: {
      'multiSort'(newVal, oldVal) {
        if (newVal === false && this.sortOrder.length > 1) {
          this.sortOrder.splice(1);
          this.loadData();
        }
      },
      'apiUrl': function (newVal, oldVal) {
        if (newVal !== oldVal)
          this.refresh()
      }
    },
  }
</script>

<style scoped>
  [v-cloak] {
    display: none;
  }

  .vuecard th.sortable:hover {
    color: #2185d0;
    cursor: pointer;
  }

  .vuecard-actions {
    width: 15%;
    padding: 12px 0px;
    text-align: center;
  }

  .vuecard-pagination {
    background: #f9fafb !important;
  }

  .vuecard-pagination-info {
    margin-top: auto;
    margin-bottom: auto;
  }
</style>
