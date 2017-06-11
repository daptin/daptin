import AdminComponent from './components/Admin'
import SignInComponent from './components/SignIn'
import SignedInComponent from './components/SignedIn'
import SignOutComponent from './components/SignOff'
import ActionComponent from './components/Action'


export default [
  {
    path: '/',
    component: AdminComponent
  },
  {
    path: '/auth/signin',
    name: 'signin',
    component: SignInComponent
  },
  {
    path: '/auth/signed-in',
    name: 'signed-in',
    component: SignedInComponent
  },
  {
    path: '/auth/signout',
    name: 'signout',
    component: SignOutComponent
  },
  {
    path: '/in/:tablename',
    name: 'tablename',
    component: AdminComponent
  },
  {
    path: '/act/:tablename/:actionname',
    name: 'tablename-actionname',
    component: ActionComponent
  },
  {
    path: '/in/:tablename/:refId',
    name: 'tablename-refId',
    component: AdminComponent
  },
  {
    path: '/in/:tablename/:refId/:subTable',
    name: 'tablename-refId-subTable',
    component: AdminComponent
  },
  {
    path: '*',
    name: 'error',
    component: AdminComponent
  }
]
