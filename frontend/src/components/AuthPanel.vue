<!-- src/components/AuthPanel.vue -->
<script setup lang="ts">
import { computed } from 'vue'
import { authMode, closeAuth } from '../panelAuth'
import LoginForm from './LoginForm.vue'
import RegisterForm from './RegisterForm.vue'

const comp = computed(() => authMode.value === 'register' ? RegisterForm : LoginForm)
</script>

<template>
  <teleport to="body">
    <div v-if="authMode" class="modal-backdrop" @click.self="closeAuth()">
      <div class="modal">
        <!-- key theo mode để remount form, tránh patchKeyedChildren lỗi -->
        <component :is="comp" :key="authMode" />
      </div>
    </div>
  </teleport>
</template>

<style scoped>
.modal-backdrop{position:fixed; inset:0; background:rgba(0,0,0,.45); display:flex; align-items:center; justify-content:center; z-index:9999;}
.modal{ width:min(460px,94vw); background:#fff; border-radius:14px; padding:16px; box-shadow:0 20px 60px rgba(0,0,0,.25); }
</style>
