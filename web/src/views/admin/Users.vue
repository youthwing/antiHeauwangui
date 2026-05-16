<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import QRCode from 'qrcode'
import {
  Search,
  RefreshCw,
  X,
  Power,
  Trash2,
  KeyRound,
  Copy,
  Building2,
  PlayCircle,
  Clock,
  History,
  Ticket,
  CalendarDays,
  AlertCircle,
  Check,
  QrCode as QrCodeIcon,
} from 'lucide-vue-next'
import type { AdminUser, Dorm, SchoolCheckinStatus } from '../../types'
import { adminApi } from '../../api'
import { formatDateTime } from '../../lib/format'
import { showToast } from '../../lib/toast'
import { copyText } from '../../lib/clipboard'
import {
  buildWechatOauthAuthorizeUrl,
  createWechatOauthState,
  detectSchoolOauthInput,
} from '../../lib/schoolOauth'
import Avatar from '../../components/Avatar.vue'

const users = ref<AdminUser[]>([])
const dorms = ref<Dorm[]>([])
const loading = ref(false)
const search = ref('')

const now = ref(new Date())
let tickHandle: number | undefined

// Per-card transient flags, keyed by userId.
const signing = ref<Record<string, boolean>>({})
const bindingDorm = ref<Record<string, boolean>>({})
const togglingAuto = ref<Record<string, boolean>>({})
const savingDays = ref<Record<string, boolean>>({})
const togglingDisabled = ref<Record<string, boolean>>({})

// School "today status" per user. 'loading' marker = fetch in flight.
type StatusEntry = SchoolCheckinStatus | 'loading' | undefined
const statusByUser = ref<Record<string, StatusEntry>>({})

// PIN reset modal state
const resetting = ref(false)
const newPinResult = ref<{ user: string; pin: string } | null>(null)

// Refresh-token modal state
const refreshTarget = ref<AdminUser | null>(null)
const rCallback = ref('')
const rQrDataUrl = ref('')
const rQrBuilding = ref(false)
const rQrState = ref(createWechatOauthState())
const refreshing = ref(false)

async function load() {
  loading.value = true
  try {
    const [u, d] = await Promise.all([
      adminApi.listUsers(search.value.trim()),
      adminApi.listDorms(),
    ])
    users.value = u
    dorms.value = d as unknown as Dorm[]
    // Fan out school CheckinStatus calls in parallel after rendering.
    fetchAllStatus()
  } catch (e: any) {
    showToast('err', e.message || '加载失败')
  } finally {
    loading.value = false
  }
}

async function fetchAllStatus() {
  // Mark every user as 'loading' first so the UI shows a skeleton.
  const next: Record<string, StatusEntry> = {}
  for (const u of users.value) next[u.userId] = 'loading'
  statusByUser.value = next
  // Fan out concurrently. We don't await all — let each card update as it
  // comes in. Settle errors silently per-user (the card shows 'error').
  await Promise.allSettled(
    users.value.map(async u => {
      try {
        const s = await adminApi.checkinStatusFor(u.userId)
        statusByUser.value[u.userId] = s
      } catch (e: any) {
        statusByUser.value[u.userId] = {
          state: 'error',
          message: e?.message || '查询失败',
        } as SchoolCheckinStatus
      }
    }),
  )
}

async function refreshStatus(u: AdminUser) {
  statusByUser.value[u.userId] = 'loading'
  try {
    statusByUser.value[u.userId] = await adminApi.checkinStatusFor(u.userId)
  } catch (e: any) {
    statusByUser.value[u.userId] = {
      state: 'error',
      message: e?.message || '查询失败',
    } as SchoolCheckinStatus
  }
}

onMounted(() => {
  load()
  tickHandle = window.setInterval(() => (now.value = new Date()), 30_000)
})
onUnmounted(() => {
  if (tickHandle) clearInterval(tickHandle)
})

// --- per-card actions ---

async function signNow(u: AdminUser) {
  if (signing.value[u.userId]) return
  if (!confirm(`代「${u.userName}」(${u.userNumber}) 立即签到？`)) return
  signing.value[u.userId] = true
  try {
    const res = await adminApi.signNowFor(u.userId)
    const tone =
      res.status === 'success' || res.status === 'already' ? 'ok' : 'err'
    showToast(tone, `${u.userName}: ${res.message || res.status}`)
    await load()
  } catch (e: any) {
    showToast('err', e.message || '签到失败')
  } finally {
    signing.value[u.userId] = false
  }
}

async function changeDorm(u: AdminUser, dormId: number) {
  if (bindingDorm.value[u.userId]) return
  bindingDorm.value[u.userId] = true
  try {
    const updated = await adminApi.updateUser(u.userId, { dormId })
    Object.assign(u, updated)
    // server returns the new user; pick up dormName via reload to be safe
    await load()
    showToast('ok', dormId === 0 ? '已解绑宿舍楼' : '已切换宿舍楼')
  } catch (e: any) {
    showToast('err', e.message || '保存失败')
  } finally {
    bindingDorm.value[u.userId] = false
  }
}

async function toggleAuto(u: AdminUser) {
  if (togglingAuto.value[u.userId]) return
  togglingAuto.value[u.userId] = true
  try {
    const updated = await adminApi.updateUser(u.userId, { autoSign: !u.autoSign })
    Object.assign(u, updated)
  } catch (e: any) {
    showToast('err', e.message || '保存失败')
  } finally {
    togglingAuto.value[u.userId] = false
  }
}

async function toggleDay(u: AdminUser, bit: number) {
  if (savingDays.value[u.userId]) return
  savingDays.value[u.userId] = true
  const original = u.signDays
  const next = u.signDays ^ (1 << bit)
  u.signDays = next // optimistic
  try {
    await adminApi.updateUser(u.userId, { signDays: next })
  } catch (e: any) {
    u.signDays = original
    showToast('err', e.message || '保存失败')
  } finally {
    savingDays.value[u.userId] = false
  }
}

async function toggleDisabled(u: AdminUser) {
  if (togglingDisabled.value[u.userId]) return
  if (!confirm(`${u.isDisabled ? '启用' : '禁用'} 「${u.userName}」(${u.userNumber})？`)) return
  togglingDisabled.value[u.userId] = true
  try {
    const updated = await adminApi.updateUser(u.userId, { isDisabled: !u.isDisabled })
    Object.assign(u, updated)
    showToast('ok', u.isDisabled ? '已禁用' : '已启用')
  } catch (e: any) {
    showToast('err', e.message || '操作失败')
  } finally {
    togglingDisabled.value[u.userId] = false
  }
}

async function resetPin(u: AdminUser) {
  if (!confirm(`重置「${u.userName}」(${u.userNumber}) 的 PIN？\n这会强制他的所有会话登出。`)) return
  resetting.value = true
  try {
    const r = await adminApi.resetUserPin(u.userId)
    newPinResult.value = { user: `${u.userName} / ${u.userNumber}`, pin: r.newPin }
  } catch (e: any) {
    showToast('err', e.message || '重置失败')
  } finally {
    resetting.value = false
  }
}

async function copyPin() {
  if (!newPinResult.value) return
  const ok = await copyText(newPinResult.value.pin)
  showToast(ok ? 'ok' : 'err', ok ? 'PIN 已复制' : '复制失败，请手动选取')
}

async function remove(u: AdminUser) {
  if (!confirm(`删除用户「${u.userName}」(${u.userNumber})？\n会同时释放他的邀请码。`)) return
  try {
    await adminApi.deleteUser(u.userId)
    showToast('ok', '已删除')
    await load()
  } catch (e: any) {
    showToast('err', e.message || '删除失败')
  }
}

// --- token refresh modal ---

async function openRefresh(u: AdminUser) {
  refreshTarget.value = u
  rCallback.value = ''
  await rebuildRefreshQr()
}

async function rebuildRefreshQr() {
  rQrBuilding.value = true
  try {
    rQrState.value = createWechatOauthState()
    rQrDataUrl.value = await QRCode.toDataURL(
      buildWechatOauthAuthorizeUrl(rQrState.value),
      { width: 240, margin: 1, errorCorrectionLevel: 'M' },
    )
  } catch (e: any) {
    rQrDataUrl.value = ''
    showToast('err', e?.message || '二维码生成失败')
  } finally {
    rQrBuilding.value = false
  }
}

const rCallbackDetect = computed(() => detectSchoolOauthInput(rCallback.value))
const rCallbackOk = computed(
  () => rCallbackDetect.value.kind === 'code' || rCallbackDetect.value.kind === 'callback-url',
)

async function submitRefresh() {
  if (!refreshTarget.value || !rCallbackOk.value) return
  refreshing.value = true
  try {
    await adminApi.refreshUserToken(refreshTarget.value.userId, {
      callbackUrl: rCallback.value.trim(),
    })
    showToast('ok', `Token 已刷新（${refreshTarget.value.userName}）`)
    const refreshed = refreshTarget.value
    refreshTarget.value = null
    await load()
    if (refreshed) await refreshStatus(refreshed)
  } catch (e: any) {
    showToast('err', e.message || '刷新失败')
  } finally {
    refreshing.value = false
  }
}

watch(refreshTarget, v => {
  if (!v) rCallback.value = ''
})

// --- display helpers ---

const DAY_LABELS = ['一', '二', '三', '四', '五', '六', '日'] as const

function dayBit(jsDay: number): number {
  // JS Date.getDay(): 0 = Sun, 1 = Mon, ..., 6 = Sat
  // Our bitmask: 0 = Mon, ..., 5 = Sat, 6 = Sun
  return (jsDay + 6) % 7
}

const todayBit = computed(() => dayBit(now.value.getDay()))

function signsToday(u: AdminUser): boolean {
  return (u.signDays & (1 << todayBit.value)) !== 0
}

function signTimeStr(u: AdminUser): string {
  return `22:${String(u.triggerMinute).padStart(2, '0')}`
}

function tokenUrgency(u: AdminUser): 'expired' | 'soon' | 'ok' {
  const cur = Math.floor(now.value.getTime() / 1000)
  if (u.tokenExp <= cur) return 'expired'
  if (u.tokenExp - cur < 48 * 3600) return 'soon'
  return 'ok'
}

// Map our coarse state bucket → chip color + label.
function statusChip(s: StatusEntry): { tone: string; label: string; hint: string } {
  if (s === 'loading') return { tone: 'loading', label: '加载中…', hint: '' }
  if (!s) return { tone: 'unknown', label: '未知', hint: '' }
  switch (s.state) {
    case 'signed':
      return { tone: 'ok', label: '学校：已签', hint: s.message }
    case 'canSign':
      return { tone: 'amber', label: '学校：待签', hint: s.message }
    case 'pending':
      return { tone: 'zinc', label: '学校：未开放', hint: s.message }
    case 'exempt':
      return { tone: 'blue', label: '学校：' + (s.exemptReason || s.message || '请假/免签'), hint: s.message }
    case 'boarding':
      return { tone: 'blue', label: '学校：走读/外宿', hint: s.message }
    case 'tokenExpired':
      return { tone: 'red', label: 'Token 失效', hint: '需要刷新' }
    case 'error':
      return { tone: 'red', label: '查询失败', hint: s.message }
    default:
      return { tone: 'zinc', label: s.state, hint: s.message }
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
      return 'bg-zinc-500/10 text-zinc-700 dark:text-zinc-300 ring-1 ring-zinc-500/25'
    case 'loading':
      return 'bg-zinc-500/5 text-zinc-500 ring-1 ring-black/[0.06] dark:ring-white/[0.06]'
    default:
      return 'bg-zinc-500/10 text-zinc-500 ring-1 ring-zinc-500/20'
  }
}

function statusDotClass(status: string): string {
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

function statusLabel(status: string): string {
  switch (status) {
    case 'success':
      return '成功'
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
</script>

<template>
  <div class="space-y-4">
    <header class="flex flex-col sm:flex-row sm:items-end sm:justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold tracking-tight">用户</h1>
        <p class="text-sm text-zinc-500 mt-1">
          每张卡片显示一个用户的全部配置 + 今晚学校状态 + 手动操作。
        </p>
      </div>
      <div class="flex items-center gap-2">
        <div class="relative w-full max-w-xs">
          <Search class="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-zinc-500" />
          <input
            v-model="search"
            @keyup.enter="load"
            placeholder="搜索姓名 / 学号 / 邀请码"
            class="w-full pl-9 pr-3 py-2 bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-lg text-sm focus-ring text-zinc-900 dark:text-zinc-200 placeholder:text-zinc-400 dark:placeholder:text-zinc-600"
          />
        </div>
        <button @click="load" :disabled="loading" title="重新加载 + 重拉学校状态"
          class="text-xs text-zinc-500 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-zinc-200 px-2 py-2 rounded-md hover:bg-black/5 dark:hover:bg-white/5 transition-colors inline-flex items-center gap-1">
          <RefreshCw class="w-3.5 h-3.5" :class="loading ? 'wangui-spin' : ''" />
        </button>
      </div>
    </header>

    <!-- Loading / empty -->
    <div
      v-if="loading && users.length === 0"
      class="py-20 flex items-center justify-center"
    >
      <div class="h-6 w-6 rounded-full border-2 border-zinc-800 border-t-emerald-400 wangui-spin" />
    </div>

    <div
      v-else-if="users.length === 0"
      class="rounded-xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] py-16 text-center text-sm text-zinc-500"
    >
      还没有用户
    </div>

    <!-- Card grid -->
    <section
      v-else
      class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3"
    >
      <article
        v-for="u in users"
        :key="u.userId"
        class="flex flex-col rounded-2xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] overflow-hidden"
        :class="u.isDisabled ? 'opacity-60' : ''"
      >
        <!-- Header -->
        <header class="p-4 flex items-start gap-3 border-b border-black/[0.05] dark:border-white/[0.04]">
          <Avatar
            :src="u.userAvatarUrl"
            :name="u.userName"
            :size="44"
            rounded="lg"
          />
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-1.5 flex-wrap">
              <h3 class="text-base font-semibold text-zinc-900 dark:text-zinc-100 truncate">
                {{ u.userName }}
              </h3>
              <span
                v-if="u.isDisabled"
                class="shrink-0 inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-red-500/15 text-red-700 dark:text-red-300 ring-1 ring-red-500/30"
              >
                已禁用
              </span>
            </div>
            <p class="text-[11px] text-zinc-500 font-mono-token truncate mt-0.5">
              {{ u.userNumber }}
            </p>
            <p
              v-if="u.userSection || u.userClass"
              class="text-[11px] text-zinc-500 truncate"
            >
              {{ u.userSection }}{{ u.userSection && u.userClass ? ' · ' : '' }}{{ u.userClass }}
            </p>
            <p
              v-if="u.inviteCode"
              class="text-[10px] text-zinc-400 dark:text-zinc-500 font-mono-token truncate mt-0.5 inline-flex items-center gap-1"
            >
              <Ticket class="w-2.5 h-2.5" />
              {{ u.inviteCode }}
            </p>
          </div>
          <div class="flex flex-col gap-1 shrink-0">
            <button
              @click="toggleDisabled(u)"
              :disabled="togglingDisabled[u.userId]"
              :title="u.isDisabled ? '启用' : '禁用'"
              class="p-1.5 rounded text-zinc-500 dark:text-zinc-400 hover:bg-amber-500/10 hover:text-amber-400 transition-colors disabled:opacity-50"
            >
              <Power class="w-4 h-4" />
            </button>
            <button
              @click="remove(u)"
              title="删除"
              class="p-1.5 rounded text-zinc-500 dark:text-zinc-400 hover:bg-red-500/10 hover:text-red-400 transition-colors"
            >
              <Trash2 class="w-4 h-4" />
            </button>
          </div>
        </header>

        <!-- Today's school status (top, prominent) -->
        <div class="px-4 pt-3">
          <div class="flex items-center justify-between gap-2 text-xs">
            <span
              class="inline-flex items-center gap-1.5 px-2 py-1 rounded-md text-[11px] font-medium"
              :class="statusChipClass(statusChip(statusByUser[u.userId]).tone)"
              :title="statusChip(statusByUser[u.userId]).hint || ''"
            >
              <span
                v-if="statusChip(statusByUser[u.userId]).tone === 'loading'"
                class="h-2.5 w-2.5 rounded-full border-2 border-zinc-400 border-t-transparent wangui-spin"
              />
              <span v-else class="w-1.5 h-1.5 rounded-full bg-current opacity-70" />
              {{ statusChip(statusByUser[u.userId]).label }}
            </span>
            <button
              @click="refreshStatus(u)"
              class="text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-200 p-1 rounded hover:bg-black/5 dark:hover:bg-white/5 transition-colors"
              title="重拉学校状态"
            >
              <RefreshCw class="w-3 h-3" />
            </button>
          </div>
          <p
            v-if="statusByUser[u.userId] && statusByUser[u.userId] !== 'loading' && (statusByUser[u.userId] as SchoolCheckinStatus).message"
            class="text-[10px] text-zinc-500 dark:text-zinc-400 mt-1 truncate"
          >
            {{ (statusByUser[u.userId] as SchoolCheckinStatus).message }}
          </p>
        </div>

        <!-- Body fields -->
        <div class="p-4 space-y-2.5 text-xs flex-1">
          <!-- Dorm picker -->
          <div class="flex items-center gap-2">
            <Building2 class="w-3.5 h-3.5 text-zinc-500 shrink-0" />
            <span class="text-zinc-500 shrink-0 w-14">宿舍楼</span>
            <select
              :value="u.dormId ?? 0"
              :disabled="bindingDorm[u.userId]"
              @change="(e) => changeDorm(u, +((e.target as HTMLSelectElement).value))"
              class="flex-1 min-w-0 bg-white dark:bg-zinc-950 ring-1 ring-black/[0.08] dark:ring-white/[0.06] rounded-md px-2 py-1 text-xs focus-ring text-zinc-900 dark:text-zinc-200 disabled:opacity-50"
            >
              <option :value="0">— 未绑定 —</option>
              <option v-for="d in dorms" :key="d.id" :value="d.id">{{ d.name }}</option>
            </select>
          </div>

          <!-- Auto sign toggle -->
          <div class="flex items-center gap-2">
            <PlayCircle class="w-3.5 h-3.5 text-zinc-500 shrink-0" />
            <span class="text-zinc-500 shrink-0 w-14">自动签到</span>
            <button
              type="button"
              :disabled="togglingAuto[u.userId]"
              @click="toggleAuto(u)"
              :class="u.autoSign ? 'bg-emerald-500' : 'bg-zinc-300 dark:bg-zinc-700'"
              class="relative inline-flex h-5 w-9 items-center rounded-full transition-colors disabled:opacity-50"
            >
              <span
                :class="u.autoSign ? 'translate-x-4' : 'translate-x-0.5'"
                class="inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform"
              />
            </button>
            <span class="text-[11px]" :class="u.autoSign ? 'text-emerald-500 dark:text-emerald-300' : 'text-zinc-500'">
              {{ u.autoSign ? '已开启' : '已关闭' }}
            </span>
          </div>

          <!-- SignDays week picker -->
          <div class="flex items-start gap-2">
            <CalendarDays class="w-3.5 h-3.5 text-zinc-500 shrink-0 mt-0.5" />
            <span class="text-zinc-500 shrink-0 w-14 mt-0.5">签到周</span>
            <div class="flex-1 flex flex-wrap items-center gap-1">
              <button
                v-for="(label, i) in DAY_LABELS"
                :key="i"
                type="button"
                :disabled="savingDays[u.userId]"
                @click="toggleDay(u, i)"
                :title="(u.signDays & (1 << i)) ? '点击关闭周' + label : '点击开启周' + label"
                :class="(u.signDays & (1 << i))
                  ? (todayBit === i
                    ? 'bg-emerald-500 text-zinc-950 ring-2 ring-emerald-400/40'
                    : 'bg-emerald-500/20 text-emerald-700 dark:text-emerald-300 ring-1 ring-emerald-500/30')
                  : (todayBit === i
                    ? 'bg-amber-500/10 text-amber-700 dark:text-amber-300 ring-2 ring-amber-500/40'
                    : 'bg-zinc-200 dark:bg-zinc-800/70 text-zinc-500 dark:text-zinc-500 ring-1 ring-black/[0.06] dark:ring-white/[0.05]')"
                class="w-6 h-6 rounded text-[11px] font-medium transition-all disabled:opacity-50"
              >
                {{ label }}
              </button>
              <span
                v-if="!signsToday(u)"
                class="ml-1 text-[10px] text-amber-500 dark:text-amber-400 font-medium"
              >
                · 今天不签
              </span>
            </div>
          </div>

          <!-- Sign time -->
          <div class="flex items-center gap-2">
            <Clock class="w-3.5 h-3.5 text-zinc-500 shrink-0" />
            <span class="text-zinc-500 shrink-0 w-14">签到时刻</span>
            <span
              class="font-semibold tabular-nums"
              :class="signsToday(u) ? 'text-emerald-500 dark:text-emerald-300' : 'text-zinc-500'"
            >
              {{ signTimeStr(u) }}
            </span>
            <span class="text-[10px] text-zinc-500">±{{ u.jitterSec }}s</span>
          </div>

          <!-- Token row -->
          <div class="flex items-center gap-2">
            <KeyRound class="w-3.5 h-3.5 text-zinc-500 shrink-0" />
            <span class="text-zinc-500 shrink-0 w-14">Token</span>
            <span
              class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium shrink-0"
              :class="tokenUrgency(u) === 'expired'
                ? 'bg-red-500/10 text-red-700 dark:text-red-300 ring-1 ring-red-500/25'
                : tokenUrgency(u) === 'soon'
                  ? 'bg-amber-500/10 text-amber-700 dark:text-amber-300 ring-1 ring-amber-500/30'
                  : 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 ring-1 ring-emerald-500/25'"
            >
              <span
                class="w-1 h-1 rounded-full"
                :class="tokenUrgency(u) === 'expired' ? 'bg-red-400' : tokenUrgency(u) === 'soon' ? 'bg-amber-400' : 'bg-emerald-400'"
              />
              {{ tokenUrgency(u) === 'expired' ? '已失效' : tokenUrgency(u) === 'soon' ? '快过期' : '有效' }}
            </span>
            <span class="text-[10px] text-zinc-500 truncate flex-1" :title="formatDateTime(u.tokenExp)">
              {{ formatDateTime(u.tokenExp) }}
            </span>
            <button
              @click="openRefresh(u)"
              :class="tokenUrgency(u) === 'expired'
                ? 'bg-red-500 hover:bg-red-400 text-white'
                : tokenUrgency(u) === 'soon'
                  ? 'bg-amber-500 hover:bg-amber-400 text-zinc-950'
                  : 'bg-white/80 dark:bg-zinc-900/80 text-zinc-700 dark:text-zinc-300 ring-1 ring-black/[0.08] dark:ring-white/[0.06] hover:ring-emerald-500/40'"
              class="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-medium transition-colors shrink-0"
              title="让朋友重新扫码以刷新 Token"
            >
              <RefreshCw class="w-3 h-3" />
              刷新
            </button>
          </div>
        </div>

        <!-- Recent records -->
        <div
          v-if="u.recentRecords && u.recentRecords.length > 0"
          class="px-4 pb-3"
        >
          <div class="flex items-center gap-1.5 mb-1.5">
            <History class="w-3 h-3 text-zinc-500" />
            <span class="text-[10px] text-zinc-500 uppercase tracking-wide">最近记录</span>
          </div>
          <ul class="space-y-1">
            <li
              v-for="r in u.recentRecords.slice(0, 3)"
              :key="r.id"
              class="flex items-center gap-2 text-[11px]"
            >
              <span class="w-1.5 h-1.5 rounded-full shrink-0" :class="statusDotClass(r.status)" />
              <span class="shrink-0 text-zinc-700 dark:text-zinc-300 font-medium w-7">
                {{ statusLabel(r.status) }}
              </span>
              <span class="text-zinc-500 font-mono-token text-[10px] shrink-0">
                {{ formatDateTime(r.occurredAt) }}
              </span>
              <span
                v-if="r.message"
                class="text-zinc-500 truncate"
                :title="r.message"
              >
                · {{ r.message }}
              </span>
            </li>
          </ul>
        </div>

        <!-- Actions -->
        <footer class="px-4 py-3 border-t border-black/[0.05] dark:border-white/[0.04] flex gap-2">
          <button
            @click="signNow(u)"
            :disabled="signing[u.userId]"
            class="flex-1 inline-flex items-center justify-center gap-1.5 bg-emerald-500 hover:bg-emerald-400 disabled:opacity-50 text-zinc-950 text-xs font-medium px-3 py-1.5 rounded-md transition-colors"
            title="代签到（应急 / 测试）"
          >
            <PlayCircle class="w-3.5 h-3.5" />
            {{ signing[u.userId] ? '签到中…' : '立即签到' }}
          </button>
          <button
            @click="resetPin(u)"
            :disabled="resetting"
            class="inline-flex items-center justify-center gap-1 px-3 py-1.5 rounded-md text-xs text-zinc-700 dark:text-zinc-300 bg-white/80 dark:bg-zinc-900/80 ring-1 ring-black/[0.08] dark:ring-white/[0.06] hover:ring-emerald-500/40 transition-colors"
            title="重置 PIN（强制对方下次激活/登录）"
          >
            <KeyRound class="w-3.5 h-3.5" />
            PIN
          </button>
        </footer>
      </article>
    </section>

    <!-- New PIN modal (after admin reset) -->
    <Transition name="modal">
      <div v-if="newPinResult" class="fixed inset-0 z-[60] bg-white/85 dark:bg-zinc-950/85 backdrop-blur flex items-center justify-center p-4"
        @click.self="newPinResult = null">
        <div class="w-full max-w-sm bg-zinc-100 dark:bg-zinc-900 ring-1 ring-emerald-500/30 rounded-2xl shadow-2xl">
          <div class="p-5 border-b border-black/[0.08] dark:border-white/[0.06] flex items-center gap-3">
            <div class="w-10 h-10 rounded-xl bg-emerald-500/15 ring-1 ring-emerald-500/30 flex items-center justify-center shrink-0">
              <KeyRound class="w-5 h-5 text-emerald-400" />
            </div>
            <div>
              <h2 class="text-base font-bold">PIN 已重置</h2>
              <p class="text-xs text-zinc-500 mt-0.5 truncate">{{ newPinResult.user }}</p>
            </div>
          </div>

          <div class="p-5">
            <p class="text-xs text-zinc-500 dark:text-zinc-400 mb-3">把这串 PIN 告诉用户（旧 PIN 已作废，所有会话已登出）：</p>
            <div class="bg-white dark:bg-zinc-950 ring-1 ring-emerald-500/30 rounded-xl px-4 py-5 text-center">
              <p class="text-4xl font-bold font-mono-token tabular-nums tracking-[0.5em] text-emerald-400 pl-[0.5em]">
                {{ newPinResult.pin }}
              </p>
            </div>
            <p class="text-[10px] text-zinc-500 dark:text-zinc-600 text-center mt-2">关闭后此 PIN 不再显示</p>
          </div>

          <div class="px-5 pb-5 flex gap-2">
            <button @click="copyPin"
              class="flex-1 inline-flex items-center justify-center gap-1.5 bg-emerald-500 hover:bg-emerald-400 text-zinc-950 text-sm font-medium py-2 rounded-lg transition-colors">
              <Copy class="w-3.5 h-3.5" />
              复制 PIN
            </button>
            <button @click="newPinResult = null"
              class="px-4 py-2 text-sm text-zinc-500 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-zinc-200 transition-colors">
              我已记下
            </button>
          </div>
        </div>
      </div>
    </Transition>

    <!-- Refresh-token modal -->
    <Transition name="modal">
      <div v-if="refreshTarget" class="fixed inset-0 z-50 bg-white/80 dark:bg-zinc-950/80 backdrop-blur flex items-center justify-center p-4 overflow-y-auto"
        @click.self="refreshTarget = null">
        <div class="w-full max-w-2xl bg-zinc-100 dark:bg-zinc-900 ring-1 ring-black/10 dark:ring-white/10 rounded-2xl shadow-2xl my-8">
          <div class="p-5 border-b border-black/[0.08] dark:border-white/[0.06] flex items-center justify-between">
            <h2 class="text-base font-bold flex items-center gap-2">
              <KeyRound class="w-4 h-4 text-amber-400" />
              刷新 Token — {{ refreshTarget.userName }}
            </h2>
            <button @click="refreshTarget = null"
              class="text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-200 transition-colors">
              <X class="w-4 h-4" />
            </button>
          </div>

          <div class="p-5 space-y-4">
            <div class="rounded-lg bg-amber-500/10 ring-1 ring-amber-500/30 px-3 py-2 text-xs text-amber-700 dark:text-amber-200">
              学校 JWT 通常 ~7 天过期。让朋友<strong>重新扫这个二维码</strong>，把回调链接发回给你粘到下面。
              新 Token 的学号必须是 <span class="font-mono-token">{{ refreshTarget.userNumber }}</span>，否则会被拒绝。
            </div>

            <div class="text-xs text-zinc-500 flex items-center gap-3">
              <Avatar :src="refreshTarget.userAvatarUrl" :name="refreshTarget.userName" :size="32" rounded="lg" />
              <div class="min-w-0 flex-1">
                <p class="text-sm text-zinc-900 dark:text-zinc-200 truncate">{{ refreshTarget.userName }}</p>
                <p class="text-[11px] font-mono-token truncate">{{ refreshTarget.userNumber }}</p>
              </div>
              <div class="text-right shrink-0">
                <p class="text-[10px] uppercase tracking-wide">当前到期</p>
                <p class="text-[11px] font-mono-token">{{ formatDateTime(refreshTarget.tokenExp) }}</p>
              </div>
            </div>

            <div class="rounded-xl bg-white/70 dark:bg-zinc-950/70 ring-1 ring-black/[0.05] dark:ring-white/[0.04] p-3">
              <div class="flex items-center gap-2 mb-2">
                <QrCodeIcon class="w-3.5 h-3.5 text-zinc-500" />
                <span class="text-xs text-zinc-500">截图发给朋友</span>
              </div>
              <div class="flex flex-col sm:flex-row gap-3">
                <div class="shrink-0 self-center sm:self-start">
                  <div class="w-36 h-36 rounded-xl bg-white ring-1 ring-black/[0.06] p-2 flex items-center justify-center overflow-hidden">
                    <img v-if="rQrDataUrl" :src="rQrDataUrl" alt="二维码" class="w-full h-full object-contain" />
                    <div v-else class="text-[11px] text-zinc-400 text-center">{{ rQrBuilding ? '生成中…' : '失败' }}</div>
                  </div>
                </div>
                <div class="min-w-0 flex-1">
                  <ol class="text-[11px] text-zinc-600 dark:text-zinc-400 space-y-1 list-decimal list-inside leading-relaxed">
                    <li>截图发给朋友（同一个朋友，<strong>不要换人</strong>）</li>
                    <li>朋友微信扫码 → 学校晚归页面登录</li>
                    <li>登录后右上「⋯」→「复制链接」→ 发回</li>
                    <li>粘到下面 ↓</li>
                  </ol>
                  <button type="button" @click="rebuildRefreshQr"
                    class="mt-2 inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-[11px] text-zinc-700 dark:text-zinc-300 bg-white/80 dark:bg-zinc-900/80 ring-1 ring-black/[0.06] dark:ring-white/[0.05] hover:ring-emerald-500/40 transition-colors">
                    <RefreshCw class="w-3 h-3" />
                    刷新二维码
                  </button>
                </div>
              </div>
            </div>

            <div>
              <label class="block text-xs text-zinc-500 mb-1.5">把朋友给的回调链接粘到这里 *</label>
              <textarea v-model="rCallback"
                placeholder="https://xhbcs.henau.edu.cn/?code=...&state=..."
                class="w-full bg-white dark:bg-zinc-950 ring-1 ring-emerald-500/30 focus:!ring-emerald-500/60 rounded-lg px-3 py-2 h-20 resize-none text-sm font-mono-token text-zinc-900 dark:text-zinc-200 placeholder:text-zinc-400 dark:placeholder:text-zinc-600 focus-ring" />
              <div v-if="rCallbackDetect.kind === 'callback-url'"
                class="mt-1.5 inline-flex items-center gap-1.5 px-2 py-1 rounded text-[11px] text-emerald-400 bg-emerald-500/10">
                <Check class="w-3 h-3" />
                已识别回调链接
              </div>
              <div v-else-if="rCallbackDetect.kind === 'code'"
                class="mt-1.5 inline-flex items-center gap-1.5 px-2 py-1 rounded text-[11px] text-emerald-400 bg-emerald-500/10">
                <Check class="w-3 h-3" />
                已识别 code
              </div>
              <div v-else-if="rCallbackDetect.kind === 'invalid' && rCallback.trim()"
                class="mt-1.5 inline-flex items-center gap-1.5 px-2 py-1 rounded text-[11px] text-amber-400 bg-amber-500/10">
                <AlertCircle class="w-3 h-3" />
                没识别到 code，请检查链接
              </div>
            </div>
          </div>

          <div class="px-5 pb-5 flex justify-end gap-2">
            <button @click="refreshTarget = null"
              class="px-4 py-2 text-sm text-zinc-500 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-zinc-200 transition-colors">
              取消
            </button>
            <button @click="submitRefresh" :disabled="!rCallbackOk || refreshing"
              class="bg-emerald-500 hover:bg-emerald-400 disabled:opacity-50 text-zinc-950 text-sm font-medium px-5 py-2 rounded-lg transition-colors">
              {{ refreshing ? '刷新中…' : '刷新 Token' }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.modal-enter-active, .modal-leave-active { transition: opacity 0.2s; }
.modal-enter-from, .modal-leave-to { opacity: 0; }
</style>
