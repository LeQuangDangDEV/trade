// src/notify.ts
import { ref, computed, watch } from 'vue'
import { currentUser } from './auth'

export type Notif = {
  id: string
  title: string
  body: string
  createdAt: string
  read: boolean
}

export const notifs = ref<Notif[]>([])
export const unreadCount = computed(() => notifs.value.filter(n => !n.read).length)

function keyFor(uid: number) { return `notifs:${uid}` }

export function loadNotifs() {
  const uid = currentUser.value?.id
  if (!uid) { notifs.value = []; return }
  try {
    const raw = localStorage.getItem(keyFor(uid))
    notifs.value = raw ? JSON.parse(raw) : []
  } catch { notifs.value = [] }
}

export function saveNotifs() {
  const uid = currentUser.value?.id
  if (!uid) return
  localStorage.setItem(keyFor(uid), JSON.stringify(notifs.value))
}

function rid() {
  // id ngắn gọn, unique tạm đủ dùng
  return (crypto?.randomUUID?.() || Math.random().toString(36).slice(2)) + Date.now()
}

/** Thêm thông báo (có thể truyền id cố định để tránh trùng lặp) */
export function addNotif(n: { id?: string; title: string; body: string }) {
  const uid = currentUser.value?.id
  if (!uid) return
  const id = n.id || rid()
  // tránh thêm trùng id (VD "welcome")
  if (notifs.value.some(x => x.id === id)) return

  notifs.value.unshift({
    id,
    title: n.title,
    body: n.body,
    createdAt: new Date().toISOString(),
    read: false,
  })
  saveNotifs()
}

export function markAllRead() {
  let changed = false
  notifs.value = notifs.value.map(n => {
    if (!n.read) { changed = true; return { ...n, read: true } }
    return n
  })
  if (changed) saveNotifs()
}

export function clearAll() {
  notifs.value = []
  saveNotifs()
}

export function removeNotif(id: string) {
  notifs.value = notifs.value.filter(n => n.id !== id)
  saveNotifs()
}

// tự load lại khi user đăng nhập/đăng xuất
watch(() => currentUser.value?.id, () => loadNotifs(), { immediate: true })
