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
      <q-bar>
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
                <q-item @click="pageSettingDialog = true" clickable v-close-popup>
                  <q-item-section>Page setting</q-item-section>
                </q-item>
                <q-item @click="saveDocument()" clickable v-close-popup>
                  <q-item-section>Save</q-item-section>
                </q-item>
                <q-item @click="printDocument()" clickable v-close-popup>
                  <q-item-section>Print</q-item-section>
                </q-item>
                <q-item @click="$router.push('/apps/files')" clickable v-close-popup>
                  <q-item-section>Close</q-item-section>
                </q-item>
              </q-list>
            </q-menu>
          </q-btn>
          <!--          <q-btn flat label="Edit"></q-btn>-->
          <!--          <q-btn flat label="Format">-->
          <!---->
          <!--          </q-btn>-->
          <!--          <q-btn flat label="Data"></q-btn>-->
          <!--          <q-btn flat label="Help"></q-btn>-->
        </q-btn-group>
        <q-space></q-space>
        <q-btn @click="showSharingBox = true" class="text-primary" flat label="Share"></q-btn>
        <q-btn v-if="decodedAuthToken() !== null" size="0.8em" class="profile-image" flat
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
      </q-bar>
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
    <q-dialog v-model="pageSettingDialog">
      <q-card>
        <q-card-section>
          <span class="text-h6">Page setting</span>
        </q-card-section>
        <q-card-section>
          <div class="row">
            <div class="col-6 q-pa-xs">
              <!--              <q-input v-model="pageSetting.height" type="number" label="Page height"></q-input>-->
              <q-input v-model="pageSetting.width" type="number" label="Page width"></q-input>
            </div>
            <div class="col-6  q-pa-xs">
              <q-input v-model="pageSetting.margin.top" type="number" label="Margin top"></q-input>
              <q-input v-model="pageSetting.margin.left" type="number" label="Margin left"></q-input>
              <q-input v-model="pageSetting.margin.right" type="number" label="Margin right"></q-input>
              <q-input v-model="pageSetting.margin.bottom" type="number" label="Margin bottom"></q-input>
            </div>
          </div>
        </q-card-section>
      </q-card>
    </q-dialog>
    <q-page>
      <main style="position: absolute; bottom: 0; left: 0; right: 0; top: -1px; overflow-y: scroll">
        <div>
          <div class="row-editor" v-for="page in pages">
            <div v-html="page.html" :id="page.id" class="editor"
                 :style="{
              'min-height': pageSetting.height + 'px',
              'width': pageSetting.width  + 'px',
              'padding-left': pageSetting.margin.left  + 'px',
              'padding-right': pageSetting.margin.right  + 'px',
              'padding-top': pageSetting.margin.top  + 'px',
              'padding-bottom': pageSetting.margin.bottom  + 'px',
            }"></div>
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

  body {
    background: #fff !important;
  }

  body[data-editor="DecoupledDocumentEditor"] .row-editor {
    background: white;
    top: 0;
    left: 0;
    border: none;
    box-shadow: none;
    position: absolute;
  }

  body[data-editor="DecoupledDocumentEditor"] .row-editor .editor {
    border: none !important;
    box-shadow: none !important;
    margin: 0 !important;
  }

}


body[data-editor="DecoupledDocumentEditor"] {
  background: #eeebeb;
  border: none;
}

</style>
<script>
import {mapActions, mapGetters} from "vuex";
import '../../statics/ckeditor/ckeditor'
import JSZip from 'jszip'

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
      pageSettingDialog: false,
      pageSetting: {
        height: 800,
        width: 600,
        header: {
          style: {
            height: "100px",
            "width": "13cm"
          },
        },
        margin: {
          top: 20,
          bottom: 20,
          left: 20,
          right: 20,
        }
      },
      pages: [{
        id: "page-1",
        html: "",
        header: ""
      }],
      pageHeight: 1200,
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
    printDocument() {
      setTimeout(function () {
        window.print();
      }, 100)
    },
    logout() {
      this.$emit("logout");
    },
    pageReflow(currentPage = 1) {
      const that = this;
      const nextPageNumber = currentPage + 1;
      let allItems = Array.prototype.slice.call(document.querySelector("#page-" + currentPage).children);
      console.log("All items", allItems);
      var currentHeight = 0;
      var currentPageItems = [];
      let currentItem = allItems[0];
      for (; true;) {
        currentHeight = currentItem ? currentItem.offsetTop + currentItem.offsetHeight : 0;
        if (!currentItem || currentHeight > that.pageSetting.height) {
          console.log("Page break here please", currentItem, currentHeight);
          let pageContents = currentPageItems.map(function (e) {
            return e.outerHTML
          }).join("");
          console.log("page contents", currentPage, pageContents);
          if (pageContents.length < 1) {
            return;
          }
          that.editor.setData("page-" + currentPage, pageContents);
          currentHeight = 0;
          that.pages.push({
            id: "page-" + nextPageNumber
          });
          (function (newPageName) {
            setTimeout(function () {
              var newPageDetails = {};
              newPageDetails[newPageName] = document.querySelector("#" + newPageName)
              that.editor.add(newPageDetails);

              var remainingItems = [];
              while (currentItem != null) {
                remainingItems.push(currentItem)
                currentItem = currentItem.nextSibling
              }
              if (remainingItems < 2) {
                return
              }
              that.editor.setData(newPageName, remainingItems.map(function (e) {
                return e.outerHTML
              }).join(""))
              that.pageReflow(nextPageNumber)
            }, 100);
          })("page-" + nextPageNumber);
          return;
        }
        if (!currentItem) {
          break;
        }
        currentPageItems.push(currentItem);
        currentItem = currentItem.nextSibling;
      }


    },
    loadEditor() {
      const that = this;


      setTimeout(function () {


        window.document.body.setAttribute("data-editor", "DecoupledDocumentEditor");


        CKSource.Editor.defaultConfig = {

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

        }

        CKSource.Editor
          .create({
            "page-1": document.querySelector('#page-1'),
          })
          .then(editor => {
            document.querySelector('.document-editor__toolbar').appendChild(editor.ui.view.toolbar.element);
            that.editor = editor;
            editor.setData("page-1", that.contents);
            // that.pageReflow()
            const saveMethod = debounce(that.saveDocument, 1000, false)
            if (that.decodedAuthToken()) {
              editor.onChange((res) => { //提供onChange方法获取数据
                // console.log("Editor on change", res)
                that.contents = editor.getData()["page-1"];
                // console.log("Editor contents", that.contents)
                saveMethod();

              })
            }


            window.editor = editor; //将实例暴露给window
          })
          .catch(err => {
            console.error(err);
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
      if (!this.newName.endsWith(".ddoc")) {
        this.newName = this.newName + ".ddoc"
      }

      var newFileName = null;
      newFileName = this.newName;


      this.document = {
        document_name: newFileName,
        document_extension: "html",
        mime_type: "text/html",
        document_path: localStorage.getItem("_last_current_path") || "/"
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

      var zip = new JSZip();
      zip.file("contents.html", this.contents);
      zip.file("page-setting.json", JSON.stringify(this.pageSetting));

      if (that.document.document_content == null || that.document.document_content.length < 0) {
        that.document.document_content = [{
          name: this.document.document_name,
          type: "application/x-ddocument",
          path: localStorage.getItem("_last_current_path")
        }]
      }

      zip.generateAsync({type: "base64"}).then(function (base64) {
        that.document.document_content[0].contents = "data:application/x-ddocument," + base64

        if (that.document.reference_id) {

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
      });


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
      if (!that.document.document_content) {
        that.loadEditor();
        return;
      }

      if (that.document.document_content.length > 0) {
        that.file = that.document.document_content[0];
      } else {
        that.loadEditor();
        return;
      }
      JSZip.loadAsync(atob(that.file.contents)).then(function (zipFile) {
        // that.contents = atob(that.file.contents);
        zipFile.file("contents.html").async("string").then(function (data) {
          // data is "Hello World\n"
          // console.log("Loaded file: ", data)
          that.contents = data;
          that.loadEditor()
        }).catch(function (err) {
          console.log("Failed to open contents.html", err)
          that.loadEditor()
        });


      }).catch(function (err) {
        console.log("Failed to load zip file", err)
        that.loadEditor()
      });


    })


  }
}
</script>
