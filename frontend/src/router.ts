import { createRouter, createWebHistory } from 'vue-router'
import Home from './views/Home.vue'
import Profile from './views/Profile.vue'
import VipLayout from './views/vip/VipLayout.vue'
import VipInfo from './views/vip/VipInfo.vue'
import VipAdmin from './views/vip/VipAdmin.vue'
import { isAuthenticated, currentUser } from './auth'
import { openAuth } from './panelAuth'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/home' },
    { path: '/home', component: Home },
    { path: '/profile', component: Profile, meta: { requiresAuth: true } },

    { // /vip có layout riêng và menu bên trong
      path: '/vip',
      component: VipLayout,
      meta: { requiresAuth: true },
      children: [
        { path: '', component: VipInfo },                       // /vip → Thông tin VIP
        { path: 'admin', component: VipAdmin, meta: { adminOnly: true } }, // /vip/admin
      ],
    },
  ],
})

router.beforeEach((to) => {
  if (to.meta.requiresAuth && !isAuthenticated.value) {
    openAuth('login', to.fullPath); return false
  }
  if (to.meta.adminOnly) {
    if (!isAuthenticated.value) { openAuth('login', to.fullPath); return false }
    if (currentUser.value?.role !== 'admin') { return { path: '/vip' } }
  }
  return true
})
