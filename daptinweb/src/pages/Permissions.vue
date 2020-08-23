<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">

  <div>
    <div class="col-12 q-pa-md">
      <span class="text-h5">Table permissions</span>
    </div>

    <div class="col-12">
      <div>


        <div class="row">

          <div class="col-12 q-pa-md q-gutter-md">

            <q-card flat class="bg-grey-3">

              <q-card-section>
                <q-select @input="saveTablePermissionModel()" option-value="value" map-options emit-value
                          option-label="label" v-model="selectedPermissionOption"
                          :options="simplePermissionOptions"></q-select>

              </q-card-section>
            </q-card>


          </div>

          <div class="col-12 q-pa-md items-start q-gutter-md">
            <q-card>
              <q-card-section>
                <div class="text-h6">Table owner</div>
              </q-card-section>

              <q-card-section class="q-pt-none">
                {{ selectedTable.user_account_id ? selectedTable.user_account_id.email : 'n/a' }}
              </q-card-section>


              <q-card-actions>
                <q-btn @click="showOwnerSelectionBox()" flat>Change owner</q-btn>
              </q-card-actions>
            </q-card>

            <q-card>
              <q-card-section>
                <div class="text-h6">Table groups</div>
              </q-card-section>
              <q-card-section class="q-pt-none">
                <q-markup-table flat>
                  <tbody>
                  <tr v-for="group in tableGroups">
                    <td>{{ group.name }}</td>
                    <td class="text-right">
                      <q-btn icon="fas fa-trash" flat size="xs" @click="removeTableFromGroup(group)"></q-btn>
                    </td>
                  </tr>
                  </tbody>
                </q-markup-table>
              </q-card-section>
              <q-card-actions>
                <div class="row">
                  <q-btn class="float-right" flat label="Add group" @click="groupChangeForTableGroups()"></q-btn>
                </div>
              </q-card-actions>
            </q-card>

            <q-card>
              <q-card-section>
                <div class="text-h6">New row to be added to following groups</div>
              </q-card-section>
              <q-card-section class="q-pt-none">
                <q-markup-table flat>
                  <tbody>
                  <tr v-for="group in tableSchema.DefaultGroups">
                    <td>{{ group }}</td>
                    <td class="text-right">
                      <q-btn icon="fas fa-trash" flat size="xs" @click="removeGroupFromDefaultGroups(group)"></q-btn>
                    </td>
                  </tr>
                  </tbody>
                </q-markup-table>

              </q-card-section>
              <q-card-actions>
                <q-btn flat label="Add group" @click="groupChangeForNewRowGroups()"></q-btn>
              </q-card-actions>
            </q-card>

          </div>

        </div>

        <q-dialog v-model="addToGroup">
          <q-card>
            <q-card-section>
              <div class="text-h6">Add table to new group</div>
            </q-card-section>


            <q-card-section class="q-pt-none">
              <q-select flat :options="userGroups" option-label="name" option-value="reference_id"
                        v-model="addToGroupId"></q-select>
            </q-card-section>

            <q-card-actions align="right">
              <q-btn flat label="Cancel" color="warning" v-close-popup/>
              <q-btn @click="updateTableGroups()" flat label="Add" color="primary" v-close-popup/>
            </q-card-actions>
          </q-card>
        </q-dialog>

        <q-dialog v-model="ownerSelectionBox">
          <q-card style="width: 600px">
            <q-card-section>
              <div class="text-h6">Set new owner</div>

            </q-card-section>

            <q-card-section class="q-pt-none">
              <q-select flat :options="userAccounts" option-label="email" option-value="reference_id"
                        v-model="newOwnerId"></q-select>
            </q-card-section>

            <q-card-actions align="right">
              <q-btn flat label="Cancel" color="warning" v-close-popup/>
              <q-btn @click="setOwner(newOwnerId)" flat label="Set" color="primary" v-close-popup/>
            </q-card-actions>
          </q-card>
        </q-dialog>

        <q-page-sticky position="top-right" :offset="[20, 20]">
          <q-btn @click="$emit('close')" flat icon="fas fa-times"></q-btn>
        </q-page-sticky>

      </div>
    </div>
  </div>


</template>

<script>
import {mapActions, mapGetters, mapState} from 'vuex';

export default {
  name: 'TablePermissions',
  props: {
    selectedTable: Object
  },
  methods: {
    removeGroupFromDefaultGroups(group) {
      const that = this;
      var currentGroups = that.tableSchema.DefaultGroups;
      console.log("Current groups", group);
      var toRemove = currentGroups.indexOf(group);
      if (toRemove === -1) {
        return
      }
      currentGroups.splice(toRemove, 1)


      that.updateRow({
        tableName: "world",
        id: that.selectedTable.reference_id,
        world_schema_json: JSON.stringify(that.tableSchema),
      }).then(function () {
        that.$q.notify({
          message: "Saved"
        });
      }).catch(function (e) {
        console.log("Failed to remove group from default groups", e);
        that.$q.notify({
          message: "Failed to save"
        });
      });
    },
    saveTablePermissionModel() {
      console.log("new table permission", this.selectedPermissionOption)
      const that = this;
      that.updateRow({
        tableName: "world",
        id: that.selectedTable.reference_id,
        permission: that.selectedPermissionOption,
        default_permission: that.selectedPermissionOption
      }).then(function (usersOfGroup) {
        console.log("Updated table permission", usersOfGroup);
        that.loadTableGroups();
      }).catch(function (err) {
        that.$q.notify({
          message: "Failed to update table permission"
        })
      })
    },
    removeTableFromGroup(group) {
      console.log("removeTableFromGroup", group);

      const that = this;
      that.removeRelation({
        tableName: "usergroup",
        id: group.relation_reference_id,
        relationName: "world_id",
        relationId: that.selectedTable.reference_id
      }).then(function (usersOfGroup) {
        console.log("Removed user ", usersOfGroup);
        that.loadTableGroups();
      }).catch(function (err) {
        that.$q.notify({
          message: "Failed to remove table from group"
        })
      })

    },
    updateTableGroups() {
      const that = this;
      console.log("Add groups", this.groupChangeFor, this.addToGroupId);
      switch (this.groupChangeFor) {
        case 'tableGroups':

          that.addManyRelation({
            tableName: "world",
            id: that.selectedTable.reference_id,
            relationId: this.addToGroupId.id,
            relationName: 'usergroup_id',
          }).then(function () {
            that.$q.notify({
              message: "Added group"
            });
            that.loadTableGroups()
          }).catch(function (e) {
            console.log("Failed to add group", e);
            that.$q.notify({
              message: "Failed to save"
            });
          });


          break;
        case 'newRowGroups':
          var currentGroups = that.tableSchema.DefaultGroups;
          console.log("Current groups", currentGroups);
          currentGroups.push(this.addToGroupId.name);


          that.updateRow({
            tableName: "world",
            id: that.selectedTable.reference_id,
            world_schema_json: JSON.stringify(that.tableSchema),
          }).then(function () {
            that.$q.notify({
              message: "Saved"
            });
          }).catch(function (e) {
            console.log("Failed to add new group", e);
            that.$q.notify({
              message: "Failed to save"
            });
          });

          break;

      }
    },
    setOwner(user) {
      const that = this;
      console.log("set new owner id", user, that.selectedTable);
      if (user != null) {
        that.selectedTable.user_account_id = user.reference_id;
        that.addRelation({
          tableName: "world",
          id: that.selectedTable.reference_id,
          relationId: that.selectedTable.user_account_id,
          relationName: 'user_account_id',
        }).then(function () {
          that.$q.notify({
            message: "Saved"
          });
          that.selectedTable.user_account_id = user;
        }).catch(function (e) {
          console.log("Failed to save new owner", e);
          that.$q.notify({
            message: "Failed to save"
          });
        });
      } else {
        that.removeRelation({
          tableName: "world",
          id: that.selectedTable.reference_id,
          relationName: 'user_account_id',
          relationId: that.selectedTable.user_account_id,
        }).then(function () {
          that.selectedTable.user_account_id = null;
          that.$q.notify({
            message: "Removed owner"
          });
        }).catch(function (e) {
          console.log("Failed to remove owner", e);
          that.$q.notify({
            message: "Failed to save"
          });
        });
      }


    },
    showOwnerSelectionBox() {
      this.ownerSelectionBox = true;
    },
    groupChangeForTableGroups() {
      this.groupChangeFor = 'tableGroups';
      this.addToGroup = true
    },
    groupChangeForNewRowGroups() {
      this.groupChangeFor = 'newRowGroups';
      this.addToGroup = true
    },
    ...mapActions(['loadData', 'loadModel', 'loadDataRelations', 'updateRow', 'removeRelation', 'addRelation', 'addManyRelation', 'loadDataRelations']),
    refresh() {
      const that = this;
      console.log("Table schema json", that.selectedTable);
      that.selectedPermissionOption = that.selectedTable.permission;

      that.tableSchema = JSON.parse(that.selectedTable.world_schema_json);

      var permissionValue = that.selectedTable.permission;
      that.parsedGuestPermission = {
        canPeek: (permissionValue & that.permissionStructure.GuestPeek) === that.permissionStructure.GuestPeek,
        canRead: (permissionValue & that.permissionStructure.GuestRead) === that.permissionStructure.GuestRead,
        canCreate: (permissionValue & that.permissionStructure.GuestCreate) === that.permissionStructure.GuestCreate,
        canUpdate: (permissionValue & that.permissionStructure.GuestUpdate) === that.permissionStructure.GuestUpdate,
        canDelete: (permissionValue & that.permissionStructure.GuestDelete) === that.permissionStructure.GuestDelete,
        canRefer: (permissionValue & that.permissionStructure.GuestRefer) === that.permissionStructure.GuestRefer,
        canExecute: (permissionValue & that.permissionStructure.GuestExecute) === that.permissionStructure.GuestExecute,
      };
      that.parsedOwnerPermission = {
        canPeek: (permissionValue & that.permissionStructure.UserPeek) === that.permissionStructure.UserPeek,
        canRead: (permissionValue & that.permissionStructure.UserRead) === that.permissionStructure.UserRead,
        canCreate: (permissionValue & that.permissionStructure.UserCreate) === that.permissionStructure.UserCreate,
        canUpdate: (permissionValue & that.permissionStructure.UserUpdate) === that.permissionStructure.UserUpdate,
        canDelete: (permissionValue & that.permissionStructure.UserDelete) === that.permissionStructure.UserDelete,
        canRefer: (permissionValue & that.permissionStructure.UserRefer) === that.permissionStructure.UserRefer,
        canExecute: (permissionValue & that.permissionStructure.UserExecute) === that.permissionStructure.UserExecute,
      };
      that.parsedGroupPermission = {
        canPeek: (permissionValue & that.permissionStructure.GroupPeek) === that.permissionStructure.GroupPeek,
        canRead: (permissionValue & that.permissionStructure.GroupRead) === that.permissionStructure.GroupRead,
        canCreate: (permissionValue & that.permissionStructure.GroupCreate) === that.permissionStructure.GroupCreate,
        canUpdate: (permissionValue & that.permissionStructure.GroupUpdate) === that.permissionStructure.GroupUpdate,
        canDelete: (permissionValue & that.permissionStructure.GroupDelete) === that.permissionStructure.GroupDelete,
        canRefer: (permissionValue & that.permissionStructure.GroupRefer) === that.permissionStructure.GroupRefer,
        canExecute: (permissionValue & that.permissionStructure.GroupExecute) === that.permissionStructure.GroupExecute,
      };
      this.loadTableGroups();
    },
    loadTableGroups() {
      const that = this;
      that.loadDataRelations({
        tableName: 'world',
        relation: 'usergroup_id',
        reference_id: that.selectedTable.reference_id,
      }).then(function (res) {
        console.log("Loaded groups of table", that.selectedTable.table_name, res);
        that.tableGroups = res.data;
      }).catch(function (err) {
        that.$q.notify({
          message: "Failed to load usergroups: " + JSON.stringify(err)
        })
      })
    }
  },
  data() {
    return {
      text: '',
      selectedPermissionOption: null,
      simplePermissionOptions: [{
        label: 'Guests cannot see the data in this table',
        value: 2097024
      }, {
        label: 'Guests can read rows',
        value: 2097027
      }, {
        label: 'Guests can read rows or execute actions on them',
        value: 2097059
      }, {
        label: 'Guests can read and create rows',
        value: 2097031
      }, {
        label: 'Guests can read, create rows and execute some actions on them',
        value: 2097063
      }, {
        label: 'Guests CANNOT read, create rows BUT execute some actions on them',
        value: 2097056
      }],
      permissionTypeTab: 'basic',
      newOwnerId: null,
      ownerSelectionBox: false,
      newGroupName: '',
      groupChangeFor: null,
      addToGroupId: null,
      tableSchema: {},
      addToGroup: false,
      addToGroupSwitch: 'addExisting',
      tableGroups: [],
      selectedTab: 'tablePermissions',
      ...mapState([]),
      permissionStructure: {
        None: 0,
        GuestPeek: 1 << 0,
        GuestRead: 1 << 1,
        GuestCreate: 1 << 2,
        GuestUpdate: 1 << 3,
        GuestDelete: 1 << 4,
        GuestExecute: 1 << 5,
        GuestRefer: 1 << 6,
        UserPeek: 1 << 7,
        UserRead: 1 << 8,
        UserCreate: 1 << 9,
        UserUpdate: 1 << 10,
        UserDelete: 1 << 11,
        UserExecute: 1 << 12,
        UserRefer: 1 << 13,
        GroupPeek: 1 << 14,
        GroupRead: 1 << 15,
        GroupCreate: 1 << 16,
        GroupUpdate: 1 << 17,
        GroupDelete: 1 << 18,
        GroupExecute: 1 << 19,
        GroupRefer: 1 << 20,
      },
      parsedGuestPermission: {
        canPeek: false,
        canRead: false,
        canCreate: false,
        canUpdate: false,
        canDelete: false,
        canRefer: false,
        canExecute: false,
      },
      parsedOwnerPermission: {
        canPeek: false,
        canRead: false,
        canCreate: false,
        canUpdate: false,
        canDelete: false,
        canRefer: false,
        canExecute: false,
      },
      parsedGroupPermission: {
        canPeek: false,
        canRead: false,
        canCreate: false,
        canUpdate: false,
        canDelete: false,
        canRefer: false,
        canExecute: false,
      },
      userAccounts: [],
      userGroups: [],
    }
  },
  mounted() {
    const that = this;
    this.loadData({
      tableName: "user_account",
      params: {
        page: 1,
        size: 500
      }
    }).then(function (res) {
      that.userAccounts = res.data;
    }).catch(function (err) {
      that.$q.notify({
        message: "Failed to load users list: " + JSON.stringify(err)
      })
    });

    that.loadData({
      tableName: "usergroup",
      params: {
        page: 1,
        size: 500
      }
    }).then(function (res) {
      that.userGroups = res.data;
    }).catch(function (err) {
      that.$q.notify({
        message: "Failed to load usergroups list: " + JSON.stringify(err)
      })
    });

    // that.loadDataRelations({
    //   tableName: "world",
    //   relation: "usergroup_id",
    //   reference_id: "",
    // }).then(function (res) {
    //   that.userGroups = res.data;
    // }).catch(function (err) {
    //   that.$q.notify({
    //     message: "Failed to load usergroups list: " + JSON.stringify(err)
    //   })
    // });


    this.refresh();
  },
  computed: {
    ...mapGetters(['tables']),
    ...mapState([])
  },

  watch: {
    'selectedTable': function (newTable, oldTable) {
      const that = this;
      this.refresh()
    },
    'parsedOwnerPermission': function (newPermission, currentPermission) {
      console.log("Permission changed", newPermission, currentPermission)
    }
  }
}
</script>
