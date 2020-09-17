<template>
  <q-page-container>
    <q-header elevated>

      <div class="q-pa-sm q-pl-md row items-center">
        <div class="cursor-pointer non-selectable">
          File
          <q-menu>
            <q-list dense style="min-width: 100px">
              <q-item @click="$router.push('/apps/files')" clickable v-close-popup>
                <q-item-section>Open...</q-item-section>
              </q-item>
              <q-item @click="newDocument" clickable v-close-popup>
                <q-item-section>New</q-item-section>
              </q-item>

              <q-separator/>

              <q-item clickable>
                <q-item-section>Preferences</q-item-section>
                <q-item-section side>
                  <q-icon name="keyboard_arrow_right"/>
                </q-item-section>

                <q-menu anchor="top right" self="top left">
                  <q-list>
                    <q-item
                      v-for="n in 3"
                      :key="n"
                      dense
                      clickable
                    >
                      <q-item-section>Submenu Label</q-item-section>
                      <q-item-section side>
                        <q-icon name="keyboard_arrow_right"/>
                      </q-item-section>
                      <q-menu auto-close anchor="top right" self="top left">
                        <q-list>
                          <q-item
                            v-for="n in 3"
                            :key="n"
                            dense
                            clickable
                          >
                            <q-item-section>3rd level Label</q-item-section>
                          </q-item>
                        </q-list>
                      </q-menu>
                    </q-item>
                  </q-list>
                </q-menu>
              </q-item>

              <q-separator/>

              <q-item @click="$router.back()" clickable v-close-popup>
                <q-item-section>Quit</q-item-section>
              </q-item>
            </q-list>
          </q-menu>
        </div>

        <div class="q-ml-md cursor-pointer non-selectable">
          Edit
          <q-menu auto-close>
            <q-list dense style="min-width: 100px">
              <q-item clickable>
                <q-item-section>Cut</q-item-section>
              </q-item>
              <q-item clickable>
                <q-item-section>Copy</q-item-section>
              </q-item>
              <q-item clickable>
                <q-item-section>Paste</q-item-section>
              </q-item>
              <q-separator/>
              <q-item clickable>
                <q-item-section>Select All</q-item-section>
              </q-item>
            </q-list>
          </q-menu>
        </div>
      </div>
    </q-header>
    <q-page>
      <div class="row">
        <div class="col-12">
          <div class="document-editor" style="height: 100vh">
            <div class="row">
              <div class="document-editor__toolbar"></div>
            </div>
            <div class="row row-editor">
              <div class="editor">
              </div>
            </div>
          </div>
        </div>
      </div>
      <q-dialog v-model="newNameDialog">
        <q-card style="min-width: 400px">
          <q-card-section>
            <q-input label="New file name" v-model="newName"></q-input>
          </q-card-section>
          <q-card-actions align="right">
            <q-btn @click="newNameDialog = false" label="Cancel"></q-btn>
            <q-btn @click="newDocument()" label="Create"></q-btn>
          </q-card-actions>
        </q-card>
      </q-dialog>
    </q-page>
  </q-page-container>
</template>
<style>
/**
 * @license Copyright (c) 2014-2020, CKSource - Frederico Knabben. All rights reserved.
 * This file is licensed under the terms of the MIT License (see LICENSE.md).
 */

:root {
  --ck-sample-base-spacing: 2em;
  --ck-sample-color-white: #fff;
  --ck-sample-color-green: #279863;
  --ck-sample-color-blue: #1a9aef;
  --ck-sample-container-width: 1285px;
  --ck-sample-sidebar-width: 350px;
  --ck-sample-editor-min-height: 400px;
}

/* --------- EDITOR STYLES  ---------------------------------------------------------------------------------------- */

.editor__editable,
  /* Classic build. */
main .ck-editor[role='application'] .ck.ck-content,
  /* Decoupled document build. */
.ck.editor__editable[role='textbox'],
.ck.ck-editor__editable[role='textbox'],
  /* Inline & Balloon build. */
.ck.editor[role='textbox'] {
  width: 100%;
  background: #fff;
  font-size: 1em;
  line-height: 1.6em;
  min-height: var(--ck-sample-editor-min-height);
  padding: 1.5em 2em;
}

.ck.ck-editor__editable {
  background: #fff;
  border: 1px solid hsl(0, 0%, 70%);
  width: 100%;
}

.ck.ck-editor {
  /* To enable toolbar wrapping. */
  width: 100%;
  overflow-x: hidden;
}

/* Because of sidebar `position: relative`, Edge is overriding the outline of a focused editor. */
.ck.ck-editor__editable {
  position: relative;
  z-index: 10;
}

/* --------- DECOUPLED (DOCUMENT) BUILD. ---------------------------------------------*/
body[data-editor='DecoupledDocumentEditor'] .document-editor__toolbar {
  width: 100%;
}

body[ data-editor='DecoupledDocumentEditor'] .collaboration-demo__editable,
body[ data-editor='DecoupledDocumentEditor'] .row-editor .editor {
  width: 18.5cm;
  height: 100%;
  min-height: 26.25cm;
  padding: 1.75cm 1.5cm;
  margin: 2.5rem;
  border: 1px hsl(0, 0%, 82.7%) solid;
  background-color: var(--ck-sample-color-white);
  box-shadow: 0 0 5px hsla(0, 0%, 0%, .1);
}

body[ data-editor='DecoupledDocumentEditor'] .row-editor {
  display: flex;
  position: relative;
  justify-content: center;
  overflow-y: auto;
  background-color: #f2f2f2;
  border: 1px solid hsl(0, 0%, 77%);
}

body[data-editor='DecoupledDocumentEditor'] .sidebar {
  background: transparent;
  border: 0;
  box-shadow: none;
}

/* --------- COMMENTS & TRACK CHANGES FEATURE ---------------------------------------------------------------------- */
.sidebar {
  padding: 0 15px;
  position: relative;
  min-width: var(--ck-sample-sidebar-width);
  max-width: var(--ck-sample-sidebar-width);
  font-size: 20px;
  border: 1px solid hsl(0, 0%, 77%);
  background: hsl(0, 0%, 98%);
  border-left: 0;
  overflow: hidden;
  min-height: 100%;
  flex-grow: 1;
}

/* Do not inherit styles related to the editable editor content. See line 25.*/
.sidebar .ck-content[role='textbox'],
.ck.ck-annotation-wrapper .ck-content[role='textbox'] {
  min-height: unset;
  width: unset;
  padding: 0;
  background: transparent;
}

.sidebar.narrow {
  min-width: 60px;
  flex-grow: 0;
}

.sidebar.hidden {
  display: none !important;
}

#sidebar-display-toggle {
  position: absolute;
  z-index: 1;
  width: 30px;
  height: 30px;
  text-align: center;
  left: 15px;
  top: 30px;
  border: 0;
  padding: 0;
  color: hsl(0, 0%, 50%);
  transition: 250ms ease color;
  background-color: transparent;
}

#sidebar-display-toggle:hover {
  color: hsl(0, 0%, 30%);
  cursor: pointer;
}

#sidebar-display-toggle:focus,
#sidebar-display-toggle:active {
  outline: none;
  border: 1px solid #a9d29d;
}

#sidebar-display-toggle svg {
  fill: currentColor;
}

/* --------- COLLABORATION FEATURES (USERS) ------------------------------------------------------------------------ */
.row-presence {
  width: 100%;
  border: 1px solid hsl(0, 0%, 77%);
  border-bottom: 0;
  background: hsl(0, 0%, 98%);
  padding: var(--ck-spacing-small);

  /* Make `border-bottom` as `box-shadow` to not overlap with the editor border. */
  box-shadow: 0 1px 0 0 hsl(0, 0%, 77%);

  /* Make `z-index` bigger than `.editor` to properly display tooltips. */
  z-index: 20;
}

.ck.ck-presence-list {
  flex: 1;
  padding: 1.25rem .75rem;
}

.presence .ck.ck-presence-list__counter {
  order: 2;
  margin-left: var(--ck-spacing-large)
}

/* --------- REAL TIME COLLABORATION FEATURES (SHARE TOPBAR CONTAINER) --------------------------------------------- */
.collaboration-demo__row {
  display: flex;
  position: relative;
  justify-content: center;
  overflow-y: auto;
  background-color: #f2f2f2;
  border: 1px solid hsl(0, 0%, 77%);
}

body[ data-editor='InlineEditor'] .collaboration-demo__row {
  border: 0;
}

.collaboration-demo__container {
  max-width: var(--ck-sample-container-width);
  margin: 0 auto;
  padding: 1.25rem;
}

.presence, .collaboration-demo__row {
  transition: .2s opacity;
}

.collaboration-demo__topbar {
  background: #fff;
  border: 1px solid var(--ck-color-toolbar-border);
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 0;
  border-radius: 4px 4px 0 0;
}

.collaboration-demo__topbar .btn {
  margin-right: 1em;
  outline-offset: 2px;
  outline-width: 2px;
  background-color: var(--ck-sample-color-blue);
}

.collaboration-demo__topbar .btn:focus,
.collaboration-demo__topbar .btn:hover {
  border-color: var(--ck-sample-color-blue);
}

.collaboration-demo__share {
  display: flex;
  align-items: center;
  padding: 1.25rem .75rem
}

.collaboration-demo__share-description p {
  margin: 0;
  font-weight: bold;
  font-size: 0.9em;
}

.collaboration-demo__share input {
  height: auto;
  font-size: 0.9em;
  min-width: 220px;
  margin: 0 10px;
  border-radius: 4px;
  border: 1px solid var(--ck-color-toolbar-border)
}

.collaboration-demo__share button,
.collaboration-demo__share input {
  height: 40px;
  padding: 5px 10px;
}

.collaboration-demo__share button {
  position: relative;
}

.collaboration-demo__share button:focus {
  outline: none;
}

.collaboration-demo__share button[data-tooltip]::before,
.collaboration-demo__share button[data-tooltip]::after {
  position: absolute;
  visibility: hidden;
  opacity: 0;
  pointer-events: none;
  transition: all .15s cubic-bezier(.5, 1, .25, 1);
  z-index: 1;
}

.collaboration-demo__share button[data-tooltip]::before {
  content: attr(data-tooltip);
  padding: 5px 15px;
  border-radius: 3px;
  background: #111;
  color: #fff;
  text-align: center;
  font-size: 11px;
  top: 100%;
  left: 50%;
  margin-top: 5px;
  transform: translateX(-50%);
}

.collaboration-demo__share button[data-tooltip]::after {
  content: '';
  border: 5px solid transparent;
  width: 0;
  font-size: 0;
  line-height: 0;
  top: 100%;
  left: 50%;
  transform: translateX(-50%);
  border-bottom: 5px solid #111;
  border-top: none;
}

.collaboration-demo__share button[data-tooltip]:hover:before,
.collaboration-demo__share button[data-tooltip]:hover:after {
  visibility: visible;
  opacity: 1;
}

.collaboration-demo--ready {
  overflow: visible;
  height: auto;
}

.collaboration-demo--ready .presence,
.collaboration-demo--ready .collaboration-demo__row {
  opacity: 1;
}

/* --------- SAMPLE GENERIC STYLES (not related to CKEditor) ------------------------------------------------------- */
body, html {
  padding: 0;
  margin: 0;

  font-family: sans-serif, Arial, Verdana, "Trebuchet MS", "Apple Color Emoji", "Segoe UI Emoji", "Segoe UI Symbol";
  font-size: 16px;
  line-height: 1.5;
}

body {
  height: 100%;
  color: #2D3A4A;
}

body * {
  box-sizing: border-box;
}

a {
  color: #38A5EE;
}

header .centered {
  display: flex;
  flex-flow: row nowrap;
  justify-content: space-between;
  align-items: center;
  min-height: 8em;
}

header h1 a {
  font-size: 20px;
  display: flex;
  align-items: center;
  color: #2D3A4A;
  text-decoration: none;
}

header h1 img {
  display: block;
  height: 64px;
}

header nav ul {
  margin: 0;
  padding: 0;
  list-style-type: none;
}

header nav ul li {
  display: inline-block;
}

header nav ul li + li {
  margin-left: 1em;
}

header nav ul li a {
  font-weight: bold;
  text-decoration: none;
  color: #2D3A4A;
}

header nav ul li a:hover {
  text-decoration: underline;
}

main .message {
  padding: 0 0 var(--ck-sample-base-spacing);
  background: var(--ck-sample-color-green);
  color: var(--ck-sample-color-white);
}

main .message::after {
  content: "";
  z-index: -1;
  display: block;
  height: 10em;
  width: 100%;
  background: var(--ck-sample-color-green);
  position: absolute;
  left: 0;
}

main .message h2 {
  position: relative;
  padding-top: 1em;
  font-size: 2em;
}

.centered {
  /* Hide overlapping comments. */
  overflow: hidden;
  max-width: var(--ck-sample-container-width);
  margin: 0 auto;
  padding: 0 var(--ck-sample-base-spacing);
}

.row {
  display: flex;
  position: relative;
}

.btn {
  cursor: pointer;
  padding: 8px 16px;
  font-size: 1rem;
  user-select: none;
  border-radius: 4px;
  transition: color .2s ease-in-out, background-color .2s ease-in-out, border-color .2s ease-in-out, opacity .2s ease-in-out;
  background-color: var(--ck-sample-color-button-blue);
  border-color: var(--ck-sample-color-button-blue);
  color: var(--ck-sample-color-white);
  display: inline-block;
}

.btn--tiny {
  padding: 6px 12px;
  font-size: .8rem;
}

footer {
  margin: calc(2 * var(--ck-sample-base-spacing)) var(--ck-sample-base-spacing);
  font-size: .8em;
  text-align: center;
  color: rgba(0, 0, 0, .4);
}

/* --------- RWD --------------------------------------------------------------------------------------------------- */
@media screen and ( max-width: 800px ) {
  :root {
    --ck-sample-base-spacing: 1em;
  }

  header h1 {
    width: 100%;
  }

  header h1 img {
    height: 40px;
  }

  header nav ul {
    text-align: right;
  }

  main .message h2 {
    font-size: 1.5em;
  }
}

</style>
<script>
import {mapActions} from "vuex";


// import DecoupledEditor from './ckeditor';
// import Base64UploadAdapter from '@ckeditor/ckeditor5-upload/src/adapters/base64uploadadapter';

// console.log("DecoupledEditor", DecoupledEditor.Dw)
// DecoupledEditor.builtinPlugins.map( plugin => console.log(plugin.pluginName) );

function debounce(func, wait, immediate) {
  var timeout;
  return function () {
    var context = this, args = arguments;
    var later = function () {
      timeout = null;
      if (!immediate) func.apply(context, args);
    };
    var callNow = immediate && !timeout;
    clearTimeout(timeout);
    timeout = setTimeout(later, wait);
    if (callNow) func.apply(context, args);
  };
}


export default {

  name: "FilesApp",
  data() {
    return {
      file: null,
      contents: "",
      newNameDialog: false,
      newName: null,
      document: null,
      containerId: "id-" + new Date().getMilliseconds(),
      screenWidth: (window.screen.width < 1200 ? window.screen.width : 1200) + "px",
    }
  },
  watch: {
    'contents': function (newVal, oldVal) {
      // console.log("Contents changed", arguments)
    }
  },
  methods: {
    newDocument() {

      if (!this.newNameDialog) {
        this.newNameDialog = true;
        return;
      }

      if (!this.newName) {
        this.$q.notify({
          message: "Please enter a name"
        });
        return
      }

      var newFileName = null;
      newFileName = this.newName;


      this.document = {
        document_name: newFileName,
        document_extension: "html",
        mime_type: "text/html",
        document_path: "/"
      }

      this.file = {
        contents: "",
        name: newFileName,
        type: "text/html"
      }
      this.newName =  null;
      this.newNameDialog =  false;
      this.document.document_content = [this.file]
      this.contents = "";
      this.editor.setData("")
    },
    saveDocument() {
      const that = this;
      console.log("save document", this.document, this.contents)
      this.document.tableName = "document";
      this.document.document_content[0].contents = "data:text/html," + btoa(this.contents)
      if (this.document.reference_id) {


        that.updateRow(that.document).then(function (res) {
          console.log("Document saved", res);
        }).catch(function (err) {
          console.log("errer", err)
          that.$q.notify({
            message: "We are offline, changes are not being stored"
          })
        })
      } else {
        that.createRow(that.document).then(function (res) {
          that.document = res.data;
          console.log("Document created", res);
          that.$router.push('/apps/document/' + that.document.reference_id)
        }).catch(function (err) {
          console.log("errer", err)
          that.$q.notify({
            message: "We are offline, changes are not being stored"
          })
        })

      }
    },
    ...mapActions(['loadData', 'updateRow', 'createRow'])
  },
  mounted() {
    const that = this;
    this.containerId = "id-" + new Date().getMilliseconds();
    var documentId = this.$route.params.documentId;
    console.log("Mounted FilesApp", this.containerId, this.$route.params.documentId);
    if (documentId === "new") {
      this.file = {
        contents: "",
        name: "New file.html"
      }
      this.contents = "";
      return
    }


    that.loadData({
      tableName: 'document',
      params: {
        query: JSON.stringify([
          {
            column: "reference_id",
            operator: "is",
            value: documentId
          }
        ]),
        included_relations: "document_content"
      }
    }).then(function (res) {
      console.log("Loaded document", res.data)
      that.document = res.data[0];
      that.file = that.document.document_content[0];
      that.contents = atob(that.file.contents);

      setTimeout(function () {

        DecoupledDocumentEditor
          .create(document.querySelector('.editor'), {

            toolbar: {
              items: [
                'heading',
                '|',
                'fontSize',
                'fontFamily',
                '|',
                'bold',
                'italic',
                'underline',
                'strikethrough',
                'highlight',
                'fontBackgroundColor',
                'fontColor',
                'removeFormat',
                '|',
                'pageBreak',
                'horizontalLine',
                'alignment',
                '|',
                'numberedList',
                'bulletedList',
                '|',
                'indent',
                'outdent',
                '|',
                'todoList',
                'link',
                'blockQuote',
                'imageUpload',
                'insertTable',
                'mediaEmbed',
                '|',
                'undo',
                'redo',
                '|',
                'superscript',
                'subscript',
                'specialCharacters'
              ]
            },
            language: 'en',
            image: {
              toolbar: [
                'imageTextAlternative',
                'imageStyle:full',
                'imageStyle:side'
              ]
            },
            table: {
              contentToolbar: [
                'tableColumn',
                'tableRow',
                'mergeTableCells',
                'tableCellProperties',
                'tableProperties'
              ]
            },
            licenseKey: '',

          })
          .then(editor => {
            that.editor = editor;
            editor.setData(that.contents)


            // Set a custom container for the toolbar.
            document.querySelector('.document-editor__toolbar').appendChild(editor.ui.view.toolbar.element);
            document.querySelector('.ck-toolbar').classList.add('ck-reset_all');

            const saveMethod = debounce(that.saveDocument, 1000, false)
            editor.model.document.on('change:data', () => {
              that.contents = editor.getData();
              saveMethod();
            });


          })
          .catch(error => {
            console.error('Oops, something went wrong!');
            console.error('Please, report the following error on https://github.com/ckeditor/ckeditor5/issues with the build id and the error stack trace:');
            console.warn('Build id: keu49w7chwo-c6p4ujty9ev0');
            console.error(error);
          });


      }, 100)
    })


  }
}
</script>
