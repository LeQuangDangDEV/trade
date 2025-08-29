import { createApp } from 'vue'
import App from './App.vue'
import { router } from './router'
import { initAuth } from './auth'


initAuth()
createApp(App).use(router).mount('#app')