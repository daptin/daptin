import AdminComponent from './components/Admin'
import SignInComponent from './components/SignIn'
import SignedInComponent from './components/SignedIn'
import SignOutComponent from './components/SignOff'
import SignUpComponent from './components/SignUp'
import ActionComponent from './components/Action'
import HomeComponent from './components/Home'


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
    path: '/auth/signup',
    name: 'signup',
    component: SignUpComponent
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
    path: '/act/:tablename/:actionname',
    name: 'tablename-actionname',
    component: ActionComponent
  },
  {
    path: '*',
    name: 'error',
    component: HomeComponent
  }
]
