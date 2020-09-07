<script>
// MyUploader.js
import {QUploaderBase} from 'quasar'

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
          const type = file.type;
          const reader = new FileReader();
          console.log("Loading file", file.name);
          reader.onload = function (fileResult) {
            resolve({
              name: name,
              file: fileResult.target.result,
              type: type
            });
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
          e["__progressLabel"] = "25%"
        })
        Promise.all(res.map(that.uploadFile)).then(function (res) {
          console.log("Upload complete");
          that.isUploadingOnGoing = false;
          that.$emit("uploadComplete")
        })
      }).catch(function (err) {
        that.isUploadingOnGoing = false;
        console.log("Failed to upload file ", err, arguments)
      })


    }
  }
}
</script>
