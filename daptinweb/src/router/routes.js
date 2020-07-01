const routes = [
  {
    path: '/tables',
    component: () => import('layouts/MainLayout.vue'),
    children: [
      {
        path: '', component: () => import('pages/Tables.vue'),
      },
      {
        path: 'create', component: () => import('pages/CreateTable.vue')
      },
      {
        path: 'edit/:tableName', component: () => import('pages/EditTable.vue')
      },
      {
        path: 'data/:tableName', component: () => import('pages/EditData.vue')
      },
    ]
  },
  {
    path: '/users',
    component: () => import('layouts/MainLayout.vue'),
    children: [
      {
        path: '/', component: () => import('pages/Users.vue'),
      },
      {
        path: '/:userId', component: () => import('pages/EditUser.vue'),
      },
    ]
  },
  {
    path: '/groups',
    component: () => import('layouts/MainLayout.vue'),
    children: [
      {
        path: '/', component: () => import('pages/UserGroups.vue')
      },
      {
        path: '/:groupId', component: () => import('pages/EditGroup.vue')
      },
    ]
  },
  {
    path: '/',
    component: () => import('layouts/MainLayout.vue'),
    children: [
      {
        path: '', component: () => import('pages/Index.vue')
      },
      {
        path: 'data', component: () => import('pages/Data.vue')
      },
    ]
  },
  {
    path: '/login',
    component: () => import('layouts/GuestLayout.vue'),
    children: [
      {path: '', component: () => import('pages/Login.vue')}
    ]
  },
  {
    path: '/register',
    component: () => import('layouts/GuestLayout.vue'),
    children: [
      {path: '', component: () => import('pages/Signup.vue')}
    ]
  },
];

// Always leave this as last one
if (process.env.MODE !== 'ssr') {
  routes.push({
    path: '*',
    component: () => import('pages/Error404.vue')
  })
}

export default routes
