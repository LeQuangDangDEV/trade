// src/panelAuth.ts
import { ref } from 'vue'

export const authMode = ref<'' | 'login' | 'register'>('')

export function ensureAuthOpen(mode: 'login' | 'register' = 'login') {
  authMode.value = mode
}
export function openAuth(mode: 'login' | 'register' = 'login') {
  authMode.value = mode
}
export function closeAuth() {
  authMode.value = ''         // ✅ CHỈ set state, KHÔNG đụng DOM
}
