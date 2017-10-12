<!-- PermissionInput.vue -->
<template>
  <table class="table">
    <tr>
      <td></td>
      <td>Peek</td>
      <td>Read</td>
      <td>Create</td>
      <td>Update</td>
      <td>Delete</td>
      <td>Execute</td>
      <td>Refer</td>
      <td>Read Strict</td>
      <td>Create Strict</td>
      <td>Update Strict</td>
      <td>Delete Strict</td>
      <td>Execute Strict</td>
      <td>Refer Strict</td>
      <td>CRUD</td>
    </tr>
    <tr>
      <td><b>Owner</b></td>
      <td><input class="checkbox" type="checkbox" v-model.sync="parsedOwnerPermission.canPeek"></td>
      <td><input type="checkbox" v-model="parsedOwnerPermission.canRead"></td>
      <td><input type="checkbox" v-model="parsedOwnerPermission.canCreate"></td>
      <td><input type="checkbox" v-model="parsedOwnerPermission.canUpdate"></td>
      <td><input type="checkbox" v-model="parsedOwnerPermission.canDelete"></td>
      <td><input type="checkbox" v-model="parsedOwnerPermission.canExecute"></td>
      <td><input type="checkbox" v-model="parsedOwnerPermission.canRefer"></td>
      <td><input type="checkbox" v-model="parsedOwnerPermission.canReadStrict"></td>
      <td><input type="checkbox" v-model="parsedOwnerPermission.canCreateStrict"></td>
      <td><input type="checkbox" v-model="parsedOwnerPermission.canUpdateStrict"></td>
      <td><input type="checkbox" v-model="parsedOwnerPermission.canDeleteStrict"></td>
      <td><input type="checkbox" v-model="parsedOwnerPermission.canExecuteStrict"></td>
      <td><input type="checkbox" v-model="parsedOwnerPermission.canReferStrict"></td>
      <td><input type="checkbox" v-model="parsedOwnerPermission.canCRUD"></td>
    </tr>
    <tr>
      <td><b>Guest</b></td>
      <td><input type="checkbox" v-model="parsedGuestPermission.canPeek"></td>
      <td><input type="checkbox" v-model="parsedGuestPermission.canRead"></td>
      <td><input type="checkbox" v-model="parsedGuestPermission.canCreate"></td>
      <td><input type="checkbox" v-model="parsedGuestPermission.canUpdate"></td>
      <td><input type="checkbox" v-model="parsedGuestPermission.canDelete"></td>
      <td><input type="checkbox" v-model="parsedGuestPermission.canExecute"></td>
      <td><input type="checkbox" v-model="parsedGuestPermission.canRefer"></td>
      <td><input type="checkbox" v-model="parsedGuestPermission.canReadStrict"></td>
      <td><input type="checkbox" v-model="parsedGuestPermission.canCreateStrict"></td>
      <td><input type="checkbox" v-model="parsedGuestPermission.canUpdateStrict"></td>
      <td><input type="checkbox" v-model="parsedGuestPermission.canDeleteStrict"></td>
      <td><input type="checkbox" v-model="parsedGuestPermission.canExecuteStrict"></td>
      <td><input type="checkbox" v-model="parsedGuestPermission.canReferStrict"></td>
      <td><input type="checkbox" v-model="parsedGuestPermission.canCRUD"></td>
    </tr>
    <tr>
      <td><b>Group</b></td>
      <td><input type="checkbox" v-model="parsedGroupPermission.canPeek"></td>
      <td><input type="checkbox" v-model="parsedGroupPermission.canRead"></td>
      <td><input type="checkbox" v-model="parsedGroupPermission.canCreate"></td>
      <td><input type="checkbox" v-model="parsedGroupPermission.canUpdate"></td>
      <td><input type="checkbox" v-model="parsedGroupPermission.canDelete"></td>
      <td><input type="checkbox" v-model="parsedGroupPermission.canExecute"></td>
      <td><input type="checkbox" v-model="parsedGroupPermission.canRefer"></td>
      <td><input type="checkbox" v-model="parsedGroupPermission.canReadStrict"></td>
      <td><input type="checkbox" v-model="parsedGroupPermission.canCreateStrict"></td>
      <td><input type="checkbox" v-model="parsedGroupPermission.canUpdateStrict"></td>
      <td><input type="checkbox" v-model="parsedGroupPermission.canDeleteStrict"></td>
      <td><input type="checkbox" v-model="parsedGroupPermission.canExecuteStrict"></td>
      <td><input type="checkbox" v-model="parsedGroupPermission.canReferStrict"></td>
      <td><input type="checkbox" v-model="parsedGroupPermission.canCRUD"></td>
    </tr>
  </table>
</template>

<script>
  import {abstractField} from "vue-form-generator";
  import {DatePicker} from "element-ui";


  export default {
    components: {DatePicker},
    mixins: [abstractField],
    data: function () {
      return {
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
      console.log("perission value", this.value)
      var permissionValue = this.value;
      this.guestValue = permissionValue % 1000;
      permissionValue = parseInt(permissionValue / 1000);
      this.groupValue = permissionValue % 1000;
      permissionValue = parseInt(permissionValue / 1000);
      this.ownerValue = permissionValue % 1000;
      permissionValue = parseInt(permissionValue / 1000);
      console.log("Owner, group, guest", this.ownerValue, this.groupValue, this.guestValue);
      this.parsedGuestPermission = this.parsePermission(this.guestValue);
      this.parsedOwnerPermission = this.parsePermission(this.ownerValue);
      this.parsedGroupPermission = this.parsePermission(this.groupValue);
    },
    methods: {
      updatePermissionValue() {
        console.log("make permission value");
        var ownerPermission = this.makePermission(this.parsedOwnerPermission);
        var guestPermission = this.makePermission(this.parsedGuestPermission);
        var groupPermission = this.makePermission(this.parsedGroupPermission);
        console.log("owner permission", ownerPermission);
        console.log("guest permission", guestPermission);
        console.log("group permission", groupPermission);

        this.value = (ownerPermission * 1000*1000) + (groupPermission * 1000) + (guestPermission)
        console.log("updated permission value to ", this.value);
      },
      makePermission(permissionObject) {
        console.log("make permission from", permissionObject);

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
        console.log("parse value to permission struct", val)
        var res = {
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
