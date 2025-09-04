<template>
  <div v-if="loading" class="card">Đang tải...</div>

  <div v-else class="card">
    <div class="head">
      <h3>VIP</h3>
      <div class="actions">
        <button class="btn" @click="openTopup">Nạp coin</button>
      </div>
    </div>

    <div class="grid kpis">
      <div><b>Trạng thái:</b> <span>{{ isVip ? 'Đang là VIP' : 'Chưa là VIP' }}</span></div>
      <div><b>Số dư:</b> <span>{{ wallet?.coins ?? 0 }}</span></div>
      <div><b>Giá VIP:</b> <span>{{ vipPrice }}</span></div>
    </div>

    <div v-if="!isVip" class="note">
      Hệ thống chỉ có <b>một loại VIP</b>. Mua 1 lần để trở thành VIP.
    </div>

    <div class="cta">
      <button
        class="btn primary"
        :disabled="isVip || buying || !canBuy"
        @click="onBuyVip"
      >
        {{ isVip ? 'Bạn đã là VIP' : (buying ? 'Đang xử lý...' : 'Mua VIP') }}
      </button>
      <small v-if="!isVip && !canBuy" class="warn">
        Số dư chưa đủ. Vui lòng nạp coin.
      </small>
    </div>

    <p v-if="msg" class="ok">{{ msg }}</p>
    <p v-if="err" class="err">{{ err }}</p>
  </div>

  <!-- ===== Modal QR Nạp coin ===== -->
  <div v-if="showTopup" class="backdrop" @click.self="closeTopup">
    <div class="modal">
      <div class="mhead">
        <h3>Nạp coin</h3>
        <button class="x" @click="closeTopup">✕</button>
      </div>

      <div class="mbody">
        <div class="qrbox">
          <img :src="bank.qrUrl" alt="QR chuyển khoản" />
        </div>

        <div class="fields">
          <div class="row">
            <label>Ngân hàng</label>
            <div class="val">{{ bank.name }}</div>
          </div>
          <div class="row">
            <label>Số tài khoản</label>
            <div class="val">
              {{ bank.account }}
              <button class="mini" @click="copyText(bank.account,'account')">
                {{ copied==='account' ? 'Đã copy' : 'Copy' }}
              </button>
            </div>
          </div>
          <div class="row">
            <label>Chủ tài khoản</label>
            <div class="val">{{ bank.owner }}</div>
          </div>
          <div class="row">
            <label>Nội dung chuyển khoản</label>
            <div class="val">
              {{ meEmail || 'email-đăng-ký-của-bạn' }}
              <button class="mini" @click="copyText(meEmail || '', 'email')" :disabled="!meEmail">
                {{ copied==='email' ? 'Đã copy' : 'Copy' }}
              </button>
            </div>
          </div>
        </div>

        <div class="hint">
          Ghi đúng <b>email đăng ký</b> ở phần nội dung chuyển khoản để hệ thống nạp coin cho bạn.
        </div>

        <div class="actions">
          <button class="btn" @click="closeTopup">Đã chuyển / Để sau</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import api from '../../api'
import { currentUser, fetchCurrentUser } from '../../auth'

const loading = ref(true)
const buying = ref(false)
const msg = ref('')
const err = ref('')

const wallet = ref<{ coins:number; totalTopup:number; vipLevel:number } | null>(null)
const tiers  = ref<Array<{ level:number; name:string; minTopup:number }>>([])

// Giá VIP hiển thị: lấy từ vip_tiers level=1 nếu có, fallback 10000
const price = computed(() => {
  const t1 = tiers.value.find(t => t.level === 1)
  return t1?.minTopup ?? 10000
})
const vipPrice = computed(() => price.value.toLocaleString() + ' coin')

const isVip = computed(() => (wallet.value?.vipLevel ?? 0) >= 1)
const canBuy = computed(() => !isVip.value && (wallet.value?.coins ?? 0) >= price.value)

const route = useRoute()
const router = useRouter()
const meEmail = computed(() => currentUser.value?.email || '')

async function load() {
  const [w, t] = await Promise.all([ api.wallet(), api.vipTiers().catch(()=>({tiers:[]})) ])
  wallet.value = w
  tiers.value = (t?.tiers ?? []).sort((a,b)=>a.level-b.level)
  loading.value = false
}

async function onBuyVip(){
  if (!canBuy.value || buying.value) return
  buying.value = true; msg.value = ''; err.value = ''
  try{
    await api.buyVip({}) // BE bỏ qua body; có thể rỗng
    await Promise.all([ load(), fetchCurrentUser() ])
    msg.value = 'Mua VIP thành công!'
  }catch(e:any){
    err.value = e?.message || 'Mua VIP thất bại'
  }finally{
    buying.value = false
  }
}

/** ====== Modal Nạp coin (QR) ====== */
const bank = {
  qrUrl:  '/qrcode.jpg', // ảnh trong public/
  name:   (import.meta as any).env.VITE_BANK_NAME    ?? 'Ngân hàng MB BANK',
  account:(import.meta as any).env.VITE_BANK_ACCOUNT ?? '0123456789',
  owner:  (import.meta as any).env.VITE_BANK_OWNER   ?? 'NGUYEN VAN A',
}
const showTopup = ref(false)
const copied = ref<'email'|'account'|''>('')

function openTopup(){ showTopup.value = true; copied.value='' }
function closeTopup(){ showTopup.value = false }
async function copyText(t: string, k:'email'|'account'){
  try{ await navigator.clipboard.writeText(t); copied.value = k; setTimeout(()=>copied.value='',1200) }catch{}
}

onMounted(async ()=>{
  await load()
  // mở modal nạp khi có ?topup=1
  if (route.query.topup === '1') {
    showTopup.value = true
    const q = { ...route.query } as any; delete q.topup
    router.replace({ query: q })
  }
})
</script>

<style scoped>
.card{ border:1px solid #eee; border-radius:12px; padding:16px; display:grid; gap:12px; background:#fff; }
.head{ display:flex; align-items:center; justify-content:space-between; gap:10px; }
.kpis{ display:grid; grid-template-columns: repeat(auto-fit,minmax(220px,1fr)); gap:8px; }
.note{ background:#f6f7f9; border-radius:8px; padding:10px; }
.cta{ display:flex; align-items:center; gap:10px; }
.btn{ height:36px; padding:0 14px; border:1px solid #ddd; border-radius:10px; background:#f7f7f7; cursor:pointer; }
.btn.primary{ background:#1e80ff; color:#fff; border-color:#1e80ff; }
.warn{ color:#b45309; }
.ok { color:#16a34a; }
.err { color:#d33; }

/* Modal */
.backdrop{ position:fixed; inset:0; background:rgba(0,0,0,.35); display:flex; align-items:center; justify-content:center; z-index:1000; }
.modal{ width:min(720px, 96vw); background:#fff; border-radius:14px; box-shadow:0 10px 30px rgba(0,0,0,.15); overflow:hidden; }
.mhead{ display:flex; align-items:center; justify-content:space-between; padding:12px 14px; border-bottom:1px solid #eee; }
.mhead .x{ background:#fff; border:1px solid #ddd; border-radius:8px; width:32px; height:32px; cursor:pointer; }
.mbody{ display:grid; grid-template-columns: 260px 1fr; gap:16px; padding:16px; }
.qrbox{ display:flex; align-items:center; justify-content:center; background:#f8fafc; border-radius:12px; padding:10px; }
.qrbox img{ width:240px; height:240px; object-fit:contain; }
.fields{ display:grid; gap:10px; }
.row{ display:grid; grid-template-columns: 160px 1fr; align-items:center; gap:10px; }
.row .val{ display:flex; align-items:center; gap:8px; }
.mini{ padding:4px 8px; border:1px solid #ddd; border-radius:8px; background:#f7f7f7; cursor:pointer; }
.hint{ grid-column:1 / -1; background:#f7f7f7; border-radius:10px; padding:10px; }
.actions{ grid-column:1 / -1; display:flex; justify-content:flex-end; }
@media (max-width: 720px){
  .mbody{ grid-template-columns: 1fr; }
  .row{ grid-template-columns: 1fr; }
}
</style>
