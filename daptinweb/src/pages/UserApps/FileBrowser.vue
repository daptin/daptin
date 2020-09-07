<template>

  <q-page-container>


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
      </q-toolbar>
    </q-header>

    <q-page>

      <div class="row q-pa-md text-white">


        <div class="col-12">
          <paginated-table-view :items="files"></paginated-table-view>
        </div>


        <q-page-sticky :offset="[10, 10]" v-if="showUploadComponent">
          <q-card style="width: 300px; height: 200px">
            <daptin-document-uploader
              multiple
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
                    <div class="q-uploader__subtitle">{{ scope.uploadSizeLabel }} / {{
                        scope.uploadProgressLabel
                      }}
                    </div>
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
            </daptin-document-uploader>

          </q-card>
        </q-page-sticky>
      </div>
    </q-page>


  </q-page-container>

</template>
<script>

import {mapActions} from "vuex";

export default {

  name: "FileBrowser",
  methods: {
    ...mapActions(['loadData', 'createRow', 'loadModel']),
    refreshData() {
      const that = this;
      that.loadData({
        tableName: "document",
        params: {
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
          return e;
        });
        that.files.unshift({
          name: '..',
          path: '..',
          icon: 'fas fa-folder',
          is_dir: true,
          color: "#9c7664"
        })
        that.files.unshift({
          name: '.',
          path: '.',
          icon: 'fas fa-folder',
          is_dir: true,
          color: "#9c7664"
        });

      })
    },
    uploadFile(file) {
      console.log("Upload file", file);
      const that = this;
      var obj = {
        tableName: "document",
        document_content: file.file,
        document_name: file.name,
        document_path: "/" + file.name,
        mime_type: file.type,
        document_extension: file.name.indexOf(".") > -1 ? file.name.split(".")[1] : "",
      }

      return new Promise(function (resolve, reject) {
        that.createRow(obj).then(function (res) {
          resolve(res.data);
          that.refreshData();
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
    filesSelected(files) {
      // returning a Promise
      console.log("Files selected", files)

      return new Promise((resolve) => {
        // simulating a delay of 2 seconds

      })
    }
  },
  data() {
    return {
      searchInput: '',
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
    this.loadModel("document").then(function () {

      that.refreshData();
    }).catch(function (err) {
      that.$q.notify({
        message: "Failed to load documents"
      })
    })

  }
}
</script>
