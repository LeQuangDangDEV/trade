// src/api.ts
import { getToken, clearAuth } from './auth';

export class AuthError extends Error {}

export const BASE = import.meta.env.VITE_API_BASE ?? 'http://localhost:8080';

/* ----------------------------- helpers ----------------------------- */
function qs(params?: Record<string, string | number | boolean | undefined | null>) {
  const p = new URLSearchParams();
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (v === undefined || v === null || v === '') continue;
      p.set(k, String(v));
    }
  }
  const s = p.toString();
  return s ? `?${s}` : '';
}

async function http<T>(path: string, opts: RequestInit = {}): Promise<T> {
  const token = getToken();
  const headers: HeadersInit = { ...(opts.headers || {}) };
  const isForm = opts.body instanceof FormData;
  if (!isForm) headers['Content-Type'] = 'application/json';
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch(`${BASE}${path}`, { ...opts, headers });

  if (res.status === 401) {
    try { clearAuth(); } catch {}
    throw new AuthError('UNAUTHORIZED');
  }

  if (!res.ok) {
    let msg = `HTTP ${res.status}`;
    try {
      const j = await res.json();
      msg = j?.error || j?.message || msg;
    } catch {}
    throw new Error(msg);
  }

  if (res.status === 204) return undefined as unknown as T;
  const text = await res.text();
  return (text ? JSON.parse(text) : (undefined as unknown)) as T;
}

/* -------------------------------- types ---------------------------- */
export type User = {
  id: number;
  username: string;   // tên đăng nhập
  name: string;       // biệt danh
  phone: string;
  avatarUrl?: string;
  role: 'admin' | 'user';
  coins: number;
  totalTopup: number;
  vipLevel: number;   // giữ tương thích
};

export type VipTier = { level: number; name: string; minTopup: number };
export type Wallet = { coins: number; totalTopup: number; vipLevel: number };

export type AdminUserRow = {
  id: number;
  username: string;
  nickname: string;
  vipLevel: number;
  totalTopup: number;
  coins: number;
};

export type InventoryItem = { code: string; qty: number };
export type AdminUserDetail = {
  id: number;
  username: string;
  name: string;
  phone: string;
  avatarUrl?: string;
  role: 'admin'|'user';
  coins: number;
  totalTopup: number;
  vipLevel: number;

  kycStatus?: 'NONE'|'VERIFIED'|'APPROVED'|'PENDING'|'REJECTED'; // tuỳ BE
  kycFullName?: string;
  kycNumber?: string;
  kycDob?: string;
  hasKycFront: boolean;
  hasKycBack: boolean;
};
export type MarketRow = {
  id: number;
  code: string;
  qty: number;
  pricePerUnit: number;
  sellerId: number;
  // hỗ trợ cả 2 để không vỡ UI cũ:
  sellerUsername?: string;
  sellerEmail?: string;
  buyQty?: number;
};

export type TopupHistoryRow = {
  id: number; amount: number; note: string; adminUsername: string; createdAt: string;
};

export type WithdrawHistoryRow = {
  id: number; amount: number; note: string; adminUsername: string; createdAt: string;
};

export type TransferHistoryRow = {
  id: number; direction: 'in' | 'out'; amount: number; fee: number; counterpart: string; createdAt: string;
};

export type VipHistoryRow = {
  id: number; level: number; price: number; oldLevel: number; createdAt: string;
};

export type CommissionHistoryRow = {
  id: number;
  buyerUsername: string; // nếu BE còn trả email, đổi mapper ở BE hoặc sửa tên ở đây
  depth: number; percent: number; amount: number; kind: 'UPLINE' | 'ADMIN'; vipLevel: number; createdAt: string;
};

/* -------------------------------- api ------------------------------ */
export const api = {
  /* ===== Public ===== */
  register: (body: { username: string; password: string; nickname: string; phone: string; ref?: string }) =>
    http<{ message: string }>('/register', { method: 'POST', body: JSON.stringify(body) }),

  login: (body: { username: string; password: string }) =>
    http<{ token: string; user: User }>('/login', { method: 'POST', body: JSON.stringify(body) }),

  vipTiers: () => http<{ tiers: VipTier[] }>('/vip-tiers'),

  /* ===== Private ===== */
  me: () => http<{ user: User }>('/private/me'),

  updateProfile: (body: { name: string; phone: string; avatarUrl?: string }) =>
    http<{ user: User; message: string }>('/private/profile', { method: 'PUT', body: JSON.stringify(body) }),

  // Thêm: đổi mật khẩu đăng nhập
  changePassword: (body: { oldPassword: string; newPassword: string }) =>
    http<{ message: string }>('/private/change-password', { method: 'PUT', body: JSON.stringify(body) }),

  wallet: () => http<Wallet>('/private/wallet'),

  uploadAvatar: async (file: File): Promise<{ url: string }> => {
    const fd = new FormData();
    fd.append('file', file);
    const token = getToken();
    const res = await fetch(`${BASE}/private/upload`, {
      method: 'POST',
      headers: token ? { Authorization: `Bearer ${token}` } : {},
      body: fd,
    });
    if (res.status === 401) {
      try { clearAuth(); } catch {}
      throw new AuthError('UNAUTHORIZED');
    }
    if (!res.ok) {
      let msg = `HTTP ${res.status}`;
      try { const j = await res.json(); msg = j?.error || j?.message || msg; } catch {}
      throw new Error(msg);
    }
    return res.json();
  },

  /* ===== Referral ===== */
  referralInfo: () =>
    http<{ code: string; link: string; count: number; total: number }>('/private/referral-info'),

  /* ===== Ví & giao dịch ===== */
transfer: (body: { toUsername: string; amount: number; note?: string; txnPin: string }) =>
  http<{ message: string; fee: number; debit: number }>('/private/transfer', {
    method: 'POST',
    body: JSON.stringify(body),
  }),

  buyVip: () => http<{ message: string; level: number; coins: number }>('/private/buy-vip', { method: 'POST' }),

  /* ===== Lịch sử ===== */
  topupHistory: () => http<{ rows: TopupHistoryRow[] }>('/private/history/topups'),
  withdrawHistory: () => http<{ rows: WithdrawHistoryRow[] }>('/private/history/withdraws'),
  transferHistory: () => http<{ rows: TransferHistoryRow[] }>('/private/history/transfers'),
  vipHistory: () => http<{ rows: VipHistoryRow[] }>('/private/history/vip'),
  myCommissions: () => http<{ rows: CommissionHistoryRow[] }>('/private/history/commissions'),
  commissionsHistory() { return this.myCommissions(); },

  /* ===== Treasure ===== */
  chestOpen: () =>
    http<{ result: 'COIN' | 'DRAGON_BALL'; code?: string; amount: number; coins: number; inv: Record<string, number> }>(
      '/private/chest-open', { method: 'POST' }
    ),

  inventory: () => http<{ items: InventoryItem[] }>('/private/inventory'),
  mergeDragon: () => http<{ message: string; coins: number }>('/private/merge-dragon', { method: 'POST' }),

  /* ===== Market ===== */
  marketList: (code?: string) =>
    http<{ rows: MarketRow[] }>(`/market${qs({ code })}`),

  marketCreate: (body: { code: string; qty: number; pricePerUnit: number }) =>
    http<{ message: string }>('/private/market/list', { method: 'POST', body: JSON.stringify(body) }),

  marketBuy: (body: { listingId: number; qty: number }) =>
    http<{ message: string }>('/private/market/buy', { method: 'POST', body: JSON.stringify(body) }),

  marketWithdraw: (body: { listingId: number; qty?: number }) =>
    http<{ message: string }>('/private/market/withdraw', { method: 'POST', body: JSON.stringify(body) }),

  /* ===== Admin ===== */
  adminUsers: (filters?: { vipLevel?: string | number; username?: string; nickname?: string }) =>
    http<{ rows: AdminUserRow[] }>(
      `/admin/users${qs({ vipLevel: filters?.vipLevel, username: filters?.username, nickname: filters?.nickname })}`
    ),

  adminTopup: (body: { userId: number; amount: number; note?: string }) =>
    http<{ message: string; userId: number }>('/admin/topup', { method: 'POST', body: JSON.stringify(body) }),

  adminWithdraw: (body: { userId: number; amount: number; note?: string }) =>
    http<{ message: string; userId: number }>('/admin/withdraw', { method: 'POST', body: JSON.stringify(body) }),

  adminDeleteUser: (id: number) =>
    http<{ message: string }>(`/admin/users/${id}`, { method: 'DELETE' }),

  /* ===== Quên mật khẩu (bằng mật khẩu cấp 2) ===== */

forgotPassword: (body:{username:string; secPassword:string; newPassword:string}) =>
  http<{message:string}>('/forgot-password', { method:'POST', body: JSON.stringify(body) }),


  /* ===== Bảo mật nâng cao ===== */
  // Hỗ trợ cả 3 dạng tham số để tương thích:
  // 1) { secondPassword, txnPin }
  // 2) { oldSecondPassword, newSecondPassword, newTxnPin }
  // 3) { oldPassword2, newPassword2, newPin }
 // api.ts
updateSecurity: (rawBody: any) => {
  const payload: any = {};

  // --- Chuẩn hoá alias về đúng key BE cần ---
  // alias kiểu cũ
  if (rawBody.oldPassword2)  payload.oldSecondPassword = String(rawBody.oldPassword2).trim();
  if (rawBody.newPassword2)  payload.newSecondPassword = String(rawBody.newPassword2).trim();
  if (rawBody.newPin)        payload.newTxnPin        = String(rawBody.newPin).trim();

  // alias UI hiện tại
  if (rawBody.secondPassword)    payload.newSecondPassword = String(rawBody.secondPassword).trim();
  if (rawBody.txnPin)            payload.newTxnPin         = String(rawBody.txnPin).trim();

  // nếu đã đúng tên rồi thì vẫn giữ
  if (rawBody.oldSecondPassword) payload.oldSecondPassword = String(rawBody.oldSecondPassword).trim();
  if (rawBody.newSecondPassword) payload.newSecondPassword = String(rawBody.newSecondPassword).trim();
  if (rawBody.newTxnPin)         payload.newTxnPin         = String(rawBody.newTxnPin).trim();

  // --- Validate nhẹ ở FE để tránh 400 không cần thiết ---
  if (payload.newTxnPin && !/^\d{6}$/.test(payload.newTxnPin)) {
    throw new Error('Mã bảo mật giao dịch (PIN) phải gồm 6 chữ số.');
  }
  if (!payload.newSecondPassword && !payload.newTxnPin) {
    throw new Error('Chưa có thay đổi nào để lưu.');
  }

  return http<{ message: string }>('/private/security', {
    method: 'PUT',
    body: JSON.stringify(payload),
  });
},


  // KYC: FE có thể gọi kycSubmit({ frontPath, backPath }) hoặc updateKyc({ frontUrl, backUrl })
  kycSubmit: (body: { frontPath: string; backPath: string }) =>
    http<{ message: string }>('/private/kyc', { method: 'PUT', body: JSON.stringify(body) }),

  updateKyc: (body: { frontUrl: string; backUrl: string }) =>
    http<{ message: string }>('/private/kyc', {
      method: 'PUT',
      body: JSON.stringify({ frontPath: body.frontUrl, backPath: body.backUrl }),
    }),
adminUserDetail: (id: number) =>
  http<{ user: AdminUserDetail }>(`/admin/users/${id}`),

// Lấy ảnh KYC (trả về ObjectURL để gán vào <img>)
adminKycImage: async (userId: number, side: 'front'|'back'): Promise<string> => {
  const token = getToken();
  const res = await fetch(`${BASE}/admin/kyc-file/${userId}/${side}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
  if (res.status === 401) { try{ clearAuth(); }catch{}; throw new AuthError('UNAUTHORIZED'); }
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  const blob = await res.blob();
  return URL.createObjectURL(blob);
},
};

export default api;
