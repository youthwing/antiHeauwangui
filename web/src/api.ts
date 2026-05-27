import type {
  Me,
  Settings,
  SignRecord,
  InviteCode,
  AdminUser,
  AdminGuest,
  AdminStats,
  AdminLog,
  Dorm,
  AdminDorm,
  DormUserBrief,
  SmtpConfig,
  SmtpUpdate,
  GuestCreateReq,
  GuestUpdateReq,
  SchoolCheckinStatus,
  SiteGateCode,
  UserStats,
  ProxyTestResult,
  ProxyNodesResult,
  Announcement,
  AnnouncementUpsertReq,
} from './types'

export interface SchoolAuthPayload {
  token?: string
  callbackUrl?: string
  oauthCode?: string
}

// Public-ish endpoint — admin authors notices and any user reads them.
// Lives outside the `api` (user-scoped) object because there's no PIN gate.
export function listAnnouncements(): Promise<Announcement[]> {
  return request('/announcements')
}

// Lifetime "已为全站用户签到 N 次" tagline counter. Public, single int.
export function getPlatformStats(): Promise<{ totalSigns: number }> {
  return request('/platform-stats')
}

export function enterSite(code: string): Promise<{ ok: boolean; expiresAt: number }> {
  return request('/gate', {
    method: 'POST',
    body: JSON.stringify({ code }),
  })
}

async function request<T>(path: string, opts: RequestInit = {}): Promise<T> {
  const res = await fetch('/api/v1' + path, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    ...opts,
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error || `HTTP ${res.status}`)
  }
  if (res.status === 204) return undefined as T
  const text = await res.text()
  if (!text) return undefined as T
  return JSON.parse(text) as T
}

export const api = {
  // ---- Public (no auth) ----
  login: (userNumber: string, pin: string) =>
    request<{ ok: boolean }>('/login', {
      method: 'POST',
      body: JSON.stringify({ userNumber, pin }),
    }),
  // Two-step activation: precheck the invite code (and PIN/disclaimer) before
  // showing the wechat-OAuth UI. Random visitors with no invite never see the
  // QR / "copy callback URL" flow at all.
  activatePrecheck: (inviteCode: string) =>
    request<{ ok: boolean; note?: string }>('/activate/precheck', {
      method: 'POST',
      body: JSON.stringify({ inviteCode }),
    }),
  activate: (
    inviteCode: string,
    pin: string,
    disclaimerAccepted: boolean,
    auth: SchoolAuthPayload,
  ) =>
    request<{ ok: boolean }>('/activate', {
      method: 'POST',
      body: JSON.stringify({ inviteCode, pin, disclaimerAccepted, ...auth }),
    }),

  // ---- User-scoped ----
  me: () => request<Me>('/me'),
  updateToken: (auth: SchoolAuthPayload) =>
    request<{ ok: boolean; expiresAt: number }>('/token', {
      method: 'PUT',
      body: JSON.stringify(auth),
    }),
  changePin: (oldPin: string, newPin: string) =>
    request<{ ok: boolean }>('/pin', {
      method: 'PUT',
      body: JSON.stringify({ oldPin, newPin }),
    }),
  getSettings: () => request<Settings>('/settings'),
  updateSettings: (s: Partial<Settings>) =>
    request<Settings>('/settings', {
      method: 'PUT',
      body: JSON.stringify(s),
    }),
  records: () => request<SignRecord[]>('/records'),
  stats: () => request<UserStats>('/stats'),
  dorms: () => request<Dorm[]>('/dorms'),
  signNow: () =>
    request<{ status: string; message: string }>('/sign-now', { method: 'POST' }),
  // Toggle today (or `date`) in the user's skip-dates list. Used by the
  // Dashboard "今晚不在校" button. Server prunes expired entries.
  skipToday: (date?: string) =>
    request<{ ok: boolean; skipDates: string[]; toggled: string; action: 'skipped' | 'unskipped' }>(
      '/skip-today',
      {
        method: 'POST',
        body: JSON.stringify(date ? { date } : {}),
      },
    ),
  // Push a test message via the user's currently SAVED Server酱 SendKey.
  // Returns 502 with upstream error if the SendKey is invalid.
  testServerChan: () =>
    request<{ ok: boolean }>('/notify/test-serverchan', { method: 'POST' }),
  testProxy: () =>
    request<ProxyTestResult>('/proxy/test', { method: 'POST' }),
  proxyNodes: () => request<ProxyNodesResult>('/proxy/nodes'),
  selectProxyNode: (name: string) =>
    request<ProxyNodesResult>('/proxy/nodes/select', {
      method: 'POST',
      body: JSON.stringify({ name }),
    }),
  autoSelectProxyNode: () =>
    request<ProxyNodesResult>('/proxy/nodes/autoselect', { method: 'POST' }),
  logout: () => request<{ ok: boolean }>('/logout', { method: 'POST' }),
  deleteMe: () => request<{ ok: boolean }>('/me', { method: 'DELETE' }),
}

export const adminApi = {
  login: (password: string) =>
    request<{ ok: boolean }>('/airvel/login', {
      method: 'POST',
      body: JSON.stringify({ password }),
    }),
  logout: () => request<{ ok: boolean }>('/airvel/logout', { method: 'POST' }),
  me: () => request<{ isAdmin: boolean }>('/airvel/me'),
  stats: () => request<AdminStats>('/airvel/stats'),
  createSiteGateCode: () =>
    request<SiteGateCode>('/airvel/gate-codes', { method: 'POST' }),

  listCodes: (params: {
    status?: 'used' | 'unused'
    search?: string
    limit?: number
    offset?: number
  } = {}) => {
    const q = new URLSearchParams()
    for (const [k, v] of Object.entries(params)) {
      if (v !== undefined && v !== '') q.set(k, String(v))
    }
    const qs = q.toString()
    return request<InviteCode[]>('/airvel/codes' + (qs ? '?' + qs : ''))
  },
  createCodes: (count: number, note: string) =>
    request<InviteCode[]>('/airvel/codes', {
      method: 'POST',
      body: JSON.stringify({ count, note }),
    }),
  updateCode: (code: string, patch: { note?: string; disabled?: boolean }) =>
    request<InviteCode>('/airvel/codes/' + encodeURIComponent(code), {
      method: 'PUT',
      body: JSON.stringify(patch),
    }),
  deleteCode: (code: string) =>
    request<{ ok: boolean }>('/airvel/codes/' + encodeURIComponent(code), {
      method: 'DELETE',
    }),

  listUsers: (search = '', limit = 100) => {
    const q = new URLSearchParams()
    if (search) q.set('search', search)
    q.set('limit', String(limit))
    return request<AdminUser[]>('/airvel/users?' + q.toString())
  },
  getUser: (id: string) => request<AdminUser>('/airvel/users/' + encodeURIComponent(id)),
  updateUser: (
    id: string,
    patch: {
      isDisabled?: boolean
      autoSign?: boolean
      dormId?: number
      signDays?: number
    },
  ) =>
    request<AdminUser>('/airvel/users/' + encodeURIComponent(id), {
      method: 'PUT',
      body: JSON.stringify(patch),
    }),
  deleteUser: (id: string) =>
    request<{ ok: boolean }>('/airvel/users/' + encodeURIComponent(id), {
      method: 'DELETE',
    }),
  resetUserPin: (id: string, newPin?: string) =>
    request<{ ok: boolean; newPin: string }>(
      '/airvel/users/' + encodeURIComponent(id) + '/pin',
      {
        method: 'POST',
        body: JSON.stringify({ newPin: newPin || '' }),
      },
    ),
  signNowFor: (id: string) =>
    request<{ status: string; message: string }>(
      '/airvel/users/' + encodeURIComponent(id) + '/sign-now',
      { method: 'POST' },
    ),
  checkinStatusFor: (id: string) =>
    request<SchoolCheckinStatus>(
      '/airvel/users/' + encodeURIComponent(id) + '/checkin-status',
    ),
  refreshUserToken: (id: string, auth: SchoolAuthPayload) =>
    request<{ ok: boolean; expiresAt: number }>(
      '/airvel/users/' + encodeURIComponent(id) + '/token',
      {
        method: 'POST',
        body: JSON.stringify(auth),
      },
    ),

  logs: (limit = 100) => request<AdminLog[]>('/airvel/logs?limit=' + limit),

  listDorms: () => request<AdminDorm[]>('/airvel/dorms'),
  createDorm: (d: Partial<AdminDorm>) =>
    request<AdminDorm>('/airvel/dorms', {
      method: 'POST',
      body: JSON.stringify(d),
    }),
  updateDorm: (id: number, d: Partial<AdminDorm>) =>
    request<AdminDorm>('/airvel/dorms/' + id, {
      method: 'PUT',
      body: JSON.stringify(d),
    }),
  deleteDorm: (id: number) =>
    request<{ ok: boolean }>('/airvel/dorms/' + id, { method: 'DELETE' }),
  dormUsers: (id: number) =>
    request<DormUserBrief[]>('/airvel/dorms/' + id + '/users'),

  listGuests: () => request<AdminGuest[]>('/airvel/guests'),
  createGuest: (req: GuestCreateReq) =>
    request<AdminGuest>('/airvel/guests', {
      method: 'POST',
      body: JSON.stringify(req),
    }),
  updateGuest: (userId: string, req: GuestUpdateReq) =>
    request<AdminGuest>('/airvel/guests/' + userId, {
      method: 'PUT',
      body: JSON.stringify(req),
    }),
  deleteGuest: (userId: string) =>
    request<{ ok: boolean }>('/airvel/guests/' + userId, { method: 'DELETE' }),

  getSmtp: () => request<SmtpConfig>('/airvel/smtp'),
  updateSmtp: (cfg: SmtpUpdate) =>
    request<SmtpConfig & { ok: boolean }>('/airvel/smtp', {
      method: 'PUT',
      body: JSON.stringify(cfg),
    }),
  testSmtp: () =>
    request<{ ok: boolean; sentTo: string }>('/airvel/smtp/test', {
      method: 'POST',
    }),
  testServerChan: () =>
    request<{ ok: boolean }>('/airvel/serverchan/test', {
      method: 'POST',
    }),

  schoolRules: () =>
    request<{ rules: unknown; updatedAt: number }>('/airvel/school-rules'),

  // --- Announcements ---
  listAnnouncements: () =>
    request<Announcement[]>('/airvel/announcements'),
  createAnnouncement: (req: AnnouncementUpsertReq) =>
    request<Announcement>('/airvel/announcements', {
      method: 'POST',
      body: JSON.stringify(req),
    }),
  updateAnnouncement: (id: number, req: Partial<AnnouncementUpsertReq>) =>
    request<Announcement>('/airvel/announcements/' + id, {
      method: 'PUT',
      body: JSON.stringify(req),
    }),
  deleteAnnouncement: (id: number) =>
    request<{ ok: boolean }>('/airvel/announcements/' + id, {
      method: 'DELETE',
    }),
}
