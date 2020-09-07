<template>
  <div class="row">
    <div class="col-12">
      <div :id="containerId" class="absolute-center" :style="{width: '100%', height:'100vh'}">
      </div>
    </div>
  </div>
</template>
<script>

export default {

  name: "FilesApp",
  data() {
    return {
      containerId: "id-" + new Date().getMilliseconds(),
      screenWidth: (window.screen.width < 1200 ? window.screen.width : 1200) + "px",
    }
  },
  mounted() {
    const that = this;
    this.containerId = "id-" + new Date().getMilliseconds();
    console.log("Mounted FilesApp", this.containerId);
    setTimeout(function () {
      Wodo.createTextEditor(that.containerId, {
        loadCallback: function () {
          console.log("load callback editor", arguments)
        },
        saveCallback: function () {
          console.log("save callback editor", arguments)
        },
        allFeaturesEnabled: true
      }, function (err, editor) {
        console.log("Editor created", arguments)
        editor.openDocumentFromUrl("statics/welcome.odt")
      });
    }, 300)


  }
}
</script>
