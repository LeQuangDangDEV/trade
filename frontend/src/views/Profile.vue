<!-- src/views/Profile.vue -->
<script setup lang="ts">
import { reactive, ref, onMounted, computed } from 'vue'
import api, { BASE } from '../api'
import { currentUser, fetchCurrentUser } from '../auth'

/* -------------------- helpers -------------------- */
function absUrl(u?: string) {
  if (!u) return ''
  return /^https?:\/\//i.test(u) ? u : `${BASE}${u}`
}

/* -------------------- state: hồ sơ cơ bản -------------------- */
const base = reactive({
  name: '',
  phone: '',
  avatarUrl: '',   // lưu giá trị BE trả về (thường là /uploads/xxx.png)
})
const savingBase = ref(false)
const baseMsg = ref(''); const baseErr = ref('')

/* -------------------- avatar upload -------------------- */
const avatarFile = ref<File | null>(null)
const avatarPreview = ref<string>('') // preview tạm thời trên FE

function pickAvatar(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (!f) return
  avatarFile.value = f
  avatarPreview.value = URL.createObjectURL(f)
}
function clearAvatar() {
  avatarFile.value = null
  avatarPreview.value = ''
}

/* -------------------- đổi mật khẩu (login) -------------------- */
const pwd = reactive({ old: '', neu: '', rep: '' })
const savingPwd = ref(false)
const pwdMsg = ref(''); const pwdErr = ref('')

/* -------------------- bảo mật nâng cao -------------------- */
/* BE yêu cầu: oldSecondPassword?, newSecondPassword?, newTxnPin? */
const sec = reactive({
  oldSecondPassword: '',
  newSecondPassword: '',
  newTxnPin: '',
})
const savingSec = ref(false)
const secMsg = ref(''); const secErr = ref('')

/* -------------------- KYC CCCD (auto-approve) -------------------- */
const kycFront = ref<File | null>(null)
const kycBack  = ref<File | null>(null)
const kycFrontPreview = ref(''); const kycBackPreview = ref('')
const kycSending = ref(false)
const kycMsg = ref(''); const kycErr = ref('')

function pickKycFront(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (!f) return
  kycFront.value = f
  kycFrontPreview.value = URL.createObjectURL(f)
}
function pickKycBack(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (!f) return
  kycBack.value = f
  kycBackPreview.value = URL.createObjectURL(f)
}
function clearKycPreviews(){
  kycFront.value = null; kycBack.value = null
  kycFrontPreview.value = ''; kycBackPreview.value = ''
}

/* -------------------- computed -------------------- */
const kycStatus = computed(() => (currentUser.value as any)?.kycStatus ?? 'NONE')
const isVerified = computed(() => kycStatus.value === 'APPROVED')

/* -------------------- load hiện tại -------------------- */
async function loadMe() {
  await fetchCurrentUser().catch(()=>{})
  const u = currentUser.value
  if (!u) return
  base.name = u.name || ''
  base.phone = u.phone || ''
  // u.avatarUrl có thể là tương đối -> hiển thị dùng absUrl trong template
  base.avatarUrl = (u as any).avatarUrl || ''
}
onMounted(loadMe)

/* -------------------- actions -------------------- */
async function saveBase() {
  baseMsg.value=''; baseErr.value=''
  savingBase.value = true
  try {
    if (avatarFile.value) {
      const r = await api.uploadAvatar(avatarFile.value)    // { url: "/uploads/xxx.png" }
      base.avatarUrl = r.url
      avatarBust.value = Date.now() // 🔧 bust cache
      clearAvatar()
    }
    const r = await api.updateProfile({
      name: base.name.trim(),
      phone: base.phone.trim(),
      avatarUrl: base.avatarUrl || undefined
    })
    baseMsg.value = r.message || 'Đã lưu hồ sơ'

    // Cập nhật store để Navbar lấy avatar mới
    await fetchCurrentUser()
  } catch (e:any) {
    baseErr.value = e?.message || 'Lưu hồ sơ thất bại'
  } finally {
    savingBase.value = false
  }
}


async function changePassword() {
  pwdMsg.value=''; pwdErr.value=''
  if (!pwd.old || !pwd.neu || !pwd.rep) {
    pwdErr.value = 'Vui lòng nhập đầy đủ các ô.'
    return
  }
  if (pwd.neu.length < 6) {
    pwdErr.value = 'Mật khẩu mới tối thiểu 6 ký tự.'
    return
  }
  if (pwd.neu !== pwd.rep) {
    pwdErr.value = 'Nhập lại mật khẩu mới không khớp.'
    return
  }
  savingPwd.value = true
  try {
    const r = await api.changePassword({ oldPassword: pwd.old, newPassword: pwd.neu } as any)
    pwdMsg.value = r?.message || 'Đổi mật khẩu thành công'
    pwd.old = ''; pwd.neu=''; pwd.rep=''
  } catch (e:any) {
    pwdErr.value = e?.message || 'Đổi mật khẩu thất bại'
  } finally {
    savingPwd.value = false
  }
}

async function saveSecurity() {
  secMsg.value=''; secErr.value=''

  const body: any = {}
  // newSecondPassword?
  if (sec.newSecondPassword.trim()) {
    if (sec.newSecondPassword.trim().length < 6) {
      secErr.value = 'Mật khẩu cấp 2 tối thiểu 6 ký tự.'
      return
    }
    body.newSecondPassword = sec.newSecondPassword.trim()
    if (sec.oldSecondPassword.trim()) body.oldSecondPassword = sec.oldSecondPassword.trim()
  }
  // newTxnPin?
  if (sec.newTxnPin.trim()) {
    if (!/^\d{6}$/.test(sec.newTxnPin.trim())) {
      secErr.value = 'Mã bảo mật giao dịch (PIN) phải gồm 6 chữ số.'
      return
    }
    body.newTxnPin = sec.newTxnPin.trim()
  }

  if (Object.keys(body).length === 0) {
    secErr.value = 'Chưa có thay đổi nào để lưu.'
    return
  }

  savingSec.value = true
  try {
    const r = await api.updateSecurity(body)
    secMsg.value = r?.message || 'Đã cập nhật bảo mật'
    // Xoá plaintext trong UI
    sec.oldSecondPassword = ''
    sec.newSecondPassword = ''
    sec.newTxnPin = ''
  } catch (e:any) {
    secErr.value = e?.message || 'Cập nhật bảo mật thất bại'
  } finally {
    savingSec.value = false
  }
}

async function submitKyc() {
  kycMsg.value=''; kycErr.value=''
  if (!kycFront.value || !kycBack.value) {
    kycErr.value = 'Vui lòng chọn đủ ảnh mặt trước và mặt sau CCCD.'
    return
  }
  kycSending.value = true
  try {
    const fr = await api.uploadAvatar(kycFront.value) // { url: "/uploads/xxx.png" }
    const br = await api.uploadAvatar(kycBack.value)

    // ✅ gọi đúng endpoint & keys
    const r = await api.updateKyc({ frontUrl: fr.url, backUrl: br.url })
    kycMsg.value = r?.message || 'Đã xác minh KYC'
    clearKycPreviews()

    // kéo lại user để thấy kycStatus = APPROVED
    await fetchCurrentUser()
  } catch (e:any) {
    kycErr.value = e?.message || 'Gửi KYC thất bại'
  } finally {
    kycSending.value = false
  }
}

</script>

<template>
  <section class="wrap">
    <div class="header">
      <h2>Hồ sơ của bạn</h2>
      <span
        class="badge"
        :class="isVerified ? 'ok-badge' : 'warn-badge'"
        >{{ isVerified ? 'ĐÃ XÁC MINH ✅' : 'CHƯA XÁC MINH' }}</span>
    </div>

    <!-- Hồ sơ cơ bản -->
    <div class="card">
      <h3>Thông tin cơ bản</h3>
      <div class="grid">
        <label>Biệt danh</label>
        <input v-model.trim="base.name" placeholder="Biệt danh" />

        <label>Số điện thoại</label>
        <input v-model.trim="base.phone" placeholder="Số điện thoại" />

        <label>Ảnh đại diện</label>
        <div class="row">
          <input type="file" accept="image/*" @change="pickAvatar" />
          <img
            v-if="avatarPreview"
            :src="avatarPreview"
            class="avatar" alt="preview"
          />
          <img
            v-else-if="base.avatarUrl"
            :src="absUrl(base.avatarUrl)"
            class="avatar" alt="avatar"
          />
        </div>
      </div>

      <div class="actions">
        <button class="btn primary" :disabled="savingBase" @click="saveBase">
          {{ savingBase ? 'Đang lưu...' : 'Lưu thay đổi' }}
        </button>
      </div>
      <p class="ok" v-if="baseMsg">{{ baseMsg }}</p>
      <p class="err" v-if="baseErr">{{ baseErr }}</p>
    </div>

<!-- Đổi mật khẩu -->
<form class="card" @submit.prevent="changePassword" novalidate>
  <h3>Đổi mật khẩu</h3>
  <div class="grid">
    <label>Mật khẩu hiện tại</label>
    <input v-model="pwd.old" type="password" name="current-password"
           autocomplete="current-password" placeholder="••••••" />

    <label>Mật khẩu mới</label>
    <input v-model="pwd.neu" type="password" name="new-password"
           autocomplete="new-password" placeholder="Tối thiểu 6 ký tự" />

    <label>Nhập lại mật khẩu mới</label>
    <input v-model="pwd.rep" type="password" name="confirm-new-password"
           autocomplete="new-password" placeholder="Nhập lại" />
  </div>
  <div class="actions">
    <button class="btn" type="submit" :disabled="savingPwd">
      {{ savingPwd ? 'Đang đổi...' : 'Đổi mật khẩu' }}
    </button>
  </div>
  <p class="ok" v-if="pwdMsg">{{ pwdMsg }}</p>
  <p class="err" v-if="pwdErr">{{ pwdErr }}</p>
</form>

<!-- Bảo mật -->
<form class="card" @submit.prevent="saveSecurity" novalidate>
  <h3>Bảo mật</h3>
  <p class="hint">
    • <b>Mật khẩu cấp 2</b> dùng để khôi phục tài khoản.<br>
    • <b>Mã bảo mật giao dịch</b> là <b>6 số</b>.
  </p>
  <div class="grid">
    <label>Mật khẩu cấp 2 mới</label>
    <input v-model="sec.newSecondPassword" type="password" name="secondary-new-password"
           autocomplete="new-password" placeholder="(≥ 6 ký tự)" />

    <label>Mật khẩu cấp 2 cũ (nếu đã đặt)</label>
    <input v-model="sec.oldSecondPassword" type="password" name="secondary-old-password"
           autocomplete="current-password" placeholder="Nhập để xác minh đổi" />

    <label>Mã bảo mật giao dịch (6 số)</label>
    <input v-model="sec.newTxnPin" inputmode="numeric" pattern="\d{6}" maxlength="6"
           name="txn-pin" autocomplete="off" placeholder="******" />
  </div>
  <div class="actions">
    <button class="btn" type="submit" :disabled="savingSec">
      {{ savingSec ? 'Đang lưu...' : 'Lưu bảo mật' }}
    </button>
  </div>
  <p class="ok" v-if="secMsg">{{ secMsg }}</p>
  <p class="err" v-if="secErr">{{ secErr }}</p>
</form>


    <!-- Xác minh danh tính (CCCD) -->
    <div class="card">
      <h3>Xác minh danh tính (CCCD)</h3>

      <template v-if="!isVerified">
        <p class="hint">Gửi ảnh mặt trước & mặt sau. Hệ thống sẽ xác minh ngay (không cần phê duyệt).</p>
        <div class="grid">
          <label>Ảnh mặt trước</label>
          <div class="row">
            <input type="file" accept="image/*" @change="pickKycFront" />
            <img v-if="kycFrontPreview" :src="kycFrontPreview" class="kyc" alt="front preview" />
          </div>

          <label>Ảnh mặt sau</label>
          <div class="row">
            <input type="file" accept="image/*" @change="pickKycBack" />
            <img v-if="kycBackPreview" :src="kycBackPreview" class="kyc" alt="back preview" />
          </div>
        </div>
        <div class="actions">
          <button class="btn" :disabled="kycSending" @click="submitKyc">
            {{ kycSending ? 'Đang gửi...' : 'Gửi xác minh' }}
          </button>
        </div>
        <p class="ok" v-if="kycMsg">{{ kycMsg }}</p>
        <p class="err" v-if="kycErr">{{ kycErr }}</p>
      </template>

      <template v-else>
        <div class="verified-box">Tài khoản của bạn đã được xác minh ✅</div>
      </template>
    </div>
  </section>
</template>

<style scoped>
.wrap{ max-width: 900px; margin: 16px auto; padding: 0 12px; display:grid; gap:16px; }
.header{ display:flex; align-items:center; justify-content:space-between; }
.badge{ padding:6px 10px; border-radius:999px; font-weight:700; font-size:13px; }
.ok-badge{ background:#e6f7ef; color:#0b8a47; border:1px solid #b7ebc6; }
.warn-badge{ background:#fff7e6; color:#ad6800; border:1px solid #ffe7ba; }

.card{ border:1px solid #eee; border-radius:12px; background:#fff; padding:16px; display:grid; gap:12px; }
.grid{ display:grid; grid-template-columns: 180px 1fr; gap:10px; align-items:center; }
.row{ display:flex; gap:10px; align-items:center; flex-wrap:wrap; }
h2{ margin:4px 0 2px; }
h3{ margin:0 0 6px; }
input{ padding:10px; border:1px solid #ddd; border-radius:10px; width:100%; }
.avatar{ width:56px; height:56px; border-radius:12px; object-fit:cover; }
.kyc{ width:140px; height:90px; object-fit:cover; border-radius:8px; border:1px solid #eee; }
.actions{ display:flex; gap:8px; justify-content:flex-end; }
.btn{ height:38px; padding:0 14px; border:1px solid #ddd; border-radius:10px; background:#f7f7f7; cursor:pointer; }
.btn.primary{ background:#1e80ff; color:#fff; border-color:#1e80ff; }
.ok{ color:#16a34a; }
.err{ color:#d33; }
.hint{ color:#666; font-size:13px; }
.verified-box{
  background:#e6f7ef; border:1px solid #b7ebc6; color:#0b8a47;
  padding:12px; border-radius:10px; text-align:center; font-weight:700;
}
@media (max-width: 680px){
  .grid{ grid-template-columns: 1fr; }
  .actions{ justify-content:stretch; }
  .btn{ width:100%; }
}
</style>
