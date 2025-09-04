import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import { initAuth } from './auth'

;(async () => {
  // lưu & dọn param ?ref=... trước khi mount
  const url = new URL(location.href)
  const ref = url.searchParams.get('ref')
  if (ref) {
    localStorage.setItem('ref', ref)
    url.searchParams.delete('ref')
    history.replaceState({}, '', url.toString())
  }

  initAuth({ refresh: true }) // refresh /me ngầm

  const app = createApp(App)
  app.use(router)
  await router.isReady()
  app.mount('#app')
})()
