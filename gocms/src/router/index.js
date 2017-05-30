import Vue from 'vue'
import Router from 'vue-router'
import Home from '@/components/Home'

Vue.use(Router)

export default new Router({
  routes: [
    {
      path: '/:tablename',
      name: 'Home',
      component: Home,
    },
    {
      path: '',
      name: 'Table view',
      component: Home,
    }
  ]
})
