<template>
  <div id="dropArea"  class="row">
    <div class="col-12">
      <q-markup-table style="background: transparent; ">
        <thead style="text-align: left">
        <tr>
          <th style="width: 50px"></th>
          <th>File name</th>
          <th>Size</th>
          <th>Last modified</th>
        </tr>
        </thead>
        <tbody @touchstart.stop @contextmenu.stop>
        <tr @dblclick="itemDoubleClicked(item)" @click="itemClicked(item)" style="cursor: pointer" v-for="item in items" v-if="item.is_dir">
          <q-menu context-menu>
            <q-list dense style="min-width: 100px">
              <q-item clickable v-close-popup>
                <q-item-section>Open</q-item-section>
              </q-item>
              <q-item @click="renameItem(item)" clickable v-close-popup>
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
          <td>
            <q-icon :style="{'color': item.color}" size="2.5em" :name="item.icon"/>
          </td>
          <td>{{ item.name }}</td>
          <td>{{ item.size }}</td>
          <td>{{ (item.updated_at || item.created_at)  }}</td>
        </tr>
        <tr @dblclick="itemDoubleClicked(item)" @click="itemClicked(item)" style="cursor: pointer" v-for="item in items" v-if="!item.is_dir">
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
          <td>
            <q-icon :style="{'color': item.color}" size="2.5em" :name="item.icon"/>
          </td>
          <td>{{ item.name }}</td>
          <td>{{ parseInt(item.document_content[0].size/1024) }} Kb</td>
          <td>{{ item.updated_at }}</td>
        </tr>
        </tbody>
      </q-markup-table>
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
    renameItem(item) {
      console.log("Item rename", item)
      this.$emit('item-rename', item)
    },
    itemClicked(item) {
      // console.log("Item clicked", item)
      this.$emit('item-clicked', item)
    },
    itemDoubleClicked(item) {
      // console.log("Item double clicked", item)
      this.$emit('item-double-clicked', item)
    },
    ...mapActions([]),
    refreshData() {
      const that = this;
    }
  },
  mounted() {
    console.log("Mounted paginated table view", this.parameters);
    const that = this;
    that.refreshData();


  }
}
</script>
