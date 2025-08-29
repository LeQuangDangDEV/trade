<template>
  <Sidebar />
<Navbar />
<main class="container">
<router-view />
</main>
 <AuthPanel />
</template>


<script setup lang="ts">
import Navbar from './components/Navbar.vue'
import AuthPanel from './components/AuthPanel.vue'
import Sidebar from './components/Sidebar.vue'
</script>


<style scoped>

/* vùng nội dung dịch phải 220px để chừa chỗ sidebar */
.page{ margin-left:220px; min-height:100vh; display:flex; flex-direction:column; }
.container{ max-width: 980px; margin: 24px auto; padding: 0 16px; width:100%; }

/* mobile: sidebar thu nhỏ 64px (đã set trong Sidebar.vue) */
@media (max-width:768px){
  .page{ margin-left:64px; }
}
</style>