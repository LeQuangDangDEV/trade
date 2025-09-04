<!-- src/components/Navbar.vue -->
<template>
  <nav class="nav">
    <div class="brand">
      <router-link to="/">MyApp</router-link>
    </div>

    <div class="spacer" />

    <div v-if="!authed" class="right">
      <button class="btn" @click="openAuth('login')">Đăng nhập</button>
      <button class="btn" @click="openAuth('register')">Đăng ký</button>
    </div>

    <div v-else class="right">
      <router-link to="/profile" class="me">
        <img
          class="avatar"
          :key="avatarKey"
          :src="avatarSrc"
          alt="avatar"
          @error="onImgErr"
        />
        <span class="name">{{ me?.name || me?.username || 'Người dùng' }}</span>
      </router-link>
      <router-link to="/profile">
        <button class="btn">Quản lý hồ sơ</button>
        
      </router-link>
       <div class="notify" ref="notifyRoot">
        <button class="icon-btn" @click="toggleDropdown" title="Thông báo">
          <!-- icon chuông đơn giản bằng SVG -->
          <svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true">
            <path d="M12 2a6 6 0 0 0-6 6v3.586l-.707.707A1 1 0 0 0 6 14h12a1 1 0 0 0 .707-1.707L18 11.586V8a6 6 0 0 0-6-6Zm0 20a3 3 0 0 0 3-3H9a3 3 0 0 0 3 3Z"/>
          </svg>
          <span v-if="unread>0" class="badge">{{ unread }}</span>
        </button>

        <div v-if="open" class="dropdown" @click.outside="open=false">
          <div class="dd-head">
            <b>Thông báo</b>
            <div class="dd-actions">
              <button class="mini" @click="markAllRead()">Đánh dấu đã đọc</button>
              <button class="mini danger" @click="clearAll()">Xóa tất cả</button>
            </div>
          </div>

          <div v-if="list.length === 0" class="dd-empty">Chưa có thông báo</div>

          <ul v-else class="dd-list">
            <li v-for="n in list" :key="n.id" :class="['dd-item', !n.read && 'unread']">
              <div class="dd-title">{{ n.title }}</div>
              <div class="dd-body">{{ n.body }}</div>
              <div class="dd-meta">
                <span>{{ fmtTime(n.createdAt) }}</span>
                <button class="link" @click="remove(n.id)">Xoá</button>
              </div>
            </li>
          </ul>
        </div>
      </div>
      <button class="btn" @click="logout">Đăng xuất</button>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { computed, ref, watch, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import { openAuth, closeAuth } from '../panelAuth'
import { currentUser, clearAuth } from '../auth'
import { BASE } from '../api'
import { notifs, unreadCount, markAllRead, clearAll, removeNotif, loadNotifs } from '../notify'

const router = useRouter()

/* ---------- auth / user ---------- */
const me = computed(() => currentUser.value)
const authed = computed(() => !!me.value)

/* ---------- dropdown thông báo ---------- */
const open = ref(false)
const unread = computed(() => unreadCount.value)
const list = computed(() => notifs.value)
const notifyRoot = ref<HTMLElement | null>(null)

function toggleDropdown() {
  open.value = !open.value
  if (open.value) markAllRead()
}
function remove(id: string) {
  removeNotif(id)
}

/* ---------- avatar ---------- */
const avatarPlaceholder = 'https://avatar.iran.liara.run/public/1'
const bust = ref(0) // cache-buster khi avatar đổi

watch(() => me.value?.avatarUrl, () => {
  // avatarUrl đổi -> tăng bust để buộc <img> reload
  bust.value = Date.now()
})

function fullUrl(u?: string) {
  if (!u) return ''
  return /^https?:\/\//i.test(u) ? u : `${BASE}${u}`
}
const avatarSrc = computed(() => {
  const base = me.value?.avatarUrl ? fullUrl(me.value.avatarUrl) : avatarPlaceholder
  return bust.value ? `${base}?v=${bust.value}` : base
})
const avatarKey = computed(() => `${me.value?.id || 'guest'}-${bust.value}`)

function onImgErr(e: Event) {
  (e.target as HTMLImageElement).src = avatarPlaceholder
}

/* ---------- click ra ngoài để đóng dropdown ---------- */
function onDocClick(e: MouseEvent) {
  if (!open.value) return
  const t = e.target as Node
  const root = notifyRoot.value
  if (root && !root.contains(t)) {
    open.value = false
  }
}


/* khi đăng nhập/đăng xuất: load/clear thông báo & đóng dropdown */
watch(authed, async (v) => {
  open.value = false
  if (v) {
    await loadNotifs().catch(() => {})
  } else {
    clearAll()
  }
})

/* ---------- format thời gian đơn giản ---------- */
function fmtTime(iso: string) {
  try {
    const d = new Date(iso)
    return d.toLocaleString()
  } catch { return iso }
}

/* ---------- open auth panels (nếu dùng trong template) ---------- */
function openLogin() { openAuth('login') }
function openRegister() { openAuth('register') }

/* ---------- logout ---------- */
async function logout() {
  clearAuth()
  closeAuth()
  router.replace({ name: 'home' })
}
</script>


<style scoped>
.nav{
  position: sticky; top: 0; background: #fff;
  height: 56px; display: flex; align-items: center;
  padding: 0 16px; border-bottom: 1px solid #eee; gap: 12px; z-index: 40;
}
.brand a{ text-decoration: none; color: #111; font-weight: 700; }
.spacer{ flex: 1; }
.right{ display: flex; align-items: center; gap: 10px; }
.btn{
  height: 36px; padding: 0 12px; border: 1px solid #ddd; border-radius: 10px;
  background: #f8f8f8; cursor: pointer;
}
.btn:hover{ background: #efefef; }
.me{
  display:flex; align-items:center; gap:8px; text-decoration:none; color:inherit;
}
.avatar{ width: 32px; height: 32px; border-radius: 50%; object-fit: cover; }
.name{ line-height: 1; }
.notify{ position: relative; }
.icon-btn{
  position: relative;
  height: 36px; width: 36px;
  display: grid; place-items: center;
  border: 1px solid #ddd; border-radius: 10px; background: #f8f8f8; cursor: pointer;
}
.icon-btn:hover{ background: #efefef; }
.icon-btn svg{ fill: #444; }
.badge{
  position: absolute; top: -6px; right: -6px;
  min-width: 18px; height: 18px; padding: 0 5px;
  display: grid; place-items: center;
  background: #ef4444; color:#fff; border-radius: 9px; font-size: 12px; font-weight: 700;
}

.dropdown{
  position: absolute; top: 44px; right: 0; width: min(340px, 92vw);
  background: #fff; border: 1px solid #eee; border-radius: 12px; box-shadow: 0 8px 24px rgba(0,0,0,.12);
  overflow: hidden; z-index: 50;
}
.dd-head{
  display:flex; align-items:center; justify-content:space-between;
  padding: 10px 12px; border-bottom: 1px solid #f1f1f1;
}
.dd-actions{ display:flex; gap:6px; }
.mini{ padding:6px 10px; border:1px solid #ddd; border-radius:8px; background:#fff; cursor:pointer; }
.mini.danger{ border-color:#fecaca; background:#fee2e2; }
.dd-empty{ padding: 12px; color:#666; }
.dd-list{ list-style:none; margin:0; padding:0; max-height: 60vh; overflow:auto; }
.dd-item{ padding:10px 12px; border-top:1px solid #f8f8f8; }
.dd-item.unread .dd-title{ font-weight: 700; }
.dd-title{ margin-bottom: 2px; }
.dd-body{ color:#444; }
.dd-meta{ margin-top:6px; font-size:12px; color:#666; display:flex; justify-content:space-between; }
.link{ background:none; border:none; color:#1e80ff; cursor:pointer; padding:0; }
</style>
