import Vue from 'vue'
import Router from 'vue-router'
import Home from '@/components/Home'

Vue.use(Router);

export default new Router({
  routes: [
    {
      path: '/:tablename',
      props: true,
      name: 'Home',
      component: Home,
    },
    {
      path: '/:tablename/:refId',
      props: true,
      name: 'Instance',
      component: Home,
    },
    {
      path: '/:tablename/:refId/:subTable',
      props: true,
      name: 'SubTables',
      component: Home,
    },
    {
      path: '',
      name: 'Table view', props: true,
      component: Home,
    }
  ]
})
