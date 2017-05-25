<template>
  <div id="app">
    <router-view></router-view>

    <link href="./static/bower_components/bootstrap/css/bootstrap.min.css" rel="stylesheet">
    <link href="./static/bower_components/bootstrap/css/bootstrap-theme.min.css" rel="stylesheet">
    <link href="./static/bower_components/font-awesome/css/font-awesome.min.css" rel="stylesheet">

    <script src="./static/bower_components/jquery/jquery-2.1.4.min.js" type="application/javascript"></script>
    <script src="./static/bower_components/bootstrap/js/bootstrap.min.js" type="application/javascript"></script>
  </div>
</template>

<script>
  export default {
    name: 'app'
  }
</script>

<style>
  #app {
    font-family: 'Avenir', Helvetica, Arial, sans-serif;
    -webkit-font-smoothing: antialiased;
    color: #2c3e50;
    margin: 10px;
    padding: 5px;
  }
</style>
