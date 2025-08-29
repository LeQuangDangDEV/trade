<template>
  <form @submit.prevent="onSubmit">
    <label>Tên</label><input v-model.trim="name" required minlength="2" />
    <label>Email</label><input v-model.trim="email" type="email" required />
    <label>SĐT</label><input v-model.trim="phone" required minlength="8" />
    <label>Mật khẩu</label><input v-model="password" type="password" required minlength="6" />
    <button :disabled="loading" class="btn">Tạo tài khoản</button>
    <p class="alt">Đã có tài khoản?
      <a href="" @click.prevent="$emit('switch','login')">Đăng nhập</a>
    </p>
    <p v-if="error" class="err">{{ error }}</p>
  </form>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { api } from '../api'

const emit = defineEmits<{ (e:'switch', m:'login'|'register'):void }>()
const name = ref(''); const email = ref(''); const phone = ref(''); const password = ref('')
const loading = ref(false); const error = ref('')

async function onSubmit() {
  loading.value = true; error.value = ''
  try {
    await api.post('/register', { name: name.value, email: email.value, phone: phone.value, password: password.value })
    // chuyển sang login ngay trong panel
    emit('switch','login')
  } catch (e:any) {
    error.value = e?.response?.data?.error || 'Lỗi đăng ký'
  } finally { loading.value = false }
}
</script>

<style scoped>
label{display:block;margin-top:12px} input{width:100%;padding:8px;border:1px solid #ddd;border-radius:8px}
.btn{margin-top:16px;padding:10px 12px;border:none;border-radius:8px;background:#1e80ff;color:#fff;cursor:pointer}
.err{color:#d33;margin-top:8px}.alt{margin-top:10px}
</style>
