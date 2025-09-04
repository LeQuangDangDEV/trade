<template>
  <div class="card">
    <h3>Quản trị hệ thống</h3>

    <!-- Bộ lọc -->
    <div class="filters">
      <select v-model="vipLevel" @change="load">
        <option value="">Tất cả</option>
        <option value="1">Chỉ VIP (level = 1)</option>
        <option value="0">Không VIP</option>
      </select>

      <input
        v-model.trim="username"
        placeholder="Tìm theo username"
        @keyup.enter="load"
      />
      <input
        v-model.trim="nickname"
        placeholder="(Tuỳ chọn) Tìm theo tên"
        @keyup.enter="load"
      />
      <button @click="load">Lọc</button>
    </div>

    <!-- Bảng kết quả -->
    <div v-if="loading">Đang tải...</div>
    <table v-else class="tbl">
      <thead>
        <tr>
          <th>ID</th>
          <th>Nickname</th>
          <th>Username</th>
          <th>VIP</th>
          <th>Tổng nạp</th>
          <th>Coins</th>
          <th>Thao tác</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="u in rows" :key="u.id">
          <td>{{ u.id }}</td>
          <td>{{ u.nickname }}</td>
          <td>@{{ u.username }}</td>
          <td>{{ u.vipLevel }}</td>
          <td>{{ u.totalTopup.toLocaleString() }}</td>
          <td>{{ u.coins.toLocaleString() }}</td>
          <td class="actions">
            <button @click="openDetail(u)">Xem</button>
            <button @click="pickTopup(u)">Nạp</button>
            <button @click="pickWithdraw(u)">Rút</button>
            <button class="danger" @click="delUser(u.id)">Xóa</button>
          </td>
        </tr>
        <tr v-if="!rows.length">
          <td colspan="7" style="text-align:center">Không có dữ liệu</td>
        </tr>
      </tbody>
    </table>

    <!-- Form thao tác (Nạp / Rút) -->
    <div v-if="selected" class="ops">
      <div class="sel">
        Đang thao tác với:
        <b>#{{ selected.id }}</b> — <code>@{{ selected.username }}</code>
        <button class="mini" @click="clearSelection">Bỏ chọn</button>
      </div>

      <div class="tabs">
        <button
          :class="['tab', action==='topup' && 'active']"
          @click="action='topup'"
        >
          Nạp coin
        </button>
        <button
          :class="['tab', action==='withdraw' && 'active']"
          @click="action='withdraw'"
        >
          Rút coin
        </button>
      </div>

      <!-- Nạp coin -->
      <div v-if="action==='topup'" class="formline">
        <input
          v-model.number="topupAmount"
          type="number"
          min="1"
          placeholder="Số coin nạp"
        />
        <input v-model="topupNote" placeholder="Ghi chú (tuỳ chọn)" />
        <button @click="doTopup">Xác nhận nạp</button>
      </div>

      <!-- Rút coin -->
      <div v-if="action==='withdraw'" class="formline">
        <input
          v-model.number="withdrawAmount"
          type="number"
          min="1"
          placeholder="Số coin rút"
        />
        <input v-model="withdrawNote" placeholder="Ghi chú (tuỳ chọn)" />
        <button @click="doWithdraw">Xác nhận rút</button>
      </div>
    </div>

    <p class="ok" v-if="msg">{{ msg }}</p>
    <p class="err" v-if="err">{{ err }}</p>
  </div>

  <!-- Modal chi tiết -->
  <div v-if="showDetail" class="modal-backdrop" @click.self="closeDetail">
    <div class="modal">
      <div class="mh">
        <b>Chi tiết người dùng</b>
        <button class="x" @click="closeDetail">✕</button>
      </div>
      <div class="mbody">
        <div v-if="detailLoading">Đang tải...</div>
        <div v-else-if="detailErr" class="err">{{ detailErr }}</div>
        <div v-else-if="ud" class="grid">
          <div class="label">ID</div><div>{{ ud.id }}</div>
          <div class="label">Username</div><div>@{{ ud.username }}</div>
          <div class="label">Biệt danh</div><div>{{ ud.name || '-' }}</div>
          <div class="label">SĐT</div><div>{{ ud.phone || '-' }}</div>
          <div class="label">Vai trò</div><div>{{ ud.role }}</div>
          <div class="label">VIP</div><div>VIP {{ ud.vipLevel }}</div>
          <div class="label">Tổng nạp</div><div>{{ ud.totalTopup.toLocaleString() }}</div>
          <div class="label">Số dư</div><div>{{ ud.coins.toLocaleString() }}</div>

          <div class="label">KYC</div>
          <div>
            <span class="tag" :class="(ud.kycStatus || 'none').toLowerCase()">
              {{ ud.kycStatus || 'NONE' }}
            </span>
            <div class="kyc-info" v-if="ud.kycFullName || ud.kycDob || ud.kycNumber">
              <div><b>Họ và tên:</b> {{ ud.kycFullName || '-' }}</div>
              <div><b>Ngày sinh:</b> {{ ud.kycDob || '-' }}</div>
              <div><b>Số CCCD:</b> {{ ud.kycNumber || '-' }}</div>
            </div>
            <div class="kyc-pics">
              <figure v-if="ud.hasKycFront">
                <img :src="kycFrontURL" alt="KYC front" />
                <figcaption>Mặt trước</figcaption>
              </figure>
              <figure v-if="ud.hasKycBack">
                <img :src="kycBackURL" alt="KYC back" />
                <figcaption>Mặt sau</figcaption>
              </figure>
              <div v-if="!ud.hasKycFront && !ud.hasKycBack" class="muted">Không có ảnh KYC</div>
            </div>
          </div>
        </div>
      </div>
      <div class="mf">
        <button class="btn" @click="closeDetail">Đóng</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import api, { type AdminUserRow, type AdminUserDetail } from '../../api'

/* ====== filters / list ====== */
const vipLevel = ref<string>('')   // '' | '1' | '0'
const nickname = ref<string>('')   // tìm theo tên
const username = ref<string>('')   // tìm theo username
const loading  = ref(false)
const rows     = ref<AdminUserRow[]>([])
const msg = ref(''); const err = ref('')

async function load() {
  loading.value = true; err.value=''
  try {
    const r = await api.adminUsers({
      vipLevel: vipLevel.value || undefined,
      nickname: nickname.value || undefined,
      username: username.value || undefined,
    })
    rows.value = r.rows || []
  } catch(e:any){
    err.value = e?.message || 'Tải danh sách thất bại'
  } finally {
    loading.value = false
  }
}

/* ====== selection & actions (Topup/Withdraw/Delete) ====== */
const selected = ref<{ id:number; username:string } | null>(null)
const action = ref<'topup'|'withdraw'|''>('')

const topupAmount = ref<number>(0)
const topupNote   = ref<string>('')

const withdrawAmount = ref<number>(0)
const withdrawNote   = ref<string>('')

function pickTopup(u: AdminUserRow) {
  selected.value = { id: u.id, username: u.username }
  action.value = 'topup'
  topupAmount.value = 0; topupNote.value = ''
}
function pickWithdraw(u: AdminUserRow) {
  selected.value = { id: u.id, username: u.username }
  action.value = 'withdraw'
  withdrawAmount.value = 0; withdrawNote.value = ''
}
function clearSelection() {
  selected.value = null
  action.value = ''
  topupAmount.value = 0
  topupNote.value = ''
  withdrawAmount.value = 0
  withdrawNote.value = ''
}

async function doTopup() {
  err.value=''; msg.value=''
  if (!selected.value || topupAmount.value <= 0) {
    err.value = 'Chọn người dùng và nhập số coin > 0'
    return
  }
  try {
    const r = await api.adminTopup({
      userId: selected.value.id,
      amount: topupAmount.value,
      note: topupNote.value
    })
    msg.value = r.message || 'Nạp coin thành công'
    await load()
    clearSelection()
  } catch (e:any) {
    err.value = e.message || 'Nạp thất bại'
  }
}

async function doWithdraw() {
  err.value=''; msg.value=''
  if (!selected.value || withdrawAmount.value <= 0) {
    err.value = 'Chọn người dùng và nhập số coin > 0'
    return
  }
  try {
    const r = await api.adminWithdraw({
      userId: selected.value.id,
      amount: withdrawAmount.value,
      note: withdrawNote.value
    })
    msg.value = r.message || 'Rút coin thành công'
    await load()
    clearSelection()
  } catch (e:any) {
    err.value = e.message || 'Rút thất bại'
  }
}

async function delUser(id:number){
  if (!confirm('Xóa tài khoản này?')) return
  msg.value=''; err.value=''
  try {
    await api.adminDeleteUser(id)
    msg.value='Đã xóa'
    await load()
  } catch(e:any) {
    err.value = e.message || 'Xóa thất bại'
  }
}

/* ====== detail modal (view profile + KYC) ====== */
const showDetail = ref(false)
const detailLoading = ref(false)
const detailErr = ref('')
const ud = ref<AdminUserDetail | null>(null)
const kycFrontURL = ref<string>('') // ObjectURL
const kycBackURL  = ref<string>('')

function revokeKycURLs(){
  if (kycFrontURL.value){ URL.revokeObjectURL(kycFrontURL.value); kycFrontURL.value='' }
  if (kycBackURL.value){  URL.revokeObjectURL(kycBackURL.value);  kycBackURL.value='' }
}

function closeDetail(){
  showDetail.value = false
  detailErr.value = ''
  ud.value = null
  revokeKycURLs()
}

async function openDetail(row: AdminUserRow){
  showDetail.value = true
  detailLoading.value = true
  detailErr.value = ''
  ud.value = null
  revokeKycURLs()
  try {
    const r = await api.adminUserDetail(row.id)
    ud.value = r.user
    if (r.user.hasKycFront) {
      try { kycFrontURL.value = await api.adminKycImage(row.id, 'front') } catch {}
    }
    if (r.user.hasKycBack) {
      try { kycBackURL.value = await api.adminKycImage(row.id, 'back') } catch {}
    }
  } catch(e:any){
    detailErr.value = e?.message || 'Không tải được chi tiết'
  } finally {
    detailLoading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.card{ border:1px solid #eee; border-radius:12px; padding:16px; display:grid; gap:12px; background:#fff; }
.filters{ display:flex; gap:8px; flex-wrap:wrap; }
.tbl{ width:100%; border-collapse:collapse; background:#fff; }
.tbl th, .tbl td{ border-top:1px solid #f0f0f0; padding:8px; text-align:left; }
.actions button{ margin-right:6px; }
input, select{ padding:8px 10px; border:1px solid #ddd; border-radius:10px; }
button{ padding:8px 12px; border:1px solid #ddd; border-radius:10px; background:#f7f7f7; cursor:pointer; }
button:hover{ background:#efefef; }
.danger{ border-color:#fca5a5; background:#fee2e2; }
.ok{ color:#16a34a; } .err{ color:#d33; }

.ops{ margin-top:8px; display:grid; gap:10px; }
.sel{ background:#f7f7f7; border-radius:8px; padding:6px 10px; display:flex; align-items:center; gap:8px; flex-wrap:wrap; }
.mini{ padding:4px 8px; border:1px solid #ddd; border-radius:8px; background:#fff; cursor:pointer; }
.tabs{ display:flex; gap:8px; }
.tab{ padding:8px 12px; border:1px solid #ddd; border-radius:10px; background:#fafafa; cursor:pointer; }
.tab.active{ background:#e8f0ff; border-color:#c6dbff; }
.formline{ display:flex; gap:8px; align-items:center; flex-wrap:wrap; }

/* modal */
.modal-backdrop{ position:fixed; inset:0; background:rgba(0,0,0,.45); display:flex; align-items:center; justify-content:center; z-index:1000; }
.modal{ width:min(760px,96vw); background:#fff; border-radius:14px; box-shadow:0 20px 60px rgba(0,0,0,.25); overflow:hidden; display:flex; flex-direction:column; }
.mh{ display:flex; align-items:center; justify-content:space-between; padding:10px 12px; border-bottom:1px solid #eee; }
.x{ width:32px; height:32px; border:1px solid #ddd; background:#fff; border-radius:8px; cursor:pointer; }
.mbody{ padding:12px; max-height:70vh; overflow:auto; }
.mf{ padding:10px 12px; border-top:1px solid #eee; display:flex; justify-content:flex-end; }
.grid{ display:grid; grid-template-columns: 180px 1fr; gap:8px; }
.label{ color:#555; }
.tag{ display:inline-block; padding:2px 8px; border-radius:999px; border:1px solid #ddd; font-size:12px; }
.tag.verified,.tag.approved{ border-color:#16a34a; color:#16a34a; }
.kyc-info{ margin-top:6px; display:grid; gap:2px; }
.kyc-pics{ display:flex; gap:12px; margin-top:8px; flex-wrap:wrap; }
figure{ margin:0; }
figure img{ width:220px; height:140px; object-fit:cover; border:1px solid #eee; border-radius:8px; }
.muted{ color:#777; }
@media (max-width:700px){ .grid{ grid-template-columns: 1fr; } }
</style>
