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
          <q-input readonly :value="endpoint() + '/#/apps/spreadsheet/' + document.reference_id"></q-input>

        </q-card-section>
      </q-card>

    </q-dialog>


    <q-header elevated class="bg-white text-black">
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
                <q-item @click="saveDocument()" clickable v-close-popup>
                  <q-item-section>Save</q-item-section>
                </q-item>
                <!--                <q-item @click="saveDocument()" clickable v-close-popup>-->
                <!--                  <q-item-section>Export</q-item-section>-->
                <!--                  <q-menu>-->
                <!--                    <q-list>-->
                <!--                      <q-item>To xlsx</q-item>-->
                <!--                    </q-list>-->
                <!--                  </q-menu>-->
                <!--                </q-item>-->
                <!--                <q-item @click="window.print()" clickable v-close-popup>-->
                <!--                  <q-item-section>Print</q-item-section>-->
                <!--                </q-item>-->
                <q-item @click="$router.back()" clickable v-close-popup>
                  <q-item-section>Close</q-item-section>
                </q-item>
              </q-list>
            </q-menu>
          </q-btn>
          <!--          <q-btn flat label="Edit"></q-btn>-->
          <!--          <q-btn flat label="Format"></q-btn>-->
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
    </q-header>
    <q-page>
      <div id="luckysheet"
           style="margin:0px;padding:0px;position:absolute;width:100%;height:calc(100% + 28px);left: 0px;top: -28px;"></div>

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
@import "../../statics/luckysheet/css/luckysheet.css";
@import "../../statics/luckysheet/plugins/css/pluginsCss.css";
@import "../../statics/luckysheet/plugins/plugins.css";
@import "../../statics/luckysheet/assets/iconfont/iconfont.css";


.q-layout__shadow::after {
  box-shadow: none;
}

.luckysheet-work-area {
  /*height: 41px !important;*/
  top: 27px;
}

/**/
/*.luckysheet-grid-container {*/
/*  top: 64px !important;*/
/*}*/

/*div.luckysheet-grid-container.luckysheet-scrollbars-enabled {*/
/*  top: 65px !important;*/
/*}*/

</style>
<script>
import {mapActions, mapGetters} from "vuex";
import JSZip from "jszip";


// import "../../statics/luckysheet/luckysheet.umd.js"
// import "../../statics/luckysheet/plugins/js/plugin.js"

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

function encodeUnicode(str) {
  // first we use encodeURIComponent to get percent-encoded UTF-8,
  // then we convert the percent encodings into raw bytes which
  // can be fed into btoa.
  return btoa(encodeURIComponent(str).replace(/%([0-9A-F]{2})/g,
    function toSolidBytes(match, p1) {
      return String.fromCharCode('0x' + p1);
    }));
}


function decodeUnicode(str) {
  // Going backwards: from bytestream, to percent-encoding, to original string.
  return decodeURIComponent(atob(str).split('').map(function (c) {
    return '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2);
  }).join(''));
}

export default {

  name: "SpreadsheetEditorApp",
  data() {
    return {
      file: null,
      ...mapGetters(['decodedAuthToken', 'endpoint']),
      saveDebounced: null,
      showSharingBox: false,
      contents: "",
      loading: true,
      newNameDialog: false,
      newName: null,
      document: null,
      containerId: "id-" + new Date().getMilliseconds(),
      screenWidth: (window.screen.width < 1200 ? window.screen.width : 1200) + "px",
    }
  },
  watch: {
    'contents': function (newVal, oldVal) {
      if (this.loading) {
        return
      }
      console.log("Contents changed", arguments)
      if (this.saveDebounced === null) {
        this.saveDebounced = debounce(this.saveDocument, 3000, true)
      }
      this.saveDebounced();
    }
  },
  methods: {
    logout() {
      this.$emit("logout");
    },
    loadEditor() {
      const that = this;
      setTimeout(function () {

        console.log("Create sheet")
        var options = {
          container: 'luckysheet', //luckysheet is the container id
          showinfobar: false,
          title: that.document ? that.document.document_name : "New document",
          userInfo: that.decodedAuthToken() !== null ? that.decodedAuthToken().email : 'Anonymous',
        }
        console.log("l", luckysheet)

        luckysheet.destroy();
        if (that.contents.length > 0) {
          try {
            console.log("set string data", that.contents)
            var item = that.contents;
            if (!item) {
              // item = workingData
            } else {
              item = JSON.parse(item)
            }

            if (item) {
              options.data = item;
              // luckysheet.buildGridData(item)
            }
            console.log("set sheet data", item)
            luckysheet.create(options);

          } catch (e) {
            console.log("Failed to parse data", e);
            luckysheet.create(options);
          }
        } else {
          console.log("Else just create")
          luckysheet.create(options);
        }
        if (that.decodedAuthToken() === null) {
          return;
        }
        setInterval(function () {
          that.loading = false;
          let newData = luckysheet.getluckysheetfile();
          if (!newData) {
            return
          }
          newData = newData.map(function (sheet) {
            // console.log("Get grid data for sheet", sheet)
            sheet.celldata = luckysheet.getGridData(sheet.data)
            // delete sheet.data
            return sheet;
          })
          var newContents = JSON.stringify(newData);
          that.contents = newContents;
          window.localStorage.setItem("d", newContents)
        }, 10000)

      }, 300)
    },
    saveDocumentState() {
      const that = this;
      // let newData = luckysheet.getluckysheetfile();
      // newData = newData.map(function (sheet) {
      //   console.log("Get grid data for sheet", sheet)
      // sheet.celldata = luckysheet.getGridData(sheet.data)
      // delete sheet.data
      // return sheet;
      // })
      let value = luckysheet.toJson();
      console.log("sheet json", value);
      that.contents = JSON.stringify(value);
      window.localStorage.setItem("d", that.contents)


    },
    newDocument() {
      const that = this;
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
      if (!this.newName.endsWith(".dsheet")) {
        this.newName = this.newName + ".dsheet"
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
        contents: that.contents,
        name: newFileName,
        type: "text/json"
      }
      this.newName = null;
      this.newNameDialog = false;
      this.document.document_content = [this.file]
    },
    saveDocument() {
      const that = this;
      console.log("save document", this.document, this.contents);
      if (!this.document) {
        this.newNameDialog = true;
        return
      }
      if (this.decodedAuthToken() === null) {
        return
      }
      this.document.tableName = "document";


      var zip = new JSZip();
      zip.file("contents_encoded.json", encodeUnicode(this.contents));

      zip.generateAsync({type: "base64"}).then(function (base64) {

        that.document.document_content[0].contents = "data:application/dspreadsheet," + base64
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
              if (documentTable.permission !== that.document.permission) {
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
            console.log("errer", err)
            that.$q.notify({
              message: "We are offline, changes are not being stored"
            })
          })
        } else {
          that.createRow(that.document).then(function (res) {
            that.document = res.data;
            console.log("Spreadsheet created", res);
            that.$router.push('/apps/spreadsheet/' + that.document.reference_id)
          }).catch(function (err) {
            console.log("eror", err)
            that.$q.notify({
              message: "We are offline, changes are not being stored"
            })
          })

        }


      })


    },
    ...mapActions(['loadData', 'updateRow', 'createRow'])
  },
  mounted() {
    const that = this;


    var script1 = document.createElement("script");
    script1.setAttribute("type", "text/javascript");
    script1.setAttribute("src", "/statics/luckysheet/plugins/js/plugin.js");
    document.getElementsByTagName("head")[0].appendChild(script1);

    var script = document.createElement("script");
    script.setAttribute("type", "text/javascript");
    script.setAttribute("src", "/statics/luckysheet/luckysheet.umd.js");

    document.getElementsByTagName("head")[0].appendChild(script);
    script.onload = function () {
      console.log("lucky loaded");

      that.containerId = "id-" + new Date().getMilliseconds();
      var documentId = that.$route.params.documentId;
      console.log("Mounted FilesApp", that.containerId, that.$route.params.documentId);
      if (documentId === "new") {
        that.file = {
          contents: "",
          name: "New file.html"
        }
        that.contents = that.file.contents;
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
        if (res.data === null || res.data.length < 1) {
          that.file = {
            contents: "",
            name: "New file.html",
            path: localStorage.getItem("_last_current_path") || "/"
          };
          that.loadEditor();
          return
        }
        that.document = res.data[0];
        if (that.document.document_content) {
          that.file = that.document.document_content[0];
        } else {
          that.file = {
            contents: btoa(""),
            name: that.document.document_name,
            type: "application/x-ddocument",
            path: localStorage.getItem("_last_current_path") || "/"
          }
          that.document.document_content = [that.file]
        }

        JSZip.loadAsync(atob(that.file.contents)).then(function (zipFile) {
          // that.contents = atob(that.file.contents);
          zipFile.file("contents_encoded.json").async("string").then(function (data) {
            console.log("Loaded file: ", data)
            that.contents = decodeUnicode(data);
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
}
</script>
