import DashView from './components/Dash.vue'
import LoginView from './components/Login.vue'
import Login2FAView from './components/Login.vue'
import NotFoundView from './components/404.vue'


import InstanceView from './components/InstanceView'
import EntityView from './components/EntityView'
import NewItem from './components/NewItem'
import RelationView from './components/RelationView'

import AdminComponent from './components/Admin'
// import AdminView from './components/AdminApp'
import SignInComponent from './components/SignIn'
import SignIn2FAComponent from './components/Login2FA'
import SignedInComponent from './components/SignedIn'
import SignOutComponent from './components/SignOut'
import OauthResponseComponent from './components/OauthResponse'
import SignUpComponent from './components/SignUp'
import ActionComponent from './components/Action'
import HomeComponent from './components/Home'


// Import Views - Dash
import DashboardView from './components/views/Dashboard.vue'
import AllInOne from './components/views/AllInOne.vue'
import ConfigurationEditor from "./components/ConfigurationEditor";

// Routes
const routes = [
  {
    name: 'SignIn',
    path: '/auth/signin',
    component: SignInComponent
  },
  {
    name: 'SignIn2FA',
    path: '/auth/signin2fa',
    component: SignIn2FAComponent
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
    name: "OauthResponse",
    path: '/oauth/response',
    component: OauthResponseComponent,
  },
  {
    path: '/',
    component: DashView,
    children: [
      {
        path: '',
        name: 'Dashboard',
        component: DashboardView,
      },
      {
        path: '/all',
        name: 'AllInOne',
        component: AllInOne,
      },
      {
        path: '/act/:tablename/:actionname',
        name: 'Action',
        component: ActionComponent
      },
      {
        path: '/',
        component: AdminComponent,
        children: [
          {
            path: '/in/item/:tablename',
            name: 'Entity',
            component: EntityView
          },
          {
            path: '/in/configuration',
            name: 'Configuration',
            component: ConfigurationEditor
          },
          {
            path: '/in/item/:tablename/new',
            name: 'NewEntity',
            component: EntityView
          },
          {
            path: '/in/item/:tablename/:refId',
            name: 'Instance',
            component: InstanceView
          },
          {
            path: '/in/meta/new',
            name: 'NewItem',
            component: NewItem
          },
          {
            path: '/in/item/:tablename/:refId/:subTable',
            name: 'Relation',
            component: RelationView
          }
        ]
      }
    ]
  },
  {
    // not found handler
    path: '*',
    component: NotFoundView
  }
];

export default routes
