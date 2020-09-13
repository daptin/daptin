<template>
  <div id="app" style="height: 100vh">
    <ejs-documenteditorcontainer class="fullscreen document-editor" style="height: 100vh" :serviceUrl='serviceUrl' :enableToolbar='true'></ejs-documenteditorcontainer>
  </div>
</template>
<style>
.document-editor{
  height: 100vh !important;
}
</style>

<script>
import Vue from 'vue';
import {
  DocumentEditorContainerPlugin,
  DocumentEditorContainerComponent,
  Toolbar
} from '@syncfusion/ej2-vue-documenteditor';

Vue.use(DocumentEditorContainerPlugin);
export default {
  data() {
    return {serviceUrl: 'https://ej2services.syncfusion.com/production/web-services/api/documenteditor/'}
  },
  provide: {
    DocumentEditorContainer: [Toolbar]
  }
}
</script>
