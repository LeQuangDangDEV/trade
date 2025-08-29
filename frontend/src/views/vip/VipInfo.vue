<template>
  <div>
    <h3>Thông tin VIP của bạn</h3>

    <div v-if="loading">Đang tải…</div>
    <div v-else>
      <p><strong>Số dư:</strong> {{ wallet.coins }} coin</p>
      <p><strong>Tổng đã nạp:</strong> {{ wallet.totalTopup }} coin</p>
      <p><strong>Cấp VIP hiện tại:</strong> {{ wallet.vipLevel }}</p>

      <div v-if="wallet.vipLevel === 0" class="callout">Bạn chưa phải là VIP. Hãy mua VIP!</div>

      <h4 style="margin-top:16px">Các cấp VIP</h4>
      <ul>
        <li v-for="t in tiers" :key="t.level">
          {{ t.name }} — cần tổng nạp ≥ {{ t.minTopup }} coin
        </li>
      </ul>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { getVipTiers, getWallet } from '../../api'

const loading = ref(true)
const tiers = ref<any[]>([])
const wallet = ref<{ coins: number; totalTopup: number; vipLevel: number }>({
  coins: 0, totalTopup: 0, vipLevel: 0,
})

onMounted(async () => {
  try {
    const [w, t] = await Promise.all([getWallet(), getVipTiers()])
    wallet.value = w.data
    tiers.value = t.data.tiers
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.callout{ margin-top:8px; padding:10px 12px; border-radius:8px; background:#fff9e6; color:#7a5a00; }
</style>
