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
  Calendar,
} from 'lucide-vue-next'
import type { AdminUser, AdminGuest, SchoolCheckinStatus } from '../../types'
import { adminApi } from '../../api'
import { formatDateTime } from '../../lib/format'
import { showToast } from '../../lib/toast'
import Avatar from '../../components/Avatar.vue'

// --- Unified row type for the tonight list. Includes both regular users
// scheduled to sign today and guests whose sign_dates include today.
type Row = {
  userId: string
  userName: string
  userNumber: string
  userSection: string
  userClass: string
  userAvatarUrl: string
  triggerMinute: number
  jitterSec: number
  isGuest: boolean
  guestLabel?: string
  dormName?: string
  tokenExp: number
  tokenValid: boolean
  recentRecords: Array<{ id: number; status: string; message: string; occurredAt: number }>
}

const rows = ref<Row[]>([])
const loading = ref(false)
const lastFetch = ref<Date | null>(null)
const now = ref(new Date())
let tickHandle: number | undefined
let pollHandle: number | undefined

const signing = ref<Record<string, boolean>>({})

type StatusEntry = SchoolCheckinStatus | 'loading' | undefined
const statusByUser = ref<Record<string, StatusEntry>>({})

// Bit 0 = Mon, ..., Bit 6 = Sun. JS Date.getDay() returns 0 = Sun.
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
      if (u.isDisabled) continue
      if (!u.autoSign) continue
      if ((u.signDays & (1 << todayBit)) === 0) continue
      out.push({
        userId: u.userId,
        userName: u.userName,
        userNumber: u.userNumber,
        userSection: u.userSection,
        userClass: u.userClass,
        userAvatarUrl: u.userAvatarUrl,
        triggerMinute: u.triggerMinute,
        jitterSec: u.jitterSec,
        isGuest: false,
        dormName: u.dormName,
        tokenExp: u.tokenExp,
        tokenValid: u.tokenValid,
        recentRecords: u.recentRecords ?? [],
      })
    }
    for (const g of guests as AdminGuest[]) {
      if (!g.signDates.includes(today)) continue
      out.push({
        userId: g.userId,
        userName: g.userName,
        userNumber: g.userNumber,
        userSection: g.userSection,
        userClass: g.userClass,
        userAvatarUrl: g.userAvatarUrl,
        triggerMinute: g.triggerMinute,
        jitterSec: g.jitterSec,
        isGuest: true,
        guestLabel: g.label,
        dormName: g.dormName,
        tokenExp: g.tokenExp,
        tokenValid: g.tokenValid,
        recentRecords: g.recentRecords ?? [],
      })
    }
    out.sort((a, b) => a.triggerMinute - b.triggerMinute || a.userName.localeCompare(b.userName))
    rows.value = out
    lastFetch.value = new Date()
    fetchAllStatus()
  } catch (e: any) {
    showToast('err', e.message || '加载失败')
  } finally {
    loading.value = false
  }
}

async function fetchAllStatus() {
  const next: Record<string, StatusEntry> = {}
  for (const r of rows.value) next[r.userId] = 'loading'
  statusByUser.value = next
  await Promise.allSettled(
    rows.value.map(async r => {
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
const summary = computed(() => {
  let signed = 0
  let pending = 0
  let exempt = 0
  let failed = 0
  let tokenBad = 0
  for (const r of rows.value) {
    const s = statusByUser.value[r.userId]
    if (!s || s === 'loading') {
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
  return { total: rows.value.length, signed, pending, exempt, failed, tokenBad }
})

// --- phase: pre-22:00 / during / post-22:30 ---
const phase = computed<'pre' | 'window' | 'post'>(() => {
  const h = now.value.getHours()
  const m = now.value.getMinutes()
  if (h < 22) return 'pre'
  if (h === 22 && m < 30) return 'window'
  if (h === 22 && m >= 30) return 'post'
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
function signTimeStr(r: Row): string {
  return `22:${String(r.triggerMinute).padStart(2, '0')}`
}

function rowState(r: Row): 'past' | 'now' | 'future' {
  const target = 22 * 60 + r.triggerMinute
  const cur = now.value.getHours() * 60 + now.value.getMinutes()
  if (cur < target - 1) return 'future'
  if (cur > target + 2) return 'past'
  return 'now'
}

function statusChip(s: StatusEntry, r: Row): { tone: string; label: string; icon: any } {
  if (s === 'loading') return { tone: 'loading', label: '加载中…', icon: RefreshCw }
  if (!s) return { tone: 'zinc', label: '未拉取', icon: Clock }
  switch (s.state) {
    case 'signed':
      return { tone: 'ok', label: '学校已确认签到', icon: UserCheck }
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

function statusChipClass(tone: string): string {
  switch (tone) {
    case 'ok':
      return 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 ring-1 ring-emerald-500/25'
    case 'amber':
      return 'bg-amber-500/10 text-amber-700 dark:text-amber-300 ring-1 ring-amber-500/30'
    case 'blue':
      return 'bg-blue-500/10 text-blue-700 dark:text-blue-300 ring-1 ring-blue-500/30'
    case 'red':
      return 'bg-red-500/10 text-red-700 dark:text-red-300 ring-1 ring-red-500/30'
    case 'zinc':
      return 'bg-zinc-500/10 text-zinc-700 dark:text-zinc-300 ring-1 ring-zinc-500/20'
    case 'loading':
      return 'bg-zinc-500/5 text-zinc-500 ring-1 ring-black/[0.06] dark:ring-white/[0.06]'
    default:
      return 'bg-zinc-500/10 text-zinc-500'
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
      return 'bg-emerald-400'
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

onMounted(() => {
  load()
  // Clock tick — drives "now" displays and phase transitions
  tickHandle = window.setInterval(() => (now.value = new Date()), 30_000)
  // Auto-refresh data + status every 60s while the page is open. Cleared
  // on unmount so leaving the page stops school-API traffic.
  pollHandle = window.setInterval(load, 60_000)
})
onUnmounted(() => {
  if (tickHandle) clearInterval(tickHandle)
  if (pollHandle) clearInterval(pollHandle)
})
</script>

<template>
  <div class="space-y-4">
    <header class="flex flex-col sm:flex-row sm:items-end sm:justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold tracking-tight flex items-center gap-2">
          <Moon class="w-5 h-5 text-amber-400" />
          今晚看板
        </h1>
        <p class="text-sm text-zinc-500 mt-1">
          {{ phaseLabel }} ·
          <span v-if="lastFetch" class="font-mono-token text-[11px]">
            上次拉取 {{ formatDateTime(Math.floor(lastFetch.getTime() / 1000)) }}
          </span>
        </p>
      </div>
      <button @click="load" :disabled="loading"
        class="self-start inline-flex items-center gap-1.5 bg-amber-500 hover:bg-amber-400 disabled:opacity-50 text-zinc-950 text-sm font-medium px-4 py-2 rounded-lg transition-colors">
        <RefreshCw class="w-4 h-4" :class="loading ? 'wangui-spin' : ''" />
        立即重拉
      </button>
    </header>

    <!-- Summary tiles -->
    <section class="grid grid-cols-2 md:grid-cols-5 gap-2">
      <div class="rounded-xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-3">
        <p class="text-[10px] uppercase tracking-wide text-zinc-500 mb-1">今晚要签</p>
        <p class="text-2xl font-bold tabular-nums">{{ summary.total }}</p>
      </div>
      <div class="rounded-xl bg-emerald-500/10 ring-1 ring-emerald-500/25 p-3">
        <p class="text-[10px] uppercase tracking-wide text-emerald-600 dark:text-emerald-400 mb-1">已签</p>
        <p class="text-2xl font-bold tabular-nums text-emerald-600 dark:text-emerald-300">{{ summary.signed }}</p>
      </div>
      <div class="rounded-xl bg-amber-500/10 ring-1 ring-amber-500/30 p-3">
        <p class="text-[10px] uppercase tracking-wide text-amber-600 dark:text-amber-400 mb-1">待签</p>
        <p class="text-2xl font-bold tabular-nums text-amber-600 dark:text-amber-300">{{ summary.pending }}</p>
      </div>
      <div class="rounded-xl bg-blue-500/10 ring-1 ring-blue-500/30 p-3">
        <p class="text-[10px] uppercase tracking-wide text-blue-600 dark:text-blue-400 mb-1">请假/免签</p>
        <p class="text-2xl font-bold tabular-nums text-blue-600 dark:text-blue-300">{{ summary.exempt }}</p>
      </div>
      <div class="rounded-xl bg-red-500/10 ring-1 ring-red-500/30 p-3 col-span-2 md:col-span-1">
        <p class="text-[10px] uppercase tracking-wide text-red-500 dark:text-red-400 mb-1">异常</p>
        <p class="text-2xl font-bold tabular-nums text-red-500 dark:text-red-300">
          {{ summary.failed + summary.tokenBad }}
        </p>
      </div>
    </section>

    <!-- Loading state -->
    <div v-if="loading && rows.length === 0" class="py-20 flex justify-center">
      <div class="h-6 w-6 rounded-full border-2 border-zinc-800 border-t-emerald-400 wangui-spin" />
    </div>

    <!-- Empty state -->
    <div
      v-else-if="rows.length === 0"
      class="rounded-xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] py-16 text-center"
    >
      <Calendar class="w-8 h-8 mx-auto text-zinc-400 dark:text-zinc-600 mb-3" />
      <p class="text-sm text-zinc-500">今晚没有需要签到的用户</p>
      <p class="text-xs text-zinc-400 dark:text-zinc-600 mt-1">所有人都在 signDays 之外或者临时朋友没有今天</p>
    </div>

    <!-- Schedule list -->
    <section
      v-else
      class="rounded-xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] overflow-hidden"
    >
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead class="bg-white/50 dark:bg-zinc-950/50 border-b border-black/[0.08] dark:border-white/[0.06]">
            <tr class="text-left text-[10px] text-zinc-500 uppercase tracking-wide">
              <th class="px-4 py-2.5 font-medium w-20">时刻</th>
              <th class="px-4 py-2.5 font-medium">用户</th>
              <th class="px-4 py-2.5 font-medium">学校状态</th>
              <th class="px-4 py-2.5 font-medium">今天本系统记录</th>
              <th class="px-4 py-2.5 font-medium text-right">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-black/[0.05] dark:divide-white/[0.04]">
            <tr
              v-for="r in rows"
              :key="r.userId"
              :class="rowState(r) === 'now'
                ? 'bg-amber-500/[0.06]'
                : rowState(r) === 'past'
                  ? ''
                  : ''"
              class="hover:bg-black/[0.02] dark:hover:bg-white/[0.02] transition-colors"
            >
              <!-- Time -->
              <td class="px-4 py-3 font-mono-token tabular-nums">
                <div class="flex flex-col">
                  <span
                    class="font-semibold"
                    :class="rowState(r) === 'now'
                      ? 'text-amber-500 dark:text-amber-300'
                      : rowState(r) === 'past'
                        ? 'text-zinc-500'
                        : 'text-emerald-500 dark:text-emerald-300'"
                  >
                    {{ signTimeStr(r) }}
                  </span>
                  <span class="text-[10px] text-zinc-500">±{{ r.jitterSec }}s</span>
                </div>
              </td>

              <!-- User -->
              <td class="px-4 py-3">
                <div class="flex items-center gap-2.5 min-w-0">
                  <Avatar :src="r.userAvatarUrl" :name="r.userName" :size="36" rounded="lg" />
                  <div class="min-w-0">
                    <div class="flex items-center gap-1.5">
                      <span class="text-sm font-medium text-zinc-900 dark:text-zinc-200 truncate">{{ r.userName }}</span>
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

              <!-- School status -->
              <td class="px-4 py-3">
                <div class="flex items-center gap-1.5 flex-wrap">
                  <span
                    class="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-medium"
                    :class="statusChipClass(statusChip(statusByUser[r.userId], r).tone)"
                  >
                    <component
                      :is="statusChip(statusByUser[r.userId], r).icon"
                      class="w-3 h-3"
                      :class="statusChip(statusByUser[r.userId], r).tone === 'loading' ? 'wangui-spin' : ''"
                    />
                    {{ statusChip(statusByUser[r.userId], r).label }}
                  </span>
                  <button
                    @click="refreshOne(r)"
                    class="text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-200 p-1 rounded hover:bg-black/5 dark:hover:bg-white/5 transition-colors"
                    title="重拉学校状态"
                  >
                    <RefreshCw class="w-3 h-3" />
                  </button>
                </div>
                <p
                  v-if="statusByUser[r.userId] && statusByUser[r.userId] !== 'loading' && (statusByUser[r.userId] as SchoolCheckinStatus).message && (statusByUser[r.userId] as SchoolCheckinStatus).state !== 'exempt'"
                  class="text-[10px] text-zinc-500 mt-1 truncate max-w-[14rem]"
                >
                  {{ (statusByUser[r.userId] as SchoolCheckinStatus).message }}
                </p>
              </td>

              <!-- Today's wangui record (if any) -->
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
                  @click="signNow(r)"
                  :disabled="signing[r.userId]"
                  class="inline-flex items-center gap-1 bg-emerald-500 hover:bg-emerald-400 disabled:opacity-50 text-zinc-950 text-xs font-medium px-2.5 py-1.5 rounded-md transition-colors"
                  title="代签到"
                >
                  <PlayCircle class="w-3.5 h-3.5" />
                  {{ signing[r.userId] ? '签到中…' : '立即签' }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <p class="text-[11px] text-zinc-500 dark:text-zinc-600">
      自动每 60 秒重拉一次（页面打开时）。状态以学校 xhbcs 接口的实时返回为准。
    </p>
  </div>
</template>
