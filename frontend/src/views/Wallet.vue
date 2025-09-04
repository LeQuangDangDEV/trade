<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import api from '../api'

const loading = ref(true)
const msg = ref(''); const err = ref('')

const wallet = ref<{coins:number; totalTopup:number; vipLevel:number} | null>(null)

async function load(){
  loading.value = true
  try { wallet.value = await api.wallet() }
  finally { loading.value = false }
}
onMounted(load)

/* -------- form chuyển tiền -------- */
const toUsername = ref('')
const amount = ref<number | null>(null)
const note = ref('')

/* Mã bảo mật giao dịch (PIN 6 số) */
const txnPin = ref('')               // người dùng nhập PIN
const pinValid = computed(() => /^\d{6}$/.test(txnPin.value.trim()))

// phí 0.5% (làm tròn lên giống backend)
const fee = computed(() => {
  const a = Math.max(0, Number(amount?.value || 0))
  return Math.floor((a*5 + 999) / 1000) // ceil(a*0.005)
})
const debit = computed(() => (Number(amount?.value || 0) + fee.value))

const canSubmit = computed(() =>
  !!toUsername.value.trim()
  && Number(amount?.value || 0) > 0
  && !!wallet.value
  && debit.value <= (wallet.value?.coins ?? 0)
  && pinValid.value
)

function fmt(n:number){ return (n ?? 0).toLocaleString() }

async function submit(){
  err.value=''; msg.value=''

  // bảo vệ thêm phía client
  if (!pinValid.value) {
    err.value = 'Mã bảo mật phải gồm đúng 6 chữ số.'
    return
  }

  try{
    await api.transfer({
      toUsername: toUsername.value.trim(),
      amount: Number(amount?.value || 0),
      note: note.value.trim(),
      txnPin: txnPin.value.trim(),              // 👈 gửi PIN lên BE
    } as any)
    msg.value = 'Chuyển coin thành công'
    toUsername.value = ''
    amount.value = null
    note.value = ''
    txnPin.value = ''
    await load()
  }catch(e:any){
    err.value = e?.message || 'Có lỗi xảy ra'
  }
}
</script>

<template>
  <section class="wrap">
    <h2>Ví</h2>

    <div class="card" v-if="loading">Đang tải...</div>

    <div class="card" v-else>
      <div class="kpis">
        <div><b>Số dư:</b> <span>{{ fmt(wallet?.coins ?? 0) }}</span></div>
        <div><b>VIP:</b> <span>VIP {{ wallet?.vipLevel ?? 0 }}</span></div>
        <div><b>Tổng nạp:</b> <span>{{ fmt(wallet?.totalTopup ?? 0) }}</span></div>
      </div>

      <h3>Chuyển coin</h3>
      <div class="form">
        <label>Username người nhận</label>
        <input v-model.trim="toUsername" placeholder="username người nhận" />

        <label>Số coin</label>
        <input v-model.number="amount" type="number" min="1" placeholder="Nhập số coin" />

        <label>Ghi chú (tuỳ chọn)</label>
        <input v-model="note" placeholder="..." />

        <label>Mã bảo mật (PIN 6 số)</label>
        <input
          v-model="txnPin"
          inputmode="numeric"
          pattern="\d{6}"
          maxlength="6"
          placeholder="******"
        />
        <small v-if="txnPin && !pinValid" class="warn">PIN phải gồm đúng 6 chữ số.</small>

        <div class="calc">
          <div>Phí (0.5%): <b>{{ fmt(fee) }}</b></div>
          <div>Tổng trừ từ ví: <b>{{ fmt(debit) }}</b></div>
          <div>Sau khi chuyển, số dư ước tính: <b>{{ fmt((wallet?.coins ?? 0) - debit) }}</b></div>
        </div>

        <div class="actions">
          <button class="btn primary" :disabled="!canSubmit" @click="submit">Xác nhận chuyển</button>
        </div>

        <p class="ok" v-if="msg">{{ msg }}</p>
        <p class="err" v-if="err">{{ err }}</p>
      </div>

      <div class="hint">
        Phí 0.5% do <b>người gửi</b> trả. Người nhận nhận đủ số coin bạn nhập.
      </div>
    </div>
  </section>
</template>

<style scoped>
.wrap{ max-width: 960px; margin: 16px auto; padding: 0 12px; }
.card{ border:1px solid #eee; border-radius:12px; padding:16px; display:grid; gap:12px; background:#fff; }
.kpis{ display:grid; grid-template-columns: repeat(auto-fit,minmax(220px,1fr)); gap:8px; }
.form{ display:grid; gap:10px; }
input{ padding:10px; border:1px solid #ddd; border-radius:10px; }
.calc{ display:grid; gap:6px; background:#f7f7f7; border-radius:10px; padding:10px; }
.actions{ margin-top:6px; }
.btn{ height:36px; padding:0 14px; border:1px solid #ddd; border-radius:10px; background:#f7f7f7; cursor:pointer; }
.btn.primary{ background:#1e80ff; color:#fff; border-color:#1e80ff; }
.ok{ color:#16a34a; } .err{ color:#d33; }
.warn{ color:#b45309; font-size:12px; }
.hint{ background:#fff7ed; border-radius:10px; padding:10px; }
</style>
