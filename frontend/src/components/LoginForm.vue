<!-- src/components/LoginForm.vue -->
<script setup lang="ts">
import { ref } from 'vue'
import api from '../api'
import { setAuth, fetchCurrentUser } from '../auth'
import { authMode, closeAuth } from '../panelAuth'
import { loadNotifs, addNotif } from '../notify'

const mode = ref<'login'|'forgot'>('login')

// --- Đăng nhập ---
const l_username = ref('')
const l_password = ref('')
const l_loading = ref(false)
const l_err = ref('')

async function onLogin() {
  if (l_loading.value) return
  l_err.value = ''
  l_loading.value = true
  try {
    const r = await api.login({ username: l_username.value.trim(), password: l_password.value })
    setAuth(r.token, r.user)
    await fetchCurrentUser()
    closeAuth()              // đóng modal auth
    authMode.value = ''      // đảm bảo quay lại app
  } catch (e:any) {
    l_err.value = e?.message || 'Đăng nhập thất bại'
  } finally {
    l_loading.value = false
  }
}

// --- Quên mật khẩu (dùng mật khẩu cấp 2 để đặt lại) ---
const f_username = ref('')
const f_secpass  = ref('')     // mật khẩu cấp 2 (đã cài trong Profile)
const f_newpass  = ref('')
const f_confirm  = ref('')
const f_loading  = ref(false)
const f_err      = ref('')
const f_msg      = ref('')

async function onForgot() {
  if (f_loading.value) return
  f_err.value = ''; f_msg.value = ''
  if (!f_username.value.trim()) { f_err.value = 'Vui lòng nhập tên đăng nhập'; return }
  if (!f_secpass.value) { f_err.value = 'Vui lòng nhập mật khẩu cấp 2'; return }
  if (!f_newpass.value || f_newpass.value.length < 6) { f_err.value = 'Mật khẩu mới phải ≥ 6 ký tự'; return }
  if (f_newpass.value !== f_confirm.value) { f_err.value = 'Mật khẩu nhập lại không khớp'; return }

  f_loading.value = true
  try {
    await api.forgotPassword({
      username: f_username.value.trim(),
      secPassword: f_secpass.value,
      newPassword: f_newpass.value,
    } as any) // nếu api.ts đặt key là secPassword
    f_msg.value = 'Đặt lại mật khẩu thành công. Hãy đăng nhập.'
    // tự chuyển về login sau 1.2s
    setTimeout(()=>{ mode.value = 'login' }, 1200)
  } catch (e:any) {
    f_err.value = e?.message || 'Không đặt lại được mật khẩu'
  } finally {
    f_loading.value = false
  }
}
   // 👇 nạp & thêm thông báo chào mừng (1 lần / user)
    loadNotifs();
    addNotif({
      id: 'welcome', // id cố định để không bị lặp
      title: 'Chào mừng!',
      body: 'Chào mừng bạn đã gia nhập với thế giới trò chơi truy tìm kho báu 🎉',
    });
// Chuyển mode
function toLogin(){ mode.value = 'login'; l_err.value=''}
function toForgot(){ mode.value = 'forgot'; f_err.value=''; f_msg.value=''}
function toRegister(){ authMode.value = 'register' }
</script>

<template>
  <div class="wrap">
    <!-- LOGIN -->
    <form v-if="mode==='login'" class="form" @submit.prevent="onLogin">
      <h3>Đăng nhập</h3>

      <label>Tên đăng nhập</label>
      <input v-model.trim="l_username" placeholder="username" required />

      <label>Mật khẩu</label>
      <input v-model="l_password" type="password" placeholder="••••••••" required minlength="6" />

      <button :disabled="l_loading">{{ l_loading ? 'Đang đăng nhập...' : 'Đăng nhập' }}</button>

      <div class="row-hint">
        <button type="button" class="link" @click="toForgot">Quên mật khẩu?</button>
        <span>•</span>
        <button type="button" class="link" @click="toRegister">Chưa có tài khoản? Đăng ký</button>
      </div>

      <p v-if="l_err" class="err">{{ l_err }}</p>
    </form>

    <!-- FORGOT -->
    <form v-else class="form" @submit.prevent="onForgot">
      <h3>Đặt lại mật khẩu</h3>
      <p class="sub">Nhập <b>mật khẩu cấp 2</b> đã cài trong Trang cá nhân để đặt lại mật khẩu đăng nhập.</p>

      <label>Tên đăng nhập</label>
      <input v-model.trim="f_username" placeholder="username" required />

      <label>Mật khẩu cấp 2</label>
      <input v-model="f_secpass" type="password" placeholder="••••••••" required minlength="6" />

      <label>Mật khẩu mới</label>
      <input v-model="f_newpass" type="password" placeholder="Mật khẩu mới" required minlength="6" />

      <label>Nhập lại mật khẩu mới</label>
      <input v-model="f_confirm" type="password" placeholder="Nhập lại mật khẩu mới" required minlength="6" />

      <button :disabled="f_loading">{{ f_loading ? 'Đang xử lý...' : 'Xác nhận đặt lại' }}</button>

      <div class="row-hint">
        <button type="button" class="link" @click="toLogin">Đã nhớ mật khẩu? Đăng nhập</button>
        <span>•</span>
        <button type="button" class="link" @click="toRegister">Chưa có tài khoản? Đăng ký</button>
      </div>

      <p v-if="f_err" class="err">{{ f_err }}</p>
      <p v-if="f_msg" class="ok">{{ f_msg }}</p>
    </form>
  </div>
</template>

<style scoped>
.wrap{ display:grid; gap:12px; }
.form{ display:grid; gap:10px; padding:6px; }
h3{ margin:0 0 6px; }
.sub{ color:#555; margin:-6px 0 6px; }
label{ font-weight:600; }
input{ padding:10px; border:1px solid #ddd; border-radius:10px; }
button{
  height:40px; padding:0 14px; border:1px solid #ddd; border-radius:10px;
  background:#1e80ff; color:#fff; cursor:pointer;
}
button:disabled{ opacity:.7; cursor:default; }

.row-hint{
  display:flex; gap:10px; align-items:center; justify-content:center;
  margin-top:4px; color:#666;
}
.link{
  background:transparent; color:#1e80ff; border:none; cursor:pointer; height:auto; padding:0;
}

.ok{ color:#16a34a; }
.err{ color:#d33; }
</style>
