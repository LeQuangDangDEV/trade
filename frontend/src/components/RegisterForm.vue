<template>
  <form class="form" @submit.prevent="onSubmit">
    <h3>Tạo tài khoản</h3>

    <label>Tên đăng nhập</label>
    <input v-model.trim="form.username" required minlength="3" placeholder="username" />

    <label>Biệt danh</label>
    <input v-model.trim="form.nickname" required minlength="2" placeholder="Nguyễn Văn A" />

    <label>Số điện thoại</label>
    <input v-model.trim="form.phone" required minlength="8" placeholder="098..." />

    <label>Mật khẩu</label>
    <input v-model="form.password" type="password" required minlength="6" placeholder="••••••••" />

    <label>Mã mời (tuỳ chọn)</label>
    <input v-model.trim="form.ref" placeholder="Nhập mã mời (nếu có)" />
    <small class="hint">Nếu mở từ link mời, mã sẽ tự điền tự động.</small>

    <button :disabled="loading">{{ loading ? 'Đang tạo...' : 'Tạo tài khoản' }}</button>

    <p v-if="msg" class="ok">{{ msg }}</p>
    <p v-if="error" class="err">{{ error }}</p>
  </form>
</template>

<script setup lang="ts">
import { reactive, ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import api from '../api'
import { authMode } from '../panelAuth' // để chuyển sang đăng nhập sau khi đăng ký

const route = useRoute()

const form = reactive({
  username: '',
  nickname: '',
  phone: '',
  password: '',
  ref: '',           // mã mời tuỳ chọn
})

const loading = ref(false)
const msg = ref('')
const error = ref('')

onMounted(() => {
  // Ưu tiên ?ref=... trên URL, nếu không có thì lấy từ localStorage (đã set ở main.ts)
  const qref = typeof route.query.ref === 'string' ? route.query.ref : ''
  const saved = localStorage.getItem('ref') || ''
  form.ref = qref || saved || ''
})

async function onSubmit() {
  if (loading.value) return
  loading.value = true; msg.value=''; error.value=''

  try {
    const payload: any = {
      username: form.username.trim(),
      password: form.password, // không trim
      nickname: form.nickname.trim(),
      phone: form.phone.trim(),
      ...(form.ref?.trim() ? { ref: form.ref.trim() } : {}),
    }

    await api.register(payload)

    // dùng xong ref thì dọn, tránh dính lần sau
    localStorage.removeItem('ref')

    msg.value = 'Đăng ký thành công! Vui lòng đăng nhập.'
    authMode.value = 'login' // chuyển panel sang đăng nhập
  } catch (e:any) {
    error.value = e?.message || 'Đăng ký thất bại'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.form{ display:grid; gap:10px; }
label{ font-weight:600; }
input{ padding:10px; border:1px solid #ddd; border-radius:10px; }
button{
  margin-top:6px; height:40px; padding:0 14px; border:1px solid #ddd;
  border-radius:10px; background:#1e80ff; color:#fff; cursor:pointer;
}
button:disabled{ opacity:.7; cursor:default; }
.ok{ color:#16a34a; }
.err{ color:#d33; }
.hint{ color:#666; font-size:12px; }
</style>
