<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import api from '../api'

type ReferralInfo = { code: string; link: string; count: number; total: number }
type CommissionRow = {
  id: number; buyerEmail: string; depth: number; percent: number;
  amount: number; kind: 'UPLINE'|'ADMIN'; vipLevel: number; createdAt: string;
}

const info = ref<ReferralInfo | null>(null)
const commissions = ref<CommissionRow[]>([])
const loading = ref(true)
const copied = ref(false)

onMounted(async () => {
  const [inf, cm] = await Promise.all([
    api.referralInfo(),                  // code/link + thống kê đăng ký (cũ)
    api.commissionsHistory(),            // ⬅️ thêm: /private/history/commissions
  ])
  info.value = inf
  commissions.value = cm.rows || []
  loading.value = false
})

const vipCommissionTotal = computed(
  () => commissions.value.reduce((s, r) => s + (r.amount || 0), 0)
)
const recent5 = computed(() => commissions.value.slice(0, 5))

async function copyLink() {
  if (!info.value) return
  await navigator.clipboard.writeText(info.value.link)
  copied.value = true; setTimeout(()=>copied.value=false, 1200)
}
</script>

<template>
  <section class="wrap">
    <h2>Mời bạn bè</h2>
    <div class="card" v-if="loading">Đang tải...</div>

    <div class="card" v-else>
      <div class="row"><b>Mã giới thiệu của bạn:</b> <code>{{ info?.code }}</code></div>
      <div class="row">
        <b>Link mời:</b>
        <input class="link" :value="info?.link" readonly />
        <button class="btn" @click="copyLink">{{ copied ? 'Đã copy' : 'Copy' }}</button>
      </div>

      <!-- Thống kê đăng ký (cơ chế cũ, nếu bạn còn giữ) -->
      <div class="stats">
        <div><b>Lượt đăng ký qua link:</b> {{ info?.count ?? 0 }}</div>
        <div><b>Thưởng đăng ký (cũ):</b> {{ (info?.total ?? 0).toLocaleString() }} coin</div>
      </div>

      <!-- Thống kê hoa hồng VIP -->
      <div class="stats">
        <div><b>Tổng hoa hồng VIP đã nhận:</b> {{ vipCommissionTotal.toLocaleString() }} coin</div>
      </div>

      <div class="sub">
        <h3>Hoa hồng gần đây</h3>
        <ul class="list">
          <li v-for="r in recent5" :key="r.id">
            <b>{{ r.buyerEmail }}</b> mua VIP 
            bạn nhận <b>{{ r.amount.toLocaleString() }}</b> coin
            (tầng {{ r.depth }}, {{ r.percent }}%) ·
            <small>{{ new Date(r.createdAt).toLocaleString() }}</small>
          </li>
          <li v-if="recent5.length === 0">Chưa có hoa hồng.</li>
        </ul>
      </div>

      <p class="hint">
        Hoa hồng chỉ phát sinh khi người được bạn giới thiệu <b>mua VIP</b>.
        Người mua dùng link/mã của bạn (F1) thì khi F2/F3 mua VIP, các tầng trên nhận 10% theo mô hình 9 tầng.
      </p>
    </div>
  </section>
</template>

<style scoped>
.wrap{ max-width: 960px; margin:16px auto; padding:0 12px; }
.card{ border:1px solid #eee; border-radius:12px; padding:16px; display:grid; gap:12px; background:#fff; }
.row{ display:flex; align-items:center; gap:10px; flex-wrap:wrap; }
.link{ flex:1; min-width:260px; padding:8px; border:1px solid #ddd; border-radius:8px; }
.btn{ padding:8px 12px; border:1px solid #ddd; border-radius:8px; background:#f7f7f7; cursor:pointer; }
.stats{ display:grid; grid-template-columns: repeat(auto-fit,minmax(240px,1fr)); gap:8px; }
.sub h3{ margin:8px 0; }
.list{ margin:0; padding-left:18px; display:grid; gap:6px; }
.hint{ background:#f7f7f7; border-radius:10px; padding:10px; }
</style>
