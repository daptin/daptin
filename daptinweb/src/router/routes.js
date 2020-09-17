const routes = [
  {
    path: '/login',
    component: () => import('layouts/GuestLayout.vue'),
    children: [
      {
        path: '', component: () => import('pages/Login.vue')
      }
    ]
  },
  {
    path: '/register',
    component: () => import('layouts/GuestLayout.vue'),
    children: [
      {path: '', component: () => import('pages/Signup.vue')}
    ]
  },
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
        path: 'apidocs', component: () => import('pages/ApiDocsPage.vue')
      },
      {
        path: 'graphql', component: () => import('pages/ApiGraphqlPage.vue')
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
        path: '', component: () => import('pages/Users.vue'),
      },
    ]
  },
  {
    path: '/user',
    component: () => import('layouts/MainLayout.vue'),
    children: [
      {
        path: 'profile', component: () => import('pages/UserProfile.vue'),
      },

      {
        path: ':emailId', component: () => import('pages/EditUser.vue'),
      },
    ]
  },
  {
    path: '/groups',
    component: () => import('layouts/MainLayout.vue'),
    children: [
      {
        path: '', component: () => import('pages/UserGroups.vue')
      },
      {
        path: ':groupId', component: () => import('pages/EditGroup.vue')
      },
    ]
  },
  {
    path: '/integrations',
    component: () => import('layouts/MainLayout.vue'),
    children: [
      {
        path: 'spec', component: () => import('pages/ApiCatalogue.vue')
      },
      {
        path: 'actions', component: () => import('pages/Actions.vue')
      },
    ]
  },
  {
    path: '/apps',
    component: () => import('layouts/UserLayout.vue'),
    children: [
      {
        path: '', component: () => import('pages/UserApps/UserAppsList.vue')
      },
      {
        path: 'files', component: () => import('pages/UserApps/FileBrowser.vue')
      },
      {
        path: 'document/:documentId', component: () => import('pages/UserApps/DocumentEditor')
      },
      {
        path: 'spreadsheet/:documentId', component: () => import('pages/UserApps/SpreadsheetEditor')
      },
      {
        path: 'drageditor', component: () => import('pages/UserApps/DragEditor')
      },
      {
        path: 'calendar', component: () => import('pages/UserApps/Calendar.vue')
      },
    ]
  },
  {
    path: '/cloudstore',
    component: () => import('layouts/MainLayout.vue'),
    children: [
      {
        path: '', component: () => import('pages/CloudStorePage.vue')
      },
      {
        path: 'sites', component: () => import('pages/SitePage.vue')
      },
      {
        path: '/site/:siteId/browse', component: () => import('pages/SiteFileBrowserPage.vue')
      },
      {
        path: '/edit/:cloudStoreId', component: () => import('pages/CloudStorePage.vue')
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
];

// Always leave this as last one
if (process.env.MODE !== 'ssr') {
  routes.push({
    path: '*',
    component: () => import('pages/Error404.vue')
  })
}

export default routes
