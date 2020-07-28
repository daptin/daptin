<template>
  <div class="row">

    <div class="col-12">

      <div class="q-pa-md q-gutter-sm">
        <q-breadcrumbs>
          <template v-slot:separator>
            <q-icon
              size="1.2em"
              name="arrow_forward"
            />
          </template>

          <q-breadcrumbs-el :key="i" v-for="(item, i) in bread" :label="item.label" :icon="item.icon"/>
        </q-breadcrumbs>
      </div>
      <q-separator></q-separator>

    </div>
    <div class="col-12">

      <div class="row">
        <div class="col-12">
          <q-btn-group flat>
            <q-btn-dropdown size="sm" icon="fas fa-plus">
              <q-list>
                <q-item clickable v-close-popup @click="createFile()">
                  <q-item-section>
                    <q-item-label>Create file</q-item-label>
                  </q-item-section>
                </q-item>

                <q-item clickable v-close-popup @click="createFolder()">
                  <q-item-section>
                    <q-item-label>Create folder</q-item-label>
                  </q-item-section>
                </q-item>
              </q-list>
            </q-btn-dropdown>
            <q-btn size="sm" @click="showUploadFile = true" icon="fas fa-upload"></q-btn>
            <q-btn size="sm" @click="getContentOnPath({name: '.', is_dir: false})" icon="fas fa-sync-alt"></q-btn>

            <q-space></q-space>
          </q-btn-group>
          <q-btn-group class="float-right" flat>
            <q-btn size="sm" @click="viewType = 'table'" v-if="viewType !== 'table'" icon="fas fa-table"></q-btn>
            <q-btn size="sm" @click="viewType = 'card'" v-if="viewType !== 'card'" icon="fas fa-th"></q-btn>
          </q-btn-group>
        </div>
      </div>
      <div class="row" v-if="showUploadFile" style="min-height: 300px">
        <file-upload
          style="height: 300px; width: 100%"
          class="bg-grey-3"
          ref="upload"
          v-model="uploadedFiles"
          post-action="/post.method"
          put-action="/put.method"
          @input-file="inputFile"
          @input-filter="inputFilter"
        >
          <span style="padding-top: 40%" class="vertical-middle">Drop files or click to select <br/></span>

        </file-upload>
        <q-btn
          @click.stop="showUploadFile = false" label="Cancel"></q-btn>
      </div>
      <div class="row" v-if="viewType == 'table'">
        <q-markup-table style="width: 100%">
          <thead>
          <tr>
            <th></th>
            <th class="text-left">Name</th>
            <th class="text-right">Size</th>
          </tr>
          </thead>
          <tbody>

          <tr style="cursor: pointer" @click="getContentOnPath({name: '..'})">
            <td><i class="fas fa-level-up-alt"></i></td>
            <td>..</td>
            <td class="text-right"></td>
          </tr>


          <tr style="cursor: pointer" @click="getContentOnPath(file)" v-for="file in fileList">
            <td><i :class="file.icon"></i></td>
            <td>{{file.name}}</td>
            <td class="text-right">{{ file.is_dir ? '' : ( parseInt(file.size / (1024 * 1024) ) + ' mb')}}</td>
          </tr>

          </tbody>
        </q-markup-table>
      </div>
      <div class="row" v-if="viewType == 'card'">

        <div @click="getContentOnPath({name: '..'})" style="min-width: 150px;"
             class="q-pa-md q-gutter-sm">
          <q-card style="cursor: pointer" bordered flat class="flex-center">
            <q-card-section>
              <q-icon size="md" name="fas fa-level-up-alt"></q-icon>
            </q-card-section>
            <q-card-section class="flex-center">
              <span class="text-bold">..</span>
            </q-card-section>
          </q-card>
        </div>

        <div @click="getContentOnPath(file)" style="min-width: 150px;" v-for="file in fileList"
             class="q-pa-md q-gutter-sm">
          <q-card style="cursor: pointer" bordered flat class="flex-center">
            <q-card-section>
              <q-icon size="md" :name="file.icon"></q-icon>
            </q-card-section>
            <q-card-section class="flex-center">
              <span class="text-bold">{{file.name}}</span>
            </q-card-section>
          </q-card>
        </div>


      </div>
    </div>

    <q-dialog :square="true" v-model="filePreview">
      <q-card class="row" flat style="width: 50%; height: 50%">
        <q-card-section style="width: 100%; height: 100%">
          <iframe style="padding: 10px; width: 100%; height: 100%;" :src="previewUrl"></iframe>
        </q-card-section>
      </q-card>
    </q-dialog>

    <q-page-sticky position="bottom-right" :offset="[20, 20]">
      <q-btn flat @click="$emit('close')" icon="fas fa-times"></q-btn>
    </q-page-sticky>
  </div>
</template>
<script>
  import {mapActions} from "vuex";

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


  export default {
    name: "FileBrowserComponent",
    props: ['site', 'path'],
    data() {
      return {
        showUploadFile: false,
        uploadedFiles: [],
        fileList: [],
        bread: [],
        currentPath: "",
        filePreview: false,
        previewUrl: null,
        viewType: 'card'
      }
    },
    methods: {

      createFile(){

      },
      createFolder(){

      },
      inputFile(uploadedFile) {
        console.log("input file", arguments)
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
              obj.tableName = "cloud_store";
              obj.actionName = "upload_file";
              obj.params.cloud_store_id = that.site.cloud_store_id.id;
              that.executeAction(obj).then(function (res) {
                console.log("Upload done", arguments);
                that.showUploadFile = false;
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
        uploadFile(uploadedFile.file)


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
          icon = "fas fa-markdown"
        }

        if (val.is_dir) {
          icon = "fas fa-folder";
        }

        val.icon = icon;
        return val;
      },


      getContentOnPath(path) {
        console.log("Get content on path", path);
        const that = this;


        if (path.is_dir) {
          that.currentPath = that.currentPath + "/" + path.name;
          that.bread.push({
            icon: "fas fa-folder",
            label: path.name,
          })
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
        console.log("Final path", that.currentPath);

        if (path.is_dir) {
          that.executeAction({
            tableName: "site",
            actionName: "list_files",
            params: {
              site_id: that.site.id,
              path: that.currentPath
            }
          }).then(function (res) {
            console.log("list files Response", res[0].Attributes["list"]);
            that.showFileBrowser = true;
            that.fileList = res[0].Attributes["list"].map(that.makeFile);
          }).catch(function (err) {
            console.log("failed to list files", err)
          })
        } else {

          let hostname = that.site.hostname;


          let portString = window.location.port !== '80' ? ':' + window.location.port : '';
          if (window.location.hostname === "site.daptin.com") {
            portString = ":6336"
          }
          let fetchUrl = "http://" + hostname + portString + that.currentPath + "/" + path.name;
          that.previewUrl = fetchUrl;
          // window.location = fetchUrl;
          that.filePreview = true;

          // console.log("Fetch url: ", fetchUrl)
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

      listFiles(site) {
        console.log("list files in site", site);
        const that = this;
        that.executeAction({
          tableName: "site",
          actionName: "list_files",
          params: {
            site_id: that.site.id
          }
        }).then(function (res) {
          console.log("list files Response", res[0].Attributes["list"]);
          that.showFileBrowser = true;
          that.fileList = res[0].Attributes["list"].map(that.makeFile);
        }).catch(function (err) {
          console.log("failed to list files", err)
        })
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
        icon: "fas fa-home"
      });
      this.listFiles(this.site)
    }
  }
</script>
