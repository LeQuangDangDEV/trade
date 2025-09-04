<template>
  <section class="wrap">
    <h1 class="title">Thợ Săn Kho Báu</h1>

    <!-- Khu mở rương -->
    <div class="board">
      <div
        v-for="i in 20"
        :key="i"
        class="chest"
        @click="openOnce"
        :class="{ disabled: opening }"
        title="Mở 1 lần (50 coin)"
      >
        <img src="/chest.jpg" alt="chest" />
      </div>
    </div>

    <p class="hint">
      Mỗi lần mở tốn <b>50 coin</b>. Phần thưởng hiếm: <b>100 coin</b> và
      <b>Ngọc Rồng (DB1..DB7)</b>.
    </p>

    <div class="cols">
      <!-- Túi đồ -->
      <div class="card">
        <h3>Túi đồ</h3>
        <div class="bag">
          <div v-for="n in 7" :key="n" class="bag-item">
            <span class="badge">DB{{ n }}</span>
            <span class="qty">x{{ inv['DB' + n] || 0 }}</span>
            <button class="mini" @click="listOne('DB' + n)" :disabled="!inv['DB' + n]">
              Đăng bán
            </button>
          </div>
        </div>
        <button class="btn" :disabled="!canMerge" @click="merge">
          Hợp nhất đủ 7 viên (+5000)
        </button>
      </div>

      <!-- Chợ -->
      <div class="card">
        <h3>Chợ</h3>

        <!-- Đăng bán nhanh -->
        <div class="market-row">
          <select v-model="sell.code">
            <option v-for="n in 7" :key="n" :value="'DB' + n">DB{{ n }}</option>
          </select>
          <input v-model.number="sell.qty" type="number" min="1" placeholder="Số lượng" />
          <input v-model.number="sell.price" type="number" min="1" placeholder="Giá / viên" />
          <button class="btn" @click="createListing">Đăng bán</button>
        </div>

        <!-- Lọc -->
        <div class="market-row">
          <select v-model="filterCode" @change="loadMarket">
            <option value="">Tất cả</option>
            <option v-for="n in 7" :key="n" :value="'DB' + n">DB{{ n }}</option>
          </select>
          <button class="btn" @click="loadMarket">Làm mới</button>
        </div>

        <!-- Bảng chợ -->
        <table class="tbl">
          <thead>
            <tr>
              <th>Mã</th>
              <th>SL</th>
              <th>Giá/viên</th>
              <th>Người bán</th>
              <th>Thao tác</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in market" :key="r.id">
              <td>{{ r.code }}</td>
              <td>{{ r.qty }}</td>
              <td>{{ r.pricePerUnit }}</td>
              <td>{{ r.sellerEmail }}</td>

              <!-- Nếu là của mình: hiện nút Rút lại -->
              <td class="act" v-if="r.sellerId === currentUser?.id">
                <!-- Rút toàn bộ -->
                <button class="mini" @click="withdrawListing(r)">Rút tất cả</button>
                <!-- Hoặc rút 1 phần (tuỳ chọn): -->
                <!--
                <input v-model.number="r.withdrawQty" type="number" :max="r.qty" min="1" style="width:80px" />
                <button class="mini" @click="withdrawListing(r, r.withdrawQty || 1)">Rút SL</button>
                -->
              </td>

              <!-- Không phải của mình: có thể mua -->
              <td class="act" v-else>
                <input v-model.number="r.buyQty" type="number" :max="r.qty" min="1" />
                <button class="mini" @click="buy(r)">Mua</button>
              </td>
            </tr>
            <tr v-if="!market.length">
              <td colspan="5" style="text-align: center">Chưa có bài đăng</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Modal thông báo phần thưởng -->
    <div v-if="rewardModal" class="rb-backdrop" @click.self="closeReward">
      <div class="rb-modal">
        <div class="rb-header">
          <h3>Kết quả mở hộp</h3>
          <button class="rb-x" @click="closeReward">✕</button>
        </div>

        <div class="rb-body">
          <!-- Khi là coin -->
          <div v-if="rewardModal.kind === 'COIN'" class="rb-center">
            <div class="rb-coin">+{{ rewardModal.amount.toLocaleString() }} coin</div>
          </div>

          <!-- Khi là ngọc rồng -->
          <div v-else class="rb-center">
            <div class="rb-ball">
              <!-- Nếu có ảnh, để vào /public/dragonballs/DB1.png ... DB7.png -->
              <img :src="ballImg(rewardModal.code)" @error="onImgError" alt="Dragon Ball" />
              <span class="rb-ball-code">{{ rewardModal.code }}</span>
            </div>
            <div class="rb-note">Bạn nhận được 1 viên {{ rewardModal.code }}</div>
          </div>
        </div>

        <div class="rb-actions">
          <button class="btn primary" @click="closeReward">Đóng</button>
        </div>
      </div>
    </div>

    <p class="ok" v-if="msg">{{ msg }}</p>
    <p class="err" v-if="error">{{ error }}</p>
  </section>
</template>


<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import api, { type InventoryItem, type MarketRow } from '../api'
import { currentUser, fetchCurrentUser, getToken } from '../auth'
import { ensureAuthOpen } from '../panelAuth'
import { addNotif } from '../notify'

/* ------------ state ------------ */
const opening = ref(false)
const inv = ref<Record<string, number>>({})
const market = ref<(MarketRow & { buyQty?: number })[]>([])
const msg = ref(''); const error = ref('')

const sell = ref<{ code: string; qty: number; price: number }>({ code: 'DB1', qty: 1, price: 100 })
const filterCode = ref<string>('')

const authed = computed(() => !!getToken())
const meId = computed(() => currentUser.value?.id ?? null)
const coins = computed(() => currentUser.value?.coins ?? 0)
const canMerge = computed(() => [1,2,3,4,5,6,7].every(n => (inv.value['DB'+n] || 0) > 0))

/* ------------ reward modal ------------ */
type Reward = { kind:'COIN'|'DRAGON_BALL'; code?:string; amount:number }
const rewardModal = ref<Reward|null>(null)
function closeReward(){ rewardModal.value = null }
function ballImg(code?: string){ return code ? `/dragonballs/${code}.png` : '' }
function onImgError(e: Event){ (e.target as HTMLImageElement).style.display = 'none' }

/* ------------ helpers ------------ */
function needAuth(): boolean {
  if (!authed.value) {
    error.value = 'Bạn cần đăng nhập để sử dụng tính năng này'
    ensureAuthOpen('login')
    return true
  }
  return false
}
function handleAuth(e: any): boolean {
  const m = String(e?.message || '')
  if (m.includes('Missing token') || m.includes('401') || /unauthor/i.test(m)) {
    return needAuth()
  }
  return false
}

/* ------------ bag / chest ------------ */
async function refreshBag() {
  if (!authed.value) return
  try {
    const r = await api.inventory()
    const next: Record<string, number> = {}
    // chịu được cả dạng "code/qty" và "Code/Qty"
    for (const it of (r.items as any[])) {
      const code = (it.code ?? it.Code) as string
      const qty  = (it.qty  ?? it.Qty ) as number
      if (code) next[code] = qty || 0
    }
    inv.value = next
  } catch (e:any) {
    if (!handleAuth(e)) error.value = e?.message || 'Không tải được túi đồ'
  }
}


async function openOnce() {
  if (opening.value) return
  if (needAuth()) return

  opening.value = true; msg.value=''; error.value=''
  if (coins.value < 50) {
    opening.value = false
    error.value = 'Số dư không đủ (cần 50 coin để mở rương).'
    return
  }

  try {
    const r = await api.chestOpen()
    await fetchCurrentUser() // cập nhật số dư navbar
    inv.value = r.inv || inv.value
    rewardModal.value = { kind: r.result, code: r.code, amount: r.amount } // hiện modal
   msg.value = r.result === 'COIN'
    ? `Bạn nhận được +${r.amount} coin`
    : `Bạn nhận được 1 viên ${r.code}`

  // đẩy thông báo lên navbar (1 thông báo tùy theo kết quả)
  if (r.result === 'COIN') {
    addNotif({ title: 'Phần thưởng', body: `+${r.amount} coin từ rương!` })
  } else {
    addNotif({ title: 'Phần thưởng', body: `Bạn nhận được 1 viên ${r.code}.` })
  }
  } catch (e:any) {
    if (!handleAuth(e)) error.value = e?.message || 'Mở rương thất bại'
  } finally {
    opening.value = false
  }
}

async function merge() {
  if (needAuth()) return
  msg.value=''; error.value=''
  try {
    const r = await api.mergeDragon()
    await fetchCurrentUser()
    await refreshBag()
    msg.value = r.message || 'Hợp nhất thành công'
  } catch (e:any) {
    if (!handleAuth(e)) error.value = e?.message || 'Hợp nhất thất bại'
  }
}

/* ------------ market ------------ */
function listOne(code:string){
  sell.value.code = code
  sell.value.qty  = 1
  sell.value.price = 100
}

async function createListing() {
  if (needAuth()) return
  msg.value=''; error.value=''

  if (!sell.value.code || sell.value.qty <= 0 || sell.value.price <= 0) {
    error.value = 'Vui lòng nhập mã, số lượng và giá hợp lệ.'
    return
  }
  const have = inv.value[sell.value.code] || 0
  if (have < sell.value.qty) {
    error.value = `Bạn chỉ có ${have} ${sell.value.code} trong túi.`
    return
  }

  try {
    await api.marketCreate({ code: sell.value.code, qty: sell.value.qty, pricePerUnit: sell.value.price })
    await refreshBag()
    await loadMarket()
    msg.value = 'Đăng bán thành công'
  } catch (e:any) {
    if (!handleAuth(e)) error.value = e?.message || 'Đăng bán thất bại'
  }
}

async function loadMarket() {
  try{
    const r = await api.marketList(filterCode.value || undefined)
    market.value = (r.rows || []).map((x:MarketRow)=>({ ...x, buyQty: 1 }))
  }catch(e:any){
    // market là public, không bật login
    error.value = e?.message || 'Tải chợ thất bại'
  }
}

async function buy(row: MarketRow & { buyQty?: number }) {
  if (needAuth()) return
  msg.value=''; error.value=''
  const qty = Number(row.buyQty || 1)
  if (qty <= 0) { error.value = 'Số lượng mua phải > 0'; return }
  if (row.sellerId === meId.value) {
    error.value = 'Bạn không thể mua chính sản phẩm mình đăng bán. Hãy dùng nút Rút về.'
    return
  }

  try {
    await api.marketBuy({ listingId: row.id, qty })
    await fetchCurrentUser()
    await refreshBag()
    await loadMarket()
    msg.value = 'Mua thành công'
  } catch (e:any) {
    if (!handleAuth(e)) error.value = e?.message || 'Mua thất bại'
  }
}

async function withdrawListing(row: MarketRow, qty?: number) {
  if (needAuth()) return
  msg.value=''; error.value=''
  try{
    await api.marketWithdraw({ listingId: row.id, qty })
    await refreshBag()
    await loadMarket()
    msg.value = 'Đã rút lại sản phẩm về túi'
  }catch(e:any){
    if (!handleAuth(e)) error.value = e?.message || 'Rút lại thất bại'
  }
}

/* ------------ lifecycle ------------ */
onMounted(async ()=>{
  // luôn load chợ (public)
  await loadMarket()
  // chỉ load túi nếu đã đăng nhập
  if (authed.value) await refreshBag()
})

// khi đăng nhập/đăng xuất thay đổi -> cập nhật túi
watch(() => currentUser.value?.id, async (id) => {
  if (id) {
    await fetchCurrentUser().catch(()=>{})
    await refreshBag()
  } else {
    inv.value = {}
  }
})

watch(filterCode, () => { loadMarket() })
</script>



<style scoped>
.wrap{ max-width: 1100px; margin: 0 auto; padding: 16px; }
.title{ text-align:center; margin: 4px 0 12px; text-transform:uppercase; }
.board{
  display:grid; grid-template-columns: repeat(5, 90px);
  gap:14px; justify-content:center; padding:16px; border-radius:16px;
  background:rgba(0,0,0,.04);
}
.chest{ width:90px; height:90px; border-radius:12px; display:flex; align-items:center; justify-content:center; background:rgba(255,255,255,.7); cursor:pointer; box-shadow:0 1px 3px rgba(0,0,0,.06); }
.chest img{ width:64px; height:64px; }
.chest.disabled{ opacity:.6; cursor:default; }

.hint{ text-align:center; color:#666; }

.cols{ display:grid; grid-template-columns: 1fr 1fr; gap:16px; margin-top:14px; }
.card{ border:1px solid #eee; border-radius:12px; padding:12px; background:#fff; display:grid; gap:10px; }
.bag{ display:grid; grid-template-columns: repeat(auto-fit,minmax(120px,1fr)); gap:8px; }
.bag-item{ display:flex; align-items:center; justify-content:space-between; background:#f7f7f7; border-radius:10px; padding:6px 8px; }
.badge{ font-weight:700; }
.qty{ color:#555; }
.btn{ padding:8px 12px; border:1px solid #ddd; border-radius:10px; background:#f7f7f7; cursor:pointer; }
.btn:disabled{ opacity:.6; cursor:default; }
.mini{ padding:6px 10px; border:1px solid #ddd; border-radius:8px; background:#fff; cursor:pointer; }
.market-row{ display:flex; gap:8px; flex-wrap:wrap; }
.tbl{ width:100%; border-collapse:collapse; }
.tbl th,.tbl td{ border-top:1px solid #f1f1f1; padding:8px; text-align:left; }
.tbl .act{ display:flex; gap:6px; align-items:center; }
.ok{ color:#16a34a; }
.err{ color:#d33; }
@media (max-width: 860px){
  .cols{ grid-template-columns: 1fr; }
}
/* Reward modal */
.rb-backdrop{
  position:fixed; inset:0; background:rgba(0,0,0,.45);
  display:flex; align-items:center; justify-content:center; z-index:1000;
}
.rb-modal{
  width:min(520px,96vw); background:#fff; border-radius:16px;
  box-shadow:0 20px 60px rgba(0,0,0,.25); overflow:hidden;
}
.rb-header{
  display:flex; align-items:center; justify-content:space-between;
  padding:12px 14px; border-bottom:1px solid #eee;
}
.rb-x{
  background:#fff; border:1px solid #ddd; border-radius:8px; width:32px; height:32px; cursor:pointer;
}
.rb-body{ padding:20px 16px; }
.rb-center{ display:grid; place-items:center; gap:12px; text-align:center; }
.rb-coin{
  font-size:28px; font-weight:800;
}
.rb-ball{
  display:grid; place-items:center; gap:8px;
}
.rb-ball img{
  width:160px; height:160px; object-fit:contain;
}
.rb-ball-code{
  font-weight:700; color:#f59e0b;
}
.rb-note{ color:#555; }
.rb-actions{
  padding:12px 14px; border-top:1px solid #eee; display:flex; justify-content:flex-end; gap:8px;
}

/* Nút dùng chung của app (nếu chưa có) */
.btn{ height:36px; padding:0 14px; border:1px solid #ddd; border-radius:10px; background:#f7f7f7; cursor:pointer; }
.btn.primary{ background:#1e80ff; color:#fff; border-color:#1e80ff; }

</style>
