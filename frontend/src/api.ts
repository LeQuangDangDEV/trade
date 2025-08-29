import axios from 'axios'
import { token } from './auth'


export const api = axios.create({
baseURL: 'http://localhost:8080', // backend Go của bạn
headers: { 'Content-Type': 'application/json' }
})
export async function getMe() {
  return api.get('/private/me')
}
export async function updateProfile(p: { name: string; phone: string; avatarUrl: string }) {
  return api.put('/private/profile', p)
}


api.interceptors.request.use((config) => {
if (token.value) {
config.headers = config.headers || {}
config.headers['Authorization'] = `Bearer ${token.value}`
}
return config
})

export async function uploadAvatar(file: File) {
  const form = new FormData()
  form.append('file', file)
  return api.post('/private/upload-avatar', form, {
    headers: { 'Content-Type': 'multipart/form-data' }
  })}

  // src/api.ts (chỉ thêm các hàm dưới)
export async function getVipTiers() {
  return api.get('/vip-tiers')
}
export async function getWallet() {
  return api.get('/private/wallet')
}
export async function adminTopup(payload: { userId: number; amount: number; note?: string }) {
  return api.post('/admin/topup', payload)
}
export async function adminSearchUsers(params: { vipLevel?: number; nickname?: string }) {
  return api.get('/admin/users', { params })
}
