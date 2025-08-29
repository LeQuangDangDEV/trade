<template>
<h2>Home</h2>
<p>Xin chào! Bạn đã đăng nhập.</p>
 <button @click="onSomeAction">Thực hiện hành động</button>
</template>


<script setup lang="ts">
import { useRouter } from 'vue-router'
import { ensureAuth } from '../auth'
import { ensureAuthOpen } from '../panelAuth'

const router = useRouter()

async function onProtectedAction() {
  const ok = await ensureAuthOpen(router.currentRoute.value.fullPath, 'login')
  if (!ok) return // panel đã mở, chờ người dùng đăng nhập
  // người dùng đã login → làm tiếp:
  // await api.post('/private/action', {...})
  alert('Bạn đã đăng nhập, hành động thực hiện!')
}



</script>


<style scoped>
</style>