<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import {
  CheckCircle2,
  XCircle,
  AlertTriangle,
  Clock,
  Zap,
  ScrollText,
  ArrowRight,
  KeyRound,
  MapPin,
} from 'lucide-vue-next'
import type { SignRecord } from '../types'
import { useAuth } from '../stores/auth'
import { api } from '../api'
import { formatDateTime, formatRemaining } from '../lib/format'
import { showToast } from '../lib/toast'
import Avatar from '../components/Avatar.vue'

const auth = useAuth()
const records = ref<SignRecord[]>([])
const now = ref(new Date())
const signing = ref(false)

let timer: number | null = null
onMounted(async () => {
  await auth.init()
  await loadRecords()
  timer = window.setInterval(() => (now.value = new Date()), 30_000)
})
onUnmounted(() => {
  if (timer) clearInterval(timer)
})

async function loadRecords() {
  try {
    records.value = await api.records()
  } catch {
    records.value = []
  }
}

const me = computed(() => auth.state.me)

function greeting(): string {
  const h = new Date().getHours()
  if (h < 6) return '夜深了'
  if (h < 12) return '早上好'
  if (h < 14) return '中午好'
  if (h < 18) return '下午好'
  if (h < 22) return '晚上好'
  return '夜深了'
}

const tokenValid = computed(
  () => !!me.value?.token.isValid && me.value!.token.remainingSec > 0,
)
const tokenColor = computed(() => {
  if (!me.value) return 'text-zinc-500 dark:text-zinc-400'
  const s = me.value.token.remainingSec
  if (s <= 0) return 'text-red-400'
  if (s < 24 * 3600) return 'text-red-400'
  if (s < 3 * 24 * 3600) return 'text-amber-400'
  return 'text-emerald-400'
})

// Today already signed?
const signedToday = computed(() => {
  const today = new Date()
  const y = today.getFullYear()
  const m = today.getMonth()
  const d = today.getDate()
  return records.value.some(r => {
    if (r.status !== 'success' && r.status !== 'already') return false
    const t = new Date(r.occurredAt * 1000)
    return t.getFullYear() === y && t.getMonth() === m && t.getDate() === d
  })
})

// Is today a sign-in day per the user's sign_days bitmask?
// JS getDay(): 0=Sun..6=Sat. We map to bit 0=Mon..bit 6=Sun.
const isSignDayToday = computed(() => {
  const sd = me.value?.settings.signDays
  if (typeof sd !== 'number') return true // legacy users: default every day
  if ((sd & 0x7f) === 0) return false
  const bit = (new Date().getDay() + 6) % 7
  return (sd & (1 << bit)) !== 0
})

// Time to next sign window (22:00).
const nextWindow = computed(() => {
  const n = now.value
  const target = new Date(n)
  target.setHours(22, 0, 0, 0)
  if (target <= n) target.setDate(target.getDate() + 1)
  const ms = target.getTime() - n.getTime()
  const totalMin = Math.floor(ms / 60000)
  const h = Math.floor(totalMin / 60)
  const m = totalMin % 60
  return { h, m, totalMin }
})

const inWindow = computed(() => {
  const n = now.value
  const h = n.getHours()
  const m = n.getMinutes()
  return h === 22 && m < 30
})

async function signNow() {
  if (signing.value || !me.value) return
  signing.value = true
  try {
    const res = await api.signNow()
    if (res.status === 'success') showToast('ok', '签到成功 🎉')
    else if (res.status === 'already') showToast('ok', '今日已签到')
    else if (res.status === 'exempt') showToast('ok', res.message || '免签')
    else showToast('err', res.message || '签到失败')
    await loadRecords()
    await auth.refresh()
  } catch (e: any) {
    showToast('err', e.message || '签到失败')
  } finally {
    signing.value = false
  }
}

const recentRecords = computed(() => records.value.slice(0, 5))

const recordMeta: Record<string, { label: string; color: string; dotBg: string }> = {
  success: { label: '签到成功', color: 'text-emerald-400', dotBg: 'bg-emerald-500' },
  already: { label: '今日已签', color: 'text-blue-400', dotBg: 'bg-blue-500' },
  exempt: { label: '免签', color: 'text-zinc-500 dark:text-zinc-400', dotBg: 'bg-zinc-500' },
  failed: { label: '签到失败', color: 'text-red-400', dotBg: 'bg-red-500' },
  skipped: { label: '跳过', color: 'text-amber-400', dotBg: 'bg-amber-500' },
}
</script>

<template>
  <div v-if="me" class="space-y-3">
    <!-- Hero -->
    <section
      class="relative overflow-hidden rounded-2xl bg-gradient-to-br from-white to-zinc-100/40 dark:from-zinc-900 dark:to-zinc-900/40 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-5 ambient-glow"
    >
      <div class="relative flex items-center gap-4">
        <Avatar
          :src="me.userAvatarUrl"
          :name="me.userName"
          :size="56"
          rounded="xl"
        />
        <div class="min-w-0 flex-1">
          <p class="text-[11px] text-zinc-500 tracking-wide uppercase">{{ greeting() }}</p>
          <h2 class="text-xl font-bold tracking-tight mt-0.5 truncate">
            欢迎回来,{{ me.userName }}
          </h2>
          <p class="text-sm text-zinc-500 dark:text-zinc-400 mt-1 truncate">
            <span>{{ me.userSection || '—' }}</span>
            <span class="mx-1.5 text-zinc-400 dark:text-zinc-700">·</span>
            <span>{{ me.userClass || '—' }}</span>
            <span class="mx-1.5 text-zinc-400 dark:text-zinc-700">·</span>
            <span class="font-mono-token text-zinc-500">{{ me.userNumber }}</span>
          </p>
        </div>
        <!-- Dorm binding status -->
        <RouterLink
          to="/settings"
          class="hidden sm:flex items-center gap-2 px-3 py-2 rounded-lg ring-1 transition-colors shrink-0 text-xs"
          :class="me.settings.dormName
            ? 'bg-emerald-500/10 ring-emerald-500/25 text-emerald-300 hover:bg-emerald-500/15'
            : 'bg-amber-500/10 ring-amber-500/25 text-amber-300 hover:bg-amber-500/15'"
        >
          <MapPin class="w-3.5 h-3.5" />
          <div class="leading-tight">
            <div class="text-[10px] opacity-70 tracking-wide uppercase">
              {{ me.settings.dormName ? '已绑定位置' : '未绑定位置' }}
            </div>
            <div class="font-medium truncate max-w-[160px]">
              {{ me.settings.dormName || '前往配置 →' }}
            </div>
          </div>
        </RouterLink>
      </div>
      <!-- Mobile-only dorm status (shown below on small screens) -->
      <RouterLink
        to="/settings"
        class="sm:hidden mt-3 flex items-center gap-2 px-3 py-2 rounded-lg ring-1 text-xs"
        :class="me.settings.dormName
          ? 'bg-emerald-500/10 ring-emerald-500/25 text-emerald-300'
          : 'bg-amber-500/10 ring-amber-500/25 text-amber-300'"
      >
        <MapPin class="w-3.5 h-3.5 shrink-0" />
        <span class="font-medium truncate">
          {{ me.settings.dormName ? `已绑定: ${me.settings.dormName}` : '未绑定位置 — 点此配置' }}
        </span>
      </RouterLink>
    </section>

    <!-- KPI row -->
    <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
      <!-- Today status -->
      <div class="rounded-xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-4">
        <div class="flex items-center gap-1.5 mb-2">
          <CheckCircle2 class="w-3.5 h-3.5 text-zinc-500" />
          <span class="text-[11px] text-zinc-500 tracking-wide uppercase">今日签到</span>
        </div>
        <template v-if="!isSignDayToday">
          <div class="flex items-center gap-2">
            <span class="w-2 h-2 rounded-full bg-zinc-500" />
            <span class="text-zinc-500 dark:text-zinc-400 font-semibold text-base">今日休息</span>
          </div>
          <p class="text-xs text-zinc-500 mt-1.5">不在你设置的签到日期内</p>
        </template>
        <template v-else-if="signedToday">
          <div class="flex items-center gap-2">
            <span class="w-2 h-2 rounded-full bg-emerald-500 shadow-[0_0_8px_rgb(16_185_129_/_0.8)]" />
            <span class="text-emerald-400 font-semibold text-base">已完成</span>
          </div>
          <p class="text-xs text-zinc-500 mt-1.5">今天的窗口已完成 ✓</p>
        </template>
        <template v-else>
          <div class="flex items-center gap-2">
            <span class="w-2 h-2 rounded-full bg-zinc-600" />
            <span class="text-zinc-500 dark:text-zinc-400 font-semibold text-base">未签到</span>
          </div>
          <p class="text-xs text-zinc-500 mt-1.5">今天 22:00 开放窗口</p>
        </template>
      </div>

      <!-- Next window -->
      <div class="rounded-xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-4">
        <div class="flex items-center gap-1.5 mb-2">
          <Clock class="w-3.5 h-3.5 text-zinc-500" />
          <span class="text-[11px] text-zinc-500 tracking-wide uppercase">
            {{ inWindow ? '签到窗口' : '下次窗口' }}
          </span>
        </div>
        <div class="flex items-baseline gap-1.5">
          <span v-if="inWindow" class="text-emerald-400 font-bold text-lg">进行中</span>
          <template v-else>
            <span class="text-2xl font-bold tabular-nums leading-none">{{ nextWindow.h }}</span>
            <span class="text-xs text-zinc-500">小时</span>
            <span class="text-2xl font-bold tabular-nums leading-none">{{ nextWindow.m }}</span>
            <span class="text-xs text-zinc-500">分</span>
          </template>
        </div>
        <p class="text-xs text-zinc-500 mt-1.5">每天 22:00–22:30</p>
      </div>

      <!-- Token -->
      <RouterLink
        to="/account"
        class="rounded-xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-4 hover:ring-black/[0.12] dark:hover:ring-white/[0.12] transition-all block"
      >
        <div class="flex items-center gap-1.5 mb-2">
          <KeyRound class="w-3.5 h-3.5 text-zinc-500" />
          <span class="text-[11px] text-zinc-500 tracking-wide uppercase">Token</span>
        </div>
        <div class="flex items-center gap-2">
          <CheckCircle2 v-if="tokenValid" class="w-4 h-4" :class="tokenColor" />
          <XCircle v-else class="w-4 h-4 text-red-400" />
          <span class="font-semibold text-base" :class="tokenColor">
            {{ tokenValid ? '有效' : '已失效' }}
          </span>
        </div>
        <p class="text-xs text-zinc-500 mt-1.5">
          {{ formatRemaining(me.token.remainingSec) }}
        </p>
      </RouterLink>
    </div>

    <!-- Action row -->
    <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
      <button
        @click="signNow"
        :disabled="signing"
        class="group rounded-xl bg-emerald-500/10 hover:bg-emerald-500/15 ring-1 ring-emerald-500/30 hover:ring-emerald-500/50 p-4 text-left transition-all disabled:opacity-50"
      >
        <div class="flex items-start justify-between mb-2">
          <Zap class="w-5 h-5 text-emerald-400" :class="signing ? 'wangui-spin' : ''" />
          <ArrowRight class="w-4 h-4 text-emerald-400/60 group-hover:translate-x-0.5 transition-transform" />
        </div>
        <p class="font-semibold text-emerald-200 text-base">立即签到</p>
        <p class="text-xs text-emerald-400/70 mt-0.5">在窗口期内手动触发一次</p>
      </button>

      <RouterLink
        to="/settings"
        class="group rounded-xl bg-white/85 dark:bg-zinc-900/60 hover:bg-zinc-100 dark:hover:bg-zinc-900 ring-1 ring-black/[0.08] dark:ring-white/[0.06] hover:ring-black/[0.12] dark:hover:ring-white/[0.12] p-4 text-left transition-all block"
      >
        <div class="flex items-start justify-between mb-2">
          <MapPin class="w-5 h-5 text-zinc-500 dark:text-zinc-400" />
          <ArrowRight class="w-4 h-4 text-zinc-500 group-hover:translate-x-0.5 transition-transform" />
        </div>
        <p class="font-semibold text-base">打卡位置 / 配置</p>
        <p class="text-xs text-zinc-500 mt-0.5">
          {{ me.settings.latitude !== 0 ? '已配置' : '⚠ 未配置坐标' }}
        </p>
      </RouterLink>
    </div>

    <!-- Recent records preview -->
    <section class="rounded-xl bg-white/85 dark:bg-zinc-900/60 ring-1 ring-black/[0.08] dark:ring-white/[0.06] p-5">
      <div class="flex items-center justify-between mb-3">
        <div class="flex items-center gap-2">
          <ScrollText class="w-4 h-4 text-zinc-500" />
          <h3 class="text-base font-semibold text-zinc-900 dark:text-zinc-200">最近签到</h3>
        </div>
        <RouterLink to="/records" class="text-xs text-zinc-500 hover:text-emerald-400 transition-colors flex items-center gap-1">
          查看全部
          <ArrowRight class="w-3 h-3" />
        </RouterLink>
      </div>

      <div v-if="recentRecords.length === 0" class="flex flex-col items-center py-8 text-center">
        <Clock class="w-10 h-10 text-zinc-400 dark:text-zinc-700 mb-2" />
        <p class="text-sm text-zinc-500">还没有签到记录</p>
      </div>

      <ol v-else class="relative">
        <div class="absolute left-[7px] top-1.5 bottom-1.5 w-px bg-gradient-to-b from-transparent via-black/[0.06] dark:via-white/[0.06] to-transparent" />
        <li v-for="r in recentRecords" :key="r.id" class="relative pl-7 pb-3 last:pb-0">
          <span
            class="absolute left-0 top-1 w-3.5 h-3.5 rounded-full ring-4 ring-white dark:ring-zinc-900"
            :class="(recordMeta[r.status] || recordMeta.failed).dotBg"
          />
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0 flex-1">
              <span class="text-sm font-medium" :class="(recordMeta[r.status] || recordMeta.failed).color">
                {{ (recordMeta[r.status] || recordMeta.failed).label }}
              </span>
              <p class="text-xs text-zinc-500 mt-0.5 leading-relaxed break-all">{{ r.message || '—' }}</p>
            </div>
            <span class="shrink-0 text-[10px] text-zinc-500 dark:text-zinc-600 tabular-nums">
              {{ formatDateTime(r.occurredAt) }}
            </span>
          </div>
        </li>
      </ol>
    </section>

    <!-- Token expiry warning -->
    <div
      v-if="me.token.remainingSec < 24 * 3600 && me.token.remainingSec > 0"
      class="rounded-xl bg-amber-500/10 ring-1 ring-amber-500/30 p-4 flex items-start gap-3"
    >
      <AlertTriangle class="w-5 h-5 text-amber-400 shrink-0 mt-0.5" />
      <div class="flex-1">
        <p class="text-sm font-medium text-amber-300">Token 即将过期</p>
        <p class="text-xs text-amber-400/80 mt-1">
          剩余 {{ formatRemaining(me.token.remainingSec) }}，请尽快在「账号」页更新。
        </p>
      </div>
      <RouterLink to="/account" class="shrink-0 text-xs px-3 py-1.5 rounded-md bg-amber-500/20 hover:bg-amber-500/30 text-amber-300 transition-colors">
        前往更新
      </RouterLink>
    </div>
    <div
      v-else-if="me.token.remainingSec <= 0"
      class="rounded-xl bg-red-500/10 ring-1 ring-red-500/30 p-4 flex items-start gap-3"
    >
      <XCircle class="w-5 h-5 text-red-400 shrink-0 mt-0.5" />
      <div class="flex-1">
        <p class="text-sm font-medium text-red-300">Token 已过期</p>
        <p class="text-xs text-red-400/80 mt-1">必须重新扫码并更新 Token，才能继续自动签到。</p>
      </div>
      <RouterLink to="/account" class="shrink-0 text-xs px-3 py-1.5 rounded-md bg-red-500/20 hover:bg-red-500/30 text-red-300 transition-colors">
        前往更新
      </RouterLink>
    </div>
  </div>
</template>
