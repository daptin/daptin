<template>

  <q-page-container style="height: 100vh; overflow: hidden;">


    <q-dialog v-model="newNamePrompt" persistent>
      <q-card style="min-width: 350px">
        <q-card-section>
          <div class="text-h6">Name</div>
        </q-card-section>

        <q-card-section class="q-pt-none">
          <q-input dense v-model="newName" autofocus @keyup.enter="createNew()"/>
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

      </q-list>
    </q-menu>

    <q-page>
      <user-header-bar style="border-bottom: 1px solid black" @search="searchDocuments" @show-uploader="showUploader"
                       :buttons="{
        before: [
            {icon: 'fas fa-search', event: 'search'},
          ],
        after: [
            {icon: viewMode === 'card' ? 'fas fa-th-list' : 'fas fa-th-large', click: () => {viewMode = viewMode === 'card' ? 'table' : 'card'}},
            {icon: 'fas fa-sync-alt', event: 'search'},
          ],
        }" title="Files"></user-header-bar>

      <div style="height: 100vh; overflow-y: scroll" class="row">
        <div class="col-2 col-sm-12 col-md-2 col-lg-2 col-xl-2 col-xs-12">
          <q-card v-if="selectedFile && !selectedFile.is_dir" flat style="background: transparent;">
            <q-card-section>
              <span class="text-bold">{{ selectedFile.name }}</span><br/>
            </q-card-section>
            <q-card-section>
              Size <span class="text-bold">{{ parseInt(selectedFile.document_content[0].size / 1024) }} Kb</span> <br/>
              Type <span class="text-bold">{{ selectedFile.mime_type }}</span>
            </q-card-section>
            <q-card-section>
              <q-list separator bordered>
                <q-item clickable @click="fileDownload(selectedFile)">
                  <q-item-section>Download</q-item-section>
                </q-item>
                <q-item clickable v-if="isEditable(selectedFile)"
                        @click="openEditor(selectedFile)">
                  <q-item-section>Open</q-item-section>
                </q-item>
                <q-item clickable v-if="isViewable(selectedFile)"
                        @click="openViewer(selectedFile)">
                  <q-item-section>View</q-item-section>
                </q-item>
              </q-list>
            </q-card-section>
          </q-card>


          <q-card flat>
            <q-card-section>
              <q-list bordered separator>
                <q-item @click="$router.push('/apps/document/new')" clickable>
                  <q-item-section>New document</q-item-section>
                </q-item>
                <q-item @click="$router.push('/apps/spreadsheet/new')" clickable>
                  <q-item-section>New spreadsheet</q-item-section>
                </q-item>
                <q-item clickable @click="() => {(newNamePrompt = true) ; (newName = '') ; ( newNameType = 'file')}">
                  <q-item-section>New file</q-item-section>
                </q-item>
              </q-list>
            </q-card-section>
            <q-card-section>

            </q-card-section>

          </q-card>
          <q-card style="border: 1px dashed black; font-size: 10px; box-shadow: none">
            <file-upload
              :multiple="true"
              style="height: 300px; width: 100%; text-align: left"
              ref="upload"
              :drop="true"
              :drop-directory="true"
              v-model="uploadedFiles"
              post-action="/post.method"
              put-action="/put.method"
              @input-file="uploadFile"
            >
              <div class="container">

                <div class="row q-pa-xs">
                  <div class="col-12 ">
                    <table style="width: 100%">
                      <thead v-if="uploadedFiles.length > 0">
                      <tr>
                        <th style="text-align: left">File</th>
                        <th style="text-align: right">Size</th>
                        <th style="text-align: right">Status</th>
                      </tr>
                      </thead>
                      <tbody>
                      <tr v-for="file in uploadedFiles">
                        <td style="text-align: left"> {{ file.name }}</td>
                        <td style="text-align: right">{{ parseInt(file.size / 1024) }} Kb</td>
                        <td style="text-align: right">{{ file.status }}</td>
                      </tr>
                      </tbody>
                    </table>
                  </div>
                </div>
                <div style="padding: 10px" class="row">
                  <div class="col-12" style="height: 100%; ">
                <span class="vertical-middle" v-if="uploadedFiles.length === 0">
                  Click here to select files, or drag and drop files here to upload</span>
                  </div>
                </div>

              </div>
            </file-upload>

          </q-card>

        </div>
        <div class="col-10 col-sm-12 col-md-10 col-lg-10 col-xl-10 col-xs-12">
          <paginated-table-view v-if="viewMode === 'table'"
                                @item-deleted="itemDelete"
                                @item-rename="itemRename"
                                @item-double-clicked="fileDblClicked"
                                @item-clicked="fileClicked"
                                :items="files"></paginated-table-view>
          <paginated-card-view v-if="viewMode === 'card'"
                               @item-deleted="itemDelete"
                               @item-rename="itemRename"
                               @item-clicked="fileClicked"
                               @item-double-clicked="fileDblClicked"
                               :items="files"></paginated-card-view>
        </div>
      </div>
      <!--      <q-page-sticky :offset="[10, 10]" v-if="showUploadComponent">-->
      <!--        -->
      <!--      </q-page-sticky>-->
    </q-page>


  </q-page-container>

</template>
<script>

import {mapActions, mapGetters} from "vuex";

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
}


export default {

  name: "FileBrowser",
  watch: {
    'currentPath': function (newVal) {
      console.log("Current path changed", newVal);
      localStorage.setItem("_last_current_path", newVal)
    }
  },
  methods: {
    searchDocuments(query) {
      console.log("search documents", query);
      this.refreshData(query);
    },
    itemRename(file) {
      console.log("rename item", file);
    },
    fileDblClicked(file) {
      console.log("Item double click", file)
      if (file.is_dir) {
        this.fileDownload(file);
      } else if (this.isEditable(file)) {
        this.openEditor(file)
      } else if (this.isViewable(file)) {
        this.openViewer(file)
      } else {
        this.fileDownload(file);
      }

    },
    isEditable(selectedFile) {
      // console.log("Check file is editable", selectedFile)
      var ext = ["ddoc", "dsheet"]
      let fileExtension = "";
      if (selectedFile.document_name.indexOf(".") > -1) {
        fileExtension = selectedFile.document_name.split(".")[1];
      }
      console.log("Check file extension", fileExtension)

      return ext.filter(function (r) {
        return r === fileExtension
      }).length > 0;
    },
    isViewable(selectedFile) {
      // console.log("Check file is editable", selectedFile)
      var ext = ["jpg", "png", "gif", "txt", "pdf", "mp4", "mp3", "wav", "mkv"]
      let fileExtension = "";
      if (selectedFile.document_name.indexOf(".") > -1) {
        fileExtension = selectedFile.document_name.split(".")[1];
      }
      return ext.filter(function (r) {
        return r === fileExtension
      }).length > 0;
    },
    openEditor(file) {
      var fileExtention = file.document_name.split(".")[1]
      switch (fileExtention) {
        case "ddoc":
          this.$router.push('/apps/document/' + file.reference_id)
          return;
        case "dsheet":
          this.$router.push('/apps/spreadsheet/' + file.reference_id)
          return;
      }
    },
    openViewer(file) {
      var fileExtension = file.document_name.split(".")[1]
      switch (fileExtension) {
        case "ddoc":
          this.$router.push('/apps/document/' + file.reference_id)
          return;
        case "dsheet":
          this.$router.push('/apps/spreadsheet/' + file.reference_id)
          return;
      }
    },
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
      console.log("file clicked", file)
      this.selectedFile = file;
    },
    fileDownload(file) {
      const that = this;
      console.log("File clicked", file);
      if (file.is_dir) {
        if (file.name === ".") {
          that.refreshData();
        } else if (file.name === "..") {
          let pathParts = this.currentPath.split("/");
          if (pathParts.length > 1) {
            pathParts.pop();
          }
          let newPath = pathParts.join("/");
          console.log("one level up %s", newPath)
          this.currentPath = newPath
        } else {
          that.currentPath = file.document_path + file.name
        }
        that.refreshData();
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
      console.log("Create ", this.newNameType, this.newName, this.currentPath);
      const that = this;
      var newRow = {
        document_name: this.newName,
        tableName: "document",
        document_extension: this.newName.indexOf(".") > -1 ? this.newName.split(".")[1] : "",
        mime_type: '',
        document_path: this.currentPath + "/",
        document_content: [{
          name: this.newName,
          type: "text/plain",
          path: this.currentPath,
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
        that.newNamePrompt = false;
      }).catch(function (e) {
        console.log("Failed to create", e)
        that.$q.notify({
          message: JSON.stringify(e)
        });
      });


    },
    ...mapActions(['loadData', 'createRow', 'loadModel', 'deleteRow']),
    handleDataLoad(documentList) {
      const that = this;
      if (documentList.data === null) {
        that.$q.notify({
          message: "Error fetching files"
        })
        return
      }
      console.log("Documents ", documentList);
      that.files = documentList.data.map(function (e) {
        // e.color = "white"
        e.icon = "fas fa-file"
        e.name = e.document_name
        e.path = e.document_path

        if (e.name.endsWith("xlsx") || e.name.endsWith("xls")) {
          e.icon = "fas fa-file-excel"
        } else if (e.name.endsWith(".doc") || e.name.endsWith("docx")) {
          e.icon = "fas fa-file-word"
        } else if (e.name.endsWith("dsheet")) {
          e.icon = "fas fa-border-none"
        } else if (e.name.endsWith("ddoc")) {
          e.icon = "fas fa-file-alt"
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
          e.icon = "fas fa-file-audio"
        } else if (e.name.endsWith("mp4") || e.name.endsWith("mkv") || e.name.endsWith("riff") || e.name.endsWith("m4a")) {
          e.icon = "fas fa-file-video"
        } else if (e.name.endsWith("zip") || e.name.endsWith("rar") || e.name.endsWith("gz") || e.name.endsWith("tar")) {
          e.icon = "fas fa-file-archive"
        }
        if (e.document_extension === "folder") {
          e.icon = "fas fa-folder"
          e.is_dir = true
          e.color = "rgb(224, 135, 94)"

        }

        return e;
      });
      if (that.currentPath !== "") {
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

      }

    },
    refreshData(searchTerm) {
      const that = this;
      that.selectedFile = null;
      let queryPayload = {
        tableName: "document",
        params: {
          query: JSON.stringify([{
            column: "document_path",
            operator: "is",
            value: that.currentPath + "/"
          }]),
          page: {
            size: 100,
          }
        }
      };
      if (searchTerm && searchTerm.trim().length > 0) {
        queryPayload.params.query = JSON.stringify([
          {
            column: "document_name",
            operator: "contains",
            value: searchTerm
          }
        ])
      }


      that.files = [];
      console.log("Query data")

      that.loadData(queryPayload).then(function (res) {
        console.log("data load complete")
        that.handleDataLoad(res)
      });


    },
    ensureDirectory(path) {
      const that = this;
      if (path === "/") {
        return
      }
      if (that.directoryEnsureCache[path]) {
        return
      }
      that.directoryEnsureCache[path] = true

      var pathParts = path.split("/");
      var dirName = pathParts[pathParts.length - 1];
      pathParts.pop()
      var parentDir = pathParts.join("/") + "/";

      console.log("Ensure directory", path)
      let query = [{
        "column": "document_name",
        "operator": "is",
        "value": dirName
      }, {
        "column": "document_path",
        "operator": "is",
        "value": parentDir
      }, {
        "column": "document_extension",
        "operator": "is",
        "value": "folder"
      }];
      console.log("Document search query", query)
      that.loadData({
        tableName: "document",
        params: {
          query: JSON.stringify(query)
        }
      }).then(function (res) {
        console.log("Ensure directory result", res)
        if (res.data.length === 0) {
          console.log("Directory does not exist", path);
          var newRow = {
            document_name: dirName,
            tableName: "document",
            document_extension: "folder",
            mime_type: '',
            document_path: parentDir,
            document_content: [],
          }
          console.log("Create folder request", newRow)

          that.createRow(newRow).then(function (res) {
            that.refreshData();
          }).catch(function (e) {
            console.log("Failed to create folder", e)
            that.$q.notify({
              message: "Failed to create folder: " + JSON.stringify(e)
            });
          });


        }
      })
    },
    uploadFile(file, fileName) {
      // console.log("Upload file", file);
      const that = this;
      file.status = "Queued"

      var uploadFile1 = function (fileToUpload) {
        return new Promise(function (resolve, reject) {
          const name = fileName || fileToUpload.name;
          const type = fileToUpload.type;
          const reader = new FileReader();
          file.status = "Reading"
          reader.onload = function (fileResult) {
            // console.log("File loaded", fileToUpload, fileResult);
            file.status = "Uploading"
            let documentPath = that.currentPath + "/";
            if (fileToUpload.webkitRelativePath && fileToUpload.webkitRelativePath.length > 0) {
              var relPath = fileToUpload.webkitRelativePath.split("/");
              relPath.pop(); //remove name
              documentPath = that.currentPath + "/" + relPath.join("/") + "/"
            }
            var pathParts = documentPath.split("/")
            if (pathParts.length > 2) {
              pathParts.pop();
              that.ensureDirectory(pathParts.join("/"))
            }
            var obj = {
              tableName: "document",
              document_content: [{
                name: fileName || fileToUpload.name,
                contents: fileResult.target.result,
                type: fileToUpload.type,
                path: documentPath
              }],
              document_name: fileName || fileToUpload.name,
              document_path: documentPath,
              mime_type: fileToUpload.type,
              document_extension: fileToUpload.name.indexOf(".") > -1 ? fileToUpload.name.split(".")[1] : "",
            }
            that.createRow(obj).then(function () {
              file.status = "Uploaded";
              that.refreshData();
              resolve()
            }).catch(reject);
          };
          reader.onerror = function () {
            console.log("Failed to load file onerror", e, arguments);
            reject(name);
          };
          reader.readAsDataURL(fileToUpload);
        })
      };
      return uploadFile1(file.file)


    },
    showUploader() {
      console.log("show uploader", this.showUploadComponent)
      const that = this;
      // if (this.showUploadComponent) {
      //   this.showUploadComponent = false;
      //   return;
      // }
      this.uploadedFiles = [];

      // this.showUploadComponent = true
      // setTimeout(function () {
      that.$refs.upload.$el.click()
      // }, 200);
    },
  },
  data() {
    return {
      searchInput: '',
      ...mapGetters(['endpoint']),
      directoryEnsureCache: {},
      newNamePrompt: false,
      viewMode: 'card',
      uploadedFiles: [],
      newName: '',
      newNameType: '',
      currentPath: '',
      selectedFile: null,
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

    var lastPath = localStorage.getItem("_last_current_path")
    if (lastPath) {
      this.currentPath = lastPath;
    }

    that.refreshData();


    document.querySelector('html').ondragenter = function (e) {
      e.stopPropagation();
      return false;
    };
    document.querySelector('html').ondragover = function (e) {
      e.stopPropagation();
      return false;
    };

    document.ondrop = function (ev) {
      console.log('File(s) dropped');

      // Prevent default behavior (Prevent file from being opened)
      ev.preventDefault();

      if (ev.dataTransfer.items) {
        // Use DataTransferItemList interface to access the file(s)
        for (var i = 0; i < ev.dataTransfer.items.length; i++) {
          // If dropped items aren't files, reject them
          if (ev.dataTransfer.items[i].kind === 'file') {
            var file = ev.dataTransfer.items[i].getAsFile();
            console.log('... file[' + i + '].name = ' + file.name);
            that.uploadFile({
              file: file
            })
          }
        }
      } else {
        // Use DataTransfer interface to access the file(s)
        for (var i = 0; i < ev.dataTransfer.files.length; i++) {
          console.log('... file[' + i + '].name = ' + ev.dataTransfer.files[i].name);
          that.uploadFile({
            file: ev.dataTransfer.files[i]
          })
        }
      }
    }

    document.onpaste = function (event) {
      var items = (event.clipboardData || event.originalEvent.clipboardData).items;
      console.log("Items", items)

      for (var index in items) {
        var item = items[index];
        console.log("Items", index, item, item)
        window.item = item;
        if (item.kind === 'file') {
          var blob = item.getAsFile();
          console.log("Upload blob", blob)
          let nameParts = blob.name.split(".");
          let newName = nameParts[0] + " pasted at " + new Date().toLocaleString() + "." + nameParts[1]
          that.uploadFile({
            file: blob,
          }, newName)
        }
      }
    }


  }
}
</script>
