<template>
  <div class="content-wrapper">
    <!-- Content Header (Page header) -->
    <section class="content-header">
      <h1>
        New Item
      </h1>
      <ol class="breadcrumb">
        <li>
          <a href="javascript:;">
            <i class="fa fa-home"></i>New</a>
        </li>
      </ol>
    </section>
    <section class="content">
      <div class="col-md-12">
        <h1>Hello</h1>
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
    mounted() {
      console.log("Loaded new meta page")
    }
  }

</script>
