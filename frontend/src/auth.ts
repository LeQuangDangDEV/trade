// src/auth.ts
import { ref } from 'vue';

export const currentUser = ref<any | null>(null);

export function setAuth(token: string, user: any) {
  localStorage.setItem('token', token);
  localStorage.setItem('user', JSON.stringify(user));
  currentUser.value = user;
}
export function setUser(user: any) {
  localStorage.setItem('user', JSON.stringify(user));
  currentUser.value = user;
}
export function getToken(): string | null { return localStorage.getItem('token'); }
export function isAuthenticated(): boolean { return !!getToken(); }
export function clearAuth() {
  localStorage.removeItem('token');
  localStorage.removeItem('user');
  currentUser.value = null;
}

export async function fetchCurrentUser() {
  const base = import.meta.env.VITE_API_BASE ?? 'http://localhost:8080';
  const token = getToken();
  if (!token) { currentUser.value = null; return; }
  const res = await fetch(`${base}/private/me`, {
    headers: { Authorization: `Bearer ${token}` }
  });
  if (!res.ok) { currentUser.value = null; return; }
  const data = await res.json();
  currentUser.value = data.user;
  localStorage.setItem('user', JSON.stringify(data.user));
}
// Khởi tạo state từ localStorage và (tùy chọn) refresh từ server
export function initAuth(opts: { refresh?: boolean } = { refresh: true }) {
  const raw = localStorage.getItem('user');
  if (raw) {
    try { currentUser.value = JSON.parse(raw); }
    catch { currentUser.value = null; }
  } else {
    currentUser.value = null;
  }
  if (opts.refresh) {
    fetchCurrentUser().catch(() => { /* ignore */ });
  }
}
