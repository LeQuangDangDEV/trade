<script setup lang="ts">
import { computed } from 'vue';
import { currentUser } from '../../auth';
const me = computed(()=> currentUser.value);
</script>

<template>
  <section class="wrap">
    <h2>VIP</h2>
    <nav class="subnav">
      <router-link :to="{ name:'vip-info' }" exact-active-class="active">Thông tin VIP</router-link>
      <router-link v-if="me?.role==='admin'" :to="{ name:'vip-admin' }" exact-active-class="active">Quản trị hệ thống</router-link>
    </nav>
    <div class="content">
      <router-view />
    </div>
  </section>
</template>

<style scoped>
.wrap{ max-width: 1100px; margin: 16px auto; padding: 0 12px; }
.subnav{ display:flex; gap:8px; border-bottom:1px solid #eee; margin-top:8px; }
.subnav a{ padding:8px 12px; border-radius:10px 10px 0 0; text-decoration:none; color:#333; }
.subnav a.active{ background:#e9f1ff; color:#1e80ff; }
.content{ padding:16px 0; }
</style>
