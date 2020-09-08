<script>
// MyUploader.js
import {QUploaderBase} from 'quasar'
import Vue from 'vue'

export default {
  name: 'DaptinDocumentUploader',

  mixins: [QUploaderBase],
  props: ['uploadFile'],
  data() {
    return {
      isUploadingOnGoing: false,
    }
  },
  computed: {
    // [REQUIRED]
    // we're working on uploading files
    isUploading() {
      // return <Boolean>
      return this.isUploadingOnGoing
    },

    // [optional]
    // shows overlay on top of the
    // uploader signaling it's waiting
    // on something (blocks all controls)
    isBusy() {
      // return <Boolean>
      return false
    }
  },

  methods: {
    // [REQUIRED]
    // abort and clean up any process
    // that is in progress
    abort() {
      // ...
    },

    // [REQUIRED]
    upload() {
      const that = this;
      console.log("Upload requested", this.files);
      that.isUploadingOnGoing = true;
      const uploadFile = function (file) {
        return new Promise(function (resolve, reject) {
          const name = file.name;
          const reader = new FileReader();
          file.__status = "Reading file"
          // Vue.set(file, "__status", "Reading file")
          console.log("Loading file", file);
          reader.onload = function (fileResult) {
            file.file = fileResult.target.result
            resolve(file);
          };
          reader.onerror = function (e) {
            console.log("Failed to load file onerror", e, arguments);
            reject(name);
          };
          reader.readAsDataURL(file);
        })
      }

      Promise.all(that.files.map(uploadFile)).then(function (res) {
        console.log("files loaded", res);
        res.map(function (e) {
          e.__status = "Uploading file"
        })
        Promise.all(res.map(that.uploadFile)).then(function (res) {
          console.log("Upload complete", res);
          res.map(function (e) {
            e.__status = "File uploaded"
          })
          that.isUploadingOnGoing = false;
          that.$emit("uploadComplete")
        }).catch(function (err) {
          console.log("Upload failed", err)
          that.$q.notify({
            title: "Upload failed",
            message: err[0].title
          })
        })
      }).catch(function (err) {
        that.isUploadingOnGoing = false;
        console.log("Failed to upload file ", err, arguments)
        that.$q.notify({
          title: "Upload failed",
          message: err
        })
      })


    }
  }
}
</script>
