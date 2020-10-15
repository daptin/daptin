<template>

  <q-page-container>
    <q-page>
      <user-header-bar style="border-bottom: 1px solid black" :buttons="{
        after: [],
        }" title="Calendar"></user-header-bar>

      <div class="row">
        <div :class="{'col-2': showSideBar}">
          <!--          <div class="row q-pa-md">-->
          <!--            <div class="col-12">-->
          <!--              &nbsp;<q-btn style="border: 1px solid black" flat label="Today"-->
          <!--                           @click="setDate()"></q-btn>-->
          <!--            </div>-->
          <!--          </div>-->
          <div class="row" v-if="showSideBar">
            <div class="col-12">
              <q-date
                v-model="date"
                @input="setDate"
                today-btn
                minimal
                flat
                style="background: transparent; width: 200px; min-width: 0px"
              />
            </div>
            <div class="col-12 q-pa-md" style="display: none">
              <div @drop="eventTrashed" class="text-center vertical-middle trash-box"
                   style="height: 100px; width: 100%; border: 1px solid red; padding: 5px; border-radius: 5px">
                <br/>
                <q-icon size="3em" name="fas fa-trash"></q-icon>
                <br/>
                <span class="text-small">Drop events here to delete them</span>
              </div>
            </div>
          </div>
        </div>
        <div style="border-left: 1px solid black" :class="{'col-10': showSideBar, 'col-12': !showSideBar}">
          <div class="row">
            <div class="col-12">
              <q-toolbar>
                <q-btn style="border: 1px solid black" flat label="Today"
                       @click="setDate()"></q-btn>


                <q-btn flat @click="calendar.refetchEvents()" icon="fas fa-sync-alt"></q-btn>
                <span class="text-h6">{{ monthNames[date.getMonth()] }} {{ date.getFullYear() }}</span>
                <q-btn @click="(showEventDialogTarget = true) && (showEventDialog = true)" icon="fas fa-plus" flat>
                  <q-menu :target="showEventDialogTarget" ref="newEventDialog" style="overflow: hidden">
                    <q-bar>
                      <div class="text-weight-bold ">
                        New event
                      </div>
                    </q-bar>
                    <q-card style="min-width: 450px; overflow: hidden;" class="q-pa-md">

                      <q-card-section>
                        <q-input label="Title" v-model="newEvent.event_title"></q-input>
                      </q-card-section>
                      <q-card-section style="padding-left: 10px">
                        <div class="row">
                          <div class="col-6">
                            <q-btn-toggle size="sm" style="padding: 5px;" dense v-model="newEvent.event_type" flat
                                          :options="[
                              {label: 'Event', value: 'event'},
                              {label: 'Reminder', value: 'reminder'},
                              {label: 'Task', value: 'task'}
                        ]">
                            </q-btn-toggle>
                          </div>
                          <div class="col-6">
                            <q-checkbox v-model="newEvent.all_day" label="Full day event"></q-checkbox>
                          </div>
                        </div>


                      </q-card-section>

                      <q-tab-panels v-model="newEvent.event_type" class="shadow-2 rounded-borders bg-transparent">
                        <q-tab-panel class="new-event-panel" name="event">
                          <q-card-section>

                            <q-input label="Event date and time" filled v-model="newEvent.date">
                              <template v-slot:prepend>
                                <q-icon name="event" class="cursor-pointer">
                                  <q-popup-proxy transition-show="scale" transition-hide="scale">
                                    <q-date v-model="newEvent.date" mask="YYYY-MM-DD HH:mm">
                                      <div class="row items-center justify-end">
                                        <q-btn v-close-popup label="Close" color="primary" flat/>
                                      </div>
                                    </q-date>
                                  </q-popup-proxy>
                                </q-icon>
                              </template>

                              <template v-slot:append>
                                <q-icon name="access_time" class="cursor-pointer">
                                  <q-popup-proxy transition-show="scale" transition-hide="scale">
                                    <q-time v-model="newEvent.date" mask="YYYY-MM-DD HH:mm" format24h>
                                      <div class="row items-center justify-end">
                                        <q-btn v-close-popup label="Close" color="primary" flat/>
                                      </div>
                                    </q-time>
                                  </q-popup-proxy>
                                </q-icon>
                              </template>
                            </q-input>

                          </q-card-section>

                          <q-card-section v-if="newEventConfig.showAddDescription">
                            <q-editor label="Description" v-model="newEvent.event_description">
                            </q-editor>
                          </q-card-section>
                          <q-card-section v-if="newEventConfig.showAddLocation">
                            <q-input label="Event location" v-model="newEvent.event_location">
                            </q-input>
                          </q-card-section>
                        </q-tab-panel>

                        <q-tab-panel class="new-event-panel" name="reminder">
                          <q-card-section>

                            <q-input label="Event date and time" filled v-model="newEvent.date">
                              <template v-slot:prepend>
                                <q-icon name="event" class="cursor-pointer">
                                  <q-popup-proxy transition-show="scale" transition-hide="scale">
                                    <q-date v-model="newEvent.date" mask="YYYY-MM-DD HH:mm">
                                      <div class="row items-center justify-end">
                                        <q-btn v-close-popup label="Close" color="primary" flat/>
                                      </div>
                                    </q-date>
                                  </q-popup-proxy>
                                </q-icon>
                              </template>

                              <template v-slot:append>
                                <q-icon name="access_time" class="cursor-pointer">
                                  <q-popup-proxy transition-show="scale" transition-hide="scale">
                                    <q-time v-model="newEvent.date" mask="YYYY-MM-DD HH:mm" format24h>
                                      <div class="row items-center justify-end">
                                        <q-btn v-close-popup label="Close" color="primary" flat/>
                                      </div>
                                    </q-time>
                                  </q-popup-proxy>
                                </q-icon>
                              </template>
                            </q-input>
                          </q-card-section>

                        </q-tab-panel>

                        <q-tab-panel class="new-event-panel" name="task">

                          <q-card-section>
                            <q-input label="Event date and time" filled v-model="newEvent.date">
                              <template v-slot:prepend>
                                <q-icon name="event" class="cursor-pointer">
                                  <q-popup-proxy transition-show="scale" transition-hide="scale">
                                    <q-date v-model="newEvent.date" mask="YYYY-MM-DD HH:mm">
                                      <div class="row items-center justify-end">
                                        <q-btn v-close-popup label="Close" color="primary" flat/>
                                      </div>
                                    </q-date>
                                  </q-popup-proxy>
                                </q-icon>
                              </template>

                              <template v-slot:append>
                                <q-icon name="access_time" class="cursor-pointer">
                                  <q-popup-proxy transition-show="scale" transition-hide="scale">
                                    <q-time v-model="newEvent.date" mask="YYYY-MM-DD HH:mm" format24h>
                                      <div class="row items-center justify-end">
                                        <q-btn v-close-popup label="Close" color="primary" flat/>
                                      </div>
                                    </q-time>
                                  </q-popup-proxy>
                                </q-icon>
                              </template>
                            </q-input>
                          </q-card-section>

                        </q-tab-panel>
                      </q-tab-panels>


                      <q-card-actions align="right">
                        <q-btn class="float-right " style="border: 1px solid black" @click="createEvent()" flat
                               label="Save">

                        </q-btn>
                      </q-card-actions>
                    </q-card>
                  </q-menu>
                </q-btn>

                <q-btn icon="fas fa-angle-left" @click="calendar.prev()" flat></q-btn>
                <q-btn icon="fas fa-angle-right" @click="calendar.next()" flat></q-btn>
                <q-space/>
                <q-btn-dropdown :label="calendarView" flat>
                  <q-list>
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
            <div class="col-12 q-pa-md">
              <div :id="containerId" style="height: calc(100vh - 30%)"></div>
            </div>
          </div>
        </div>

      </div>
    </q-page>
  </q-page-container>

</template>
<style>
.trash-box:hover {
  color: red;
}

.new-event-panel .q-card__section {
  padding: 0;
}

.q-btn-toggle button.q-btn {
  padding: 5px;
}

.fc .fc-list-sticky .fc-list-day > * {
  background: transparent;
}

.fc .fc-list-event:hover td {
}
</style>
<script>
import {Calendar} from '@fullcalendar/core';
import dayGridPlugin from '@fullcalendar/daygrid';
import timeGridPlugin from '@fullcalendar/timegrid';
import interactionPlugin from '@fullcalendar/interaction';
import listPlugin from '@fullcalendar/list';
import {mapActions} from "vuex";

export default {

  name: "Calendar",
  data() {
    return {
      searchInput: '',
      showEventDialog: false,
      showEventDialogTarget: true,
      monthNames: ["January", "February", "March", "April", "May", "June",
        "July", "August", "September", "October", "November", "December"
      ],
      pdate: new Date(),
      newEventConfig: {
        showAddDescription: false,
        showAddLocation: false,
      },
      newEvent: {
        event_title: 'New event',
        event_type: 'event',
        event_description: null,
        event_location: null,
        all_day: false,
        date: new Date(),
        event_start_date: new Date(),
        event_end_date: null,
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
    eventTrashed() {
      console.log("Event trashed", arguments)
    },
    ...mapActions(['createRow', "loadData", "updateRow"]),
    createEvent() {
      const that = this;
      console.log("Create new event", this.newEvent);
      this.newEvent.tableName = "calendar";
      this.newEvent.event_start_date = this.newEvent.date;
      this.createRow(this.newEvent).then(function (res) {
        console.log("created event", res)
        that.calendar.refetchEvents();
        that.$refs.newEventDialog.hide();
        that.newEvent = {
          event_title: that.newEvent.event_title,
          event_type: that.newEvent.event_type,
          event_description: null,
          event_location: null,
          all_day: false,
          date: new Date(),
          event_start_date: new Date(),
          event_end_date: null,
        }
      }).catch(function (err) {
        console.log("Failed to create event", err);
        that.$q.notify({
          message: "Failed to create event " + JSON.stringify(err)
        })

      })
    },
    setDate(date) {
      console.log("set date", date)
      if (!date) {
        date = new Date();
      } else {
        date = new Date(Date.parse(date));
      }
      this.date = date;
      this.calendar.gotoDate(date);
    },
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
      // this.calendar.gotoDate(this.date.toString())
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
        editable: true,
        eventResize: function (dropInfo) {
          console.log("drop info", dropInfo)
          var referenceId = dropInfo.oldEvent._def.extendedProps.reference_id;
          that.updateRow({
            tableName: "calendar",
            id: referenceId,
            event_end_date: dropInfo.event.end
          }).then(function (res) {
            console.log("Event saved");
          }).catch(function (err) {
            console.log("Failed to save event", err);
            that.calendar.refetchEvents();
            that.$q.notify({
              message: "Failed to save event: " + JSON.stringify(err)
            })
          })
        },
        eventDrop: function (dropInfo) {
          console.log("drop info", dropInfo)
          var referenceId = dropInfo.oldEvent._def.extendedProps.reference_id;
          that.updateRow({
            tableName: "calendar",
            id: referenceId,
            event_start_date: dropInfo.event.start,
            event_end_date: dropInfo.event.end
          }).then(function (res) {
            console.log("Event saved");
          }).catch(function (err) {
            console.log("Failed to save event", err);
            dropInfo.revert()
            that.$q.notify({
              message: "Failed to save event: " + JSON.stringify(err)
            })
          })
        },
        eventClick: function (info) {
          console.log("Event clicked", info)
        },

        events: function (info, successCallback, failureCallback) {
          console.log("get events for date: ", info);
          that.loadData({
            tableName: 'calendar',
            params: {
              query: JSON.stringify([
                {
                  column: "event_start_date",
                  operator: "after",
                  value: info.start
                },
                {
                  column: "event_start_date",
                  operator: "before",
                  value: info.end
                }
              ])
            }
          }).then(function (res) {
            console.log("Events", res.data)
            successCallback(res.data.map(function (e) {
              e.title = e.event_title;
              e.start = e.event_start_date;
              e.end = e.event_end_date;
              e.id = e.reference_id
              return e;
            }));
          }).catch(function (err) {
            console.log("Failed to load events", err)
            that.$q.notify({
              message: "Failed to load events: " + JSON.stringify(err)
            });
            failureCallback(err)
          })
        },
        dateClick: function (info) {
          if (that.showEventDialog) {
            that.showEventDialogTarget = true;
            that.showEventDialog = false;
            that.$refs.newEventDialog.hide();
            return
          }
          console.log('Clicked on : ', info);
          that.newEvent.date = info.date;
          that.showEventDialog = true;
          that.showEventDialogTarget = info.dayEl;
          that.$refs.newEventDialog.show();
          // change the day's background color just for fun
          // info.dayEl.style.backgroundColor = 'red';
        },
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
