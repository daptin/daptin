<template>
  <div id="dropArea" class="row">
    <div class="col-1" style="padding: 8px" v-for="item in items">
      <q-card class="table-item" flat :style="{cursor: 'pointer', color: item.color}">
        <q-tooltip :delay="1000">{{ item.name }}</q-tooltip>
        <q-card-section class="text-center" avatar>
          <q-menu
            touch-position
            context-menu
          >

            <q-list dense style="min-width: 100px">
              <q-item clickable v-close-popup>
                <q-item-section>Open...</q-item-section>
              </q-item>
              <q-item clickable v-close-popup>
                <q-item-section>New</q-item-section>
              </q-item>
              <q-separator />
              <q-item clickable>
                <q-item-section>Preferences</q-item-section>
                <q-item-section side>
                  <q-icon name="keyboard_arrow_right" />
                </q-item-section>

                <q-menu anchor="top right" self="top left">
                  <q-list>
                    <q-item
                      v-for="n in 3"
                      :key="n"
                      dense
                      clickable
                    >
                      <q-item-section>Submenu Label</q-item-section>
                      <q-item-section side>
                        <q-icon name="keyboard_arrow_right" />
                      </q-item-section>
                      <q-menu auto-close anchor="top right" self="top left">
                        <q-list>
                          <q-item
                            v-for="n in 3"
                            :key="n"
                            dense
                            clickable
                          >
                            <q-item-section>3rd level Label</q-item-section>
                          </q-item>
                        </q-list>
                      </q-menu>
                    </q-item>
                  </q-list>
                </q-menu>

              </q-item>
              <q-separator />
              <q-item clickable v-close-popup>
                <q-item-section>Quit</q-item-section>
              </q-item>
            </q-list>

          </q-menu>
          <q-icon size="3em" :name="item.icon"/>
        </q-card-section>
        <q-card-section class="text-center text-white" style="padding: 4px">
          {{ item.name.substring(0, item.name.length > 20 ? 20 : item.name.length) }}
        </q-card-section>


      </q-card>
    </div>
  </div>
</template>
<style>
.table-item:hover {
  background: rgb(80, 80, 90);
}

.table-item {
  border-radius: 5px;
  background: rgb(40, 40, 45)
}
</style>
<script>
import {mapActions} from "vuex";

export default {
  name: "PaginatedTableView",
  props: ["items"],
  methods: {
    ...mapActions([]),
    traverseFileTree(item, path) {
      const that = this;
      path = path || "";
      if (item.isFile) {
        // Get file
        item.file(function (file) {
          console.log("File:", path + file.name);
        });
      } else if (item.isDirectory) {
        // Get folder contents
        var dirReader = item.createReader();
        dirReader.readEntries(function (entries) {
          for (var i = 0; i < entries.length; i++) {
            that.traverseFileTree(entries[i], path + item.name + "/");
          }
        });
      }
    },
    refreshData() {
      const that = this;
    }
  },
  mounted() {
    console.log("Mounted paginated table view", this.parameters);
    const that = this;
    that.refreshData();

    document.body.addEventListener("drop", function (event) {
      event.preventDefault();

      var items = event.dataTransfer.items;
      for (var i = 0; i < items.length; i++) {
        // webkitGetAsEntry is where the magic happens
        var item = items[i].webkitGetAsEntry();
        if (item) {
          that.traverseFileTree(item);
        }
      }
    }, false);

  }
}
</script>
