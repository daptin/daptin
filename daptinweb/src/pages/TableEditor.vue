<template>
  <q-form class="q-gutter-md">
    <div style="padding-bottom: 10px" class="row">
      <div class="col-md-6">
        <span class="text-h4">{{!isEdit ? 'Create table' : 'Edit table'}}</span>
      </div>
    </div>
    <div class="row">

      <div class="col-4">

        <q-input
          filled
          v-model="localTable.TableName"
          size="sm"
          placeholder="Table name"
          :readonly="isEdit"
          lazy-rules
          :rules="[ val => val && val.length > 0 || 'Please type something']"></q-input>
      </div>
      <div class="col-12">
        <hr/>
      </div>
    </div>
    <div class="row">

      <div class="col-12">
        <span class="text-h6">Columns</span>
        <small> ({{ (table.TableName ? (localTable.ColumnModel.length - StandardColumns.length) :
          (Object.keys(localTable.ColumnModel).length)) + ' plus '
          + StandardColumns.length + ' base columns'}})
        </small>

        <div class="row"
             v-for="column in localTable.ColumnModel
             .filter(e => e.ColumnName && StandardColumns.indexOf(e.ColumnName) === -1 && !e.IsForeignKey)">

          <div class="col-3" style="padding: 5px">
            <q-input placeholder="column Name" readonly v-model="column.ColumnName"></q-input>
          </div>

          <div class="col-2" style="padding: 5px">
            <q-select placeholder="column type" readonly v-model="column.ColumnType"
                      :options="ColumnTypes.map(e => e.columnType + ' - ' + e.dataType)"
                      label="Column Type"></q-select>
          </div>


          <div class="col-3" style="padding: 5px">
            <q-input placeholder="default value (string values inside single quote)"
                     v-model="column.DefaultValue"></q-input>
          </div>


          <div class="col-4" style="padding: 5px">
            <q-checkbox :disable="column.IsNullable" size="xs" v-model="column.IsNullable" label="Nullable"></q-checkbox>
            <q-checkbox :disable="column.IsUnique" size="xs" v-model="column.IsUnique" label="Unique"></q-checkbox>
            <q-checkbox :disable="column.IsIndexed" size="xs" v-model="column.IsIndexed" label="Indexed"></q-checkbox>
            <q-btn @click="$emit('deleteColumn', column)" icon="fas fa-trash" flat size="sm"></q-btn>
          </div>


        </div>

        <div class="row">

          <div class="col-3" style="padding: 5px">
            <q-input @blur="columnNameUpdated()" placeholder="column name" v-model="newColumn.ColumnName"></q-input>
          </div>

          <div class="col-2" style="padding: 5px">
            <q-select @input="columnNameUpdated()" placeholder="column Type" v-model="newColumn.ColumnType"
                      :options="ColumnTypes.map(e => e.columnType + ' - ' + e.dataType)"
                      label="column type"></q-select>
          </div>

          <div class="col-3" style="padding: 5px">
            <q-input placeholder="default value (string values inside single quote)"
                     v-model="newColumn.DefaultValue"></q-input>
          </div>

          <div class="col-4" style="padding: 5px">
            <q-checkbox size="xs" v-model="newColumn.IsNullable" label="Nullable"></q-checkbox>
            <q-checkbox size="xs" v-model="newColumn.IsUnique" label="Unique"></q-checkbox>
            <q-checkbox size="xs" v-model="newColumn.IsIndexed" label="Indexed"></q-checkbox>
          </div>

        </div>
      </div>
      <div class="col-12">
        <hr/>
      </div>
    </div>


    <div class="row">

      <div class="col-12">
        <span class="text-h6">Relations {{isEdit}}</span>
        <small>({{(isEdit ? localTable.Relations.length - StandardRelations.length:localTable.Relations.length)}} + 2
          default)
        </small>

        <div class="row" v-for="relation in localTable.Relations || []"
             v-if="StandardRelations.indexOf(relation.SubjectName) == -1 && StandardRelations.indexOf(relation.ObjectName) == -1">


          <div class="col-2" style="padding: 5px">
            <q-input v-model="relation.SubjectName"></q-input>
          </div>

          <div class="col-2" style="padding: 5px">
            <q-select @input="checkRelation(relation, 'subject')" v-model="relation.Subject"
                      :options="tables.map(e => e.table_name)"></q-select>
          </div>


          <div class="col-2" style="padding: 5px">
            <q-select v-model="relation.Relation"
                      :options="RelationTypes"></q-select>
          </div>

          <div class="col-2" style="padding: 5px">
            <q-select @input="checkRelation(relation, 'object')" v-model="relation.Object"
                      :options="tables.map(e => e.table_name)"></q-select>
          </div>

          <div class="col-2" style="padding: 5px">
            <q-input v-model="relation.ObjectName"></q-input>
          </div>


          <div class="col-2" style="padding: 5px">
            <q-btn @click="$emit('deleteRelation', relation)" icon="fas fa-times" flat size="sm"></q-btn>
          </div>


        </div>

        <div class="row">


          <div class="col-2" style="padding: 5px">
            <q-input placeholder="optional subject name" v-model="newRelation.SubjectName"></q-input>
          </div>

          <div class="col-2" style="padding: 5px">
            <q-select placeholder="column Name" @input="updatedRelation('subject')" v-model="newRelation.Subject"
                      :options="tables.map(e => e.table_name)"></q-select>
          </div>


          <div class="col-2" style="padding: 5px">
            <q-select hint="relation type" @input="updatedRelation('relation')" placeholder="relation type"
                      v-model="newRelation.Relation" :options="RelationTypes"></q-select>
          </div>

          <div class="col-2" style="padding: 5px">
            <q-select hint="related object" @input="updatedRelation('object')" placeholder="related object"
                      v-model="newRelation.Object"
                      :options="tables.map(e => e.table_name)"></q-select>
          </div>


          <div class="col-2" style="padding: 5px">
            <q-input placeholder="optional object name" v-model="newRelation.ObjectName"></q-input>
          </div>


        </div>
      </div>
      <div class="col-12">
        <hr/>
      </div>
    </div>


    <div class="row">
      <div class="col-md-4">
        <div>
          <q-btn @click="$emit('save', localTable)" label="Create" type="submit" color="primary"/>
        </div>

      </div>

    </div>


  </q-form>
</template>

<script>
  import {mapActions, mapGetters, mapState} from 'vuex';

  export default {
    props: {
      table: Object
    },
    mounted() {
      console.log("Mounted table editor ", this.table);
      const that = this;
      if (this.table.ColumnModel) {
        this.table.Relations = [];
        this.localTable = {
          TableName: this.$route.params.tableName,
          ColumnModel: [],
          Relations: [],
        };
        this.localTable.ColumnModel = Object.keys(this.table.ColumnModel).map(function (colName) {
          return that.table.ColumnModel[colName]
        }).filter(e => !e.jsonApi && e.ColumnName !== "__type" && that.StandardColumns.indexOf(e.ColumnName) === -1);

        this.localTable.Relations = Object.keys(this.table.ColumnModel).filter(e => this.table.ColumnModel[e].jsonApi).map(function (colName) {
          console.log("Relation ", colName)
          const col = that.table.ColumnModel[colName];
          let rel = "has_one";
          switch (col.jsonApi) {
            case "hasOne":
              rel = "has_one";
              break;
            case "belongsTo":
              rel = "belongs_to";
              break;
            case "hasMany":
              rel = "has_many";
              break;
          }
          return {
            Subject: that.tableName,
            Relation: rel,
            Object: col.type
          }
        });
        this.isEdit = true;
      }
      this.newRelation.Subject = this.table.TableName;
    },
    methods: {
      createTable() {
        if (!this.localTable.TableName) {
          this.$q.notify("Table name cannot be empty");

        } else if (this.tables.filter(e => e.table_name === this.localTable.TableName).length > 0) {
          this.$q.notify("Table name already used");

        }
      },
      checkRelation(relation, updateType) {

        if (relation.Subject !== this.table.TableName) {
          if (relation.Object !== this.table.TableName) {
            if (updateType === "subject") {
              relation.Object = this.table.TableName;
            } else if (updateType === "object") {
              relation.Subject = this.table.TableName;
            }
          }
        }
      },

      updatedRelation(col) {

        if (this.newRelation.Subject !== this.localTable.TableName) {
          if (this.newRelation.Object !== this.localTable.TableName) {
            if (col === "subject") {
              this.newRelation.Object = this.localTable.TableName;
              return
            } else if (col === "object") {
              this.newRelation.Subject = this.localTable.TableName;
              return;
            }
          }
        }


        if (this.newRelation.Object && this.newRelation.Relation) {
          this.localTable.Relations.push(this.newRelation);
          this.newRelation = {
            Subject: this.localTable.TableName,
            Relation: null,
            Object: null,
          }
        }
      },
      columnNameUpdated() {
        console.log("new column updated", arguments);
        if (this.newColumn.ColumnName && this.newColumn.ColumnType) {
          this.localTable.ColumnModel.push(this.newColumn);
          this.newColumn = {
            ColumnName: null,
            ColumnType: null,
            DefaultValue: null,
            IsIndexed: false,
            IsUnique: false,
            IsNullable: false,
          };
        }
      },
    },
    name: "TableEditor",
    data() {
      return {
        StandardColumns: ["id", "created_at", "updated_at", "reference_id", "permission", "version"],
        StandardRelations: ["user_account_id", "usergroup_id"],
        ColumnTypes: [
          {
            columnType: 'label',
            dataType: 'varchar(50)'
          },
          {
            columnType: 'label',
            dataType: 'varchar(100)'
          },
          {
            columnType: 'content',
            dataType: 'varchar(500)'
          },
          {
            columnType: 'content',
            dataType: 'varchar(1000)'
          },
          {
            columnType: 'content',
            dataType: 'text'
          },
          {
            columnType: 'measurement',
            dataType: 'int(4)'
          },
          {
            columnType: 'measurement',
            dataType: 'int(11)'
          },
          {
            columnType: 'measurement',
            dataType: 'float(11)'
          },
          {
            columnType: 'file.mp3|wav',
            dataType: 'blob'
          },
          {
            columnType: 'file.mp4|mkv',
            dataType: 'blob'
          },
          {
            columnType: 'file.jpg|png|gif',
            dataType: 'blob'
          },
          {
            columnType: 'json',
            dataType: 'json'
          },
          {
            columnType: 'datetime',
            dataType: 'datetime'
          },
          {
            columnType: 'value',
            dataType: 'int(11)'
          },
          {
            columnType: 'alias',
            dataType: 'varchar(30)'
          },
          {
            columnType: 'truefalse',
            dataType: 'int(1)'
          },
          {
            columnType: 'gzip',
            dataType: 'blob'
          },
        ],
        RelationTypes: ['has_one', 'belongs_to', 'has_many'],
        tableName: null,
        isEdit: false,
        localTable: {
          TableName: null,
          ColumnModel: [],
          Relations: [],
        },
        newColumn: {
          ColumnName: null,
          ColumnType: null,
          DefaultValue: null,
          IsIndexed: false,
          IsUnique: false,
          IsNullable: false,
        },
        newRelation: {
          Subject: null,
          Relation: null,
          Object: null,
        }
      }
    },

    computed: {
      ...mapGetters(['tables'])
    },
    watch: {
      'localTable.TableName': function (newName, oldName) {
        console.log("Name changed", newName, oldName, this.localTable.Relations);
        this.tableName = newName;
        if (this.localTable && this.localTable.Relations) {
          this.localTable.TableName = newName;
          for (var i = 0; i < this.localTable.Relations.length; i++) {
            if (this.localTable.Relations[i].Subject === oldName) {
              this.localTable.Relations[i].Subject = newName
            } else if (this.localTable.Relations[i].Object === oldName) {
              this.localTable.Relations[i].Object = newName
            }
          }
          this.newRelation.Subject = newName;
        }
      }
    }
  }
</script>

<style scoped>

</style>
