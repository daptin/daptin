import DashView from './components/Dash.vue'
import LoginView from './components/Login.vue'
import NotFoundView from './components/404.vue'


import InstanceView from './components/InstanceView'
import EntityView from './components/EntityView'
import RelationView from './components/RelationView'

import AdminComponent from './components/Admin'
// import AdminView from './components/AdminApp'
import SignInComponent from './components/SignIn'
import SignedInComponent from './components/SignedIn'
import SignOutComponent from './components/SignOut'
import SignUpComponent from './components/SignUp'
import ActionComponent from './components/Action'
import HomeComponent from './components/Home'


// Import Views - Dash
import DashboardView from './components/views/Dashboard.vue'
import TasksView from './components/views/Tasks.vue'
import SettingView from './components/views/Setting.vue'
import AccessView from './components/views/Access.vue'
import ServerView from './components/views/Server.vue'
import ReposView from './components/views/Repos.vue'

// Routes
const routes = [
  {
    name: 'SignIn',
    path: '/auth/signin',
    component: SignInComponent
  },
  {
    name: 'SignedIn',
    path: '/auth/signedin',
    component: SignedInComponent
  },
  {
    name: 'SignUp',
    path: '/auth/signup',
    component: SignUpComponent
  },
  {
    name: 'SignOut',
    path: '/auth/signout',
    component: SignOutComponent
  },
  {
    path: '/',
    component: DashView,
    children: [
      {
        path: '/act/:tablename/:actionname',
        name: 'Action',
        component: ActionComponent
      },
      {
        path: '/in',
        component: AdminComponent,
        children: [
          {
            path: ':tablename',
            name: 'Entity',
            component: EntityView
          },
          {
            path: ':tablename/:refId',
            name: 'Instance',
            component: InstanceView
          },
          {
            path: ':tablename/:refId/:subTable',
            name: 'Relation',
            component: RelationView
          }
        ]
      },
      {
        path: 'dashboard',
        alias: '',
        component: DashboardView,
        name: 'Dashboard',
        meta: {description: 'Overview of environment'}
      }, {
        path: 'tasks',
        component: TasksView,
        name: 'Tasks',
        meta: {description: 'Tasks page in the form of a timeline'}
      }, {
        path: 'setting',
        component: SettingView,
        name: 'Settings',
        meta: {description: 'User settings page'}
      }, {
        path: 'access',
        component: AccessView,
        name: 'Access',
        meta: {description: 'Example of using maps'}
      }, {
        path: 'server',
        component: ServerView,
        name: 'Servers',
        meta: {description: 'List of our servers'}
      }, {
        path: 'repos',
        component: ReposView,
        name: 'Repository',
        meta: {description: 'List of popular javascript repos'}
      }
    ]
  },
  {
    // not found handler
    path: '*',
    component: NotFoundView
  }
]

export default routes
