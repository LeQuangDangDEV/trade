// router.ts
import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import { getToken } from './auth' 

const routes: RouteRecordRaw[] = [
  { path: '/', name: 'home', component: () => import('./views/Home.vue') },
 { path: '/home', redirect: { name: 'home' } }, 
  { path: '/profile', name: 'profile', meta: { requiresAuth: true }, component: () => import('./views/Profile.vue') },
   {
    path: '/vip',
    component: () => import('./views/vip/VipLayout.vue'), // 👈 Sửa path nếu file bạn ở chỗ khác
    meta: { requiresAuth: true },
    children: [
      { path: '', redirect: { name: 'vip-info' } }, // /vip -> /vip/info
      { path: 'info',  name: 'vip-info',  component: () => import('./views/vip/VipInfo.vue') },
      { path: 'admin', name: 'vip-admin', component: () => import('./views/vip/VipAdmin.vue'), meta: { requiresAdmin: true } },
      { path: '/wallet', name: 'wallet', meta: { requiresAuth: true }, component: () => import('./views/Wallet.vue') },
      // router.ts
{ path: '/referral', name: 'referral', meta: { requiresAuth: true }, component: () => import('./views/Referral.vue') },


    ]
  },
]

const router = createRouter({ history: createWebHistory(), routes })

router.beforeEach((to, _from, next) => {
  const authed = !!localStorage.getItem('token')
  const isHome = (to.name === 'home') || (to.path === '/' || to.path === '/home')
  if (!authed && !isHome) {
    return { name: 'home' }
  }
  next()
})

export default router
