import { getUserFromCookie, getUserFromLocalStorage } from '~/utils/auth'

export default function ({ isServer, store, req }) {
   // If nuxt generate, pass this middleware
  if (isServer && !req) return
  const loggedUser = isServer ? getUserFromCookie(req) : getUserFromLocalStorage()
  store.commit('SET_USER', loggedUser)
}
