<!-- PermissionInput.vue -->
<template>
  <div>
    <el-row>
      <el-tabs v-model="activeTabName">
        <el-tab-pane label="User" name="user">
          <div>

            <el-row>

              <el-col :span="8">
                <el-checkbox v-model.sync="parsedOwnerPermission.canPeek">Peek</el-checkbox>
              </el-col>

              <el-col :span="8">
                <el-checkbox v-model="parsedOwnerPermission.canCRUD">CRUD</el-checkbox>
              </el-col>

            </el-row>


            <el-row>

              <el-col :span="8">
                <el-checkbox v-model="parsedOwnerPermission.canRead">Read</el-checkbox>
              </el-col>

              <el-col :span="8">
                <el-checkbox v-model="parsedOwnerPermission.canCreate">Create</el-checkbox>
              </el-col>

              <el-col :span="8">
                <el-checkbox v-model="parsedOwnerPermission.canUpdate">Update</el-checkbox>
              </el-col>


              <el-col :span="8">
                <el-checkbox v-model="parsedOwnerPermission.canDelete">Delete</el-checkbox>
              </el-col>


              <el-col :span="8">
                <el-checkbox v-model="parsedOwnerPermission.canExecute">Execute</el-checkbox>
              </el-col>


              <el-col :span="8">
                <el-checkbox v-model="parsedOwnerPermission.canRefer">Refer</el-checkbox>
              </el-col>

            </el-row>


          </div>
        </el-tab-pane>
        <el-tab-pane label="Group" name="group">
          <div>

            <el-row>

              <el-col :span="8">
                <el-checkbox v-model.sync="parsedGroupPermission.canPeek">Peek</el-checkbox>
              </el-col>

              <el-col :span="8">
                <el-checkbox v-model="parsedGroupPermission.canCRUD">CRUD</el-checkbox>
              </el-col>

            </el-row>


            <el-row>

              <el-col :span="8">
                <el-checkbox v-model="parsedGroupPermission.canRead">Read</el-checkbox>
              </el-col>


              <el-col :span="8">
                <el-checkbox v-model="parsedGroupPermission.canCreate">Create</el-checkbox>
              </el-col>

              <el-col :span="8">
                <el-checkbox v-model="parsedGroupPermission.canUpdate">Update</el-checkbox>
              </el-col>


              <el-col :span="8">
                <el-checkbox v-model="parsedGroupPermission.canDelete">Delete</el-checkbox>
              </el-col>


              <el-col :span="8">
                <el-checkbox v-model="parsedGroupPermission.canExecute">Execute</el-checkbox>
              </el-col>


              <el-col :span="8">
                <el-checkbox v-model="parsedGroupPermission.canRefer">Refer</el-checkbox>
              </el-col>

            </el-row>


          </div>
        </el-tab-pane>
        <el-tab-pane label="Guest" name="guest">
          <div class>
            <el-row>

              <el-col :span="8">
                <el-checkbox v-model.sync="parsedGuestPermission.canPeek">Peek</el-checkbox>
              </el-col>

              <el-col :span="8">
                <el-checkbox v-model="parsedGuestPermission.canCRUD">CRUD</el-checkbox>
              </el-col>

            </el-row>


            <el-row>

              <el-col :span="8">
                <el-checkbox v-model="parsedGuestPermission.canRead">Read</el-checkbox>
              </el-col>


              <el-col :span="8">
                <el-checkbox v-model="parsedGuestPermission.canCreate">Create</el-checkbox>
              </el-col>

              <el-col :span="8">
                <el-checkbox v-model="parsedGuestPermission.canUpdate">Update</el-checkbox>
              </el-col>


              <el-col :span="8">
                <el-checkbox v-model="parsedGuestPermission.canDelete">Delete</el-checkbox>
              </el-col>


              <el-col :span="8">
                <el-checkbox v-model="parsedGuestPermission.canExecute">Execute</el-checkbox>
              </el-col>


              <el-col :span="8">
                <el-checkbox v-model="parsedGuestPermission.canRefer">Refer</el-checkbox>
              </el-col>

            </el-row>


          </div>
        </el-tab-pane>
      </el-tabs>
    </el-row>
    <el-row>
      <el-button @click="clearAll">Clear all</el-button>
      <el-button @click="enableAll">Enable all</el-button>
      <el-button @click="toggleSelectionAll">Toggle</el-button>
    </el-row>
  </div>

</template>

<script>
  import {abstractField} from "vue-form-generator";
  import {DatePicker} from "element-ui";

  export default {
    components: {DatePicker},
    mixins: [abstractField],
    data: function () {
      return {
        activeTabName: 'user',
        editorOptions: {},
        guestValue: {},
        ownerValue: {},
        groupValue: {},
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
          canCRUD: false,
          canExecute: false,
        },
        parsedOwnerPermission: {
          canPeek: false,
          canRead: false,
          canCreate: false,
          canUpdate: false,
          canDelete: false,
          canRefer: false,
          canCRUD: false,
          canExecute: false,
        },
        parsedGroupPermission: {
          canPeek: false,
          canRead: false,
          canCreate: false,
          canUpdate: false,
          canDelete: false,
          canRefer: false,
          canCRUD: false,
          canExecute: false,
        },
      }
    },
    mounted() {
      var that = this;
      setTimeout(function () {
        console.log("permission value", that.value)
        var permissionValue = that.value;
        that.guestValue = {
          canPeek: (permissionValue & that.permissionStructure.GuestPeek) == that.permissionStructure.GuestPeek,
          canRead: (permissionValue & that.permissionStructure.GuestRead) == that.permissionStructure.GuestRead,
          canCreate: (permissionValue & that.permissionStructure.GuestCreate) == that.permissionStructure.GuestCreate,
          canUpdate: (permissionValue & that.permissionStructure.GuestUpdate) == that.permissionStructure.GuestUpdate,
          canDelete: (permissionValue & that.permissionStructure.GuestDelete) == that.permissionStructure.GuestDelete,
          canRefer: (permissionValue & that.permissionStructure.GuestRefer) == that.permissionStructure.GuestRefer,
          canCRUD: (permissionValue & that.permissionStructure.GuestPeek) == that.permissionStructure.GuestPeek,
          canExecute: (permissionValue & that.permissionStructure.GuestExecute) == that.permissionStructure.GuestExecute,
        };
        that.guestValue.canCRUD = that.guestValue.canRead & that.guestValue.canCreate & that.guestValue.canUpdate & that.guestValue.canDelete;

        that.userValue = {
          canPeek: (permissionValue & that.permissionStructure.UserPeek) == that.permissionStructure.UserPeek,
          canRead: (permissionValue & that.permissionStructure.UserRead) == that.permissionStructure.UserRead,
          canCreate: (permissionValue & that.permissionStructure.UserCreate) == that.permissionStructure.UserCreate,
          canUpdate: (permissionValue & that.permissionStructure.UserUpdate) == that.permissionStructure.UserUpdate,
          canDelete: (permissionValue & that.permissionStructure.UserDelete) == that.permissionStructure.UserDelete,
          canRefer: (permissionValue & that.permissionStructure.UserRefer) == that.permissionStructure.UserRefer,
          canCRUD: (permissionValue & that.permissionStructure.UserPeek) == that.permissionStructure.UserPeek,
          canExecute: (permissionValue & that.permissionStructure.UserExecute) == that.permissionStructure.UserExecute,
        };
        that.userValue.canCRUD = that.userValue.canRead & that.userValue.canCreate & that.userValue.canUpdate & that.userValue.canDelete;


        that.groupValue = {
          canPeek: (permissionValue & that.permissionStructure.GroupPeek) == that.permissionStructure.GroupPeek,
          canRead: (permissionValue & that.permissionStructure.GroupRead) == that.permissionStructure.GroupRead,
          canCreate: (permissionValue & that.permissionStructure.GroupCreate) == that.permissionStructure.GroupCreate,
          canUpdate: (permissionValue & that.permissionStructure.GroupUpdate) == that.permissionStructure.GroupUpdate,
          canDelete: (permissionValue & that.permissionStructure.GroupDelete) == that.permissionStructure.GroupDelete,
          canRefer: (permissionValue & that.permissionStructure.GroupRefer) == that.permissionStructure.GroupRefer,
          canCRUD: (permissionValue & that.permissionStructure.GroupPeek) == that.permissionStructure.GroupPeek,
          canExecute: (permissionValue & that.permissionStructure.GroupExecute) == that.permissionStructure.GroupExecute,
        };
        that.groupValue.canCRUD = that.groupValue.canRead & that.groupValue.canCreate & that.groupValue.canUpdate & that.groupValue.canDelete;

        that.parsedGuestPermission = that.guestValue;
        that.parsedOwnerPermission = that.userValue;
        that.parsedGroupPermission = that.groupValue;
      }, 200);

    },
    methods: {
      setValue(obj, newValue) {
        var keys = Object.keys(obj);
        for (var i = 0; i < keys.length; i++) {
          if (newValue === undefined) {
            obj[keys[i]] = !obj[keys[i]]
          } else {
            obj[keys[i]] = newValue
          }
        }
      },
      clearAll() {
        switch (this.activeTabName) {
          case "user":
            this.setValue(this.parsedOwnerPermission, false);
            break;
          case "group":
            this.setValue(this.parsedGroupPermission, false);
            break;
          case "guest":
            this.setValue(this.parsedGuestPermission, false);
            break;
        }
      },
      enableAll() {
        switch (this.activeTabName) {
          case "user":
            this.setValue(this.parsedOwnerPermission, true);
            break;
          case "group":
            this.setValue(this.parsedGroupPermission, true);
            break;
          case "guest":
            this.setValue(this.parsedGuestPermission, true);
            break;
        }
      },
      toggleSelectionAll() {
        switch (this.activeTabName) {
          case "user":
            this.setValue(this.parsedOwnerPermission);
            break;
          case "group":
            this.setValue(this.parsedGroupPermission);
            break;
          case "guest":
            this.setValue(this.parsedGuestPermission);
            break;
        }

      },
      updatePermissionValue() {
        var ownerPermission = this.makePermission(this.parsedOwnerPermission, "User");
        var guestPermission = this.makePermission(this.parsedGuestPermission, "Guest");
        var groupPermission = this.makePermission(this.parsedGroupPermission, "Group");
        console.log("owner permission", ownerPermission);
        console.log("guest permission", guestPermission);
        console.log("group permission", groupPermission);

        this.value = (ownerPermission  | groupPermission | guestPermission)
        console.log("updated permission value to ", this.value);
      },
      makePermission(permissionObject, userType) {
        var value = 0;
        var perms = Object.keys(this.permissionStructure);
        for (var i = 0; i < perms.length; i++) {
          let permissionName = perms[i];

          if (!permissionName.startsWith(userType)) {
            continue
          }
          var permission = this.permissionStructure[permissionName];
          if (permissionObject["can" + permissionName.substring(userType.length)]) {
            value = value | permission;
          }
        }
        return value
      },
    },
    watch: {
      'parsedGuestPermission': {
        handler: function (newValue) {
          console.log("guest value updated", this.parsedGuestPermission);
          this.updatePermissionValue()
        },
        deep: true
      },
      'parsedOwnerPermission': {
        handler: function (newValue) {
          console.log("owner value updated", this.parsedGuestPermission)
          this.updatePermissionValue()
        },
        deep: true
      },
      'parsedGroupPermission': {
        handler: function (newValue) {
          console.log("group value updated", this.parsedGuestPermission)
          this.updatePermissionValue()
        },
        deep: true
      }
    }
  };
</script>
