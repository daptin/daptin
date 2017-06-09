<template>
  <svg class="icon" :class="'icon-' + name">
    <use :xlink:href="xlink"/>
  </svg>
</template>

<script>
export default {
  props: ['name'],
  computed: {
    xlink () {
      return require(`assets/svg/${this.name}.svg`)
    }
  }
}
</script>

<style></style>
