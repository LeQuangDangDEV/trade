<template>
  <SlideOver :open="authPanelOpen" @close="closeAuth" :title="title">
    <!-- ✅ Dùng biến component (KHÔNG dùng chuỗi) -->
    <component
      :is="authPanelMode === 'login' ? LoginForm : RegisterForm"
      @switch="onSwitch"
      @done="onDone"
    />
  </SlideOver>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import SlideOver from './SlideOver.vue'
import LoginForm from './LoginForm.vue'      // ⬅️ biến component
import RegisterForm from './RegisterForm.vue'// ⬅️ biến component
import { authPanelOpen, authPanelMode, authNextPath, closeAuth } from '../panelAuth'
import { useRouter } from 'vue-router'

const router = useRouter()
const title = computed(() =>
  authPanelMode.value === 'login' ? 'Đăng nhập' : 'Đăng ký'
)

function onSwitch(mode: 'login' | 'register') {
  authPanelMode.value = mode
}

function onDone() {
  const next = authNextPath.value || '/home'
  closeAuth()
  router.push(next)
}
</script>
