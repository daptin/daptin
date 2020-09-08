<template>
  <div id="dropArea" class="row">
    <div @click="itemClicked(item)" class="col-1" @touchstart.stop @contextmenu.stop style="padding: 8px"
         v-for="item in items">
      <q-menu context-menu>
        <q-list dense style="min-width: 100px">
          <q-item clickable v-close-popup>
            <q-item-section>Open</q-item-section>
          </q-item>
          <q-item cl ickable v-close-popup>
            <q-item-section>Rename</q-item-section>
          </q-item>
          <q-separator/>
          <q-item @click="deleteItem(item)" clickable v-close-popup>
            <q-item-section>Delete</q-item-section>
          </q-item>
          <q-separator/>
          <q-separator/>
        </q-list>
      </q-menu>
      <q-card class="table-item" flat :style="{cursor: 'pointer', color: item.color}">
        <q-tooltip :delay="1000">{{ item.name }}</q-tooltip>
        <q-card-section class="text-center" avatar>
          <q-icon size="2.5em" :name="item.icon"/>
        </q-card-section>
        <q-card-section class="text-center text-white" style="padding: 1px; overflow-wrap: anywhere; overflow: hidden">
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
  background: transparent;
}
</style>
<script>
import {mapActions} from "vuex";

export default {
  name: "PaginatedTableView",
  props: ["items"],
  methods: {
    deleteItem(item) {
      console.log("Item deleted", item)
      this.$emit('item-deleted', item)
    },
    itemClicked(item) {
      console.log("Item clicked", item)
      this.$emit('item-clicked', item)
    },
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
