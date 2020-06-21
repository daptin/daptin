<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <div class="q-pa-md q-gutter-sm">
    <div>
      <q-breadcrumbs class="text-orange" active-color="secondary">
        <template v-slot:separator>
          <q-icon
            size="1.2em"
            name="arrow_forward"
            color="primary"
          />
        </template>

        <q-breadcrumbs-el label="Database" icon="fas fa-database"/>
        <q-breadcrumbs-el label="Permissions" icon="fas fa-table"/>
      </q-breadcrumbs>
    </div>

    <div class="row">
      <div class="col-md-12">
        <span class="text-h4">Permissions</span>
      </div>
      <div class="col-md-2">
        <q-select option-value="table_name"
                  option-label="table_name"
                  v-model="selectedTable" :options="tables" label="Table"/>
      </div>
    </div>

    <div class="row" v-if="selectedTable">
      <div class="col-6 q-pa-md items-start q-gutter-md">
        <q-card>
          <q-card-section>
            <div class="text-h6">Table owner</div>
          </q-card-section>

          <q-card-section class="q-pt-none">
            {{selectedTable.user_account_id ? selectedTable.user_account_id : 'n/a'}}
          </q-card-section>


          <q-card-actions>
            <q-btn flat>Remove owner</q-btn>
            <q-btn flat>Change owner</q-btn>
          </q-card-actions>
        </q-card>

        <q-card>
          <q-card-section>
            <div class="text-h6">Table groups</div>
          </q-card-section>
          <q-card-section class="q-pt-none">
            <ul>
              <li v-for="group in tableGroups">{{group.name}}</li>
            </ul>
          </q-card-section>
          <q-card-actions>
            <q-btn label="Add group" @click="groupChangeForTableGroups()"></q-btn>
          </q-card-actions>
        </q-card>

        <q-card>
          <q-card-section>
            <div class="text-h6">New row default groups</div>
          </q-card-section>
          <q-card-section class="q-pt-none">
            <ul>
              <li v-for="group in tableSchema.DefaultGroups">{{group}}</li>
            </ul>
          </q-card-section>
          <q-card-actions>
            <q-btn label="Add group" @click="groupChangeForNewRowGroups()"></q-btn>
          </q-card-actions>
        </q-card>

      </div>
      <div class="col-6 q-pa-md items-start q-gutter-md">
        <q-card>
          <q-card-section>
            <q-tabs
              v-model="selectedTab"
              dense
              class="text-grey"
              active-color="primary"
              indicator-color="primary"
              align="justify"
              narrow-indicator  >
              <q-tab name="tablePermissions" label="Table Permissions"/>
              <q-tab name="rowPermissions" label="New Row Permissions"/>
              <q-tab name="groups" label="Groups"/>
            </q-tabs>
          </q-card-section>
          <q-card-section>
            <q-tab-panels v-model="selectedTab">
              <q-tab-panel name="tablePermissions">
                <span class="text-h5">Table permissions</span>
                <div class="q-pa-md">
                  <span class="text-h6">Guest</span>
                  <div class="q-gutter-sm">

                    <q-checkbox v-model="parsedGuestPermission.canPeek" label="Peek"/>
                    <q-checkbox v-model="parsedGuestPermission.canCreate" label="Create"/>
                    <q-checkbox v-model="parsedGuestPermission.canRead" label="Read"/>
                    <q-checkbox v-model="parsedGuestPermission.canUpdate" label="Update"/>
                    <q-checkbox v-model="parsedGuestPermission.canDelete" label="Delete"/>
                    <q-checkbox v-model="parsedGuestPermission.canRefer" label="Refer"/>
                    <q-checkbox v-model="parsedGuestPermission.canExecute" label="Execute"/>

                  </div>
                </div>

                <div class="col-12 q-pa-md">
                  <span class="text-h5">Owner</span>
                  <div class="q-gutter-sm">

                    <q-checkbox v-model="parsedOwnerPermission.canPeek" label="Peek"/>
                    <q-checkbox v-model="parsedOwnerPermission.canCreate" label="Create"/>
                    <q-checkbox v-model="parsedOwnerPermission.canRead" label="Read"/>
                    <q-checkbox v-model="parsedOwnerPermission.canUpdate" label="Update"/>
                    <q-checkbox v-model="parsedOwnerPermission.canDelete" label="Delete"/>
                    <q-checkbox v-model="parsedOwnerPermission.canRefer" label="Refer"/>
                    <q-checkbox v-model="parsedOwnerPermission.canExecute" label="Execute"/>

                  </div>
                </div>

              </q-tab-panel>

              <q-tab-panel name="rowPermissions">
            <span class="text-h5">
              Default row permissions
            </span>
                <div class="col-12 q-pa-md">
                  <span class="text-h6">Guest</span>
                  <div class="q-gutter-sm">

                    <q-checkbox v-model="parsedGuestPermission.canPeek" label="Peek"/>
                    <q-checkbox v-model="parsedGuestPermission.canCreate" label="Create"/>
                    <q-checkbox v-model="parsedGuestPermission.canRead" label="Read"/>
                    <q-checkbox v-model="parsedGuestPermission.canUpdate" label="Update"/>
                    <q-checkbox v-model="parsedGuestPermission.canDelete" label="Delete"/>
                    <q-checkbox v-model="parsedGuestPermission.canRefer" label="Refer"/>
                    <q-checkbox v-model="parsedGuestPermission.canExecute" label="Execute"/>

                  </div>
                </div>

                <div class="col-12 q-pa-md">
                  <span class="text-h5">Owner</span>
                  <div class="q-gutter-sm">

                    <q-checkbox v-model="parsedOwnerPermission.canPeek" label="Peek"/>
                    <q-checkbox v-model="parsedOwnerPermission.canCreate" label="Create"/>
                    <q-checkbox v-model="parsedOwnerPermission.canRead" label="Read"/>
                    <q-checkbox v-model="parsedOwnerPermission.canUpdate" label="Update"/>
                    <q-checkbox v-model="parsedOwnerPermission.canDelete" label="Delete"/>
                    <q-checkbox v-model="parsedOwnerPermission.canRefer" label="Refer"/>
                    <q-checkbox v-model="parsedOwnerPermission.canExecute" label="Execute"/>

                  </div>
                </div>

              </q-tab-panel>
              <q-tab-panel name="groups">
                <span class="text-h5">Group Permissions</span>
                <div class="col-12 q-pa-md">
                  <div class="q-gutter-sm">
                    <q-checkbox v-model="parsedGroupPermission.canPeek" label="Peek"/>
                    <q-checkbox v-model="parsedGroupPermission.canCreate" label="Create"/>
                    <q-checkbox v-model="parsedGroupPermission.canRead" label="Read"/>
                    <q-checkbox v-model="parsedGroupPermission.canUpdate" label="Update"/>
                    <q-checkbox v-model="parsedGroupPermission.canDelete" label="Delete"/>
                    <q-checkbox v-model="parsedGroupPermission.canRefer" label="Refer"/>
                    <q-checkbox v-model="parsedGroupPermission.canExecute" label="Execute"/>
                  </div>
                </div>
              </q-tab-panel>
            </q-tab-panels>
          </q-card-section>
        </q-card>


      </div>
    </div>

    <q-dialog v-model="addToGroup">
      <q-card>
        <q-card-section>
          <div class="text-h6">Add table to new group</div>

        </q-card-section>
        <q-card-section>
          <q-tabs
            v-model="addToGroupSwitch"
            dense
            class="text-grey"
            active-color="primary"
            indicator-color="primary"
            align="justify"
            narrow-indicator>
            <q-tab name="addExisting" label="Add to existing group"/>
            <q-tab name="addNewGroup" label="Create new group"/>
          </q-tabs>
        </q-card-section>

        <q-card-section class="q-pt-none">
          <q-tab-panels v-model="addToGroupSwitch" class="">
            <q-tab-panel name="addExisting">
              <q-select flat :options="userGroups" option-label="name" option-value="reference_id"
                        v-model="addToGroupId"></q-select>
            </q-tab-panel>
            <q-tab-panel name="addNewGroup">
              <q-input label="New group name" v-model="newGroupName"></q-input>
            </q-tab-panel>
          </q-tab-panels>
        </q-card-section>

        <q-card-actions align="right">
          <q-btn flat label="Cancel" color="warning" v-close-popup/>
          <q-btn flat label="Add" color="primary" v-close-popup/>
        </q-card-actions>
      </q-card>
    </q-dialog>


  </div>
</template>

<script>
  import {mapActions, mapGetters, mapState} from 'vuex';

  export default {
    name: 'TablePage',
    methods: {
      groupChangeForTableGroups() {
        this.groupChangeFor = 'tableGroups';
        this.addToGroup = true
      },
      groupChangeForNewRowGroups() {
        this.groupChangeFor = 'newRowGroups';
        this.addToGroup = true
      },
      ...mapActions(['loadData', 'loadDataRelations'])
    },
    data() {
      return {
        text: '',
        newGroupName: '',
        groupChangeFor: null,
        addToGroupId: null,
        tableSchema: {},
        addToGroup: false,
        addToGroupSwitch: 'addExisting',
        tableGroups: [],
        selectedTab: 'tablePermissions',
        selectedTable: null,
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
        tableGroups: [],
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
    },
    computed: {
      ...mapGetters(['tables']),
      ...mapState([])
    },

    watch: {
      'selectedTable': function (newTable, oldTable) {
        const that = this;
        console.log("Selection changed", newTable, oldTable);
        that.tableSchema = JSON.parse(newTable.world_schema_json);
        console.log("Table schema json", that.tableSchema)

        var permissionValue = newTable.permission;
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

        that.loadDataRelations({
          tableName: 'world',
          relation: 'usergroup_id',
          reference_id: newTable.reference_id,
        }).then(function (res) {
          console.log("Loaded groups of table", newTable.table_name, res);
          that.tableGroups = res.data;
        }).catch(function (err) {
          that.$q.notify({
            message: "Failed to load usergroups: " + JSON.stringify(err)
          })
        })
      }
    }
  }
</script>
