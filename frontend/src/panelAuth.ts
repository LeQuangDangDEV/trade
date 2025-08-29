import { ref } from 'vue'
import { isAuthenticated } from './auth'

export type AuthMode = 'login' | 'register'

export const authPanelOpen = ref(false)
export const authPanelMode = ref<AuthMode>('login')
export const authNextPath = ref<string | null>(null)

export function openAuth(mode: AuthMode = 'login', next?: string) {
  authPanelMode.value = mode
  authNextPath.value = next || null
  authPanelOpen.value = true
}

export function closeAuth() {
  authPanelOpen.value = false
}

export async function ensureAuthOpen(next?: string, mode: AuthMode = 'login') {
  if (!isAuthenticated.value) {
    openAuth(mode, next)
    // trả về false để caller biết là chưa login
    return false
  }
  return true
}
