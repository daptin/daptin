<template>
  <div class="content-wrapper">
    <!-- Content Header (Page header) -->
    <section class="content-header">
      <ol class="breadcrumb">
        <li>
          <a href="javascript:;">
            <i class="fa fa-home"></i>New item</a>
        </li>
      </ol>
    </section>
    <section class="content">
      <div class="col-md-6">
        <h3>Lets create a new item type</h3>
        <form role="form">
          <div class="form-group">
            <label>What do we call this</label>
            <input type="text" class="form-control" v-model="data.table_name" name="name"
                   placeholder="Use a singular name">
          </div>
          <div class="form-group">
            <label>List down all the attributes you want to manage for {{data.table_name}}</label>
            <div class="row">
              <div class="col-md-12" v-for="col in data.columns">
                {{col.name}} <input type="text" class="form-control" name="attr" placeholder="Attribute name"
                                    v-model="col.name">
                <select>
                  <option>

                  </option>
                </select>
              </div>
            </div>
            <div class="row">
              <div class="col-md-12">
                <button class="btn btn-xs"><i class="fa fa-plus"></i></button>
              </div>
            </div>
          </div>

        </form>
      </div>
    </section>
  </div>
</template>
<script>

  var typeMeta = [
    {
      name: "entity",
      label: "Entity"
    },
  ];

  export default {
    data() {
      return {
        data: {
          table_name: null,
          columns: [{
            name: null,
          }]
        },
        columnTypes: [{
          name: "varchar",
          label: "Small text",
          description: "For names"
        }, {

        }],
      }
    },
    mounted() {
      console.log("Loaded new meta page")
    }
  }

</script>
