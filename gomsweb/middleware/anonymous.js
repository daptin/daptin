export default function ({ store, redirect }) {
  console.log("anonymous middleware check")
  if (store.getters.isAuthenticated) {
    return redirect('/')
  }
}
