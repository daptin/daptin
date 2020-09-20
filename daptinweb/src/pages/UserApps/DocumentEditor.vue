<template>
  <q-page-container>

    <q-dialog v-model="showSharingBox" v-if="document">
      <q-card style="min-width: 33vw; width: 43vw">
        <q-item>
          <q-item-section avatar>
            <q-avatar>
              <q-icon name="fas fa-link" size="1.8em"></q-icon>
            </q-avatar>
          </q-item-section>
          <q-item-section>
            <span class="text-h6">Share</span>
          </q-item-section>
        </q-item>
        <q-separator/>
        <q-card-section>
          <q-btn-toggle @input="saveDocument()" v-model="document.permission" :options="[
            {
             value: 2097027,
             label: 'Enable'
            },
            {
             value: 16289,
             label: 'Disable'
            }
          ]">
          </q-btn-toggle>
        </q-card-section>
        <q-card-section v-if="document.permission === 2097027">
          <span class="text-bold">Sharing by link</span>
        </q-card-section>
        <q-card-section v-if="document.permission === 2097027">
<!--          <q-input readonly :value="endpoint() + '/asset/document/' + document.reference_id + '/document_content.' + document.document_extension"></q-input>-->
          <q-input readonly :value="endpoint() + '/#/apps/document/' + document.reference_id"></q-input>
        </q-card-section>
      </q-card>

    </q-dialog>

    <q-header class="bg-white text-black document-heading">
      <q-toolbar>
        <q-btn-group flat>
          <q-btn flat label="File">
            <q-menu>
              <q-list dense style="min-width: 100px">
                <q-item @click="newDocument()" clickable v-close-popup>
                  <q-item-section>New</q-item-section>
                </q-item>
                <q-item @click="$router.push('/apps/files')" clickable v-close-popup>
                  <q-item-section>Open</q-item-section>
                </q-item>
                <q-item clickable v-close-popup>
                  <q-item-section>Save as</q-item-section>
                </q-item>
                <q-item @click="window.print()" clickable v-close-popup>
                  <q-item-section>Print</q-item-section>
                </q-item>
                <q-item @click="$router.push('/apps/files')" clickable v-close-popup>
                  <q-item-section>Close</q-item-section>
                </q-item>
              </q-list>
            </q-menu>
          </q-btn>
          <q-btn flat label="Edit"></q-btn>
          <q-btn flat label="Format"></q-btn>
          <q-btn flat label="Data"></q-btn>
          <q-btn flat label="Help"></q-btn>
        </q-btn-group>
        <q-space></q-space>
        <q-btn @click="showSharingBox = true" class="text-primary" flat label="Share"></q-btn>
        <q-btn v-if="decodedAuthToken() !== null" size="1.2em" class="profile-image" flat
               :icon="'img:' + decodedAuthToken().picture">
          <q-menu>
            <div class="row no-wrap q-pa-md">

              <div class="column items-center">
                <q-avatar size="72px">
                  <img :src="decodedAuthToken().picture">
                </q-avatar>

                <div class="text-subtitle1 q-mt-md q-mb-xs">{{ decodedAuthToken().name }}</div>

                <q-btn
                  color="black"
                  label="Logout"
                  push
                  @click="logout()"
                  size="sm"
                  v-close-popup
                />
              </div>
            </div>
          </q-menu>
        </q-btn>
      </q-toolbar>
      <div class="row">
        <div class="12">

        </div>
      </div>
      <div class="row">
        <div class="col-12">
          <div class="document-editor__toolbar"></div>
        </div>
      </div>
    </q-header>
    <q-page>

      <main>
        <div>
          <div class="row-editor" style="overflow-y: scroll; height: 85vh">
            <div class="editor"></div>
          </div>
        </div>
      </main>

      <q-dialog v-model="newNameDialog">
        <q-card style="min-width: 400px">
          <q-card-section>
            <q-input label="New file name" v-model="newName"></q-input>
          </q-card-section>
          <q-card-actions align="right">
            <q-btn @click="newNameDialog = false" label="Cancel"></q-btn>
            <q-btn @click="newDocument()" color="primary" label="Create"></q-btn>
          </q-card-actions>
        </q-card>
      </q-dialog>
    </q-page>
  </q-page-container>
</template>
<style>
@import '../../statics/ckeditor/ckeditor.css';

@page {
  size: 5.5in 8.5in;
  margin-top: 2cm;
  margin-bottom: 2cm;
  margin-left: 2cm;
  margin-right: 2cm;
}

@page :right {
  @bottom-right {
    content: counter(page);
  }
}


@media print {
  .document-heading {
    display: none;
  }

  body[data-editor="DecoupledDocumentEditor"] .row-editor {
    background: white;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    width: 100vw !important;
    height: 100vh !important;
    border: none;
    box-shadow: none;
  }

  body[data-editor="DecoupledDocumentEditor"] .row-editor .editor {
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    width: 100vw !important;
    height: 100vh !important;
    border: none;
    box-shadow: none;
  }
}

/*.ck.ck-dropdown .ck-dropdown__panel.ck-dropdown__panel-visible {*/
/*  position: fixed !important;*/
/*  top: 100px;*/
/*}*/

body[data-editor="DecoupledDocumentEditor"] .row-editor .editor {
  /*width: 816px;*/
  /*height: 1056px;*/
}

body[data-editor="DecoupledDocumentEditor"] {
  background: #eeebeb;
  border: none;
}

.ck {
  /*overflow: hidden !important;*/
  /*height: 100% !important;*/
}
</style>
<script>
import {mapActions, mapGetters} from "vuex";
import '../../statics/ckeditor/ckeditor'

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

  name: "DocumentEditorApp",
  data() {
    return {
      file: null,
      showSharingBox: false,
      ...mapGetters(['endpoint', 'decodedAuthToken']),
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
    logout() {
      this.$emit("logout");
    },
    loadEditor() {
      const that = this;


      setTimeout(function () {


        const watchdog = new CKSource.Watchdog();

        window.watchdog = watchdog;

        watchdog.setCreator((element, config) => {
          return CKSource.Editor
            .create(element, config)
            .then(editor => {


              // Set a custom container for the toolbar.
              document.querySelector('.document-editor__toolbar').appendChild(editor.ui.view.toolbar.element);
              document.querySelector('.ck-toolbar').classList.add('ck-reset_all');


              that.editor = editor;
              editor.setData(that.contents)
              if (that.decodedAuthToken()) {
                const saveMethod = debounce(that.saveDocument, 1000, false)
                editor.model.document.on('change:data', () => {
                  that.contents = editor.getData();
                  console.log("Editor contents", that.contents)
                  saveMethod();
                });
              }

              return editor;
            })
        });

        watchdog.setDestructor(editor => {
          // Set a custom container for the toolbar.
          document.querySelector('.document-editor__toolbar').removeChild(editor.ui.view.toolbar.element);

          return editor.destroy();
        });

        watchdog.on('error', function (err) {
          console.log("Failed to create editor", err)
        });


        window.document.body.setAttribute("data-editor", "DecoupledDocumentEditor")
        watchdog
          .create(document.querySelector('.editor'), {

            toolbar: {
              items: [
                'undo',
                'redo',
                'removeFormat',
                '|',
                'heading',
                'fontSize',
                'fontFamily',
                'fontBackgroundColor',
                'fontColor',
                '|',
                'bold',
                'italic',
                'underline',
                'strikethrough',
                'highlight',
                '|',
                'numberedList',
                'bulletedList',
                'todoList',
                '|',
                'alignment',
                'indent',
                'outdent',
                '|',
                'link',
                'blockQuote',
                'imageUpload',
                'insertTable',
                'mediaEmbed'
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

          })
          .catch(error => {
            console.error('Oops, something went wrong!', error);
            console.error('Please, report the following error on https://github.com/ckeditor/ckeditor5/issues with the build id and the error stack trace:');
            console.warn('Build id: keu49w7chwo-c6p4ujty9ev0');
            console.error(error);
          });


      }, 100)

    },
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
      this.newName = null;
      this.newNameDialog = false;
      this.document.document_content = [this.file]
      this.contents = "";
      this.editor.setData("")
    },
    saveDocument() {
      const that = this;
      if (!this.document) {
        this.newNameDialog = true;
        return;
      }
      if (this.decodedAuthToken() === null) {
        return
      }
      this.document.tableName = "document";
      this.document.document_content[0].contents = "data:text/html," + btoa(this.contents)
      if (this.document.reference_id) {

        if (that.document.permission === 2097027) {
          that.loadData({
            tableName: "world",
            params: {
              query: JSON.stringify([{
                column: "table_name",
                operator: "is",
                value: "document"
              }]),
              page: {
                size: 1,
              }
            }
          }).then(function (res) {
            console.log("Document", res);
            var documentTable = res.data[0];
            if (documentTable.permission != that.document.permission) {
              that.updateRow({
                tableName: "world",
                id: documentTable.reference_id,
                permission: that.document.permission
              }).then(function (res) {
                console.log("Updated permission")
              }).catch(function (res) {
                console.log("Failed to get table document", res)
                that.$q.notify({
                  message: "Failed to check table permissions, share link might not be working"
                })
              })
            }
          }).catch(function (res) {
            console.log("Failed to get table document", res)
            that.$q.notify({
              message: "Failed to check table permissions, share link might not be working"
            })
          })
        }

        that.updateRow(that.document).then(function (res) {
          console.log("Document saved", res);
        }).catch(function (err) {
          console.log("error", err)
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
      this.newNameDialog = true;
      this.contents = "";
      that.loadEditor();
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
      that.loadEditor()

    })


  }
}
</script>
