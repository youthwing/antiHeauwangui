<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import {
  RefreshCw,
  Clock,
  PlayCircle,
  ShieldAlert,
  UserCheck,
  UserX,
  Coffee,
  Moon,
  CalendarOff,
  PowerOff,
  Ban,
  Activity,
  Users as UsersIcon,
} from 'lucide-vue-next'
import type { AdminUser, AdminGuest, SchoolCheckinStatus } from '../../types'
import { adminApi } from '../../api'
import { formatDateTime } from '../../lib/format'
import { showToast } from '../../lib/toast'
import Avatar from '../../components/Avatar.vue'

// --- categories ---
//   tonight     — regular user scheduled to sign tonight
//   days-skip   — regular user, autoSign on, but today not in signDays
//   auto-off    — regular user, autoSign deliberately off
//   disabled    — regular user, admin-disabled
//   guest-today — guest whose sign_dates includes today
//   guest-other — guest with sign_dates but not today
type Category = 'tonight' | 'days-skip' | 'auto-off' | 'disabled' | 'guest-today' | 'guest-other'

type Row = {
  userId: string
  userName: string
  userNumber: string
  userSection: string
  userClass: string
  userAvatarUrl: string
  triggerMinute: number
  jitterSec: number
  signDays: number
  isGuest: boolean
  guestLabel?: string
  dormName?: string
  tokenExp: number
  tokenValid: boolean
  recentRecords: Array<{ id: number; status: string; message: string; occurredAt: number }>
  category: Category
}

const rows = ref<Row[]>([])
const loading = ref(false)
const lastFetch = ref<Date | null>(null)
const now = ref(new Date())
let tickHandle: number | undefined
let pollHandle: number | undefined

// --- Server-Sent Events: real-time push of sign results / token warnings /
// rules changes. EventSource auto-reconnects on drops; we use a ref so the
// UI can show a connection-state pill.
const sseStatus = ref<'connecting' | 'open' | 'closed'>('closed')
const lastEvent = ref<{ type: string; at: number } | null>(null)
const flashRows = ref<Record<string, number>>({}) // userId → expiresAt unix ms
let sseSource: EventSource | null = null
let flashTimer: number | undefined

function flashRow(userId: string) {
  flashRows.value[userId] = Date.now() + 4000
  if (!flashTimer) {
    flashTimer = window.setInterval(() => {
      const cur = Date.now()
      let changed = false
      const next: Record<string, number> = {}
      for (const k in flashRows.value) {
        if (flashRows.value[k] > cur) next[k] = flashRows.value[k]
        else changed = true
      }
      if (changed) flashRows.value = next
      if (Object.keys(next).length === 0 && flashTimer) {
        clearInterval(flashTimer)
        flashTimer = undefined
      }
    }, 500)
  }
}

function isFlashing(userId: string): boolean {
  const exp = flashRows.value[userId]
  return !!exp && exp > Date.now()
}

const signing = ref<Record<string, boolean>>({})

type StatusEntry = SchoolCheckinStatus | 'loading' | 'skipped' | undefined
const statusByUser = ref<Record<string, StatusEntry>>({})

const activeFilter = ref<'all' | Category>('all')

// Restore filter from URL on mount so refreshing keeps the selection.
const filterFromStorage = () => {
  try {
    const v = localStorage.getItem('admin-monitor-filter')
    if (v && ['all', 'tonight', 'days-skip', 'auto-off', 'disabled', 'guest-today', 'guest-other'].includes(v)) {
      activeFilter.value = v as typeof activeFilter.value
    }
  } catch {
    /* localStorage unavailable, fall back to default */
  }
}
function setFilter(f: typeof activeFilter.value) {
  activeFilter.value = f
  try {
    localStorage.setItem('admin-monitor-filter', f)
  } catch {
    /* ignore */
  }
}

function dayBit(jsDay: number): number {
  return (jsDay + 6) % 7
}

const todayStr = computed(() => {
  const d = now.value
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
})

async function load() {
  loading.value = true
  try {
    const [users, guests] = await Promise.all([
      adminApi.listUsers('', 500),
      adminApi.listGuests(),
    ])
    const today = todayStr.value
    const todayBit = dayBit(now.value.getDay())
    const out: Row[] = []
    for (const u of users) {
      let category: Category
      if (u.isDisabled) category = 'disabled'
      else if (!u.autoSign) category = 'auto-off'
      else if ((u.signDays & (1 << todayBit)) === 0) category = 'days-skip'
      else category = 'tonight'
      out.push({
        userId: u.userId,
        userName: u.userName,
        userNumber: u.userNumber,
        userSection: u.userSection,
        userClass: u.userClass,
        userAvatarUrl: u.userAvatarUrl,
        triggerMinute: u.triggerMinute,
        jitterSec: u.jitterSec,
        signDays: u.signDays,
        isGuest: false,
        dormName: u.dormName,
        tokenExp: u.tokenExp,
        tokenValid: u.tokenValid,
        recentRecords: u.recentRecords ?? [],
        category,
      })
    }
    for (const g of guests as AdminGuest[]) {
      const isToday = g.signDates.includes(today)
      out.push({
        userId: g.userId,
        userName: g.userName,
        userNumber: g.userNumber,
        userSection: g.userSection,
        userClass: g.userClass,
        userAvatarUrl: g.userAvatarUrl,
        triggerMinute: g.triggerMinute,
        jitterSec: g.jitterSec,
        signDays: 0,
        isGuest: true,
        guestLabel: g.label,
        dormName: g.dormName,
        tokenExp: g.tokenExp,
        tokenValid: g.tokenValid,
        recentRecords: g.recentRecords ?? [],
        category: isToday ? 'guest-today' : 'guest-other',
      })
    }
    // Sort: active categories first (tonight + guest-today by triggerMinute),
    // then days-skip, auto-off, disabled, guest-other. Within each by name.
    const order: Record<Category, number> = {
      tonight: 0,
      'guest-today': 1,
      'days-skip': 2,
      'auto-off': 3,
      disabled: 4,
      'guest-other': 5,
    }
    out.sort((a, b) => {
      const oa = order[a.category]
      const ob = order[b.category]
      if (oa !== ob) return oa - ob
      if (a.category === 'tonight' || a.category === 'guest-today') {
        if (a.triggerMinute !== b.triggerMinute) return a.triggerMinute - b.triggerMinute
      }
      return a.userName.localeCompare(b.userName)
    })
    rows.value = out
    lastFetch.value = new Date()
    fetchTonightStatus()
  } catch (e: any) {
    showToast('err', e.message || '加载失败')
  } finally {
    loading.value = false
  }
}

// Pull school CheckinStatus only for rows that are actually scheduled to
// sign tonight. Others get a 'skipped' marker so the column shows "—" with
// a hover hint instead of burning API calls on idle users.
async function fetchTonightStatus() {
  const next: Record<string, StatusEntry> = {}
  for (const r of rows.value) {
    if (r.category === 'tonight' || r.category === 'guest-today') {
      next[r.userId] = 'loading'
    } else {
      next[r.userId] = 'skipped'
    }
  }
  statusByUser.value = next
  await Promise.allSettled(
    rows.value
      .filter(r => r.category === 'tonight' || r.category === 'guest-today')
      .map(async r => {
        try {
          statusByUser.value[r.userId] = await adminApi.checkinStatusFor(r.userId)
        } catch (e: any) {
          statusByUser.value[r.userId] = {
            state: 'error',
            message: e?.message || '查询失败',
          } as SchoolCheckinStatus
        }
      }),
  )
}

async function refreshOne(r: Row) {
  if (r.category !== 'tonight' && r.category !== 'guest-today') return
  statusByUser.value[r.userId] = 'loading'
  try {
    statusByUser.value[r.userId] = await adminApi.checkinStatusFor(r.userId)
  } catch (e: any) {
    statusByUser.value[r.userId] = {
      state: 'error',
      message: e?.message || '查询失败',
    } as SchoolCheckinStatus
  }
}

async function signNow(r: Row) {
  if (signing.value[r.userId]) return
  if (!confirm(`代「${r.userName}」(${r.userNumber}) 立即签到？`)) return
  signing.value[r.userId] = true
  try {
    const res = await adminApi.signNowFor(r.userId)
    const tone = res.status === 'success' || res.status === 'already' ? 'ok' : 'err'
    showToast(tone, `${r.userName}: ${res.message || res.status}`)
    await load()
  } catch (e: any) {
    showToast('err', e.message || '签到失败')
  } finally {
    signing.value[r.userId] = false
  }
}

// --- summary stats ---
const counts = computed(() => {
  const c: Record<Category | 'total', number> = {
    total: rows.value.length,
    tonight: 0,
    'guest-today': 0,
    'days-skip': 0,
    'auto-off': 0,
    disabled: 0,
    'guest-other': 0,
  }
  for (const r of rows.value) c[r.category]++
  return c
})

const filteredRows = computed(() => {
  if (activeFilter.value === 'all') return rows.value
  return rows.value.filter(r => r.category === activeFilter.value)
})

// Aggregated outcome among "tonight + guest-today" rows.
const outcome = computed(() => {
  let signed = 0
  let pending = 0
  let exempt = 0
  let failed = 0
  let tokenBad = 0
  for (const r of rows.value) {
    if (r.category !== 'tonight' && r.category !== 'guest-today') continue
    const s = statusByUser.value[r.userId]
    if (!s || s === 'loading' || s === 'skipped') {
      pending++
      continue
    }
    switch (s.state) {
      case 'signed':
        signed++
        break
      case 'exempt':
      case 'boarding':
        exempt++
        break
      case 'tokenExpired':
        tokenBad++
        break
      case 'error':
        failed++
        break
      default:
        pending++
    }
  }
  return { signed, pending, exempt, failed, tokenBad }
})

// --- phase: pre-22:00 / during / post-22:30 ---
const phase = computed<'pre' | 'window' | 'post'>(() => {
  const h = now.value.getHours()
  const m = now.value.getMinutes()
  if (h < 22) return 'pre'
  if (h === 22 && m < 30) return 'window'
  return 'post'
})

const phaseLabel = computed(() => {
  switch (phase.value) {
    case 'pre':
      return '22:00 前预览'
    case 'window':
      return '签到窗口进行中 (22:00 – 22:30)'
    case 'post':
      return '今晚战报'
  }
  return ''
})

// --- display helpers ---
function signTimeStr(r: Row): string | null {
  if (r.category !== 'tonight' && r.category !== 'guest-today') return null
  return `22:${String(r.triggerMinute).padStart(2, '0')}`
}

function rowTimeState(r: Row): 'past' | 'now' | 'future' {
  if (r.category !== 'tonight' && r.category !== 'guest-today') return 'past'
  const target = 22 * 60 + r.triggerMinute
  const cur = now.value.getHours() * 60 + now.value.getMinutes()
  if (cur < target - 1) return 'future'
  if (cur > target + 2) return 'past'
  return 'now'
}

const CATEGORY_META: Record<Category, { tone: string; label: string; icon: any; hint: string }> = {
  tonight: { tone: 'red', label: '今晚要签', icon: Activity, hint: '自动签开启 + 今天在 signDays' },
  'guest-today': { tone: 'amber', label: '临时·今晚', icon: Moon, hint: '临时朋友今天有签到任务' },
  'days-skip': { tone: 'zinc', label: '周次跳过', icon: CalendarOff, hint: '今天不在 signDays（如周六没勾）' },
  'auto-off': { tone: 'amber', label: '自动签关', icon: PowerOff, hint: '用户主动关闭了自动签到' },
  disabled: { tone: 'red', label: '已禁用', icon: Ban, hint: 'admin 后台禁用，所有签到跳过' },
  'guest-other': { tone: 'zinc', label: '临时·非今天', icon: CalendarOff, hint: '临时朋友今天不在 sign_dates' },
}

function categoryClass(tone: string): string {
  switch (tone) {
    case 'red':
      return 'bg-red-500/10 text-red-700 dark:text-red-300 ring-1 ring-red-500/25'
    case 'amber':
      return 'bg-amber-500/10 text-amber-700 dark:text-amber-300 ring-1 ring-amber-500/30'
    case 'blue':
      return 'bg-sky-500/10 text-blue-700 dark:text-blue-300 ring-1 ring-sky-500/30'
    case 'zinc':
      return 'bg-zinc-500/10 text-zinc-700 dark:text-zinc-300 ring-1 ring-zinc-500/20'
    default:
      return 'bg-zinc-500/10 text-zinc-500'
  }
}

function statusChip(s: StatusEntry, r: Row): { tone: string; label: string; icon: any } | null {
  if (r.category !== 'tonight' && r.category !== 'guest-today') return null
  if (s === 'loading') return { tone: 'loading', label: '加载中…', icon: RefreshCw }
  if (!s || s === 'skipped') return { tone: 'zinc', label: '未拉取', icon: Clock }
  switch (s.state) {
    case 'signed':
      return { tone: 'red', label: '学校已确认签到', icon: UserCheck }
    case 'canSign':
      return { tone: 'amber', label: '待签到', icon: Clock }
    case 'pending':
      return { tone: 'zinc', label: '窗口未开放', icon: Moon }
    case 'exempt':
      return {
        tone: 'blue',
        label: s.exemptReason || s.message || '请假/免签',
        icon: Coffee,
      }
    case 'boarding':
      return { tone: 'blue', label: '走读/外宿', icon: Coffee }
    case 'tokenExpired':
      return { tone: 'red', label: 'Token 失效', icon: ShieldAlert }
    case 'error':
      return { tone: 'red', label: '查询失败', icon: UserX }
    default:
      return { tone: 'zinc', label: r.userName, icon: Clock }
  }
}

function todayRecordOf(r: Row): { status: string; message: string; occurredAt: number } | null {
  const t = todayStr.value
  for (const rec of r.recentRecords) {
    const d = new Date(rec.occurredAt * 1000)
    const ds = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
    if (ds === t) return rec
  }
  return null
}

function recordDotClass(status: string): string {
  switch (status) {
    case 'success':
    case 'already':
    case 'exempt':
      return 'bg-red-400'
    case 'failed':
      return 'bg-red-400'
    case 'skipped':
      return 'bg-zinc-500'
    default:
      return 'bg-zinc-400'
  }
}

function recordLabel(status: string): string {
  switch (status) {
    case 'success':
      return '签到成功'
    case 'already':
      return '已签'
    case 'exempt':
      return '免签'
    case 'failed':
      return '失败'
    case 'skipped':
      return '跳过'
    default:
      return status
  }
}

const FILTER_OPTIONS: { key: 'all' | Category; label: string }[] = [
  { key: 'all', label: '全部' },
  { key: 'tonight', label: '今晚要签' },
  { key: 'guest-today', label: '临时·今晚' },
  { key: 'days-skip', label: '周次跳过' },
  { key: 'auto-off', label: '自动签关' },
  { key: 'disabled', label: '已禁用' },
  { key: 'guest-other', label: '临时·非今天' },
]

function filterCount(key: 'all' | Category): number {
  if (key === 'all') return counts.value.total
  return counts.value[key as Category]
}

function openSSE() {
  if (sseSource) {
    sseSource.close()
    sseSource = null
  }
  sseStatus.value = 'connecting'
  // EventSource sends cookies by default (same-origin), so the admin
  // session cookie auths the stream like any other admin endpoint.
  const es = new EventSource('/api/v1/airvel/events')
  sseSource = es
  es.addEventListener('hello', () => {
    sseStatus.value = 'open'
  })
  es.addEventListener('sign.result', (e: MessageEvent) => {
    try {
      const data = JSON.parse(e.data)
      const p = data.payload || {}
      lastEvent.value = { type: data.type, at: data.at }
      // Briefly highlight the affected row and refresh status for it.
      if (p.userId) {
        flashRow(p.userId)
        // Reload after a short delay so the DB record + state lands.
        window.setTimeout(load, 800)
      }
    } catch {
      /* ignore malformed event */
    }
  })
  es.addEventListener('window.open', () => {
    lastEvent.value = { type: 'window.open', at: Math.floor(Date.now() / 1000) }
    load()
  })
  es.addEventListener('window.close', () => {
    lastEvent.value = { type: 'window.close', at: Math.floor(Date.now() / 1000) }
    load()
  })
  es.addEventListener('token.warn', (e: MessageEvent) => {
    try {
      const data = JSON.parse(e.data)
      lastEvent.value = { type: data.type, at: data.at }
    } catch { /* ignore */ }
  })
  es.addEventListener('school.rules', (e: MessageEvent) => {
    try {
      const data = JSON.parse(e.data)
      lastEvent.value = { type: data.type, at: data.at }
    } catch { /* ignore */ }
  })
  es.onerror = () => {
    sseStatus.value = 'closed'
    // EventSource auto-reconnects per the retry: directive; nothing else
    // to do here. Browsers will hit /events again with the next backoff.
  }
}

onMounted(() => {
  filterFromStorage()
  load()
  // Clock tick — drives "now" displays and phase transitions
  tickHandle = window.setInterval(() => (now.value = new Date()), 30_000)
  // Auto-refresh data + status every 60s while the page is open. SSE
  // covers most updates, but the periodic poll is a safety net in case
  // any event got dropped (or the connection was briefly down).
  pollHandle = window.setInterval(load, 60_000)
  openSSE()
})
onUnmounted(() => {
  if (tickHandle) clearInterval(tickHandle)
  if (pollHandle) clearInterval(pollHandle)
  if (flashTimer) clearInterval(flashTimer)
  if (sseSource) {
    sseSource.close()
    sseSource = null
  }
})
</script>

<template>
  <div class="space-y-4">
    <header class="flex flex-col sm:flex-row sm:items-end sm:justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold tracking-tight flex items-center gap-2">
          <Activity class="w-5 h-5 text-red-400" />
          监控看板
          <!-- SSE connection indicator -->
          <span
            class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium ring-1"
            :class="sseStatus === 'open'
              ? 'bg-red-500/10 text-red-700 dark:text-red-300 ring-red-500/30'
              : sseStatus === 'connecting'
                ? 'bg-amber-500/10 text-amber-700 dark:text-amber-300 ring-amber-500/30'
                : 'bg-red-500/10 text-red-700 dark:text-red-300 ring-red-500/30'"
            :title="sseStatus === 'open' ? '实时事件已连接' : sseStatus === 'connecting' ? '连接中…' : '已断开（每 60s 仍轮询）'"
          >
            <span
              class="w-1.5 h-1.5 rounded-full"
              :class="sseStatus === 'open' ? 'bg-red-400 animate-pulse' : sseStatus === 'connecting' ? 'bg-amber-400 animate-pulse' : 'bg-red-400'"
            />
            {{ sseStatus === 'open' ? '实时' : sseStatus === 'connecting' ? '连接中' : '已断开' }}
          </span>
        </h1>
        <p class="text-sm text-zinc-500 mt-1">
          {{ phaseLabel }} ·
          <span v-if="lastFetch" class="font-mono-token text-[11px]">
            上次拉取 {{ formatDateTime(Math.floor(lastFetch.getTime() / 1000)) }}
          </span>
          <span class="ml-2 text-[11px] text-zinc-500">· 自动每 60s 重拉</span>
          <span v-if="lastEvent" class="ml-2 text-[11px] text-red-500 dark:text-red-400">
            · 上一事件 {{ lastEvent.type }} @ {{ formatDateTime(lastEvent.at) }}
          </span>
        </p>
      </div>
      <button @click="load" :disabled="loading"
        class="self-start inline-flex items-center gap-1.5 bg-red-500 hover:bg-red-400 disabled:opacity-50 text-[#0d1117] text-sm font-medium px-4 py-2 rounded-lg transition-colors">
        <RefreshCw class="w-4 h-4" :class="loading ? 'wangui-spin' : ''" />
        立即重拉
      </button>
    </header>

    <!-- Outcome tiles (only meaningful for "tonight" subset) -->
    <section class="grid grid-cols-2 md:grid-cols-5 gap-2">
      <div class="rounded-xl bg-white/85 dark:bg-[#161b22]/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-3">
        <p class="text-[10px] uppercase tracking-wide text-zinc-500 mb-1 flex items-center gap-1">
          <UsersIcon class="w-3 h-3" />
          全部用户
        </p>
        <p class="text-2xl font-bold tabular-nums">{{ counts.total }}</p>
        <p class="text-[10px] text-zinc-500 mt-0.5">含临时朋友</p>
      </div>
      <div class="rounded-xl bg-red-500/10 ring-1 ring-red-500/25 p-3">
        <p class="text-[10px] uppercase tracking-wide text-red-600 dark:text-red-400 mb-1">今晚已签</p>
        <p class="text-2xl font-bold tabular-nums text-red-600 dark:text-red-300">{{ outcome.signed }}</p>
      </div>
      <div class="rounded-xl bg-amber-500/10 ring-1 ring-amber-500/30 p-3">
        <p class="text-[10px] uppercase tracking-wide text-amber-600 dark:text-amber-400 mb-1">待签</p>
        <p class="text-2xl font-bold tabular-nums text-amber-600 dark:text-amber-300">{{ outcome.pending }}</p>
      </div>
      <div class="rounded-xl bg-sky-500/10 ring-1 ring-sky-500/30 p-3">
        <p class="text-[10px] uppercase tracking-wide text-blue-600 dark:text-blue-400 mb-1">请假/免签</p>
        <p class="text-2xl font-bold tabular-nums text-blue-600 dark:text-blue-300">{{ outcome.exempt }}</p>
      </div>
      <div class="rounded-xl bg-red-500/10 ring-1 ring-red-500/30 p-3 col-span-2 md:col-span-1">
        <p class="text-[10px] uppercase tracking-wide text-red-500 dark:text-red-400 mb-1">异常</p>
        <p class="text-2xl font-bold tabular-nums text-red-500 dark:text-red-300">
          {{ outcome.failed + outcome.tokenBad }}
        </p>
      </div>
    </section>

    <!-- Filter tabs -->
    <div class="flex flex-wrap gap-1.5">
      <button
        v-for="opt in FILTER_OPTIONS"
        :key="opt.key"
        @click="setFilter(opt.key)"
        :class="activeFilter === opt.key
          ? 'bg-red-500 text-[#0d1117] ring-red-500'
          : 'bg-white/85 dark:bg-[#161b22]/60 text-zinc-700 dark:text-zinc-300 ring-black/[0.08] dark:ring-white/[0.06] hover:ring-red-500/30'"
        class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-medium ring-1 transition-colors"
      >
        {{ opt.label }}
        <span
          class="inline-flex items-center justify-center min-w-[1.25rem] px-1 py-0.5 rounded-full text-[10px] tabular-nums"
          :class="activeFilter === opt.key
            ? 'bg-[#0d1117]/20 text-[#0d1117]'
            : 'bg-zinc-200 dark:bg-zinc-800 text-zinc-500'"
        >
          {{ filterCount(opt.key) }}
        </span>
      </button>
    </div>

    <!-- Loading state -->
    <div v-if="loading && rows.length === 0" class="py-20 flex justify-center">
      <div class="h-6 w-6 rounded-full border-2 border-zinc-800 border-t-red-400 wangui-spin" />
    </div>

    <!-- Empty state for current filter -->
    <div
      v-else-if="filteredRows.length === 0"
      class="rounded-xl bg-white/85 dark:bg-[#161b22]/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] py-14 text-center text-sm text-zinc-500"
    >
      没有符合当前筛选的用户
    </div>

    <!-- Schedule list -->
    <section
      v-else
      class="rounded-xl bg-white/85 dark:bg-[#161b22]/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] overflow-hidden"
    >
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead class="bg-white/50 dark:bg-[#0d1117]/50 border-b border-black/[0.08] dark:border-white/[0.06]">
            <tr class="text-left text-[10px] text-zinc-500 uppercase tracking-wide">
              <th class="px-4 py-2.5 font-medium w-20">时刻</th>
              <th class="px-4 py-2.5 font-medium">用户</th>
              <th class="px-4 py-2.5 font-medium">分类</th>
              <th class="px-4 py-2.5 font-medium">学校状态</th>
              <th class="px-4 py-2.5 font-medium">今日记录</th>
              <th class="px-4 py-2.5 font-medium text-right">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-black/[0.05] dark:divide-white/[0.04]">
            <tr
              v-for="r in filteredRows"
              :key="r.userId"
              :class="[
                rowTimeState(r) === 'now' ? 'bg-amber-500/[0.06]' : '',
                isFlashing(r.userId) ? 'row-flash' : '',
              ]"
              class="hover:bg-black/[0.02] dark:hover:bg-white/[0.02] transition-colors"
            >
              <!-- Time -->
              <td class="px-4 py-3 font-mono-token tabular-nums">
                <template v-if="signTimeStr(r)">
                  <div class="flex flex-col">
                    <span
                      class="font-semibold"
                      :class="rowTimeState(r) === 'now'
                        ? 'text-amber-500 dark:text-amber-300'
                        : rowTimeState(r) === 'past'
                          ? 'text-zinc-500'
                          : 'text-red-500 dark:text-red-300'"
                    >
                      {{ signTimeStr(r) }}
                    </span>
                    <span class="text-[10px] text-zinc-500">±{{ r.jitterSec }}s</span>
                  </div>
                </template>
                <span v-else class="text-zinc-500">—</span>
              </td>

              <!-- User -->
              <td class="px-4 py-3">
                <div class="flex items-center gap-2.5 min-w-0">
                  <Avatar :src="r.userAvatarUrl" :name="r.userName" :size="36" rounded="lg" />
                  <div class="min-w-0">
                    <div class="flex items-center gap-1.5">
                      <span class="text-sm font-medium text-[#161b22] dark:text-zinc-200 truncate">{{ r.userName }}</span>
                      <span
                        v-if="r.isGuest"
                        class="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-amber-500/15 text-amber-700 dark:text-amber-300 ring-1 ring-amber-500/30 shrink-0"
                      >
                        临时
                      </span>
                    </div>
                    <p class="text-[11px] text-zinc-500 font-mono-token truncate">{{ r.userNumber }}</p>
                    <p
                      v-if="r.dormName"
                      class="text-[10px] text-zinc-500 truncate"
                    >
                      {{ r.dormName }}
                    </p>
                  </div>
                </div>
              </td>

              <!-- Category -->
              <td class="px-4 py-3">
                <span
                  class="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-medium"
                  :class="categoryClass(CATEGORY_META[r.category].tone)"
                  :title="CATEGORY_META[r.category].hint"
                >
                  <component :is="CATEGORY_META[r.category].icon" class="w-3 h-3" />
                  {{ CATEGORY_META[r.category].label }}
                </span>
              </td>

              <!-- School status -->
              <td class="px-4 py-3">
                <template v-if="statusChip(statusByUser[r.userId], r)">
                  <div class="flex items-center gap-1.5 flex-wrap">
                    <span
                      class="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-medium"
                      :class="categoryClass(statusChip(statusByUser[r.userId], r)!.tone)"
                    >
                      <component
                        :is="statusChip(statusByUser[r.userId], r)!.icon"
                        class="w-3 h-3"
                        :class="statusChip(statusByUser[r.userId], r)!.tone === 'loading' ? 'wangui-spin' : ''"
                      />
                      {{ statusChip(statusByUser[r.userId], r)!.label }}
                    </span>
                    <button
                      @click="refreshOne(r)"
                      class="text-zinc-500 hover:text-[#161b22] dark:hover:text-zinc-200 p-1 rounded hover:bg-black/5 dark:hover:bg-white/5 transition-colors"
                      title="重拉学校状态"
                    >
                      <RefreshCw class="w-3 h-3" />
                    </button>
                  </div>
                  <p
                    v-if="statusByUser[r.userId] && statusByUser[r.userId] !== 'loading' && statusByUser[r.userId] !== 'skipped' && (statusByUser[r.userId] as SchoolCheckinStatus).message && (statusByUser[r.userId] as SchoolCheckinStatus).state !== 'exempt'"
                    class="text-[10px] text-zinc-500 mt-1 truncate max-w-[14rem]"
                  >
                    {{ (statusByUser[r.userId] as SchoolCheckinStatus).message }}
                  </p>
                </template>
                <span v-else class="text-zinc-500 text-xs" title="今天不签的用户不查学校状态以节省 API">
                  —
                </span>
              </td>

              <!-- Today's wangui record -->
              <td class="px-4 py-3 text-xs">
                <template v-if="todayRecordOf(r)">
                  <div class="flex items-center gap-1.5">
                    <span class="w-1.5 h-1.5 rounded-full" :class="recordDotClass(todayRecordOf(r)!.status)" />
                    <span class="font-medium text-zinc-700 dark:text-zinc-300">{{ recordLabel(todayRecordOf(r)!.status) }}</span>
                  </div>
                  <p class="text-[10px] text-zinc-500 mt-0.5 truncate max-w-[14rem]">
                    {{ todayRecordOf(r)!.message || '—' }}
                  </p>
                  <p class="text-[10px] text-zinc-500 font-mono-token">{{ formatDateTime(todayRecordOf(r)!.occurredAt) }}</p>
                </template>
                <span v-else class="text-zinc-500">—</span>
              </td>

              <!-- Action -->
              <td class="px-4 py-3 text-right">
                <button
                  v-if="r.category === 'tonight' || r.category === 'guest-today'"
                  @click="signNow(r)"
                  :disabled="signing[r.userId]"
                  class="inline-flex items-center gap-1 bg-red-500 hover:bg-red-400 disabled:opacity-50 text-[#0d1117] text-xs font-medium px-2.5 py-1.5 rounded-md transition-colors"
                  title="代签到"
                >
                  <PlayCircle class="w-3.5 h-3.5" />
                  {{ signing[r.userId] ? '签到中…' : '立即签' }}
                </button>
                <span v-else class="text-[10px] text-zinc-500">—</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <p class="text-[11px] text-zinc-500 dark:text-zinc-600">
      实时事件 (SSE) + 每 60 秒兜底轮询。学校状态以 xhbcs 接口实时返回为准，仅对「今晚要签」分类拉取以节省 API。
    </p>
  </div>
</template>

<style scoped>
/* Briefly flash a row green when an event for that user arrives via SSE.
   The animation fades out so the highlight doesn't linger long after the
   user has noticed. */
@keyframes row-flash-anim {
  0%   { background-color: rgba(16, 185, 129, 0.18); }
  100% { background-color: transparent; }
}
.row-flash {
  animation: row-flash-anim 4s ease-out forwards;
}
</style>
