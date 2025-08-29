<!-- src/views/vip/VipAdmin.vue -->
<template>
  <section class="wrap">
    <!-- Header -->
    <header class="heading">
      <h2>Quản lý thành viên VIP</h2>
    </header>

    <!-- Search toolbar -->
    <div class="toolbar">
      <div class="left">
        <label class="label">Tìm kiếm bằng</label>
        <div class="btn-group">
          <button
            class="btn"
            :class="{ primary: mode === 'level' }"
            @click="mode = 'level'"
          >
            Cấp bậc
          </button>
          <button
            class="btn"
            :class="{ primary: mode === 'nickname' }"
            @click="mode = 'nickname'"
          >
            Biệt danh
          </button>
        </div>

        <!-- Mode: level -->
        <div v-if="mode === 'level'" class="level-search">
          <label class="label small">Xem người bạn giới thiệu bằng Cấp bậc</label>
          <div class="row">
            <div class="select">
              <button class="btn select-btn" @click="openLevel = !openLevel">
                Cấp bậc {{ level }}
                <span class="caret">▾</span>
              </button>
              <ul v-if="openLevel" class="dropdown" @mouseleave="openLevel=false">
                <li v-for="l in [1,2,3,4,5]" :key="l" @click="pickLevel(l)">
                  Cấp bậc {{ l }}
                </li>
              </ul>
            </div>
            <button class="btn primary" @click="doSearch">Tìm kiếm</button>
          </div>
        </div>

        <!-- Mode: nickname -->
        <div v-else class="nick-search">
          <label class="label small">Nhập biệt danh</label>
          <div class="row">
            <input class="input" v-model.trim="nickname" placeholder="vd: dangpro" />
            <button class="btn primary" @click="doSearch">Tìm kiếm</button>
          </div>
        </div>
      </div>

      <!-- Date range -->
      <div class="right">
        <label class="label">Khoảng thời gian:</label>
        <div class="chips">
          <button class="chip" :class="{ on: datePreset==='today' }" @click="setPreset('today')">
            Hôm nay
          </button>
          <button class="chip" :class="{ on: datePreset==='yesterday' }" @click="setPreset('yesterday')">
            Hôm qua
          </button>
          <button class="chip" :class="{ on: datePreset==='7d' }" @click="setPreset('7d')">
            7 ngày
          </button>
          <button class="chip" :class="{ on: datePreset==='30d' }" @click="setPreset('30d')">
            30 ngày
          </button>
          <div class="custom-range">
            <input type="date" class="date" v-model="from" />
            <span class="sep">—</span>
            <input type="date" class="date" v-model="to" />
          </div>
        </div>
      </div>
    </div>

    <!-- Results -->
    <section class="results">
      <h3>Kết quả tìm kiếm</h3>

      <div class="table">
        <div class="thead">
          <div class="th col-nick">Biệt danh</div>
          <div class="th col-level">Cấp bậc</div>
          <div class="th col-vol">Tổng KLGD</div>
          <div class="th col-comm">HH Nhận</div>
        </div>

        <div v-if="rows.length === 0" class="empty">
          Không có dữ liệu
        </div>

        <div v-else class="tbody">
          <div class="tr" v-for="r in rows" :key="r.id">
            <div class="td col-nick">{{ r.nickname }}</div>
            <div class="td col-level">VIP {{ r.level }}</div>
            <div class="td col-vol">{{ formatNumber(r.volume) }}</div>
            <div class="td col-comm">{{ formatNumber(r.commission) }}</div>
          </div>
        </div>
      </div>
    </section>
  </section>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { adminSearchUsers } from '../../api'

type Mode = 'level' | 'nickname'
type Row = { id: number; nickname: string; level: number; volume: number; commission: number }

const mode = ref<Mode>('level')
const level = ref<number>(1)
const openLevel = ref(false)
const nickname = ref('')

const datePreset = ref<'today' | 'yesterday' | '7d' | '30d' | ''>('yesterday')
const from = ref<string>('') 
const to = ref<string>('')

const rows = ref<Row[]>([])
const loading = ref(false)
const error = ref<string>('')

function pickLevel(l: number) {
  level.value = l
  openLevel.value = false
}
function setPreset(p: typeof datePreset.value) { datePreset.value = p }

async function doSearch() {
  loading.value = true; error.value = ''
  try {
    const params: any = {}
    if (mode.value === 'level') params.vipLevel = level.value
    if (mode.value === 'nickname' && nickname.value) params.nickname = nickname.value

    const { data } = await adminSearchUsers(params)
    // Map dữ liệu về bảng (volume/commission tạm = 0)
    rows.value = (data.rows || []).map((u: any) => ({
      id: u.id,
      nickname: u.nickname,
      level: u.vipLevel,
      volume: u.totalTopup ?? 0,     // hoặc 0 nếu bạn muốn đúng nghĩa KLGD
      commission: 0,                 // chưa có dữ liệu => tạm 0
    }))
  } catch (e: any) {
    error.value = e?.response?.data?.error || 'Không tải được dữ liệu'
    rows.value = []
  } finally {
    loading.value = false
  }
}

function formatNumber(n: number) {
  return new Intl.NumberFormat('vi-VN').format(n)
}
</script>


<style scoped>
/* Theme tối “same same” */
:host, .wrap { color: #eaeef7; }
.wrap { display:flex; flex-direction:column; gap:18px; }
.heading { background:#141b2d; padding:18px 20px; border-radius:12px; }
.heading h2 { margin:0; font-size:28px; font-weight:800; }

.toolbar {
  background:#141b2d; border-radius:12px; padding:18px 20px;
  display:flex; gap:24px; justify-content:space-between; align-items:flex-start;
}
.left { display:flex; flex-direction:column; gap:12px; }
.right { display:flex; flex-direction:column; gap:10px; align-items:flex-end; }
.label { color:#b8c0d9; font-size:14px; }
.label.small { font-size:13px; }

.btn-group { display:flex; gap:12px; }
.btn {
  height:40px; padding:0 16px; border:1px solid #3a4769; border-radius:10px;
  background:#1a2340; color:#cfe0ff; cursor:pointer;
}
.btn:hover { background:#21305a; }
.btn.primary { background:#2f66ff; border-color:#2f66ff; color:#fff; }

.row { display:flex; gap:12px; align-items:center; margin-top:6px; }
.input {
  width:220px; height:40px; padding:0 12px; border-radius:10px;
  border:1px solid #3a4769; background:#0f1526; color:#eaeef7;
}

.select { position:relative; }
.select-btn { min-width:140px; display:flex; align-items:center; justify-content:space-between; gap:8px; }
.caret { font-size:12px; }
.dropdown {
  position:absolute; top:44px; left:0; width:160px; background:#0f1526; border:1px solid #3a4769;
  border-radius:10px; overflow:hidden; z-index:10;
}
.dropdown li { list-style:none; padding:10px 12px; cursor:pointer; color:#cfe0ff; }
.dropdown li:hover { background:#1a2340; }

.chips { display:flex; gap:8px; align-items:center; }
.chip {
  height:36px; padding:0 12px; border-radius:10px; background:#1a2340; color:#cfe0ff; border:1px solid #3a4769; cursor:pointer;
}
.chip.on, .chip:hover { background:#2f66ff; border-color:#2f66ff; color:#fff; }

.custom-range { display:flex; align-items:center; gap:6px; }
.date {
  height:36px; padding:0 10px; border-radius:10px; border:1px solid #3a4769;
  background:#0f1526; color:#eaeef7;
}
.sep { color:#8ea2d6; }

.results { background:#0e1424; border-radius:12px; padding:18px 20px; }
.results h3 { margin:0 0 12px; font-size:22px; font-weight:800; }

.table {
  border:1px solid #2a3556; border-radius:14px; overflow:hidden;
}
.thead, .tr {
  display:grid; grid-template-columns: 2fr 1fr 1.5fr 1.5fr;
}
.thead { background:#0b1020; color:#a9b6de; }
.th, .td { padding:12px 14px; border-bottom:1px solid #1c2544; }
.tbody .tr:hover { background:#0f152a; }
.empty {
  text-align:center; color:#98a6d7; padding:24px 12px;
}

/* responsive */
@media (max-width: 900px) {
  .toolbar { flex-direction:column; }
  .right { align-items:flex-start; }
  .thead, .tr { grid-template-columns: 1.5fr .8fr 1fr 1fr; }
}
</style>
