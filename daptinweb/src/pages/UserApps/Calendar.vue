<template>

  <q-page-container>
    <q-page>
      <div class="row q-pa-md text-white">
        <div class="col-12">
          <div :id="containerId"></div>
        </div>

      </div>
    </q-page>
  </q-page-container>

</template>
<script>
import {Calendar} from '@fullcalendar/core';
import dayGridPlugin from '@fullcalendar/daygrid';
import timeGridPlugin from '@fullcalendar/timegrid';
import listPlugin from '@fullcalendar/list';

export default {

  name: "FileBrowser",
  data() {
    return {
      searchInput: '',
      showSearchInput: false,
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
    that.containerId = "id-" + new Date().getMilliseconds();
    console.log("Mounted Calendar", that.containerId);
    setTimeout(function () {
      let calendar = new Calendar(document.getElementById(that.containerId), {
        plugins: [dayGridPlugin, timeGridPlugin, listPlugin],
        initialView: 'dayGridMonth',
        height: window.screen.height - 200,
        headerToolbar: {
          start: 'title', // will normally be on the left. if RTL, will be on the right
          center: 'dayGridMonth timeGridWeek listWeek',
          end: 'today prev,next' // will normally be on the right. if RTL, will be on the left
        },
        navLinks: true
      });
      calendar.render();

    }, 300)
  }
}
</script>
