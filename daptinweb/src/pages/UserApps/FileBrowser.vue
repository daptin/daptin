<template>

  <q-page-container style="height: 100vh">


    <q-dialog v-model="newNamePrompt" persistent>
      <q-card style="min-width: 350px">
        <q-card-section>
          <div class="text-h6">Name</div>
        </q-card-section>

        <q-card-section class="q-pt-none">
          <q-input dense v-model="newName" autofocus @keyup.enter="newNamePrompt = false"/>
        </q-card-section>

        <q-card-actions align="right" class="text-primary">
          <q-btn flat label="Cancel" v-close-popup/>
          <q-btn flat label="Create" @click="createNew()" v-close-popup/>
        </q-card-actions>
      </q-card>
    </q-dialog>


    <q-menu context-menu>
      <q-list dense style="min-width: 100px">
        <q-item @click="() => {(newNamePrompt = true) ; (newName = '') ; ( newNameType = 'file')}" clickable
                v-close-popup>
          <q-item-section>New file</q-item-section>
        </q-item>
        <q-separator/>
        <q-item @click="() => {(newNamePrompt = true) ; (newName = '') ; ( newNameType = 'folder')}" clickable
                v-close-popup>
          <q-item-section>New folder</q-item-section>
        </q-item>
        <q-separator/>
        <q-item @click="showUploader()" clickable v-close-popup>
          <q-item-section>Upload file</q-item-section>
        </q-item>

      </q-list>
    </q-menu>
    <q-header>
      <q-toolbar class="user-area-pattern text-black">
        <q-btn flat icon="fas fa-plus" @click="showUploader()" color="white"></q-btn>
        <q-btn flat icon="fas fa-search" v-if="!showSearchInput"
               @click="(showSearchInput = true) && ($refs.search.focus())" color="white"></q-btn>
        <q-input ref="search" v-if="showSearchInput" dense standout v-model="searchInput"
                 input-class="text-right text-white" class="q-ml-md text-white">
          <template v-slot:append>
            <q-icon v-if="searchInput === ''" name="fas fa-search" color="white"/>
            <q-icon v-else name="clear" class="cursor-pointer text-white"
                    @click="(showSearchInput = false) && (searchInput = '')"/>
          </template>
        </q-input>
        <q-btn flat icon="fas fa-sync-alt"
               @click="refreshData()" color="white"></q-btn>
      </q-toolbar>
    </q-header>

    <q-page>


      <div class="row q-pa-md text-white">


        <div class="col-12">
          <paginated-table-view @item-deleted="itemDelete" @item-clicked="fileClicked"
                                :items="files"></paginated-table-view>
        </div>

        <q-page-sticky :offset="[10, 10]" v-if="showUploadComponent">
          <q-card style="width: 300px; height: 200px">
            <daptin-document-uploader
              multiple
              @uploadComplete="refreshData()"
              ref="uploader"
              :uploadFile="uploadFile"
              :auto-upload="true"
              style="max-width: 300px; color: black"
            >
              <template v-slot:header="scope">
                <div class="bg-black row no-wrap items-center q-pa-sm q-gutter-xs">
                  <q-btn v-if="scope.uploadedFiles.length > 0" icon="done_all" @click="scope.removeUploadedFiles" round
                         dense flat>
                    <q-tooltip>Remove Uploaded Files</q-tooltip>
                  </q-btn>
                  <q-spinner v-if="scope.isUploading" class="q-uploader__spinner"/>
                  <div class="col">
                    <div class="q-uploader__title">Upload your files</div>
                  </div>
                  <q-btn v-if="scope.canAddFiles" type="a" icon="add_box" round dense flat>
                    <q-uploader-add-trigger/>
                    <q-tooltip>Pick Files</q-tooltip>
                  </q-btn>
                  <q-btn icon="fas fa-times" @click="showUploadComponent = false" round dense flat>
                    <q-tooltip>Close</q-tooltip>
                  </q-btn>
                </div>
              </template>
              <template v-slot:list="scope">
                <q-list separator>

                  <q-item v-for="file in scope.files" :key="file.name">
                    <q-item-section>
                      <q-item-label class="full-width ellipsis">
                        {{ file.name }}
                      </q-item-label>

                      <q-item-label caption>
                        Status: {{ file.__status }}
                      </q-item-label>

                    </q-item-section>

                    <q-item-section
                      v-if="file.__img"
                      thumbnail
                      class="gt-xs"
                    >
                      <img :src="file.__img.src">
                    </q-item-section>

                  </q-item>

                </q-list>
              </template>

            </daptin-document-uploader>

          </q-card>
        </q-page-sticky>
      </div>
    </q-page>


  </q-page-container>

</template>
<script>

import {mapActions} from "vuex";

function base64ToArrayBuffer(base64) {
  var binaryString = window.atob(base64);
  var binaryLen = binaryString.length;
  var bytes = new Uint8Array(binaryLen);
  for (var i = 0; i < binaryLen; i++) {
    var ascii = binaryString.charCodeAt(i);
    bytes[i] = ascii;
  }
  return bytes;
}

function saveByteArray(reportName, fileType, byte) {
  var blob = new Blob([byte], {type: fileType});
  var link = document.createElement('a');
  link.href = window.URL.createObjectURL(blob);
  var fileName = reportName;
  link.download = fileName;
  link.click();
};

export default {

  name: "FileBrowser",
  methods: {
    itemDelete(file) {
      console.log("Delete file", file);
      const that = this;
      this.deleteRow({
        tableName: "document",
        reference_id: file.reference_id
      }).then(function () {
        that.refreshData();
      }).catch(function (er) {
        that.$q.notify({
          message: er[0].title
        })
      })
    },
    fileClicked(file) {
      const that = this;
      console.log("File clicked", file);
      if (file.document_extension === "directory") {
        that.currentPath = file.document_path
      } else {
        that.$q.loading = true;
        that.loadData({
          tableName: "document",
          params: {
            query: JSON.stringify([{
              column: "reference_id",
              operator: "is",
              value: file.reference_id
            }]),
            "included_relations": "document_content",
            page: {
              size: 1,
            }
          }
        }).then(function (res) {
          that.$q.loading = false;
          // console.log("File ", res.data[0].document_content[0].contents);
          const file = res.data[0];
          saveByteArray(file.document_name, file.mime_type, base64ToArrayBuffer(res.data[0].document_content[0].contents))
        })

      }
    },
    createNew() {
      console.log("Create ", this.newNameType, this.newName);
      const that = this;
      var newRow = {
        document_name: this.newName,
        tableName: "document",
        document_extension: this.newName.indexOf(".") > -1 ? this.newName.split(".")[1] : "",
        mime_type: '',
        document_path: this.currentPath + this.newName,
        document_content: [{
          name: this.newName,
          type: "text/plain",
          contents: "data:base64," + btoa(""),
        }],
      }
      if (this.newNameType === "folder") {
        newRow.document_extension = "folder"
        newRow.document_content = []
      }

      this.createRow(newRow).then(function (res) {
        // resolve(file);
        that.refreshData();
      }).catch(function (e) {
        console.log("Failed to create", e)
        that.$q.notify({
          message: JSON.stringify(e)
        })
      });


    },
    ...mapActions(['loadData', 'createRow', 'loadModel', 'deleteRow']),
    refreshData() {
      const that = this;
      that.loadData({
        tableName: "document",
        params: {
          query: JSON.stringify([{
            column: "document_path",
            operator: "like",
            value: that.currentPath + "%"
          }]),
          page: {
            size: 1000,
          }
        }
      }).then(function (res) {
        console.log("Documents ", res);
        that.files = res.data.map(function (e) {
          e.color = "white"
          e.icon = "fas fa-file"
          e.name = e.document_name
          e.path = e.document_path

          if (e.name.endsWith("xlsx") || e.name.endsWith("xls")) {
            e.icon = "fas fa-file-excel"
          } else if (e.name.endsWith("doc") || e.name.endsWith("docx")) {
            e.icon = "fas fa-file-word"
          } else if (e.name.endsWith("ppt") || e.name.endsWith("pptx")) {
            e.icon = "fas fa-file-powerpoint"
          } else if (e.name.endsWith("pdf")) {
            e.icon = "fas fa-file-pdf"
          } else if (e.name.endsWith("txt") || e.name.endsWith("yaml") || e.name.endsWith("json")) {
            e.icon = "fas fa-file-alt"
          } else if (e.name.endsWith("html") || e.name.endsWith("xml") || e.name.endsWith("css")) {
            e.icon = "fas fa-file-code"
          } else if (e.name.endsWith("csv")) {
            e.icon = "fas fa-file-csv"
          } else if (e.name.endsWith("jpg") || e.name.endsWith("tiff") || e.name.endsWith("gif") || e.name.endsWith("png")) {
            e.icon = "fas fa-image"
          } else if (e.name.endsWith("mp3") || e.name.endsWith("wav") || e.name.endsWith("riff") || e.name.endsWith("ogg")) {
            e.icon = "fas fa-audio"
          } else if (e.name.endsWith("mp4") || e.name.endsWith("mkv") || e.name.endsWith("riff") || e.name.endsWith("m4a")) {
            e.icon = "fas fa-audio"
          } else if (e.name.endsWith("zip") || e.name.endsWith("rar") || e.name.endsWith("gz") || e.name.endsWith("tar")) {
            e.icon = "fas fa-file-archive"
          }

          return e;
        });
        that.files.unshift({
          name: '..',
          path: '..',
          icon: 'fas fa-folder',
          is_dir: true,
          color: "rgb(224, 135, 94)"
        })
        that.files.unshift({
          name: '.',
          path: '.',
          icon: 'fas fa-folder',
          is_dir: true,
          color: "rgb(224, 135, 94)"
        });

      })
    },
    uploadFile(file) {
      console.log("Upload file", file);
      const that = this;
      var obj = {
        tableName: "document",
        document_content: [{
          name: file.name,
          contents: file.file,
          type: file.type
        }],
        document_name: file.name,
        document_path: this.currentPath + file.name,
        mime_type: file.type,
        document_extension: file.name.indexOf(".") > -1 ? file.name.split(".")[1] : "",
      }

      return new Promise(function (resolve, reject) {
        that.createRow(obj).then(function (res) {
          resolve(file);
          // that.refreshData();
        }).catch(function (e) {
          reject(e)
        });
      });
    },
    showUploader() {
      const that = this;
      this.showUploadComponent = true
      setTimeout(function () {
        that.$refs.uploader.pickFiles()
      }, 100);
    },
  },
  data() {
    return {
      searchInput: '',
      newNamePrompt: false,
      newName: '',
      newNameType: '',
      currentPath: '/',
      showSearchInput: false,
      files: [],
      showUploadComponent: false,
      viewParameters: {
        tableName: 'document'
      },
      containerId: "id-" + new Date().getMilliseconds(),
      screenWidth: (window.screen.width < 1200 ? window.screen.width : 1200) + "px",
    }
  },
  mounted() {
    const that = this;
    this.containerId = "id-" + new Date().getMilliseconds();
    console.log("Mounted FilesBrowser", this.containerId);
    that.refreshData();

    // this.loadModel("document").then(function () {
    // }).catch(function (err) {
    //   that.$q.notify({
    //     message: "Failed to load documents"
    //   })
    // })

  }
}
</script>
