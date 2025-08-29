import { ref, computed, onMounted } from 'vue'
import { getMe } from './api'

// === token ===
export const token = ref<string | null>(localStorage.getItem('token'))
export const isAuthenticated = computed(() => !!token.value)

// === currentUser ===
export type CurrentUser = {
  id: number; name: string; email: string; phone: string; avatarUrl: string; role: 'admin' | 'user'  
}
export const currentUser = ref<CurrentUser | null>(null)

// --- helpers lưu/đọc user từ localStorage (giảm nháy UI sau reload) ---
const USER_KEY = 'currentUser'
function saveUserToStorage(u: CurrentUser | null) {
  if (!u) localStorage.removeItem(USER_KEY)
  else localStorage.setItem(USER_KEY, JSON.stringify(u))
}
function loadUserFromStorage(): CurrentUser | null {
  try { return JSON.parse(localStorage.getItem(USER_KEY) || 'null') } catch { return null }
}

export function setToken(t: string) {
  token.value = t
  localStorage.setItem('token', t)
}
export function clearToken() {
  token.value = null
  localStorage.removeItem('token')
  currentUser.value = null
  saveUserToStorage(null)
}

// gọi sau khi đăng nhập / hoặc lúc khởi động
export async function fetchCurrentUser() {
  if (!token.value) { currentUser.value = null; saveUserToStorage(null); return }
  try {
    const { data } = await getMe()
    currentUser.value = data.user as CurrentUser
    saveUserToStorage(currentUser.value)
  } catch {
    // token hỏng
    clearToken()
  }
}

// === Khởi tạo khi load app ===
// 1) hydrate user từ localStorage để không nháy
// 2) nếu có token thì gọi /me để đồng bộ thật
export function initAuth() {
  const cached = loadUserFromStorage()
  if (cached) currentUser.value = cached
  if (token.value) {
    // không await để app mount nhanh, nhưng bạn có thể await trong main.ts nếu muốn
    fetchCurrentUser()
  }
}

// Đồng bộ token giữa nhiều tab
export function useAuthSync() {
  onMounted(() => {
    window.addEventListener('storage', (e) => {
      if (e.key === 'token') token.value = e.newValue
      if (e.key === USER_KEY) {
        try { currentUser.value = JSON.parse(e.newValue || 'null') } catch { /* noop */ }
      }
    })
  })
}
