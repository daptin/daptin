<template>

  <q-page-container>
    <q-page>
      <user-header-bar :buttons="{
        after: [],
        }" title="Calendar"></user-header-bar>

      <div class="row text-white">
        <div :class="{'col-2': showSideBar}">
          <div class="row">
            <div class="col-12">
              &nbsp;
            </div>
          </div>
          <div class="row" v-if="showSideBar">
            <div class="col-12">
              <q-date
                today-btn
                mask="YYYY-MM-DD HH:mm:ss"
                v-model="date"
                minimal
                flat
                style="background: transparent; width: 200px; min-width: 0px"
                dark
              />
            </div>
          </div>
        </div>
        <div :class="{'col-10': showSideBar, 'col-12': !showSideBar, 'q-pa-md': true}">
          <div class="row">
            <div class="col-12">
              <q-toolbar>
                <q-btn flat label="Today" @click="calendar.gotoDate(new Date())"></q-btn>
                <q-btn icon="fas fa-plus" label="New Event" flat>
                  <q-menu>
                    <q-card dark style="width: 500px">
                      <q-card-section>
                        <q-input dark v-model="newEvent.title"></q-input>
                      </q-card-section>
                      <q-card-section>
                        @ <q-btn :label="newEvent.date.toDateString()" flat>
                          <q-popup-proxy>
                            <q-date v-model="pdate">
                              <div class="row items-center justify-end q-gutter-sm">
                                <q-btn label="Cancel" color="primary" flat v-close-popup/>
                                <q-btn label="OK" color="primary" flat @click="newEvent.date = pdate" v-close-popup/>
                              </div>
                            </q-date>
                          </q-popup-proxy>
                        </q-btn>

                      </q-card-section>
                      <q-card-actions align="right">
                        <q-btn class="float-right" label="Save">

                        </q-btn>
                      </q-card-actions>
                    </q-card>
                  </q-menu>
                </q-btn>

                <q-btn icon="fas fa-angle-left" @click="calendar.prev()" flat></q-btn>
                <q-btn icon="fas fa-angle-right" @click="calendar.next()" flat></q-btn>
                <q-space/>
                <q-btn-dropdown :label="calendarView" content-style="background: black" flat>
                  <q-list dark>
                    <q-item v-close-popup @click="setCalenderView('day')" clickable>
                      <q-item-section>Day</q-item-section>
                    </q-item>
                    <q-item v-close-popup @click="setCalenderView('week')" clickable>
                      <q-item-section>Week</q-item-section>
                    </q-item>
                    <q-item v-close-popup @click="setCalenderView('month')" clickable>
                      <q-item-section>Month</q-item-section>
                    </q-item>
                    <q-item v-close-popup @click="setCalenderView('schedule')" clickable>
                      <q-item-section>Day Schedule</q-item-section>
                    </q-item>
                    <q-item v-close-popup @click="setCalenderView('week schedule')" clickable>
                      <q-item-section>Week Schedule</q-item-section>
                    </q-item>
                  </q-list>
                </q-btn-dropdown>
              </q-toolbar>
            </div>
            <div class="col-12">
              <div :id="containerId"></div>
            </div>
          </div>
        </div>

      </div>
    </q-page>
  </q-page-container>

</template>
<style>

.fc .fc-list-sticky .fc-list-day > * {
  background: transparent;
}

.fc .fc-list-event:hover td {
  background: black;
}
</style>
<script>
import {Calendar} from '@fullcalendar/core';
import dayGridPlugin from '@fullcalendar/daygrid';
import timeGridPlugin from '@fullcalendar/timegrid';
import interactionPlugin from '@fullcalendar/interaction';
import listPlugin from '@fullcalendar/list';

export default {

  name: "FileBrowser",
  data() {
    return {
      searchInput: '',
      pdate: null,
      newEvent: {
        title: 'New event',
        date: new Date()
      },
      calendarView: 'month',
      date: new Date(),
      showSearchInput: false,
      calendar: null,
      showUploadComponent: false,
      showSideBar: true,
      viewParameters: {
        tableName: 'document'
      },
      containerId: "id-" + new Date().getMilliseconds(),
      screenWidth: (window.screen.width < 1200 ? window.screen.width : 1200) + "px",
    }
  },
  methods: {
    setCalenderView(view) {
      this.calendarView = view;
      switch (view) {
        case "week":
          this.calendar.changeView('timeGridWeek');
          return;
        case "day":
          this.calendar.changeView('timeGridDay');
          return;
        case "month":
          this.calendar.changeView('dayGridMonth');
          return;
        case "schedule":
          this.calendar.changeView('listDay');
          return;
        case "week schedule":
          this.calendar.changeView('listWeek');
          return;
      }
    },
    addNewEvent() {
      console.log("Add new event")
    }
  },
  computed: {},

  watch: {
    'date': function () {
      console.log("Date changed", this.date.toString())
      this.calendar.gotoDate(this.date.toString())
    }
  },
  mounted() {
    const that = this;
    that.containerId = "id-" + new Date().getMilliseconds();
    console.log("Mounted Calendar", that.containerId);

    window.onresize = function () {
      if (document.body.clientWidth > 1400 && !that.showSideBar) {
        that.showSideBar = true;
      } else if (document.body.clientWidth < 1400 && that.showSideBar) {
        that.showSideBar = false;
      }
    }
    window.onresize()

    setTimeout(function () {
      that.calendar = new Calendar(document.getElementById(that.containerId), {
        plugins: [dayGridPlugin, timeGridPlugin, listPlugin, interactionPlugin],
        initialView: 'dayGridMonth',
        selectable: true,
        events: [
          { // this object will be "parsed" into an Event Object
            title: 'The Title', // a property!
            start: '2020-09-08', // a property!
            end: '2020-09-10' // a property! ** see important note below about 'end' **
          }
        ],
        nowIndicator: true,
        height: window.screen.height - 200,
        headerToolbar: {
          start: '', // will normally be on the left. if RTL, will be on the right
          center: '',
          end: '' // will normally be on the right. if RTL, will be on the left
        },
        buttonIcons: {
          prev: 'left-single-arrow',
          next: 'right-single-arrow',
          prevYear: 'left-double-arrow',
          nextYear: 'right-double-arrow'
        },
        buttonText: {
          today: 'today',
          month: 'month',
          next: '>',
          prev: '<',
          week: 'week',
          day: 'day',
          list: 'list'
        },
        navLinks: true,
      });
      that.calendar.render();

    }, 300)
  }
}
</script>
