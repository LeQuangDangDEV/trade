<template>
  <aside class="sidebar">
    <div class="logo">
      <router-link to="/">MyApp</router-link>
    </div>

    <nav class="menu">
  <router-link to="/home" class="item" active-class="active">
    <span class="icon" aria-hidden="true" v-html="icons.home"></span>
    <span>Home</span>
  </router-link>

  <router-link to="/vip" class="item" active-class="active">
    <span class="icon" aria-hidden="true" v-html="icons.crown"></span>
    <span>VIP</span>
  </router-link>
</nav>

      
      <!-- thêm mục khác ở đây -->
    
  </aside>
</template>

<script setup lang="ts">
import { currentUser } from '../auth'
const icons = {
  home: `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor"
    stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <path d="M3 11l9-8 9 8"/><path d="M9 22V12h6v10"/></svg>`,
  crown: `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor"
    stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <path d="M3 7l5 5 4-8 4 8 5-5"/><path d="M2 19h20"/></svg>`,
}
</script>

<style scoped>
.sidebar{
  position:fixed; inset:0 auto 0 0; width:220px; background:#fff;
  border-right:1px solid #eee; display:flex; flex-direction:column; margin-top: 13px;
}
.logo{ padding:16px; font-weight:800; border-bottom:1px solid #eee; }
.logo a{ text-decoration:none; color:#333; }
.menu{ padding:8px; display:flex; flex-direction:column; gap:4px; }
.item{
  display:flex; align-items:center; gap:10px;
  padding:10px 12px; border-radius:10px; color:#333; text-decoration:none;
}
.item:hover{ background:#f5f5f7; }
.item.active{ background:#1e80ff12; color:#1e80ff; }
.icon{ display:inline-flex; width:18px; height:18px; }
@media (max-width: 768px){
  .sidebar{ width:64px; }
  .item span:last-child{ display:none; }
  .logo{ padding:12px; }
}
.section{ margin-top:8px; }
.section-title{ display:flex; align-items:center; gap:10px; padding:10px 12px; color:#666; font-weight:600; }
.submenu{ padding-left:12px; display:flex; flex-direction:column; gap:4px; }
.subitem{ display:block; padding:8px 12px; border-radius:10px; text-decoration:none; color:#333; }
.subitem:hover{ background:#f5f5f7; }
.subitem.active{ background:#1e80ff12; color:#1e80ff; }
</style>
