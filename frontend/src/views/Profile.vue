<template>
  <h2>Quản lý hồ sơ</h2>
  <form v-if="me" @submit.prevent="onSubmit">
    <label>Tên</label>
    <input v-model.trim="form.name" required minlength="2" />

    <label>SĐT</label>
    <input v-model.trim="form.phone" required minlength="8" />

    <label>Avatar URL</label>
    <input v-model.trim="form.avatarUrl" placeholder="https://..." />

    <label>Tải ảnh từ máy</label>
    <input type="file" accept="image/*" @change="onFileChange" />

    <div class="preview">
      <img :src="form.avatarUrl || placeholder" alt="preview" />
    </div>

    <button :disabled="loading">Lưu thay đổi</button>
    <p v-if="msg" class="ok">{{ msg }}</p>
    <p v-if="error" class="err">{{ error }}</p>
  </form>

  <p v-else>Đang tải dữ liệu người dùng…</p>
</template>

<script setup lang="ts">
import { reactive, ref, onMounted, computed } from 'vue'
import { updateProfile, uploadAvatar } from '../api'
import { currentUser, fetchCurrentUser } from '../auth'

const loading = ref(false)
const error = ref('')
const msg = ref('')
const me = computed(() => currentUser.value)

const placeholder =
  'data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="80" height="80"><rect width="100%" height="100%" fill="#eee"/></svg>'

const form = reactive({
  name: '',
  phone: '',
  avatarUrl: '',
})

onMounted(async () => {
  if (!me.value) await fetchCurrentUser()
  if (me.value) {
    form.name = me.value.name
    form.phone = me.value.phone
    form.avatarUrl = me.value.avatarUrl || ''
  }
})

async function onFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  const f = input.files?.[0]
  if (!f) return
  try {
    loading.value = true; error.value = ''
    const { data } = await uploadAvatar(f)
    // backend trả { url }, gán luôn vào form để preview
form.avatarUrl = data.url || (api.defaults.baseURL + data.path)
  } catch (err: any) {
    error.value = err?.response?.data?.error || 'Upload thất bại'
  } finally {
    loading.value = false
  }
}

async function onSubmit() {
  loading.value = true; error.value = ''; msg.value = ''
  try {
    await updateProfile({
      name: form.name,
      phone: form.phone,
      avatarUrl: form.avatarUrl,
    })
    await fetchCurrentUser()
    msg.value = 'Cập nhật thành công'
  } catch (e: any) {
    error.value = e?.response?.data?.error || 'Có lỗi xảy ra'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
label { display:block; margin-top:12px; }
input { width:100%; padding:8px; border:1px solid #ddd; border-radius:8px; }
button { margin-top:16px; padding:10px 12px; border:none; border-radius:8px; background:#1e80ff; color:#fff; cursor:pointer; }
.err { color:#d33; margin-top:8px; }
.ok { color:#2d7; margin-top:8px; }
.preview { margin-top:12px; }
.preview img { width:80px; height:80px; border-radius:50%; object-fit:cover; background:#eee; }
</style>
