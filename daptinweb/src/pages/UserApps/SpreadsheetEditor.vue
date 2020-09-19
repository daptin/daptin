<template>
  <q-page-container>

    <q-header elevated class="bg-white text-black">
      <div class="row">
        <div class="12">
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
                      <q-item-section>Save spreadsheet</q-item-section>
                    </q-item>
                    <q-item @click="saveDocument()" clickable v-close-popup>
                      <q-item-section>Export</q-item-section>
                      <q-menu>
                        <q-list>
                          <q-item>To xlsx</q-item>
                        </q-list>
                      </q-menu>
                    </q-item>
                    <q-item @click="window.print()" clickable v-close-popup>
                      <q-item-section>Print</q-item-section>
                    </q-item>
                  </q-list>
                </q-menu>
              </q-btn>
              <q-btn flat label="Edit"></q-btn>
              <q-btn flat label="Format"></q-btn>
              <q-btn flat label="Data"></q-btn>
              <q-btn flat label="Help"></q-btn>
            </q-btn-group>
          </q-bar>
        </div>
      </div>
      <div class="row">
      </div>
    </q-header>
    <q-page>
      <div id="luckysheet"
           style="margin:0px;padding:0px;position:absolute;width:100%;height:100%;left: 0px;top: -25px; bottom: 0"></div>

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
      ...mapGetters(['decodedAuthToken']),
      saveDebounced: null,
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
      // console.log("Contents changed", arguments)
      if (this.saveDebounced === null) {
        this.saveDebounced = debounce(this.saveDocument, 3000, true)
      }
      this.saveDebounced();
    }
  },
  methods: {
    loadEditor() {
      const that = this;
      setTimeout(function () {

        console.log("Create sheet")
        var options = {
          container: 'luckysheet', //luckysheet is the container id
          showinfobar: false,
          title: that.document ? that.document.document_name : "New document",
          userInfo: that.decodedAuthToken.email,
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
          luckysheet.create(options);
        }
        setInterval(function () {
          that.loading = false;
          let newData = luckysheet.getluckysheetfile();
          newData = newData.map(function (sheet) {
            console.log("Get grid data for sheet", sheet)
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
      let newData = luckysheet.getluckysheetfile();
      newData = newData.map(function (sheet) {
        console.log("Get grid data for sheet", sheet)
        sheet.celldata = luckysheet.getGridData(sheet.data)
        // delete sheet.data
        return sheet;
      })
      that.contents = JSON.stringify(newData);
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

      var newFileName = null;
      newFileName = this.newName;

      this.document = {
        document_name: newFileName,
        document_extension: "html",
        mime_type: "text/html",
        document_path: "/"
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
      this.document.tableName = "document";
      this.document.document_content[0].contents = "data:text/html," + encodeUnicode(this.contents)
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
          console.log("Spreadsheet created", res);
          that.$router.push('/apps/spreadsheet/' + that.document.reference_id)
        }).catch(function (err) {
          console.log("eror", err)
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
        that.document = res.data[0];
        that.file = that.document.document_content[0];
        that.contents = decodeUnicode(that.file.contents);


        that.loadEditor()
      })


    }


  }
}
</script>
