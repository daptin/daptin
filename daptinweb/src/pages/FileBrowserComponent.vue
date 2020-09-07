<template>
  <div class="row q-pa-xs" style="height: 100vh">

    <div
      class="col-2 col-xl-2 col-lg-3 col-md-3 col-sm-4 col-xs-0"
      style="border-right: 3px solid black">
      <span @click="window.open(site.hostname)" class="text-bold"><i class="fas fa-home"
                                                                     style="font-size: 1.2em; cursor: pointer"></i> {{
          site.name
        }}</span>
      <v-jstree
        show-checkbox
        multiple
        allow-batch
        ref="tree"
        :async="loadFilePathDataForTree" :data="asyncFileData"
        draggable
        @item-click="fileTreeItemClicked"
        @item-drag-start="fileTreeItemDragStart"
        @item-drag-end="fileTreeItemDragEnd"
        @item-drop-before="fileTreeItemDropBefore"
        @item-drop="fileTreeItemDrop"
        whole-row></v-jstree>
    </div>

    <div class="col-10 col-md-9 col-sm-8 col-xs-12" v-if="!showFileEditor && !showFilePreview">

      <div class="row" style="height: 5vh; min-height: 40px">
        <div class="col-12">
          <q-btn-group flat>
            <q-btn icon="fas fa-tasks" @click="fileList.map(e => e.selected = !e.selected)"></q-btn>
            <q-btn-dropdown size="md" icon="fas fa-plus">
              <q-list>
                <q-item clickable v-close-popup @click="showNewFileName = true">
                  <q-item-section>
                    <q-item-label>Create file</q-item-label>
                  </q-item-section>
                </q-item>

                <q-item clickable v-close-popup @click="showNewFolderName = true">
                  <q-item-section>
                    <q-item-label>Create folder</q-item-label>
                  </q-item-section>
                </q-item>
                <q-item clickable v-close-popup
                        @click="(showUploadFile = true)  && (uploadedFiles = []) && (showFileEditor = false)  && (showFilePreview = false) ">
                  <q-item-section>
                    <q-item-label>Upload/Drag and drop files</q-item-label>
                  </q-item-section>
                </q-item>
              </q-list>
            </q-btn-dropdown>
            <q-btn size="md" @click="refreshCache()"
                   icon="fas fa-sync-alt"></q-btn>
            <q-btn @click="deleteSelectedFiles" flat size="md" class="float-right" color="negative"
                   icon="fas fa-times"></q-btn>

            <q-space></q-space>
          </q-btn-group>
        </div>
      </div>


      <div class="row" v-if="showUploadFile" style="min-height: 300px">
        <file-upload
          :multiple="true"
          style="height: 300px; width: 100%"
          class="bg-grey-3"
          ref="upload"
          :drop="true"
          :drop-directory="false"
          v-model="uploadedFiles"
          post-action="/post.method"
          put-action="/put.method"
          @input-file="inputFile"
          @input-filter="inputFilter"
        >
          <div class="container">
            <div class="row">
              <div class="col-12" style="height: 100%; ">
                <span class="vertical-middle" style="padding-top: 10%">
                  Click here to select files, or drag and drop files here to upload</span>
              </div>
            </div>
            <span v-if="uploadedFiles.length == 0" style="padding-top: 40%" class="vertical-middle">Drop files or click to select <br/></span>
            <div class="row" v-if="uploadedFiles.length > 0">
              <div class="col-12" v-for="file in uploadedFiles">{{ file.name }} - Error: {{ file.error }}, Success:
                {{ file.success }}
              </div>
            </div>
          </div>
        </file-upload>
        <q-btn
          @click.stop="(showUploadFile = false) && (uploadedFiles = [])" label="Close"></q-btn>
      </div>


      <div class="row" style="height: 90vh; overflow-y: scroll" v-if="viewType === 'table'">
        <q-markup-table style="width: 100%; box-shadow: none;">
          <tbody>

          <tr style="cursor: pointer" @click="getContentOnPath({name: '..'})">
            <td class="text-right"></td>
            <td><i class="fas fa-level-up-alt"></i></td>
            <td>..</td>
            <td class="text-right"></td>
          </tr>


          <tr style="cursor: pointer" @click="getContentOnPath(file)" v-for="file in fileList">
            <td style="width: 50px">
              <q-checkbox :size="showDelete ? 'xl' : 'md'" @input="selectFile(file)" v-model="file.selected" flat
                          icon="fas fa-wrench"></q-checkbox>
            </td>

            <td style="width: 50px"><i :class="file.icon"></i></td>
            <td>{{ file.name }}</td>
            <td class="text-right">{{
                file.is_dir ? '' : file.size > 1024 * 1024 ? (parseInt(file.size / (1024 * 1024))
                  + ' mb') : (parseInt(file.size / (1024)) + ' kbs')
              }}
            </td>

          </tr>
          </tbody>
        </q-markup-table>
      </div>

    </div>


    <div class="col-10 col-md-9 col-sm-8 col-xs-12" v-if="showFileEditor">
      <div class="row">
        <div class="col-12">
          <q-btn @click="editor.undo()" icon="fas fa-undo" flat></q-btn>
        </div>
        <div class="col-12">
          <div style="height: 100%;">
            <!--        <textarea id="fileEditor" style="height: 90vh"></textarea>-->
            <ace-editor @input="saveFile()" ref="myEditor"
                        @init="loadDependencies"
                        :lang="selectedFile.language" theme="chrome" width="95%" height="90vh"
                        v-model="selectedFile.content"></ace-editor>
          </div>
        </div>
      </div>

      <q-page-sticky style="z-index: 3000" position="bottom-right" :offset="[20, 20]">
        <q-btn flat @click="(showFileEditor = false ) && (fileType = null)" icon="fas fa-long-arrow-alt-left"></q-btn>
      </q-page-sticky>
    </div>


    <div class="col-10 col-md-9 col-sm-8 col-xs-12" v-if="showFilePreview">
      <div style="height: 100%;">
        <div id="filePreviewDiv"></div>
      </div>
      <q-page-sticky position="bottom-right" :offset="[20, 20]">
        <q-btn flat @click="(showFilePreview = false ) && (fileType = null)" icon="fas fa-long-arrow-alt-left"></q-btn>
      </q-page-sticky>
    </div>

    <q-dialog v-model="showNewFolderName" persistent>
      <q-card style="min-width: 350px">
        <q-card-section>
          <div class="text-h6">Folder name</div>
        </q-card-section>

        <q-card-section class="q-pt-none">
          <q-input dense v-model="newFolderName" autofocus/>
        </q-card-section>

        <q-card-actions align="right" class="text-primary">
          <q-btn @click="showNewFolderName = false" flat label="Cancel" v-close-popup/>
          <q-btn @click="createFolder()" flat label="Create" v-close-popup/>
        </q-card-actions>
      </q-card>
    </q-dialog>


    <q-dialog v-model="showNewFileName" persistent>
      <q-card style="min-width: 350px">
        <q-card-section>
          <div class="text-h6">File name</div>
        </q-card-section>

        <q-card-section class="q-pt-none">
          <q-input dense v-model="newFileName" autofocus/>
        </q-card-section>

        <q-card-actions align="right" class="text-primary">
          <q-btn @click="showNewFileName = false" flat label="Cancel" v-close-popup/>
          <q-btn @click="createFile()" flat label="Create" v-close-popup/>
        </q-card-actions>
      </q-card>
    </q-dialog>

    <q-dialog :square="true" v-model="filePreview">
      <q-card class="row" flat style="width: 80%; height: 80%">
        <q-card-section style="width: 100%; height: 100%">
          <iframe style="padding: 10px; width: 100%; height: 100%;" :src="previewUrl"></iframe>
        </q-card-section>
      </q-card>
    </q-dialog>

    <q-page-sticky position="bottom-right" v-if="!showFileEditor && !showFilePreview" :offset="[20, 20]">
      <q-btn flat @click="$emit('close')" icon="fas fa-times"></q-btn>
    </q-page-sticky>
  </div>
</template>
<style>
.file-editor-frame {
  height: 80vh;
}

</style>
<script>
import {mapActions} from "vuex";

var ace = require('brace');
window.ace = ace;
require('brace/ext/language_tools'); //language extension prerequsite...
require('brace/mode/html');
require('brace/mode/javascript');    //language
require('brace/mode/markdown');    //language
require('brace/mode/toml');    //language
require('brace/mode/xml');    //language
require('brace/mode/text');    //language
require('brace/mode/less');
require('brace/mode/yaml');
require('brace/mode/json');
require('brace/mode/css');
require('brace/theme/chrome');

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

var saveData = (function () {
  var a = document.createElement("a");
  document.body.appendChild(a);
  a.style = "display: none";
  return function (blob, fileName) {
    var url = window.URL.createObjectURL(blob);
    a.href = url;
    a.download = fileName;
    a.click();
    window.URL.revokeObjectURL(url);
  };
}());

function folderNameFromPath(path) {
  var pathParts = path.split("/");
  if (pathParts.length < 2) {
    return pathParts[0];
  }
  if (pathParts[pathParts.length - 1].trim().length > 0 || pathParts.length < 3) {
    return pathParts[pathParts.length - 1].trim()
  }
  return pathParts[pathParts.length - 2].trim();
}

export default {
  name: "FileBrowser",
  props: ['site', 'path'],
  data() {
    return {
      asyncFileData: [],
      pathFileList: {
        root: []
      },
      vjsData: [
        {
          "text": "Same but with checkboxes",
          "children": [
            {
              "text": "initially selected",
              "selected": true
            },
            {
              "text": "custom icon",
              "icon": "fa fa-warning icon-state-danger"
            },
            {
              "text": "initially open",
              "icon": "fa fa-folder icon-state-default",
              "opened": true,
              "children": [
                {
                  "text": "Another node"
                }
              ]
            },
            {
              "text": "custom icon",
              "icon": "fa fa-warning icon-state-warning"
            },
            {
              "text": "disabled node",
              "icon": "fa fa-check icon-state-success",
              "disabled": true
            }
          ]
        },
        {
          "text": "Same but with checkboxes",
          "opened": true,
          "children": [
            {
              "text": "initially selected",
              "selected": true
            },
            {
              "text": "custom icon",
              "icon": "fa fa-warning icon-state-danger"
            },
            {
              "text": "initially open",
              "icon": "fa fa-folder icon-state-default",
              "opened": true,
              "children": [
                {
                  "text": "Another node"
                }
              ]
            },
            {
              "text": "custom icon",
              "icon": "fa fa-warning icon-state-warning"
            },
            {
              "text": "disabled node",
              "icon": "fa fa-check icon-state-success",
              "disabled": true
            }
          ]
        },
        {
          "text": "And wholerow selection"
        }
      ],
      showDelete: false,
      fileType: null,
      saver: null,
      editor: null,
      showFileEditor: false,
      showFilePreview: false,
      selectedFile: null,
      showNewFileName: false,
      newFileName: null,
      newFolderName: null,
      showNewFolderName: false,
      showUploadFile: false,
      uploadedFiles: [],
      fileList: [],
      bread: [],
      currentPath: "",
      filePreview: false,
      previewUrl: null,
      viewType: 'table'
    }
  },
  computed: {},
  watch: {},
  methods: {
    fileTreeItemClicked(node, itemClicked, mouseEvent) {
      if (!node.model.is_dir) {
        this.getContentOnPath(itemClicked);
      } else {
        node.model.opened = !node.model.opened;
      }
    },
    fileTreeItemDragStart(fileTree, itemClicked, mouseEvent) {
      console.log("tree file item fileTreeItemDragStart", fileTree.model.text, itemClicked, mouseEvent);
      // this.getContentOnPath(itemClicked);
    },
    fileTreeItemDragEnd(fileTree, destination, source) {
      // console.log("tree file item fileTreeItemDragEnd", fileTree.model.text, itemClicked, mouseEvent);
      // this.getContentOnPath(itemClicked);
    },
    fileTreeItemDrop(fileTree, destination, source) {
      console.log("tree file item fileTreeItemDrop", fileTree.model.text, destination, source);
      if (!destination.is_dir) {
        return false
      }
      const that = this;
      if (source.full_path[0] !== '/') {
        source.full_path = "/" + source.full_path
      }
      if (destination.full_path[0] !== '/') {
        destination.full_path = "/" + destination.full_path
      }

      var promise = that.executeAction({
        tableName: "cloud_store",
        actionName: "move_path",
        params: {
          cloud_store_id: that.site.cloud_store_id.id,
          source: that.site.path  + source.full_path,
          destination: that.site.path  + destination.full_path
        }
      }).then(function (res) {
        console.log("moved", res);
      }).catch(function (err) {
        console.log("failed to delete", err)
      });

      // this.getContentOnPath(itemClicked);
    },
    fileTreeItemDropBefore(fileTree, destination, source) {
      if (!destination.is_dir) {
        this.$q.notify({
          message: "Cannot move to a non-folder"
        });
        return false;
      }
      console.log("tree file item fileTreeItemDropBefore", this.currentPath, fileTree, destination, source);
      // this.getContentOnPath(itemClicked);
    },
    loadFilePathDataForTree(node, resolve) {
      console.log("load file path data for tree", node.data.value, resolve);
      const that = this;
      var path = null;


      if (node.data.value) {
        path = {
          full_path: node.data.value,
          is_dir: node.data.is_dir
        }
      }
      return new Promise(function (resolve1, reject) {
        that.getContentOnPath(path).then(function (files) {
          if (files) {
            files.map(e => e.text = e.name);
            files.map(e => e.value = e.full_path);
          }
          console.log("Got file list", files);

          that.showFileEditor = false;
          that.showFileBrowser = true;
          that.showFilePreview = false;
          that.fileType = null;

          // node.openChildren()
          resolve(files)
        }).catch(reject)
      })
    },
    loadDependencies() {
      // require('brace/mode/html');
      // require('brace/theme/chrome');
    },
    deleteSelectedFiles() {
      const that = this;
      var selectedFiles = this.fileList.filter(e => e.selected);
      var promises = [];
      for (var fileIndex in selectedFiles) {
        console.log("Delete fileIndex", this.site, this.currentPath, selectedFiles[fileIndex]);

        var promise = that.executeAction({
          tableName: "cloud_store",
          actionName: "delete_path",
          params: {
            cloud_store_id: that.site.cloud_store_id.id,
            path: that.site.path + "/" + (this.currentPath.length > 0 ? this.currentPath + "/" : "") + selectedFiles[fileIndex].name
          }
        }).then(function (res) {
          console.log("deleted", res);
          selectedFiles.selected = false;
          selectedFiles.splice(fileIndex, 1)
        }).catch(function (err) {
          console.log("failed to delete", err)
        });
        promises.push(promise)

      }
      Promise.all(promises).then(function () {

        console.log("File delete complete", arguments)
        setTimeout(function () {
          that.getContentOnPath()
        }, 2000)

      })


    },
    selectFile(file) {
      console.log("Select file", file, this.fileList);
      this.showDelete = this.fileList.filter(e => e.selected).length > 0;
    },

    createFile() {

      const that = this;

      if (that.newFileName === "" || !that.newFileName) {
        return
      }

      that.executeAction({
        tableName: "cloud_store",
        actionName: "upload_file",
        params: {
          "file": [{"name": that.newFileName, "file": "data:text/plain;base64,", "type": "text/plain"}],
          "path": that.site.path + "/" + that.currentPath,
          "cloud_store_id": that.site.cloud_store_id.id
        }
      }).then(function () {
        that.getContentOnPath({name: '.', is_dir: false});
        that.$q.notify({
          message: "File created"
        });
        setTimeout(function () {
          that.getContentOnPath();
        }, 1500);
      }).catch(function (err) {
        console.log("Failed to create file", err);
        that.$q.notify({
          message: "Failed to create create file"
        })
      });

      that.showNewFileName = false;
    },
    createFolder() {
      const that = this;

      that.executeAction({
        tableName: "cloud_store",
        actionName: "create_folder",
        params: {
          "cloud_store_id": that.site.cloud_store_id.id,
          "path": that.site.path + '/' + that.currentPath,
          "name": that.newFolderName
        }
      }).then(function () {
        that.getContentOnPath({name: '.', is_dir: false})
      }).catch(function (err) {
        console.log("Failed to create folder", err);
        that.$q.notify({
          message: "Failed to create folder"
        })
      })

    },
    inputFile(uploadedFile) {
      console.log("input file", arguments);
      const that = this;

      var uploadFile = function (file) {
        return new Promise(function (resolve, reject) {
          const name = file.name;
          const type = file.type;
          const reader = new FileReader();
          reader.onload = function (fileResult) {
            console.log("File loaded", fileResult);
            var obj = {params: {"file": []}};
            obj["params"]["file"].push({
              name: name,
              file: fileResult.target.result,
              type: type
            });
            console.log("Upload file current path", that.currentPath);
            obj.params.path = that.site.path + "/" + that.currentPath;
            obj.tableName = "cloud_store";
            obj.actionName = "upload_file";
            obj.params.cloud_store_id = that.site.cloud_store_id.id;
            that.executeAction(obj).then(function (res) {
              console.log("Upload done", arguments);
              // that.showUploadFile = false;
              uploadedFile.success = true;
              that.refreshCache();
              that.getContentOnPath({is_dir: false, name: '.'})
            }).catch(function (err) {
              console.log("Failed to upload", arguments)
            });
            resolve();
          };
          reader.onerror = function () {
            console.log("Failed to load file onerror", e, arguments);
            reject(name);
          };
          reader.readAsDataURL(file);
        })
      };
      return uploadFile(uploadedFile.file)


    },
    inputFilter() {
      console.log("input filter", arguments)
    },
    makeFile(val) {
      var valName = val.name;
      let icon = "fas fa-file";
      if (valName.endsWith("html")) {
        icon = "fas fa-code"
      } else if (valName.endsWith("mp3") || valName.endsWith("wav")) {
        icon = "fas fa-file-audio"
      } else if (valName.endsWith("mp4") || valName.endsWith("mkv")) {
        icon = "fas fa-file-video"
      } else if (valName.endsWith("jpg") || valName.endsWith("jpeg") || valName.endsWith("png") || valName.endsWith("gif")) {
        icon = "fas fa-image"
      } else if (valName.endsWith("md")) {
        icon = "fab fa-markdown"
      }

      if (val.is_dir) {
        icon = "fas fa-folder";
      }

      val.icon = icon;
      return val;
    },

    refreshCache() {
      const that = this;
      that.executeAction({
        tableName: "site",
        actionName: "sync_site_storage",
        params: {
          site_id: that.site.id,
          path: "",
        }
      }).then(function () {
        that.getContentOnPath({name: '.', is_dir: false})
      }).catch(function (err) {
        that.$q.notify({
          message: "Failed to sync site cache"
        })
      })
    },

    getContentOnPath(path) {
      const that = this;
      return new Promise(function (resolve, reject) {
        path = path || {name: '.', is_dir: false};
        console.log("Get content on path", path);
        that.showDelete = false;

        if (path.full_path) {
          var parts = path.full_path.split("/")
          console.log("Full path", path.full_path)
          if (parts[0] === "" && parts.length < 2) {
            that.currentPath = ""
            path.name = parts[1];
          } else {
            path.name = parts.pop();
            if (parts[0] === "") {
              parts.unshift()
            }
            that.currentPath = parts.join("/")
          }
          console.log("Final full path", that.currentPath, path)
        }

        if (path.is_dir && path.name !== '.') {
          if (path.name !== '/' && path.name !== '') {

            that.currentPath = that.currentPath + (that.currentPath === "" ? "" : "/") + path.name;
            that.bread.push({
              icon: "fas fa-folder",
              label: path.name,
            })
          } else {
            that.currentPath = "";
            that.bread = [that.bread[0]]
          }

        } else if (path.name === "..") {

          if (that.bread.length === 1) {
            return
          }
          let parts = that.currentPath.split("/").filter(function (e) {
            return e.length > 0
          });
          if (parts.length < 1) {
            that.currentPath = "";
          } else {
            path.is_dir = true;
            parts.pop();
            that.bread.pop();
            that.currentPath = parts.join("/")
          }

        }
        if (path.name === ".") {
          path.is_dir = true;
        }
        console.log("Final path", that.currentPath, path.is_dir, that.site.name);

        if (path.is_dir || path.name === '..') {
          that.executeAction({
            tableName: "site",
            actionName: "list_files",
            params: {
              site_id: that.site.id,
              path: that.currentPath
            }
          }).then(function (res) {
            let fileList = res[0].Attributes["list"];
            console.log("list files Response", fileList);

            if (!fileList) {
              that.fileList = [];
              path.children = [];
              path.opened = true;
              resolve([]);
              return;
            }
            that.showFileBrowser = true;
            let files = fileList.map(that.makeFile);

            files.sort(function (a, b) {
              return a.is_dir < b.is_dir
            });
            files = files.map(function (item) {
              item.selected = false;
              item.text = item.name;
              item.full_path = that.currentPath + "/" + item.name
              item.value = that.currentPath + "/" + item.name
              item.isLeaf = !item.is_dir;
              if (item.is_dir) {
                item.children = [that.$refs.tree.initializeLoading()];
              }
              return item;
            });
            console.log("Current path was", that.currentPath);
            // files.unshift({
            //   name: '..',
            //   text: '..',
            //   is_dir: false,
            //   selected: false,
            // });
            if (that.currentPath === "" && that.pathFileList.root.length === 0) {
              that.pathFileList.root = files;
              console.log("Resolve file list promise 1", files)
              // that.asyncFileData = files;
              resolve(files)
            } else {
              console.log("Adding children to path", path)
              path.children = files;
              path.opened = true;

              var newRoot = JSON.parse(JSON.stringify(that.pathFileList.root));
              that.pathFileList.root = [];
              that.pathFileList.root = newRoot;
              console.log("Resolve file list promise 2", files)
              resolve(files)
            }

            that.fileList = files;
          }).catch(function (err) {
            console.log("failed to list files", err);
            that.getContentOnPath({name: '', is_dir: false});
            reject(err)
          })
        } else {


          let hostname = that.site.hostname;


          let portString = window.location.port !== '80' ? ':' + window.location.port : '';
          if (window.location.hostname === "site.daptin.com") {
            portString = ":6336"
          }

          that.selectedFile = {
            path: that.currentPath + "/" + path.name,
            type: "text"
          };

          if (path.name.endsWith(".md")) {
            that.selectedFile.type = 'markdown';
          }


          let fetchUrl = "http://" + hostname + portString + "/" + that.currentPath + (that.currentPath !== "" ? "/" : "") + path.name;
          // that.previewUrl = fetchUrl;
          // window.location = fetchUrl;
          // that.filePreview = true;

          console.log("Fetch url: ", fetchUrl);

          that.executeAction({
            tableName: "site",
            actionName: "get_file",
            params: {
              "site_id": that.site.id,
              "path": that.currentPath + "/" + path.name
            }
          }).then(function (res) {
            console.log("Get file contents", res);

            let split = path.name.split(".");
            var fileNameExtension = split[split.length - 1];
            console.log("File name extension is", fileNameExtension)
            switch (fileNameExtension) {
              case "md":
                that.selectedFile.language = "markdown";
                break;
              case "xml":
                that.selectedFile.language = "xml";
                break;
              case "html":
                that.selectedFile.language = "html";
                break;
              case "toml":
                that.selectedFile.language = "toml";
                break;
              case "js":
                that.selectedFile.language = "javascript";
                break;
              case "py":
                that.selectedFile.language = "python";
                break;
              case "sql":
                that.selectedFile.language = "mysql";
                break;
              case "css":
                that.selectedFile.language = "css";
                break;
              default:
                that.selectedFile.language = "text";
                break
            }

            that.selectedFile.content = atob(res[0].Attributes.data);
            that.showFileEditor = true;


            setTimeout(function () {


              require('brace/ext/language_tools'); //language extension prerequsite...
              require('brace/mode/html');
              require('brace/mode/javascript');    //language
              require('brace/mode/markdown');    //language
              require('brace/mode/toml');    //language
              require('brace/mode/xml');    //language
              require('brace/mode/less');
              require('brace/theme/chrome');


              that.fileType = "text";
              if (path.name.endsWith("jpg") || path.name.endsWith("png") || path.name.endsWith("gif")) {
                that.fileType = "image"
              }
              if (path.name.endsWith("md")) {
                that.fileType = "markdown"
              }
              if (path.name.endsWith("mkv") || path.name.endsWith("mp4")) {
                that.fileType = "video"
              }
              if (path.name.endsWith("mp3") || path.name.endsWith("wav")) {
                that.fileType = "audio"
              }

              if (that.fileType === "text" || that.fileType === "markdown") {
                that.showFileEditor = true;
                that.showFilePreview = false;
                if (!that.editor) {
                  that.editor = that.$refs.myEditor.editor;
                  that.editor.setOption("wrap", true);
                }
                that.editor.setValue(that.selectedFile.content);
                that.editor.setOptions({
                  fontSize: "18px"
                });
                that.editor.selection.moveCursorToPosition({row: 0, column: 0});
                that.editor.focus()
              } else if (that.fileType === "image") {
                that.showFileEditor = false;
                that.showFilePreview = true;
                setTimeout(function () {
                  document.getElementById("filePreviewDiv").innerHTML = "<img style='width: 100%' src='data:image/jpg;base64," + res[0].Attributes.data + "' > </img>"
                }, 300)
              } else if (that.fileType === "audio") {
                that.showFileEditor = false;
                that.showFilePreview = true;
                setTimeout(function () {
                  document.getElementById("filePreviewDiv").innerHTML = "<audio style='width: 100%' src='data:image/jpg;base64," + res[0].Attributes.data + "' > </audio>"
                }, 300)
              } else if (that.fileType === "video") {
                that.showFileEditor = false;
                that.showFilePreview = true;
                setTimeout(function () {
                  document.getElementById("filePreviewDiv").innerHTML = "<video style='width: 100%' src='data:image/jpg;base64," + res[0].Attributes.data + "' > </video>"
                }, 300)
              }


            }, 100)

          }).catch(function (err) {
            console.log("Failed to get file contents", err);
            reject(err)
          });


        }
      });

    },

    saveFile: function () {
      return debounce(function () {
        const that = this;
        let content = that.selectedFile.content;
        console.log("save", JSON.stringify(that.selectedFile), content, that.editor.getValue());
        let pathParts = that.selectedFile.path.split("/");
        var fileName = pathParts[pathParts.length - 1];
        that.executeAction({
          tableName: "cloud_store",
          actionName: "upload_file",
          params: {
            "file": [{
              "name": fileName,
              "file": "data:text/plain;base64," + btoa(content),
              "type": "text/plain"
            }],
            "path": that.site.path + "/" + that.currentPath,
            "cloud_store_id": that.site.cloud_store_id.id
          }
        }).then(function () {
          that.refreshCache();
        }).catch(function (err) {
          console.log("Failed to save file", err);
          that.$q.notify({
            message: "Failed to save file"
          })
        })

      }, 1300, false)
    }(),
    listFiles(site) {
      console.log("list files in site", site);
      const that = this;
      // that.getContentOnPath({name: '', is_dir: true})
    },
    ...mapActions(['executeAction'])

  },
  mounted() {
    const that = this;
    console.log("Mounted file browser", this.site);
    // if (!this.site) {
    //   this.$emit("close");
    //   return
    // }
    this.currentPath = "";
    this.bread.push({
      label: that.site.hostname,
      icon: "fas fa-home",
      click: function () {
        that.getContentOnPath({name: '/', is_dir: true})
      }
    });
    this.listFiles(this.site)
  }
}
</script>
