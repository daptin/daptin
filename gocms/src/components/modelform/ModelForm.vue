<template>

  <div class="ui segment attached">
    <form class="form ui" @submit.prevent="saveRow(model)">
      <div class="field" v-for="col in formBuildData">
        <label :for="col"> {{col.label}} </label>
        <input class="form-control" :id="col" :value="model[col.name]" v-model="model[col.name]">
      </div>
      <el-button @click.prevent="saveRow(model)">
        Save
      </el-button>
      <el-button @click="cancel()">Cancel</el-button>
    </form>
  </div>

</template>

<script>
  export default {
    props: [
      "model",
      "meta"
    ],
    data: function () {
//      console.log("this data", this);
//      console.log(arguments);
//      console.log(this.model);
      return {
        currentElement: "el-input",
        formBuildData: []
      }
    },
    created () {
    },
    computed: {},
    methods: {
      titleCase: function (str) {
        return str.replace(/[-_]/g, " ").split(' ')
                .map(w => w[0].toUpperCase() + w.substr(1).toLowerCase()
        ).join(' ')
      },
      saveRow: function () {
        console.log("save row");
        this.$emit('save', this.model)
      },
      cancel: function () {
        console.log("canel row");
        this.$emit('cancel')
      },
      endsWith: function (str1, str2) {
        if (str1.length < str2.length) {
          return false;
        }
        if (str1.substring(str1.length - str2.length) == str2) {
          return true;
        }
        return false;
      },
      reinit: function () {
        console.log("model form", this.meta, arguments);

        var colKeys = Object.keys(this.meta);

        for (var i = 0; i < colKeys.length; i++) {
          var column = colKeys[i];
          var colMeta = this.meta[column];

//          console.log("title case")
          var label = this.titleCase(column);
          if (typeof colMeta == "string") {
            colMeta = {
              name: column,
              columnType: colMeta,
            }
          } else {
            colMeta.name = column
          }
          colMeta.label = label;

//          console.log("col meta", colMeta);

          if (colMeta.columnType == "datetime") {
            continue;
          }

          if (colMeta.columnType == "entity") {
            continue;
          }

          if (colMeta.name == "status" || colMeta.name == "pending" || colMeta.name == "permission" || colMeta.name == "reference_id") {
            continue
          }


          this.formBuildData.push({
            name: colKeys[i],
            type: colMeta.type,
            label: label,
          })
        }
      }
    },
    mounted: function () {
      this.reinit()
    },
    watch: {},
  }
</script>s
