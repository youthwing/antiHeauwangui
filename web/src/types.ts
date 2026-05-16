export interface TokenInfo {
  expiresAt: number
  validUntil: string
  remainingSec: number
  isValid: boolean
}

export interface SavedLocation {
  name: string
  latitude: number
  longitude: number
  address: string
  city: string
  road: string
  poi: string
}

export interface Settings {
  autoSign: boolean
  dormId: number | null
  dormName?: string
  latitude: number
  longitude: number
  address: string
  city: string
  road: string
  poi: string
  deviceModel: string
  deviceSystem: string
  triggerMinute: number
  jitterSec: number
  retryCount: number
  retryGapMin: number
  savedLocations: SavedLocation[]
  notifyEmail: string
  notifyEnabled: boolean
  // 7-bit bitmask of which weekdays to auto-sign on.
  // bit 0 = Mon, bit 1 = Tue, … bit 5 = Sat, bit 6 = Sun. 127 = every day.
  signDays: number
}

export interface SmtpConfig {
  enabled: boolean
  host: string
  port: number
  username: string
  from: string
  adminBcc: string
  passwordSet: boolean
}

export interface SmtpUpdate {
  enabled: boolean
  host: string
  port: number
  username: string
  password: string // empty = keep current
  from: string
  adminBcc: string
}

export interface Dorm {
  id: number
  name: string
  latitude: number
  longitude: number
  address: string
  city: string
  road: string
  poi: string
  note: string
  sendAddressFields: boolean
}

export interface AdminDorm extends Dorm {
  users: number
  createdAt: number
  updatedAt: number
}

export interface Me {
  userId: string
  userName: string
  userNumber: string
  userSection: string
  userClass: string
  userAvatarUrl: string
  inviteCode: string
  isDisabled: boolean
  hasPin: boolean
  token: TokenInfo
  settings: Settings
}

export type SignStatus = 'success' | 'already' | 'exempt' | 'failed' | 'skipped'

export interface SignRecord {
  id: number
  ruleId: number
  status: SignStatus
  message: string
  occurredAt: number
}

// ---- Admin types ----

export interface InviteCode {
  code: string
  boundUserId: string | null
  boundUserName: string
  boundAt: number | null
  note: string
  disabled: boolean
  createdAt: number
  createdBy: string
  used: boolean
}

export interface AdminUser {
  userId: string
  userName: string
  userNumber: string
  userSection: string
  userClass: string
  inviteCode: string
  isDisabled: boolean
  autoSign: boolean
  latitude: number
  longitude: number
  dormId?: number | null
  dormName?: string
  tokenExp: number
  tokenValid: boolean
  createdAt: number
  updatedAt: number
  recentRecords?: Array<{
    id: number
    status: SignStatus
    message: string
    occurredAt: number
  }>
}

export interface DormUserBrief {
  userId: string
  userName: string
  userNumber: string
  userSection: string
  userClass: string
  isDisabled: boolean
  autoSign: boolean
}

export interface AdminStats {
  users: { total: number; guests: number; disabled: number; expiring: number }
  codes: { total: number; used: number; unused: number }
  today: Record<string, number>
}

export interface AdminGuest {
  userId: string
  userName: string
  userNumber: string
  userSection: string
  userClass: string
  userAvatarUrl: string
  label: string
  signDates: string[]
  tokenExp: number
  tokenValid: boolean
  createdAt: number
  dormId?: number | null
  dormName?: string
  expiresAt: number | null
  autoSign: boolean
  isDisabled: boolean
  triggerMinute: number
  jitterSec: number
  recentRecords?: Array<{
    id: number
    status: SignStatus
    message: string
    occurredAt: number
  }>
}

export interface GuestCreateReq {
  label: string
  signDates: string[]
  dormId?: number
  callbackUrl?: string
  oauthCode?: string
  token?: string
}

export interface GuestUpdateReq {
  label?: string
  signDates?: string[]
}

export interface AdminLog {
  id: number
  userId: string
  userName: string
  status: SignStatus
  message: string
  occurredAt: number
}
