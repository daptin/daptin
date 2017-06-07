export default function ({ store, redirect }) {
  console.log("auth middleware check")
  if (!store.getters.isAuthenticated) {
    return redirect('/auth/sign-in')
  }
}
