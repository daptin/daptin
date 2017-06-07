module.exports = {
  /*
   ** Headers of the page
   */
  router: {
    middleware: 'check-auth'
  },
  head: {
    title: 'starter',
    meta: [
      {charset: 'utf-8'},
      {name: 'viewport', content: 'width=device-width, initial-scale=1'},
      {hid: 'description', name: 'description', content: 'Nuxt.js project'}
    ],
    link: [
      {rel: 'icon', type: 'image/x-icon', href: '/favicon.ico'},
      {rel: 'javascript', href: 'https://fonts.googleapis.com/css?family=Roboto'}

    ]
  },
  css: [
    "~assets/semanticui/semantic.css",
    "element-ui/lib/theme-default/index.css",
    "~/static/colors.css",
    {src: 'font-awesome/css/font-awesome.css', lang: 'css'}

  ],
  js: [
    "~assets/semanticui/semantic.js"
  ],


  plugins: [
    '~/plugins/main',
    '~/plugins/worldmanager',
    '~/plugins/jsonapi',
    '~/plugins/actionmanager',
    '~/plugins/main',
    "~/plugins/axios"
  ],
  /*
   ** Customize the progress-bar color
   */
  loading: {color: '#3B8070'},
  /*
   ** Build configuration
   */
  build: {
    /*
     ** Run ESLINT on save
     */
    extend (config, ctx) {
      if (ctx.isClient) {
        config.module.rules.push({
          enforce: 'pre',
          test: /\.(js|vue)$/,
          loader: 'eslint-loader',
          exclude: /(node_modules)/
        })
      }
    },
    vendor: [],
  }
}
