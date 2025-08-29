<template>
  <section class="vip-wrap">
    <header class="vip-header">
      <h2>Khu vực VIP</h2>
      <router-link to="/vip" class="tab" exact-active-class="tab--active">Thông tin VIP</router-link>
      <router-link
        v-if="currentUser?.role === 'admin'"
        to="/vip/admin"
        class="tab"
        exact-active-class="tab--active"
      >Quản lý hệ thống</router-link>
    </header>
    <div class="vip-body">
      <router-view />
    </div>
  </section>
</template>

<script setup lang="ts">
import { currentUser } from '../../auth'
</script>

<style scoped>
.vip-header{ display:flex; align-items:center; gap:12px; margin-bottom:12px; }
.tab{ padding:6px 10px; border-radius:8px; text-decoration:none; color:#555; border:1px solid transparent; }
.tab--active{ border-color:#1e80ff33; background:#1e80ff12; color:#1e80ff; }
.vip-body{ padding:8px 0; }
</style>
