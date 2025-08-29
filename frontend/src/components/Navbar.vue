<template>
  <nav class="nav">
    <div class="brand">
      <router-link to="/">MyApp</router-link>
    </div>

    <div class="spacer" />

    <div class="right" v-if="!isAuthenticated">
      <button class="btn" @click="openAuth('login')">Đăng nhập</button>
      <button class="btn" @click="openAuth('register')">Đăng ký</button>
    </div>

    <div class="right" v-else>
      <img class="avatar" :src="currentUser?.avatarUrl || placeholder" alt="avatar" />
      <span class="name">{{ currentUser?.name || 'Người dùng' }}</span>
      <router-link to="/profile"><button class="btn">Quản lý hồ sơ</button></router-link>
      <button class="btn" @click="onLogout">Đăng xuất</button>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { isAuthenticated, clearToken, currentUser } from '../auth'
import { openAuth } from '../panelAuth'
import { useRouter } from 'vue-router'
const router = useRouter()
const placeholder =
  'data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32"><rect width="100%" height="100%" fill="#eee"/></svg>'
function onLogout(){ clearToken(); router.push('/home') }
</script>

<style scoped>
/* Navbar cao cố định + căn giữa dọc */
.nav{
  height:56px; display:flex; align-items:center;
  padding:0 16px; border-bottom:1px solid #eee; gap:12px;
  box-sizing:border-box;
}

/* Link brand là inline nên set line-height 1 và block để không lệch baseline */
.brand a{ display:block; text-decoration:none; color:#333; font-weight:700; line-height:1; }

/* Nhóm bên phải: flex-center + gap đồng đều */
.right{ display:flex; align-items:center; gap:10px; }

/* Avatar: block để không ảnh hưởng baseline, size cố định */
.avatar{ width:32px; height:32px; border-radius:50%; object-fit:cover; display:block; }

/* Nút đồng bộ chiều cao bằng flex, không dùng padding dọc tuỳ ý */
.btn{
  height:36px; padding:0 12px;
  display:inline-flex; align-items:center; justify-content:center;
  border:1px solid #ddd; border-radius:10px; background:#f8f8f8; cursor:pointer;
  line-height:1; /* tránh cao hơn do line-height kế thừa */
}
.btn:hover{ background:#efefef; }

/* Tên user không kéo cao dòng */
.name{ line-height:1; }

/* Spacer đẩy nhóm phải sang mép phải */
.spacer{ flex:1; }
</style>
