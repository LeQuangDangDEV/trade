<template>
  <form @submit.prevent="onSubmit">
    <label>Email</label>
    <input v-model.trim="email" type="email" required />
    <label>Mật khẩu</label>
    <input v-model="password" type="password" required />
    <button :disabled="loading" class="btn">Đăng nhập</button>
    <p class="alt">Chưa có tài khoản?
      <a href="" @click.prevent="$emit('switch','register')">Đăng ký</a>
    </p>
    <p v-if="error" class="err">{{ error }}</p>
  </form>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { api } from '../api'
import { setToken, fetchCurrentUser } from '../auth'

const emit = defineEmits<{ (e:'switch', m:'login'|'register'):void; (e:'done'):void }>()
const email = ref(''); const password = ref('')
const loading = ref(false); const error = ref('')

async function onSubmit() {
  loading.value = true; error.value = ''
  try {
    const res = await api.post('/login', { email: email.value, password: password.value })
    const data = res.data
    if (data?.token) {
      setToken(data.token)
      await fetchCurrentUser()
      emit('done')
    } else error.value = 'Đăng nhập thất bại'
  } catch (e:any) {
    error.value = e?.response?.data?.error || 'Lỗi đăng nhập'
  } finally { loading.value = false }
}
</script>

<style scoped>
label{display:block;margin-top:12px} input{width:100%;padding:8px;border:1px solid #ddd;border-radius:8px}
.btn{margin-top:16px;padding:10px 12px;border:none;border-radius:8px;background:#1e80ff;color:#fff;cursor:pointer}
.err{color:#d33;margin-top:8px}.alt{margin-top:10px}
</style>
