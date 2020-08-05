<template>
  <div class="row">

    <div class="col-12" v-if="!showFileEditor && !showFilePreview">

      <div class="q-pa-md q-gutter-sm">
        <q-breadcrumbs>
          <template v-slot:separator>
            <q-icon
              size="1.2em"
              name="arrow_forward"
            />
          </template>

          <q-breadcrumbs-el :style="{cursor: item.click ? 'pointer' :''}" :key="i" v-for="(item, i) in bread"
                            @click="item.click ? item.click() : true" :label="item.label" :icon="item.icon"/>
        </q-breadcrumbs>
      </div>
      <q-separator></q-separator>

    </div>
    <div class="col-12" v-if="!showFileEditor && !showFilePreview">

      <div class="row">
        <div class="col-12">
          <q-btn-group flat>
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
            <q-btn @click="deleteSelectedFiles" flat size="md" class="float-right" color="negative" v-if="showDelete"
                   icon="fas fa-times"></q-btn>

            <q-space></q-space>
          </q-btn-group>
          <q-btn-group class="float-right" flat>
            <q-btn size="md" @click="fullScreenBrowser()" icon="fas fa-expand"></q-btn>
            <!--            <q-btn size="sm" @click="viewType = 'card'" v-if="viewType !== 'card'" icon="fas fa-th"></q-btn>-->
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
            <span v-if="uploadedFiles.length == 0" style="padding-top: 40%" class="vertical-middle">Drop files or click to select <br/></span>
            <div class="row" v-if="uploadedFiles.length > 0">
              <div class="col-12" v-for="file in uploadedFiles">{{file.name}} - Error: {{file.error}}, Success:
                {{file.success}}
              </div>
            </div>
          </div>
        </file-upload>
        <q-btn
          @click.stop="(showUploadFile = false) && (uploadedFiles = [])" label="Close"></q-btn>
      </div>
      <div class="row" v-if="viewType === 'table'">
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
            <td>{{file.name}}</td>
            <td class="text-right">{{ file.is_dir ? '' : file.size > 1024 *1024 ? ( parseInt(file.size / (1024 * 1024) )
              + ' mb') : ( parseInt(file.size / (1024 ) ) + ' kbs') }}
            </td>

          </tr>
          </tbody>
        </q-markup-table>
      </div>

    </div>
    <div class="col-12" v-if="showFileEditor">
      <div class="row">
        <q-drawer side="left">
          <v-jstree :async="loadFilePathDataForTree()" :data="pathFileList['']" whole-row></v-jstree>
        </q-drawer>
        <div class="col-12">
          <q-btn @click="editor.undo()" icon="fas fa-undo" flat></q-btn>
        </div>
        <div class="col-12" style="margin-right: 10px">
          <div style="height: 100%;" v-if="selectedFile.language">
            <!--        <textarea id="fileEditor" style="height: 90vh"></textarea>-->
            <ace-editor @input="saveFile()" ref="myEditor" style="font-family: 'JetBrains Mono';font-size: 16px;"
                        @init="loadDependencies"
                        :lang="selectedFile.language" theme="chrome" width="100%" height="90vh"
                        v-model="selectedFile.content"></ace-editor>
          </div>
        </div>
      </div>

      <q-page-sticky style="z-index: 3000" position="bottom-right" :offset="[20, 20]">
        <q-btn flat @click="(showFileEditor = false ) && (fileType = null)" icon="fas fa-long-arrow-alt-left"></q-btn>
      </q-page-sticky>
    </div>
    <div class="col-12" v-if="showFilePreview">
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
    name: "FileBrowserComponent",
    props: ['site', 'path'],
    data() {
      return {
        pathFileList: {},
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
      loadFilePathDataForTree() {
        console.log("load file path data for tree", arguments)
      },
      loadDependencies() {
        // require('brace/mode/html');
        // require('brace/theme/chrome');
      },
      deleteSelectedFiles() {
        const that = this;
        var selectedFiles = this.fileList.filter(e => e.selected);
        for (var fileIndex in selectedFiles) {
          console.log("Delete fileIndex", this.site, this.currentPath, selectedFiles[fileIndex]);

          that.executeAction({
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
          })


        }


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
          })
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

        debugger
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
        console.log("Get content on path", path);
        const that = this;
        that.showDelete = false;


        if (path.is_dir) {
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

        if (!that.pathFileList[that.currentPath]) {
          let folderName = folderNameFromPath(that.currentPath);
          that.pathFileList[that.currentPath] = {
            text: folderName.length > 0 ? folderName : that.site.name,
            children: []
          }
        }

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
              item.isLeaf = !item.is_dir;
              return item;
            });
            that.pathFileList[that.currentPath] = files;

            that.fileList = files;
          }).catch(function (err) {
            console.log("failed to list files", err);
            that.getContentOnPath({name: '', is_dir: false})
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


          let fetchUrl = "http://" + hostname + portString + "/" + that.currentPath + (that.currentPath !== "" ? "/" : "") + +path.name;
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
                that.editor = that.$refs.myEditor.editor;
                that.editor.setOption("wrap", true)

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
            console.log("Failed to get file contents", err)
          });


          // fetch(fetchUrl).then(function (response) {
          //   response.blob().then(function (blob) {
          //     console.log("Blob is ready", saveData(blob, path.name))
          //   }).catch(function (err) {
          //     console.log("Failed to blob", arguments)
          //   });
          //   console.log("Fetch response", response.body)
          // }).catch(function (err) {
          //
          //   console.log("Failed to fetch", arguments)
          // });
        }
      },

      saveFile: function () {
        return debounce(function () {
          const that = this;
          let content = that.editor.getValue();
          console.log("save", that.selectedFile, that.editor.getValue());
          that.selectedFile.content = content;
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
        that.getContentOnPath({name: '', is_dir: true})
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
