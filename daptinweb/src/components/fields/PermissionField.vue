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
            <el-row>


              <el-col :span="8">
                <el-checkbox v-model="parsedOwnerPermission.canReadStrict">Read Strict</el-checkbox>
              </el-col>

              <el-col :span="8">
                <el-checkbox v-model="parsedOwnerPermission.canCreateStrict">Create Strict</el-checkbox>
              </el-col>


              <el-col :span="8">
                <el-checkbox v-model="parsedOwnerPermission.canUpdateStrict">Update Strict</el-checkbox>
              </el-col>


              <el-col :span="8">
                <el-checkbox v-model="parsedOwnerPermission.canDeleteStrict">Delete Strict</el-checkbox>
              </el-col>


              <el-col :span="8">
                <el-checkbox v-model="parsedOwnerPermission.canExecuteStrict">Execute Strict</el-checkbox>
              </el-col>


              <el-col :span="8">
                <el-checkbox v-model="parsedOwnerPermission.canReferStrict">Refer Strict</el-checkbox>
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
            <el-row>


              <el-col :span="8">
                <el-checkbox v-model="parsedGroupPermission.canReadStrict">Read Strict</el-checkbox>
              </el-col>

              <el-col :span="8">
                <el-checkbox v-model="parsedGroupPermission.canCreateStrict">Create Strict</el-checkbox>
              </el-col>


              <el-col :span="8">
                <el-checkbox v-model="parsedGroupPermission.canUpdateStrict">Update Strict</el-checkbox>
              </el-col>


              <el-col :span="8">
                <el-checkbox v-model="parsedGroupPermission.canDeleteStrict">Delete Strict</el-checkbox>
              </el-col>


              <el-col :span="8">
                <el-checkbox v-model="parsedGroupPermission.canExecuteStrict">Execute Strict</el-checkbox>
              </el-col>


              <el-col :span="8">
                <el-checkbox v-model="parsedGroupPermission.canReferStrict">Refer Strict</el-checkbox>
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
            <el-row>


              <el-col :span="8">
                <el-checkbox v-model="parsedGuestPermission.canReadStrict">Read Strict</el-checkbox>
              </el-col>

              <el-col :span="8">
                <el-checkbox v-model="parsedGuestPermission.canCreateStrict">Create Strict</el-checkbox>
              </el-col>


              <el-col :span="8">
                <el-checkbox v-model="parsedGuestPermission.canUpdateStrict">Update Strict</el-checkbox>
              </el-col>


              <el-col :span="8">
                <el-checkbox v-model="parsedGuestPermission.canDeleteStrict">Delete Strict</el-checkbox>
              </el-col>


              <el-col :span="8">
                <el-checkbox v-model="parsedGuestPermission.canExecuteStrict">Execute Strict</el-checkbox>
              </el-col>


              <el-col :span="8">
                <el-checkbox v-model="parsedGuestPermission.canReferStrict">Refer Strict</el-checkbox>
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
          Peek: 1 << 0,
          ReadStrict: 1 << 1,
          CreateStrict: 1 << 2,
          UpdateStrict: 1 << 3,
          DeleteStrict: 1 << 4,
          ExecuteStrict: 1 << 5,
          ReferStrict: 1 << 6,
          Read: 1 << 1 | 1 << 0,
          Refer: 1 << 6 | 1 << 1 | 1 << 0,
          Create: 1 << 2 | 1 << 1 | 1 << 0, // Create strict, read, peek
          Update: 1 << 3 | 1 << 1 | 1 << 0, // Update strict, read, peek
          Delete: 1 << 4 | 1 << 1 | 1 << 0, // Delete strict, read, peek
          Execute: 1 << 5 | 1 << 0,
          CRUD: 1 << 0 | 1 << 1 | 1 << 2 | 1 << 3 | 1 << 4 | 1 << 6,
        },
        parsedGuestPermission: {
          canPeek: false,
          canRead: false,
          canCreate: false,
          canUpdate: false,
          canDelete: false,
          canRefer: false,
          canReadStrict: false,
          canCreateStrict: false,
          canUpdateStrict: false,
          canDeleteStrict: false,
          canReferStrict: false,
          canCRUD: false,
          canExecute: false,
          canExecuteStrict: false,
        },
        parsedOwnerPermission: {
          canPeek: false,
          canRead: false,
          canCreate: false,
          canUpdate: false,
          canDelete: false,
          canRefer: false,
          canReadStrict: false,
          canCreateStrict: false,
          canUpdateStrict: false,
          canDeleteStrict: false,
          canReferStrict: false,
          canCRUD: false,
          canExecute: false,
          canExecuteStrict: false,
        },
        parsedGroupPermission: {
          canPeek: false,
          canRead: false,
          canCreate: false,
          canUpdate: false,
          canDelete: false,
          canRefer: false,
          canReadStrict: false,
          canCreateStrict: false,
          canUpdateStrict: false,
          canDeleteStrict: false,
          canReferStrict: false,
          canCRUD: false,
          canExecute: false,
          canExecuteStrict: false,
        },
      }
    },
    mounted() {
      var that = this;
      setTimeout(function(){
        console.log("permission value", that.value)
        var permissionValue = that.value;
        that.guestValue = permissionValue % 1000;
        permissionValue = parseInt(permissionValue / 1000);
        that.groupValue = permissionValue % 1000;
        permissionValue = parseInt(permissionValue / 1000);
        that.ownerValue = permissionValue % 1000;
        permissionValue = parseInt(permissionValue / 1000);
        console.log("Owner, group, guest", that.ownerValue, that.groupValue, that.guestValue);
        that.parsedGuestPermission = that.parsePermission(that.guestValue);
        that.parsedOwnerPermission = that.parsePermission(that.ownerValue);
        that.parsedGroupPermission = that.parsePermission(that.groupValue);
      }, 200);

    },
    methods: {
      setValue(obj, newValue) {
        var keys = Object.keys(obj);
        for(var i=0; i<keys.length;i++) {
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
        };
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
        };
      },
      updatePermissionValue() {
//        console.log("make permission value");
        var ownerPermission = this.makePermission(this.parsedOwnerPermission);
        var guestPermission = this.makePermission(this.parsedGuestPermission);
        var groupPermission = this.makePermission(this.parsedGroupPermission);
        console.log("owner permission", ownerPermission);
        console.log("guest permission", guestPermission);
        console.log("group permission", groupPermission);

        this.value = (ownerPermission * 1000 * 1000) + (groupPermission * 1000) + (guestPermission)
        console.log("updated permission value to ", this.value);
      },
      makePermission(permissionObject) {
//        console.log("make permission from", permissionObject);

        var value = 0;
        var perms = Object.keys(this.permissionStructure);
        for (var i = 0; i < perms.length; i++) {
          let permissionName = perms[i];
          var permission = this.permissionStructure[permissionName];
//          console.log("Check for ", permissionName, permission)

          if (permissionObject["can" + permissionName]) {
            value = value | permission;
          }

        }

        return value
      },
      parsePermission(val) {
        const res = {
          canPeek: (val & this.permissionStructure.Peek ) == this.permissionStructure.Peek,
          canRead: (val & this.permissionStructure.Read ) == this.permissionStructure.Read,
          canCreate: (val & this.permissionStructure.Create ) == this.permissionStructure.Create,
          canUpdate: (val & this.permissionStructure.Update ) == this.permissionStructure.Update,
          canDelete: (val & this.permissionStructure.Delete ) == this.permissionStructure.Delete,
          canRefer: (val & this.permissionStructure.Refer ) == this.permissionStructure.Refer,
          canReadStrict: (val & this.permissionStructure.ReadStrict ) == this.permissionStructure.ReadStrict,
          canCreateStrict: (val & this.permissionStructure.CreateStrict ) == this.permissionStructure.CreateStrict,
          canUpdateStrict: (val & this.permissionStructure.UpdateStrict ) == this.permissionStructure.UpdateStrict,
          canDeleteStrict: (val & this.permissionStructure.DeleteStrict ) == this.permissionStructure.DeleteStrict,
          canReferStrict: (val & this.permissionStructure.ReferStrict ) == this.permissionStructure.ReferStrict,
          canCRUD: (val & this.permissionStructure.CRUD ) == this.permissionStructure.CRUD,
          canExecute: (val & this.permissionStructure.Execute ) == this.permissionStructure.Execute,
          canExecuteStrict: (val & this.permissionStructure.ExecuteStrict ) == this.permissionStructure.ExecuteStrict,
        }
        console.log("parsed permission", res)
        return res;
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
